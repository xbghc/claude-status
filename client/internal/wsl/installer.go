//go:build windows

package wsl

import (
	"fmt"
	"os/exec"
	"strings"

	"claude-status/internal/config"
	"claude-status/internal/installer"
	"claude-status/internal/logger"
)

// Installer WSL 安装器
type Installer struct {
	cfg *config.Config
}

// NewInstaller 创建安装器
func NewInstaller(cfg *config.Config) *Installer {
	return &Installer{cfg: cfg}
}

// Connect WSL 不需要连接
func (i *Installer) Connect() error {
	return nil
}

// Close WSL 不需要关闭
func (i *Installer) Close() {}

// CheckDependencies 检查依赖
func (i *Installer) CheckDependencies() (bool, string) {
	output, err := i.runCommand("command -v inotifywait && command -v jq")
	if err != nil {
		missing := []string{}
		if !strings.Contains(output, "inotifywait") {
			missing = append(missing, "inotify-tools")
		}
		if !strings.Contains(output, "jq") {
			missing = append(missing, "jq")
		}
		return false, fmt.Sprintf("缺少依赖: %s\n请在 WSL 中运行: sudo apt install %s",
			strings.Join(missing, ", "), strings.Join(missing, " "))
	}
	return true, ""
}

// Install 执行安装
func (i *Installer) Install() error {
	logger.Info("开始 WSL 安装...")

	// 1. 创建目录
	if _, err := i.runCommand("mkdir -p ~/.claude-status/hooks ~/.claude"); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 2. 上传脚本
	if err := i.writeFile("~/.claude-status/hooks/status-hook.sh", installer.StatusHookScript); err != nil {
		return fmt.Errorf("写入 status-hook.sh 失败: %w", err)
	}

	if err := i.writeFile("~/.claude-status/monitor.sh", installer.MonitorScript); err != nil {
		return fmt.Errorf("写入 monitor.sh 失败: %w", err)
	}

	// 3. 设置执行权限
	if _, err := i.runCommand("chmod +x ~/.claude-status/hooks/status-hook.sh ~/.claude-status/monitor.sh"); err != nil {
		return fmt.Errorf("设置权限失败: %w", err)
	}

	// 4. 配置 Claude Code hooks
	if err := i.configureHooks(); err != nil {
		return fmt.Errorf("配置 hooks 失败: %w", err)
	}

	logger.Info("WSL 安装完成")
	return nil
}

// runCommand 执行 WSL 命令
func (i *Installer) runCommand(command string) (string, error) {
	args := []string{}
	if i.cfg.WSL.Distro != "" {
		args = append(args, "-d", i.cfg.WSL.Distro)
	}
	args = append(args, "--", "bash", "-c", command)

	cmd := exec.Command("wsl", args...)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

// writeFile 写入文件
func (i *Installer) writeFile(path, content string) error {
	// 使用 heredoc 写入文件
	cmd := fmt.Sprintf("cat > %s << 'EOFSCRIPT'\n%s\nEOFSCRIPT", path, content)
	_, err := i.runCommand(cmd)
	return err
}

// configureHooks 配置 hooks
func (i *Installer) configureHooks() error {
	cmd := fmt.Sprintf("bash -s << 'EOFSCRIPT'\n%s\nEOFSCRIPT", installer.InstallRemoteScript)
	_, err := i.runCommand(cmd)
	return err
}
