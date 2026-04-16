//go:build windows

package app

import (
	"fmt"

	"claude-status/internal/config"
	"claude-status/internal/installer"
	"claude-status/internal/logger"
	"claude-status/internal/monitor"
	"claude-status/internal/wsl"
)

// RunUninstall 读取配置并在服务端执行卸载脚本，
// 清理 ~/.claude/settings.json 中与 status-hook.sh 相关的 Hook，
// 删除 ~/.claude-status/ 下的全部脚本与状态文件。
//
// purge=true 时额外：
//   - 删除所有 ~/.claude/settings.json.backup.* 备份
//   - 若 settings.json 退化为空 {} 则一并删除
//   - 若 ~/.claude 目录因此变空也一并删除
//
// 该流程独立于托盘主循环，由 --uninstall 命令行参数触发后直接退出。
func RunUninstall(configPath string, purge bool) error {
	// 初始化日志（失败不影响卸载流程）
	if err := logger.Init(); err == nil {
		defer logger.Close()
	}

	logger.Info("RunUninstall: configPath=%s purge=%v", configPath, purge)

	if !config.Exists(configPath) {
		return fmt.Errorf("配置文件不存在: %s", configPath)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	var inst monitor.Installer
	if cfg.WSL.Enabled {
		inst = wsl.NewInstaller(cfg)
	} else {
		inst = installer.NewInstaller(cfg)
	}

	if err := inst.Connect(); err != nil {
		return fmt.Errorf("连接失败: %w", err)
	}
	defer inst.Close()

	if err := inst.Uninstall(purge); err != nil {
		return fmt.Errorf("卸载失败: %w", err)
	}

	logger.Info("服务端卸载完成")
	return nil
}
