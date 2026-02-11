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
	var cfg *config.Config
	var initialState State

	if !config.Exists(configPath) {
		logger.Info("Config file does not exist")
		if len(servers) > 0 {
			initialState = StateUnconfigured
		} else {
			trayApp.SetError("no_config", "配置文件不存在且无预设服务器")
			select {
			case <-trayApp.QuitChan():
				return
			case <-sigCh:
				return
			}
		}
	} else {
		// 加载配置
		logger.Info("Loading config from: %s", configPath)
		cfg, err = config.Load(configPath)
		if err != nil {
			logger.Error("Failed to load config: %v", err)
			if len(servers) > 0 {
				initialState = StateUnconfigured
			} else {
				trayApp.SetError("no_config", "配置文件无效")
				select {
				case <-trayApp.QuitChan():
					return
				case <-sigCh:
					return
				}
			}
		} else {
			// 设置调试模式
			if cfg.Debug {
				logger.SetDebug(true)
			}
			trayApp.SetStatusTimeout(cfg.StatusTimeout)
			initialState = StateConnecting
		}
	}

	// 启动状态机驱动的主循环
	eventLoop(initialState, cfg, trayApp, configPath, sigCh)
}

// eventLoop 状态机驱动的主循环
func eventLoop(initialState State, cfg *config.Config, trayApp *tray.App, configPath string, sigCh chan os.Signal) {
	sm := NewStateMachine(initialState, func(change StateChange) {
		logger.Info("State: %s -> %s (event: %s)", change.From, change.To, change.Event)
		applyTrayState(trayApp, change, cfg)
	})

	// 应用初始状态的 UI
	applyTrayState(trayApp, StateChange{To: initialState, Valid: true}, cfg)

	for sm.Current() != StateQuitting {
		switch sm.Current() {
		case StateUnconfigured:
			cfg = handleUnconfigured(sm, cfg, trayApp, configPath, sigCh)

		case StateConnecting:
			handleConnecting(sm, cfg, trayApp, configPath, sigCh)

		case StateInstalling:
			handleInstalling(sm, cfg, trayApp)

		case StateReinstalling:
			handleReinstalling(sm, cfg, trayApp)

		case StateDisconnected, StateError:
			cfg = handleWaitForUser(sm, cfg, trayApp, configPath, sigCh)
		}
	}
}

// handleUnconfigured 处理未配置状态，等待用户选择服务器
func handleUnconfigured(sm *StateMachine, cfg *config.Config, trayApp *tray.App, configPath string, sigCh chan os.Signal) *config.Config {
	select {
	case server := <-trayApp.ServerSelectChan():
		newCfg := applyServerConfig(server, trayApp, configPath)
		if newCfg != nil {
			cfg = newCfg
			sm.Transition(EventServerSelected)
		} else {
			trayApp.SetError("session_error", "保存配置失败")
		}
	case <-trayApp.QuitChan():
		sm.Transition(EventUserQuit)
	case <-sigCh:
		sm.Transition(EventUserQuit)
	}
	return cfg
}

// handleConnecting 处理连接状态
func handleConnecting(sm *StateMachine, cfg *config.Config, trayApp *tray.App, configPath string, sigCh chan os.Signal) {
	result := runConnection(sm, cfg, trayApp, sigCh)

	// 连接结束后，根据结果触发事件（EventConnectSuccess 已在 runConnection 内部触发）
	switch result.Event {
	case EventConnectFailed, EventSessionError, EventSessionClosed:
		trayApp.SetError(result.ErrorType, result.ErrorMsg)
		sm.Transition(result.Event)
	case EventVersionMismatch, EventNotConfigured:
		sm.Transition(result.Event)
	case EventUserDisconnect, EventUserQuit:
		sm.Transition(result.Event)
	case EventSwitchServer:
		if result.NewServer != nil {
			newCfg := applyServerConfig(*result.NewServer, trayApp, configPath)
			if newCfg != nil {
				*cfg = *newCfg
			}
		}
		sm.Transition(EventSwitchServer)
	}
}

// handleInstalling 处理安装状态
func handleInstalling(sm *StateMachine, cfg *config.Config, trayApp *tray.App) {
	if doInstall(cfg, trayApp) {
		sm.Transition(EventInstallSuccess)
	} else {
		sm.Transition(EventInstallFailed)
	}
}

// handleReinstalling 处理重新安装状态
func handleReinstalling(sm *StateMachine, cfg *config.Config, trayApp *tray.App) {
	if doReinstall(cfg, trayApp) {
		sm.Transition(EventInstallSuccess)
	} else {
		sm.Transition(EventInstallFailed)
	}
}

// handleWaitForUser 处理断开/错误状态，等待用户操作
func handleWaitForUser(sm *StateMachine, cfg *config.Config, trayApp *tray.App, configPath string, sigCh chan os.Signal) *config.Config {
	select {
	case server := <-trayApp.ServerSelectChan():
		newCfg := applyServerConfig(server, trayApp, configPath)
		if newCfg != nil {
			cfg = newCfg
		}
		sm.Transition(EventServerSelected)
	case <-trayApp.QuitChan():
		sm.Transition(EventUserQuit)
	case <-sigCh:
		sm.Transition(EventUserQuit)
	}
	return cfg
}

