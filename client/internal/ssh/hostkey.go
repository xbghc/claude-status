package ssh

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// GetHostKeyCallback 返回主机密钥验证回调函数
// 优先使用 known_hosts 文件验证，如果文件不存在则自动添加（首次信任）
func GetHostKeyCallback() (ssh.HostKeyCallback, error) {
	knownHostsPath := getKnownHostsPath()

	// 确保 .ssh 目录存在
	sshDir := filepath.Dir(knownHostsPath)
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		return nil, fmt.Errorf("创建 .ssh 目录失败: %w", err)
	}

	// 如果 known_hosts 文件不存在，创建空文件
	if _, err := os.Stat(knownHostsPath); os.IsNotExist(err) {
		f, err := os.OpenFile(knownHostsPath, os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return nil, fmt.Errorf("创建 known_hosts 文件失败: %w", err)
		}
		f.Close()
	}

	// 创建 known_hosts 回调
	hostKeyCallback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("加载 known_hosts 失败: %w", err)
	}

	// 包装回调，处理未知主机（首次信任并添加）
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := hostKeyCallback(hostname, remote, key)
		if err == nil {
			return nil
		}

		// 检查是否是未知主机错误
		keyErr, ok := err.(*knownhosts.KeyError)
		if !ok {
			return err
		}

		// 如果是主机密钥不匹配（可能是中间人攻击），拒绝连接
		if len(keyErr.Want) > 0 {
			return fmt.Errorf("主机密钥不匹配，可能存在安全风险: %w", err)
		}

		// 未知主机，首次信任并添加到 known_hosts
		if err := addHostKey(knownHostsPath, hostname, key); err != nil {
			return fmt.Errorf("添加主机密钥失败: %w", err)
		}

		return nil
	}, nil
}

// getKnownHostsPath 获取 known_hosts 文件路径
func getKnownHostsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".ssh", "known_hosts")
}

// addHostKey 添加主机密钥到 known_hosts 文件
func addHostKey(path string, hostname string, key ssh.PublicKey) error {
	// 规范化主机名（移除端口如果是默认端口）
	host := normalizeHostname(hostname)

	// 格式化为 known_hosts 行
	line := knownhosts.Line([]string{host}, key)

	// 追加到文件
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = fmt.Fprintln(f, line)
	return err
}

// normalizeHostname 规范化主机名
func normalizeHostname(hostname string) string {
	// 如果是默认端口，移除端口号
	host, port, err := net.SplitHostPort(hostname)
	if err != nil {
		return hostname
	}
	if port == "22" {
		return host
	}
	return hostname
}

// IsHostInKnownHosts 检查主机是否在 known_hosts 中
func IsHostInKnownHosts(hostname string) bool {
	knownHostsPath := getKnownHostsPath()
	if knownHostsPath == "" {
		return false
	}

	f, err := os.Open(knownHostsPath)
	if err != nil {
		return false
	}
	defer f.Close()

	host := normalizeHostname(hostname)
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, host+" ") || strings.HasPrefix(line, host+",") {
			return true
		}
	}
	return false
}
