//go:build windows

package app

import (
	"errors"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"claude-status/internal/config"
	"claude-status/internal/installer"
	"claude-status/internal/logger"
	"claude-status/internal/monitor"
	"claude-status/internal/ssh"
	"claude-status/internal/tray"
	"claude-status/internal/wsl"
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
		shouldContinue, newServer, userDisconnected, autoReconnect := runConnection(cfg, trayApp, sigCh)
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

		// 如果需要自动重连（版本更新后），直接继续循环
		if autoReconnect {
			continue
		}

		// 如果是用户主动断开，设置断开状态
		if userDisconnected {
			trayApp.SetDisconnected()
		}

		// 等待用户选择服务器或退出
		select {
		case server := <-trayApp.ServerSelectChan():
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

// runConnection 运行一次连接，返回 (是否继续, 新选择的服务器, 是否用户主动断开, 是否自动重连)
func runConnection(cfg *config.Config, trayApp *tray.App, sigCh chan os.Signal) (bool, *config.ServerConfig, bool, bool) {
	// 获取显示名称
	displayName := getDisplayName(cfg)

	logger.Info("runConnection: mode=%s, display=%s", getMode(cfg), displayName)
	trayApp.SetConnecting(displayName)

	// 创建客户端（SSH 或 WSL）
	var client monitor.Client
	var inst monitor.Installer

	if cfg.WSL.Enabled {
		client = wsl.NewClient(cfg)
		inst = wsl.NewInstaller(cfg)
	} else {
		client = ssh.NewClient(cfg)
		inst = installer.NewInstaller(cfg)
	}

	// 连接
	if err := client.Connect(); err != nil {
		logger.Error("Connect failed: %v", err)
		trayApp.SetError("connection_failed", err.Error())
		return true, nil, false, false // 等待用户操作
	}
	defer client.Close()
	logger.Info("Connected successfully")

	// 启动监听
	if err := client.Start(); err != nil {
		logger.Error("Start failed: %v", err)
		errMsg := err.Error()

		// 检测是否是版本不匹配，触发重新安装
		if errors.Is(err, ssh.ErrVersionMismatch) || errors.Is(err, wsl.ErrVersionMismatch) {
			logger.Info("版本不匹配，触发重新安装...")
			client.Close()
			shouldContinue, newServer := triggerReinstall(cfg, inst, trayApp)
			return shouldContinue, newServer, false, true // 自动重连
		}

		// 检测是否是服务端未配置，尝试自动安装
		if isNotConfiguredError(errMsg) {
			logger.Info("服务端未配置，尝试自动安装...")
			shouldContinue, newServer := triggerInstall(cfg, inst, trayApp)
			return shouldContinue, newServer, false, true // 自动重连
		}

		trayApp.SetError("session_error", errMsg)
		return true, nil, false, false
	}
	logger.Info("Session started")
	trayApp.SetConnected(true, displayName)

	// 主循环
	for {
		select {
		case statuses := <-client.StatusChan():
			trayApp.UpdateStatus(statuses)

		case err := <-client.ErrorChan():
			errMsg := err.Error()
			if isNotConfiguredError(errMsg) {
				trayApp.SetError("not_configured", errMsg)
			} else {
				trayApp.SetError("session_error", errMsg)
			}
			return true, nil, false, false

		case <-client.Done():
			trayApp.SetError("session_error", "连接已断开")
			return true, nil, false, false

		case <-trayApp.QuitChan():
			return false, nil, false, false

		case <-trayApp.DisconnectChan():
			// 用户主动断开连接
			logger.Info("用户主动断开连接")
			return true, nil, true, false

		case server := <-trayApp.ServerSelectChan():
			// 收到服务器切换请求，返回新服务器
			return true, &server, false, false

		case <-sigCh:
			return false, nil, false, false
		}
	}
}

// getDisplayName 获取显示名称
func getDisplayName(cfg *config.Config) string {
	if cfg.WSL.Enabled {
		if cfg.WSL.Distro != "" {
			return "WSL: " + cfg.WSL.Distro
		}
		return "WSL"
	}
	if cfg.Server.Name != "" {
		return cfg.Server.Name
	}
	return cfg.Server.Host
}

// getMode 获取模式名称
func getMode(cfg *config.Config) string {
	if cfg.WSL.Enabled {
		return "WSL"
	}
	return "SSH"
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

// triggerInstall 触发首次安装
func triggerInstall(cfg *config.Config, inst monitor.Installer, trayApp *tray.App) (bool, *config.ServerConfig) {
	trayApp.SetConnecting("正在安装服务端...")

	if err := inst.Connect(); err != nil {
		logger.Error("安装器连接失败: %v", err)
		trayApp.SetError("install_failed", "安装失败: "+err.Error())
		return true, nil
	}
	defer inst.Close()

	// 检查依赖
	if ok, msg := inst.CheckDependencies(); !ok {
		logger.Error("依赖检查失败: %s", msg)
		trayApp.SetError("install_failed", msg)
		return true, nil
	}

	// 执行安装
	if err := inst.Install(); err != nil {
		logger.Error("安装失败: %v", err)
		trayApp.SetError("install_failed", "安装失败: "+err.Error())
		return true, nil
	}

	logger.Info("服务端安装完成，等待重新连接...")
	// 返回 true 让外层循环重新连接
	return true, nil
}

// triggerReinstall 触发重新安装（版本不匹配时）
func triggerReinstall(cfg *config.Config, inst monitor.Installer, trayApp *tray.App) (bool, *config.ServerConfig) {
	trayApp.SetConnecting("版本不匹配，正在更新服务端...")

	if err := inst.Connect(); err != nil {
		logger.Error("安装器连接失败: %v", err)
		trayApp.SetError("install_failed", "更新失败: "+err.Error())
		return true, nil
	}
	defer inst.Close()

	// 版本不匹配时跳过依赖检查（依赖应该已经安装）

	// 执行安装（会覆盖旧脚本和 Hook 配置）
	if err := inst.Install(); err != nil {
		logger.Error("更新失败: %v", err)
		trayApp.SetError("install_failed", "更新失败: "+err.Error())
		return true, nil
	}

	logger.Info("服务端更新完成，等待重新连接...")
	return true, nil
}
