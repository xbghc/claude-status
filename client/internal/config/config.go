package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kevinburke/ssh_config"
	"gopkg.in/yaml.v3"
)

// Config 应用配置
type Config struct {
	Server        ServerConfig `yaml:"server"`
	Debug         bool         `yaml:"debug,omitempty"`
	StatusTimeout int          `yaml:"status_timeout,omitempty"` // 状态超时（秒），默认 300，0 禁用
}

// ServerConfig SSH 服务器配置
type ServerConfig struct {
	Name          string `yaml:"name,omitempty"`
	Host          string `yaml:"host"`
	Port          int    `yaml:"port"`
	User          string `yaml:"user"`
	IdentityFile  string `yaml:"identity_file"`
	SSHConfigPath string `yaml:"ssh_config_path"`
}

// Exists 检查配置文件是否存在
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// Load 加载配置文件
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 设置默认值
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 22
	}
	// StatusTimeout: -1 表示未配置（使用默认值），0 表示禁用，>0 表示具体秒数
	// 注意：YAML 中未设置的字段默认为 0，所以需要特殊处理

	// 从 SSH config 补充配置
	if err := cfg.ApplySSHConfig(); err != nil {
		// SSH config 读取失败不是致命错误，忽略
	}

	// 验证必要配置
	if cfg.Server.Host == "" {
		return nil, fmt.Errorf("缺少必要配置: server.host")
	}

	return &cfg, nil
}

// LoadSSHHosts 从 ~/.ssh/config 读取主机列表
func LoadSSHHosts() ([]ServerConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	sshConfigPath := filepath.Join(home, ".ssh", "config")
	f, err := os.Open(sshConfigPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cfg, err := ssh_config.Decode(f)
	if err != nil {
		return nil, err
	}

	var servers []ServerConfig
	seen := make(map[string]bool)

	for _, host := range cfg.Hosts {
		for _, pattern := range host.Patterns {
			name := pattern.String()
			// 跳过通配符和已处理的
			if name == "*" || name == "" || seen[name] {
				continue
			}
			seen[name] = true

			// 获取配置
			hostname, _ := cfg.Get(name, "Hostname")
			if hostname == "" {
				hostname = name
			}
			user, _ := cfg.Get(name, "User")
			portStr, _ := cfg.Get(name, "Port")
			port := 22
			if portStr != "" {
				fmt.Sscanf(portStr, "%d", &port)
			}
			identityFile, _ := cfg.Get(name, "IdentityFile")

			servers = append(servers, ServerConfig{
				Name:         name,
				Host:         hostname,
				Port:         port,
				User:         user,
				IdentityFile: expandPath(identityFile),
			})
		}
	}

	return servers, nil
}

// Save 保存配置到文件
func Save(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	// 确保目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}

	return nil
}

// NewFromServer 从服务器配置创建完整配置
func NewFromServer(server ServerConfig) *Config {
	return &Config{
		Server: server,
	}
}

// ApplySSHConfig 从 ~/.ssh/config 读取补充配置
func (c *Config) ApplySSHConfig() error {
	// 确定 SSH config 路径
	sshConfigPath := c.Server.SSHConfigPath
	if sshConfigPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		sshConfigPath = filepath.Join(home, ".ssh", "config")
	}

	// 读取 SSH config
	f, err := os.Open(sshConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // SSH config 不存在不是错误
		}
		return err
	}
	defer f.Close()

	cfg, err := ssh_config.Decode(f)
	if err != nil {
		return err
	}

	host := c.Server.Host

	// 从 SSH config 获取配置
	if c.Server.User == "" {
		if user, err := cfg.Get(host, "User"); err == nil && user != "" {
			c.Server.User = user
		}
	}

	if c.Server.IdentityFile == "" {
		if identityFile, err := cfg.Get(host, "IdentityFile"); err == nil && identityFile != "" {
			c.Server.IdentityFile = expandPath(identityFile)
		}
	}

	if c.Server.Port == 22 {
		if port, err := cfg.Get(host, "Port"); err == nil && port != "" {
			var p int
			if _, err := fmt.Sscanf(port, "%d", &p); err == nil {
				c.Server.Port = p
			}
		}
	}

	// 获取真实主机名（如果配置了别名）
	if hostname, err := cfg.Get(host, "Hostname"); err == nil && hostname != "" {
		c.Server.Host = hostname
	}

	return nil
}

// GetIdentityFile 获取密钥文件路径，如果未配置则返回默认路径
func (c *Config) GetIdentityFile() string {
	if c.Server.IdentityFile != "" {
		return expandPath(c.Server.IdentityFile)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	// 尝试常见的密钥文件
	candidates := []string{
		filepath.Join(home, ".ssh", "id_rsa"),
		filepath.Join(home, ".ssh", "id_ed25519"),
		filepath.Join(home, ".ssh", "id_ecdsa"),
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return filepath.Join(home, ".ssh", "id_rsa")
}

// expandPath 展开路径中的 ~ 符号
func expandPath(path string) string {
	if len(path) > 0 && path[0] == '~' {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[1:])
	}
	return path
}
