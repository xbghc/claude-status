//go:build windows

package app

import (
	"claude-status/internal/config"
	"claude-status/internal/tray"
)

// ConnectionResult 连接结果（替代原来的 4 元组返回值）
type ConnectionResult struct {
	Event     Event                // 触发的事件
	NewServer *config.ServerConfig // EventSwitchServer 时非 nil
	ErrorMsg  string               // 错误信息
	ErrorType string               // 错误类型（用于 tray.SetError）
}

// applyTrayState 根据状态更新托盘 UI
func applyTrayState(trayApp *tray.App, change StateChange, cfg *config.Config) {
	var displayName string
	if cfg != nil {
		displayName = getDisplayName(cfg)
	}

	switch change.To {
	case StateUnconfigured:
		trayApp.ShowServerSelection()
	case StateConnecting:
		trayApp.SetConnecting(displayName)
	case StateInstalling:
		trayApp.SetConnecting("正在安装服务端...")
	case StateReinstalling:
		trayApp.SetConnecting("版本不匹配，正在更新服务端...")
	case StateConnected:
		trayApp.SetConnected(true, displayName)
	case StateDisconnected:
		trayApp.SetDisconnected()
	case StateError:
		// 错误状态的具体信息由调用方在 Transition 前通过 trayApp.SetError 设置
	case StateQuitting:
		// 退出状态无需更新 UI
	}
}