// applyServerConfig 应用服务器配置并保存
func applyServerConfig(server config.ServerConfig, trayApp *tray.App, configPath string) *config.Config {
	cfg := config.NewFromServer(server)
	cfg.ApplySSHConfig()

	if cfg.StatusTimeout == 0 {
		cfg.StatusTimeout = 300
	}
	trayApp.SetStatusTimeout(cfg.StatusTimeout)

	if configPath != "" {
		if err := config.Save(configPath, cfg); err != nil {
			logger.Error("Failed to save config: %v", err)
			return nil
		}
	}

	return cfg
}

// runConnection 运行一次连接，返回 ConnectionResult
func runConnection(sm *StateMachine, cfg *config.Config, trayApp *tray.App, sigCh chan os.Signal) ConnectionResult {
	logger.Info("runConnection: mode=%s, display=%s", getMode(cfg), getDisplayName(cfg))

	// 创建客户端（SSH 或 WSL）
	var client monitor.Client

	if cfg.WSL.Enabled {
		client = wsl.NewClient(cfg)
	} else {
		client = ssh.NewClient(cfg)
	}

	// 连接
	if err := client.Connect(); err != nil {
		logger.Error("Connect failed: %v", err)
		return ConnectionResult{
			Event:     EventConnectFailed,
			ErrorType: "connection_failed",
			ErrorMsg:  err.Error(),
		}
	}
	defer client.Close()
	logger.Info("Connected successfully")

	// 启动监听
	if err := client.Start(); err != nil {
		logger.Error("Start failed: %v", err)
		errMsg := err.Error()

		// 检测是否是版本不匹配
		if errors.Is(err, ssh.ErrVersionMismatch) || errors.Is(err, wsl.ErrVersionMismatch) {
			logger.Info("版本不匹配，触发重新安装...")
			return ConnectionResult{Event: EventVersionMismatch}
		}

		// 检测是否是服务端未配置
		if isNotConfiguredError(errMsg) {
			logger.Info("服务端未配置，尝试自动安装...")
			return ConnectionResult{Event: EventNotConfigured}
		}

		return ConnectionResult{
			Event:     EventConnectFailed,
			ErrorType: "session_error",
			ErrorMsg:  errMsg,
		}
	}

	// 连接成功，触发状态转换
	logger.Info("Session started")
	sm.Transition(EventConnectSuccess)

	// 主监控循环
	for {
		select {
		case statuses := <-client.StatusChan():
			trayApp.UpdateStatus(statuses)

		case err := <-client.ErrorChan():
			errMsg := err.Error()
			errType := "session_error"
			if isNotConfiguredError(errMsg) {
				errType = "not_configured"
			}
			return ConnectionResult{
				Event:     EventSessionError,
				ErrorType: errType,
				ErrorMsg:  errMsg,
			}

		case <-client.Done():
			return ConnectionResult{
				Event:     EventSessionClosed,
				ErrorType: "session_error",
				ErrorMsg:  "连接已断开",
			}

		case <-trayApp.QuitChan():
			return ConnectionResult{Event: EventUserQuit}

		case <-trayApp.DisconnectChan():
			logger.Info("用户主动断开连接")
			return ConnectionResult{Event: EventUserDisconnect}

		case server := <-trayApp.ServerSelectChan():
			return ConnectionResult{
				Event:     EventSwitchServer,
				NewServer: &server,
			}

		case <-sigCh:
			return ConnectionResult{Event: EventUserQuit}
		}
	}
}

// doInstall 执行首次安装，返回是否成功
func doInstall(cfg *config.Config, trayApp *tray.App) bool {
	var inst monitor.Installer
	if cfg.WSL.Enabled {
		inst = wsl.NewInstaller(cfg)
	} else {
		inst = installer.NewInstaller(cfg)
	}

	if err := inst.Connect(); err != nil {
		logger.Error("安装器连接失败: %v", err)
		trayApp.SetError("install_failed", "安装失败: "+err.Error())
		return false
	}
	defer inst.Close()

	// 检查依赖
	if ok, msg := inst.CheckDependencies(); !ok {
		logger.Error("依赖检查失败: %s", msg)
		trayApp.SetError("install_failed", msg)
		return false
	}

	// 执行安装
	if err := inst.Install(); err != nil {
		logger.Error("安装失败: %v", err)
		trayApp.SetError("install_failed", "安装失败: "+err.Error())
		return false
	}

	logger.Info("服务端安装完成，等待重新连接...")
	return true
}

// doReinstall 执行重新安装（版本不匹配时），返回是否成功
func doReinstall(cfg *config.Config, trayApp *tray.App) bool {
	var inst monitor.Installer
	if cfg.WSL.Enabled {
		inst = wsl.NewInstaller(cfg)
	} else {
		inst = installer.NewInstaller(cfg)
	}

	if err := inst.Connect(); err != nil {
		logger.Error("安装器连接失败: %v", err)
		trayApp.SetError("install_failed", "更新失败: "+err.Error())
		return false
	}
	defer inst.Close()

	// 版本不匹配时跳过依赖检查

	// 执行安装
	if err := inst.Install(); err != nil {
		logger.Error("更新失败: %v", err)
		trayApp.SetError("install_failed", "更新失败: "+err.Error())
		return false
	}

	logger.Info("服务端更新完成，等待重新连接...")
	return true
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
