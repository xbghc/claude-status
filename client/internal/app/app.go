//go:build windows

package app

import (
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"claude-status/internal/config"
	"claude-status/internal/logger"
	"claude-status/internal/ssh"
	"claude-status/internal/tray"
)

// Run 运行应用
func Run(configPath string) {
	// 初始化日志
	if err := logger.Init(); err != nil {
		// 日志初始化失败，静默继续
	}
	defer logger.Close()

	logger.Info("Starting application...")

	// 创建托盘应用
	trayApp := tray.NewApp()

	// 处理系统信号
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// 启动托盘应用
	trayApp.Run(func() {
		// 托盘就绪后启动主逻辑
		go appMain(trayApp, configPath, sigCh)
	}, nil)
}

// appMain 应用主逻辑
func appMain(trayApp *tray.App, configPath string, sigCh chan os.Signal) {
	logger.Info("appMain started")

	// 从 SSH config 加载主机列表
	servers, err := config.LoadSSHHosts()
	if err != nil {
		logger.Error("Failed to load SSH hosts: %v", err)
	}
	logger.Info("Loaded %d SSH hosts", len(servers))
	if len(servers) > 0 {
		trayApp.SetServers(servers)
	}

	// 检查配置文件是否存在
	logger.Info("Checking config file: %s", configPath)
	if !config.Exists(configPath) {
		logger.Info("Config file does not exist")
		// 配置不存在，等待用户选择服务器
		if len(servers) > 0 {
			trayApp.ShowServerSelection()
			waitForServerSelection(trayApp, configPath, sigCh)
		} else {
			trayApp.SetError("no_config", "配置文件不存在且无预设服务器")
			// 等待用户退出
			select {
			case <-trayApp.QuitChan():
				return
			case <-sigCh:
				return
			}
		}
		return
	}

	// 加载配置
	logger.Info("Loading config from: %s", configPath)
	cfg, err := config.Load(configPath)
	if err != nil {
		logger.Error("Failed to load config: %v", err)
		if len(servers) > 0 {
			trayApp.ShowServerSelection()
			waitForServerSelection(trayApp, configPath, sigCh)
		} else {
			trayApp.SetError("no_config", "配置文件无效")
			select {
			case <-trayApp.QuitChan():
				return
			case <-sigCh:
				return
			}
		}
		return
	}

	// 设置调试模式
	if cfg.Debug {
		logger.SetDebug(true)
	}

	// 设置状态超时
	trayApp.SetStatusTimeout(cfg.StatusTimeout)

	// 启动连接管理
	connectionManager(cfg, trayApp, configPath, sigCh)
}

// waitForServerSelection 等待用户选择服务器
func waitForServerSelection(trayApp *tray.App, configPath string, sigCh chan os.Signal) {
	for {
		select {
		case server := <-trayApp.ServerSelectChan():
			// 用户选择了服务器，保存配置
			cfg := config.NewFromServer(server)

			// 应用 SSH config
			cfg.ApplySSHConfig()

			// 设置默认超时
			if cfg.StatusTimeout == 0 {
				cfg.StatusTimeout = 300
			}
			trayApp.SetStatusTimeout(cfg.StatusTimeout)

			// 保存到配置文件
			if err := config.Save(configPath, cfg); err != nil {
				trayApp.SetError("session_error", "保存配置失败")
				continue
			}

			// 启动连接
			connectionManager(cfg, trayApp, configPath, sigCh)
			return

		case <-trayApp.QuitChan():
			return

		case <-sigCh:
			return
		}
	}
}

// connectionManager 管理 SSH 连接，支持重连
func connectionManager(cfg *config.Config, trayApp *tray.App, configPath string, sigCh chan os.Signal) {
	for {
		// 尝试连接
		shouldContinue, newServer := runConnection(cfg, trayApp, sigCh)
		if !shouldContinue {
			return
		}

		// 如果连接过程中选择了新服务器，直接使用
		if newServer != nil {
			cfg = config.NewFromServer(*newServer)
			cfg.ApplySSHConfig()
			trayApp.SetStatusTimeout(cfg.StatusTimeout)
			config.Save(configPath, cfg)
			continue
		}

		// 等待重连信号、服务器选择或退出
		select {
		case <-trayApp.ReconnectChan():
			// 用户请求重连，继续循环使用当前配置
			continue

		case server := <-trayApp.ServerSelectChan():
			// 用户选择了新服务器
			cfg = config.NewFromServer(server)
			cfg.ApplySSHConfig()
			trayApp.SetStatusTimeout(cfg.StatusTimeout)
			config.Save(configPath, cfg)
			continue

		case <-trayApp.QuitChan():
			return

		case <-sigCh:
			return
		}
	}
}

// runConnection 运行一次连接，返回 (是否继续, 新选择的服务器)
func runConnection(cfg *config.Config, trayApp *tray.App, sigCh chan os.Signal) (bool, *config.ServerConfig) {
	// 获取显示名称
	displayName := cfg.Server.Name
	if displayName == "" {
		displayName = cfg.Server.Host
	}

	logger.Info("runConnection: connecting to %s@%s:%d", cfg.Server.User, cfg.Server.Host, cfg.Server.Port)
	trayApp.SetConnecting(displayName)

	// 创建 SSH 客户端
	sshClient := ssh.NewClient(cfg)

	// 连接
	if err := sshClient.Connect(); err != nil {
		logger.Error("SSH connect failed: %v", err)
		trayApp.SetError("connection_failed", err.Error())
		return true, nil // 继续等待重连
	}
	defer sshClient.Close()
	logger.Info("SSH connected successfully")

	// 启动监听
	if err := sshClient.Start(); err != nil {
		logger.Error("SSH start failed: %v", err)
		errMsg := err.Error()
		if isNotConfiguredError(errMsg) {
			trayApp.SetError("not_configured", errMsg)
		} else {
			trayApp.SetError("session_error", errMsg)
		}
		return true, nil
	}
	logger.Info("SSH session started")
	trayApp.SetConnected(true, displayName)

	// 主循环
	for {
		select {
		case statuses := <-sshClient.StatusChan():
			trayApp.UpdateStatus(statuses)

		case err := <-sshClient.ErrorChan():
			errMsg := err.Error()
			if isNotConfiguredError(errMsg) {
				trayApp.SetError("not_configured", errMsg)
			} else {
				trayApp.SetError("session_error", errMsg)
			}
			return true, nil

		case <-sshClient.Done():
			trayApp.SetError("session_error", "连接已断开")
			return true, nil

		case <-trayApp.QuitChan():
			return false, nil

		case <-trayApp.ReconnectChan():
			// 收到重连请求，关闭当前连接并返回
			return true, nil

		case server := <-trayApp.ServerSelectChan():
			// 收到服务器切换请求，返回新服务器
			return true, &server

		case <-sigCh:
			return false, nil
		}
	}
}

// isNotConfiguredError 检测是否是服务端未配置的错误
func isNotConfiguredError(errMsg string) bool {
	notConfiguredPatterns := []string{
		"No such file or directory",
		"not found",
		"command not found",
		"monitor.sh",
		"Permission denied",
	}

	for _, pattern := range notConfiguredPatterns {
		if strings.Contains(errMsg, pattern) {
			return true
		}
	}
	return false
}

// GetExecutableDir 获取可执行文件所在目录
func GetExecutableDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}
