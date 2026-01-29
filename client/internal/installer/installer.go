//go:build windows

package installer

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"strings"

	"claude-status/internal/config"
	"claude-status/internal/logger"
	sshclient "claude-status/internal/ssh"
	"claude-status/internal/version"

	"golang.org/x/crypto/ssh"
)

//go:embed scripts/status-hook.sh
var statusHookScriptTemplate string

//go:embed scripts/monitor.sh
var monitorScriptTemplate string

//go:embed scripts/install-remote.sh
var installRemoteScriptTemplate string

// GetStatusHookScript 返回替换版本号后的脚本
func GetStatusHookScript() string {
	return strings.ReplaceAll(statusHookScriptTemplate, "__VERSION__", version.Version)
}

// GetMonitorScript 返回替换版本号后的脚本
func GetMonitorScript() string {
	return strings.ReplaceAll(monitorScriptTemplate, "__VERSION__", version.Version)
}

// GetInstallRemoteScript 返回替换版本号后的脚本
func GetInstallRemoteScript() string {
	return strings.ReplaceAll(installRemoteScriptTemplate, "__VERSION__", version.Version)
}

// 为保持兼容性，提供变量访问（延迟初始化）
var (
	StatusHookScript    = ""
	MonitorScript       = ""
	InstallRemoteScript = ""
)

func init() {
	StatusHookScript = GetStatusHookScript()
	MonitorScript = GetMonitorScript()
	InstallRemoteScript = GetInstallRemoteScript()
}

// Installer 远程安装器
type Installer struct {
	cfg    *config.Config
	client *ssh.Client
}

// NewInstaller 创建安装器
func NewInstaller(cfg *config.Config) *Installer {
	return &Installer{cfg: cfg}
}

// Connect 连接到服务器
func (i *Installer) Connect() error {
	// 读取私钥
	keyPath := i.cfg.GetIdentityFile()
	key, err := loadPrivateKey(keyPath)
	if err != nil {
		return fmt.Errorf("加载密钥失败: %w", err)
	}

	// 获取主机密钥验证回调
	hostKeyCallback, err := sshclient.GetHostKeyCallback()
	if err != nil {
		return fmt.Errorf("初始化主机密钥验证失败: %w", err)
	}

	// SSH 配置
	sshConfig := &ssh.ClientConfig{
		User: i.cfg.Server.User,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(key),
		},
		HostKeyCallback: hostKeyCallback,
	}

	// 连接
	addr := fmt.Sprintf("%s:%d", i.cfg.Server.Host, i.cfg.Server.Port)
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return fmt.Errorf("SSH 连接失败: %w", err)
	}

	i.client = client
	return nil
}

// Close 关闭连接
func (i *Installer) Close() {
	if i.client != nil {
		i.client.Close()
	}
}

// CheckDependencies 检查服务器依赖
func (i *Installer) CheckDependencies() (bool, string) {
	session, err := i.client.NewSession()
	if err != nil {
		return false, "创建会话失败"
	}
	defer session.Close()

	// 检查 inotifywait 和 jq
	output, err := session.CombinedOutput("command -v inotifywait && command -v jq")
	if err != nil {
		missing := []string{}
		if !strings.Contains(string(output), "inotifywait") {
			missing = append(missing, "inotify-tools")
		}
		if !strings.Contains(string(output), "jq") {
			missing = append(missing, "jq")
		}
		return false, fmt.Sprintf("缺少依赖: %s", strings.Join(missing, ", "))
	}
	return true, ""
}

// Install 执行安装
func (i *Installer) Install() error {
	logger.Info("开始远程安装...")

	// 1. 创建目录
	if err := i.runCommand("mkdir -p ~/.claude-status/hooks ~/.claude"); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	// 2. 上传脚本
	if err := i.uploadFile("~/.claude-status/hooks/status-hook.sh", StatusHookScript); err != nil {
		return fmt.Errorf("上传 status-hook.sh 失败: %w", err)
	}

	if err := i.uploadFile("~/.claude-status/monitor.sh", MonitorScript); err != nil {
		return fmt.Errorf("上传 monitor.sh 失败: %w", err)
	}

	// 3. 设置执行权限
	if err := i.runCommand("chmod +x ~/.claude-status/hooks/status-hook.sh ~/.claude-status/monitor.sh"); err != nil {
		return fmt.Errorf("设置权限失败: %w", err)
	}

	// 4. 配置 Claude Code hooks
	if err := i.configureHooks(); err != nil {
		return fmt.Errorf("配置 hooks 失败: %w", err)
	}

	logger.Info("远程安装完成")
	return nil
}

// runCommand 执行远程命令
func (i *Installer) runCommand(cmd string) error {
	session, err := i.client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	return session.Run(cmd)
}

// uploadFile 上传文件内容
func (i *Installer) uploadFile(remotePath, content string) error {
	session, err := i.client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	// 使用 cat 写入文件
	stdin, err := session.StdinPipe()
	if err != nil {
		return err
	}

	go func() {
		defer stdin.Close()
		io.WriteString(stdin, content)
	}()

	// 展开 ~ 路径
	cmd := fmt.Sprintf("cat > %s", remotePath)
	return session.Run(cmd)
}

// configureHooks 配置 Claude Code hooks
func (i *Installer) configureHooks() error {
	session, err := i.client.NewSession()
	if err != nil {
		return err
	}
	defer session.Close()

	// 使用嵌入的安装脚本配置 hooks
	stdin, err := session.StdinPipe()
	if err != nil {
		return err
	}

	go func() {
		defer stdin.Close()
		io.WriteString(stdin, InstallRemoteScript)
	}()

	return session.Run("bash -s")
}

// loadPrivateKey 加载私钥
func loadPrivateKey(path string) (ssh.Signer, error) {
	// 复用 ssh 包中的加载逻辑
	key, err := loadKeyFromFile(path)
	if err != nil {
		return nil, err
	}
	return ssh.ParsePrivateKey(key)
}

// loadKeyFromFile 从文件加载密钥
func loadKeyFromFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}
