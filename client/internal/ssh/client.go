package ssh

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"claude-status/internal/config"
	"claude-status/internal/logger"
	"claude-status/internal/monitor"

	"golang.org/x/crypto/ssh"
)

// Client SSH 客户端
type Client struct {
	config   *config.Config
	client   *ssh.Client
	session  *ssh.Session
	statusCh chan []monitor.ProjectStatus
	errorCh  chan error
	done     chan struct{}
}

// NewClient 创建 SSH 客户端
func NewClient(cfg *config.Config) *Client {
	return &Client{
		config:   cfg,
		statusCh: make(chan []monitor.ProjectStatus, 10),
		errorCh:  make(chan error, 1),
		done:     make(chan struct{}),
	}
}

// Connect 连接到服务器
func (c *Client) Connect() error {
	// 读取私钥
	keyPath := c.config.GetIdentityFile()
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return fmt.Errorf("读取密钥文件失败 (%s): %w", keyPath, err)
	}

	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return fmt.Errorf("解析密钥失败: %w", err)
	}

	// 获取主机密钥验证回调
	hostKeyCallback, err := GetHostKeyCallback()
	if err != nil {
		return fmt.Errorf("初始化主机密钥验证失败: %w", err)
	}

	// SSH 配置
	sshConfig := &ssh.ClientConfig{
		User: c.config.Server.User,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: hostKeyCallback,
		Timeout:         10 * time.Second,
	}

	// 连接服务器
	addr := fmt.Sprintf("%s:%d", c.config.Server.Host, c.config.Server.Port)
	client, err := ssh.Dial("tcp", addr, sshConfig)
	if err != nil {
		return fmt.Errorf("连接服务器失败 (%s): %w", addr, err)
	}
	c.client = client

	// 创建会话
	session, err := client.NewSession()
	if err != nil {
		client.Close()
		return fmt.Errorf("创建会话失败: %w", err)
	}
	c.session = session

	return nil
}

// Start 启动监听
func (c *Client) Start() error {
	// 获取 stdout
	stdout, err := c.session.StdoutPipe()
	if err != nil {
		return fmt.Errorf("获取 stdout 失败: %w", err)
	}

	// 获取 stderr 用于调试
	stderr, err := c.session.StderrPipe()
	if err != nil {
		return fmt.Errorf("获取 stderr 失败: %w", err)
	}

	// 启动远程命令
	monitorCmd := "$HOME/.claude-status/monitor.sh"
	if err := c.session.Start(monitorCmd); err != nil {
		return fmt.Errorf("启动远程命令失败: %w", err)
	}

	// 读取 stderr（异步，用于调试）
	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			fmt.Printf("[远程错误] %s\n", scanner.Text())
		}
	}()

	// 读取 stdout（状态数据）
	go c.readOutput(stdout)

	// 等待会话结束
	go func() {
		err := c.session.Wait()
		if err != nil {
			select {
			case c.errorCh <- fmt.Errorf("会话结束: %w", err):
			default:
			}
		}
		close(c.done)
	}()

	return nil
}

// readOutput 读取输出流
func (c *Client) readOutput(r io.Reader) {
	logger.Info("readOutput: started")
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		logger.Debug("readOutput: received line: %s", line)
		if line == "" {
			continue
		}

		var msg monitor.StatusMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			logger.Error("readOutput: JSON parse error: %v, line: %s", err, line)
			continue
		}

		logger.Info("readOutput: parsed message type=%s", msg.Type)
		switch msg.Type {
		case "status":
			logger.Info("readOutput: status update with %d projects", len(msg.Data))
			select {
			case c.statusCh <- msg.Data:
			default:
				// channel 满了，丢弃旧数据
				select {
				case <-c.statusCh:
				default:
				}
				c.statusCh <- msg.Data
			}
		case "error":
			logger.Error("readOutput: remote error: %s", msg.Message)
		}
	}

	if err := scanner.Err(); err != nil {
		logger.Error("readOutput: scanner error: %v", err)
		select {
		case c.errorCh <- fmt.Errorf("读取输出失败: %w", err):
		default:
		}
	}
	logger.Info("readOutput: ended")
}

// StatusChan 返回状态 channel
func (c *Client) StatusChan() <-chan []monitor.ProjectStatus {
	return c.statusCh
}

// ErrorChan 返回错误 channel
func (c *Client) ErrorChan() <-chan error {
	return c.errorCh
}

// Done 返回完成 channel
func (c *Client) Done() <-chan struct{} {
	return c.done
}

// Close 关闭连接
func (c *Client) Close() {
	if c.session != nil {
		c.session.Close()
	}
	if c.client != nil {
		c.client.Close()
	}
}
