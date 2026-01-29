//go:build windows

package wsl

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"

	"claude-status/internal/config"
	"claude-status/internal/logger"
	"claude-status/internal/monitor"
	"claude-status/internal/version"
)

// ErrVersionMismatch 版本不匹配错误
var ErrVersionMismatch = errors.New("version mismatch")

// Client WSL 客户端
type Client struct {
	cfg       *config.Config
	cmd       *exec.Cmd
	statusCh  chan []monitor.ProjectStatus
	errorCh   chan error
	doneCh    chan struct{}
	closeOnce sync.Once
	versionOK chan bool // 版本检查结果
}

// NewClient 创建 WSL 客户端
func NewClient(cfg *config.Config) *Client {
	return &Client{
		cfg:       cfg,
		statusCh:  make(chan []monitor.ProjectStatus, 10),
		errorCh:   make(chan error, 1),
		doneCh:    make(chan struct{}),
		versionOK: make(chan bool, 1),
	}
}

// Connect 连接（对于 WSL 只是验证 wsl 命令可用）
func (c *Client) Connect() error {
	// 检查 wsl 命令是否可用
	cmd := exec.Command("wsl", "--status")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("WSL 不可用: %w", err)
	}
	return nil
}

// Start 启动监听
func (c *Client) Start() error {
	// 构建 wsl 命令
	args := []string{}
	if c.cfg.WSL.Distro != "" {
		args = append(args, "-d", c.cfg.WSL.Distro)
	}
	args = append(args, "--", "bash", "-c", "$HOME/.claude-status/monitor.sh")

	c.cmd = exec.Command("wsl", args...)

	// 获取 stdout
	stdout, err := c.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("获取 stdout 失败: %w", err)
	}

	// 获取 stderr
	stderr, err := c.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("获取 stderr 失败: %w", err)
	}

	// 启动命令
	if err := c.cmd.Start(); err != nil {
		return fmt.Errorf("启动 WSL 命令失败: %w", err)
	}

	// 读取 stderr（用于错误检测）
	go c.readStderr(stderr)

	// 读取 stdout（状态更新）
	go c.readOutput(stdout)

	// 等待命令结束
	go func() {
		c.cmd.Wait()
		c.closeOnce.Do(func() {
			close(c.doneCh)
		})
	}()

	// 等待版本检查结果（5秒超时）
	select {
	case ok := <-c.versionOK:
		if !ok {
			return ErrVersionMismatch
		}
	case <-time.After(5 * time.Second):
		// 超时，假设是旧版本脚本（不输出版本信息）
		logger.Info("版本检查超时，可能是旧版本脚本")
		return ErrVersionMismatch
	}

	return nil
}

// readOutput 读取输出
func (c *Client) readOutput(r io.Reader) {
	scanner := bufio.NewScanner(r)
	firstMessage := true

	for scanner.Scan() {
		line := scanner.Text()
		logger.Debug("WSL output: %s", line)

		var msg monitor.StatusMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			logger.Debug("JSON parse error: %v", err)
			continue
		}

		switch msg.Type {
		case monitor.MsgTypeVersion:
			if firstMessage {
				firstMessage = false
				// 检查版本
				if msg.Version == version.Version {
					logger.Info("版本匹配: %s", msg.Version)
					select {
					case c.versionOK <- true:
					default:
					}
				} else {
					logger.Info("版本不匹配: 服务端=%s, 客户端=%s", msg.Version, version.Version)
					select {
					case c.versionOK <- false:
					default:
					}
				}
			}

		case monitor.MsgTypeStatus:
			select {
			case c.statusCh <- msg.Data:
			default:
				// 丢弃旧消息
				select {
				case <-c.statusCh:
				default:
				}
				c.statusCh <- msg.Data
			}

		case monitor.MsgTypeError:
			select {
			case c.errorCh <- fmt.Errorf(msg.Message):
			default:
			}
		}
	}
}

// readStderr 读取错误输出
func (c *Client) readStderr(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		logger.Error("WSL stderr: %s", line)
	}
}

// Close 关闭连接
func (c *Client) Close() {
	if c.cmd != nil && c.cmd.Process != nil {
		c.cmd.Process.Kill()
	}
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
	return c.doneCh
}
