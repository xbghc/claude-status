//go:build windows

package tray

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"claude-status/assets/icons"
	"claude-status/internal/config"
	"claude-status/internal/logger"
	"claude-status/internal/monitor"

	"github.com/getlantern/systray"
)

// serverMenuItem 服务器菜单项结构
type serverMenuItem struct {
	server      config.ServerConfig
	menuItem    *systray.MenuItem
	mConnect    *systray.MenuItem // 连接/重新连接
	mDisconnect *systray.MenuItem // 断开连接（仅当前服务器显示）
}

// App 系统托盘应用
type App struct {
	statuses        []monitor.ProjectStatus
	servers         []config.ServerConfig
	quitCh          chan struct{}
	disconnectCh    chan struct{}
	serverSelectCh  chan config.ServerConfig
	updateCh        chan []monitor.ProjectStatus
	currentIcon     string
	mStatus         *systray.MenuItem
	mConnectionMenu *systray.MenuItem
	serverMenuItems []*serverMenuItem
	connectedServer string

	// 主题相关
	isDarkMode bool

	// 动画相关
	animMu      sync.Mutex
	animRunning bool
	animStopCh  chan struct{}
	animFrame   int

	// 状态超时（秒），0 或负数表示禁用
	statusTimeout int64

	// 记录每个项目的 working 开始时间（项目路径 -> Unix 时间戳）
	workingStartTimes map[string]int64
}

// NewApp 创建托盘应用
func NewApp() *App {
	return &App{
		statuses:          make([]monitor.ProjectStatus, 0),
		servers:           make([]config.ServerConfig, 0),
		quitCh:            make(chan struct{}),
		disconnectCh:      make(chan struct{}, 1),
		serverSelectCh:    make(chan config.ServerConfig, 1),
		updateCh:          make(chan []monitor.ProjectStatus, 10),
		currentIcon:       "",
		serverMenuItems:   make([]*serverMenuItem, 0),
		connectedServer:   "",
		isDarkMode:        IsDarkMode(),
		animFrame:         0,
		workingStartTimes: make(map[string]int64),
	}
}

// Run 运行托盘应用
func (t *App) Run(onReady func(), onQuit func()) {
	systray.Run(func() {
		t.onReady()
		if onReady != nil {
			onReady()
		}
	}, func() {
		t.stopAnimation()
		if onQuit != nil {
			onQuit()
		}
	})
}

// onReady 托盘就绪回调
func (t *App) onReady() {
	logger.Info("Tray onReady called, isDarkMode=%v", t.isDarkMode)
	t.setIcon("disconnected")
	systray.SetTitle("Claude Status")
	systray.SetTooltip("Claude Code Status Monitor - 未连接")

	// 监听系统主题变化
	MonitorThemeChange(func(isDark bool) {
		logger.Info("Theme changed: isDarkMode=%v", isDark)
		t.isDarkMode = isDark
		t.refreshIcon()
	})

	logger.Info("Tray initialized")

	// 添加状态信息菜单项
	t.mStatus = systray.AddMenuItem("未配置", "")
	t.mStatus.Disable()

	systray.AddSeparator()

	// 添加连接菜单
	t.mConnectionMenu = systray.AddMenuItem("连接", "连接管理")

	systray.AddSeparator()

	// 添加退出菜单项
	mQuit := systray.AddMenuItem("退出", "退出程序")

	// 监听菜单点击
	go func() {
		for {
			select {
			case <-mQuit.ClickedCh:
				close(t.quitCh)
				systray.Quit()
				return
			}
		}
	}()

	// 监听状态更新
	go t.watchUpdates()
}

// startAnimation 启动动画
func (t *App) startAnimation() {
	t.animMu.Lock()
	defer t.animMu.Unlock()

	if t.animRunning {
		return
	}

	t.animRunning = true
	t.animStopCh = make(chan struct{})
	t.animFrame = 0

	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				t.animMu.Lock()
				if !t.animRunning {
					t.animMu.Unlock()
					return
				}
				frames := icons.RunningDarkFrames
				if !t.isDarkMode {
					frames = icons.RunningLightFrames
				}
				frame := t.animFrame % len(frames)
				systray.SetIcon(frames[frame])
				t.animFrame++
				t.animMu.Unlock()
			case <-t.animStopCh:
				return
			case <-t.quitCh:
				return
			}
		}
	}()
}

// stopAnimation 停止动画
func (t *App) stopAnimation() {
	t.animMu.Lock()
	defer t.animMu.Unlock()

	if !t.animRunning {
		return
	}

	t.animRunning = false
	if t.animStopCh != nil {
		close(t.animStopCh)
		t.animStopCh = nil
	}
}

// setIcon 设置图标
func (t *App) setIcon(name string) {
	if t.currentIcon == name {
		return
	}
	logger.Info("setIcon: %s -> %s (isDark=%v)", t.currentIcon, name, t.isDarkMode)
	t.currentIcon = name
	t.applyIcon(name)
}

// refreshIcon 刷新当前图标（主题变化时调用）
func (t *App) refreshIcon() {
	if t.currentIcon != "" {
		t.applyIcon(t.currentIcon)
	}
}

// applyIcon 应用图标
func (t *App) applyIcon(name string) {
	switch name {
	case "disconnected":
		t.stopAnimation()
		if t.isDarkMode {
			systray.SetIcon(icons.DisconnectedDark)
		} else {
			systray.SetIcon(icons.DisconnectedLight)
		}
	case "input-needed":
		t.stopAnimation()
		if t.isDarkMode {
			systray.SetIcon(icons.InputNeededDark)
		} else {
			systray.SetIcon(icons.InputNeededLight)
		}
	case "running":
		t.startAnimation()
	}
}

// SetStatusTimeout 设置状态超时时间（秒），0 或负数表示禁用
func (t *App) SetStatusTimeout(seconds int) {
	t.statusTimeout = int64(seconds)
}

// SetServers 设置预设服务器列表
func (t *App) SetServers(servers []config.ServerConfig) {
	t.servers = servers

	if len(servers) == 0 {
		return
	}

	t.mStatus.SetTitle("请选择服务器")
	systray.SetTooltip("Claude Code Status Monitor\n\n请从菜单选择要连接的服务器")

	// 创建服务器子菜单项（三级菜单）
	t.serverMenuItems = make([]*serverMenuItem, len(servers))
	for i, server := range servers {
		item := &serverMenuItem{server: server}

		// 创建服务器子菜单
		item.menuItem = t.mConnectionMenu.AddSubMenuItem(server.Name, fmt.Sprintf("连接到 %s", server.Host))

		// 添加操作子菜单
		item.mConnect = item.menuItem.AddSubMenuItem("连接", "连接到此服务器")
		item.mDisconnect = item.menuItem.AddSubMenuItem("断开连接", "断开此服务器连接")
		item.mDisconnect.Hide() // 初始隐藏断开选项

		t.serverMenuItems[i] = item

		// 监听点击事件
		go func(srv config.ServerConfig, mi *serverMenuItem) {
			for {
				select {
				case <-mi.mConnect.ClickedCh:
					t.mStatus.SetTitle("正在连接...")
					select {
					case t.serverSelectCh <- srv:
					default:
					}
				case <-mi.mDisconnect.ClickedCh:
					t.mStatus.SetTitle("正在断开...")
					select {
					case t.disconnectCh <- struct{}{}:
					default:
					}
				case <-t.quitCh:
					return
				}
			}
		}(server, item)
	}
}

// updateServerMenus 更新服务器菜单状态
func (t *App) updateServerMenus() {
	for _, item := range t.serverMenuItems {
		if item.server.Name == t.connectedServer {
			// 当前连接的服务器
			item.menuItem.SetTitle("✓ " + item.server.Name + " (已连接)")
			item.mConnect.SetTitle("重新连接")
			item.mDisconnect.Show()
		} else {
			// 未连接的服务器
			item.menuItem.SetTitle(item.server.Name)
			item.mConnect.SetTitle("连接")
			item.mDisconnect.Hide()
		}
	}
}

// watchUpdates 监听状态更新
func (t *App) watchUpdates() {
	for {
		select {
		case statuses := <-t.updateCh:
			t.updateStatus(statuses)
		case <-t.quitCh:
			return
		}
	}
}

// UpdateStatus 更新状态（外部调用）
func (t *App) UpdateStatus(statuses []monitor.ProjectStatus) {
	select {
	case t.updateCh <- statuses:
	default:
		select {
		case <-t.updateCh:
		default:
		}
		t.updateCh <- statuses
	}
}

// updateStatus 内部更新状态
func (t *App) updateStatus(statuses []monitor.ProjectStatus) {
	now := time.Now().Unix()

	// 过滤掉 stopped 状态和超时的实例，同时更新 working 开始时间
	filtered := make([]monitor.ProjectStatus, 0, len(statuses))
	activeProjects := make(map[string]bool)

	for _, s := range statuses {
		// 跳过已停止的会话，并清除其计时
		if s.Status == "stopped" {
			delete(t.workingStartTimes, s.Project)
			continue
		}
		// 跳过超时的项目（statusTimeout > 0 时启用）
		if t.statusTimeout > 0 && now-s.UpdatedAt > t.statusTimeout {
			delete(t.workingStartTimes, s.Project)
			continue
		}

		activeProjects[s.Project] = true

		// 更新 working 开始时间
		if s.Status == "working" {
			// 如果之前没有记录开始时间，使用服务器的 updated_at 作为开始时间
			if _, exists := t.workingStartTimes[s.Project]; !exists {
				t.workingStartTimes[s.Project] = s.UpdatedAt
			}
		} else {
			// idle 状态清除计时
			delete(t.workingStartTimes, s.Project)
		}

		filtered = append(filtered, s)
	}

	// 清理不再存在的项目
	for project := range t.workingStartTimes {
		if !activeProjects[project] {
			delete(t.workingStartTimes, project)
		}
	}

	t.statuses = filtered

	// 判断是否有项目在工作中
	hasWorking := false
	for _, s := range filtered {
		if s.Status == "working" {
			hasWorking = true
			break
		}
	}

	// 更新图标
	if hasWorking {
		t.setIcon("running")
	} else {
		t.setIcon("input-needed")
	}

	// 更新 Tooltip 和状态菜单
	tooltip := t.buildTooltip()
	systray.SetTooltip(tooltip)

	// 更新状态菜单项
	if len(t.statuses) == 0 {
		t.mStatus.SetTitle("已连接 - 无活动项目")
	} else {
		workingCount := 0
		for _, s := range t.statuses {
			if s.Status == "working" {
				workingCount++
			}
		}
		if workingCount > 0 {
			t.mStatus.SetTitle(fmt.Sprintf("运行中 (%d 个项目)", workingCount))
		} else {
			t.mStatus.SetTitle(fmt.Sprintf("等待输入 (%d 个项目)", len(t.statuses)))
		}
	}
}

// buildTooltip 构建 Tooltip 文本
func (t *App) buildTooltip() string {
	if len(t.statuses) == 0 {
		return "Claude Code Status Monitor - 无活动项目"
	}

	var sb strings.Builder
	sb.WriteString("Claude Code Status:\n")

	workingCount := 0
	idleCount := 0

	for _, s := range t.statuses {
		if s.Status == "working" {
			workingCount++
			// working 状态显示运行时间（从开始时间算起）
			if startTime, exists := t.workingStartTimes[s.Project]; exists {
				duration := time.Since(time.Unix(startTime, 0))
				fmt.Fprintf(&sb, "● %s (%s)\n", s.ProjectName, formatDuration(duration))
			} else {
				fmt.Fprintf(&sb, "● %s\n", s.ProjectName)
			}
		} else {
			idleCount++
			// idle 状态不显示计时
			fmt.Fprintf(&sb, "○ %s\n", s.ProjectName)
		}
	}

	fmt.Fprintf(&sb, "\n运行中: %d | 等待: %d", workingCount, idleCount)

	return sb.String()
}

// formatDuration 格式化时间间隔（显示精确时间如 1m30s, 2h15m）
func formatDuration(d time.Duration) string {
	totalSeconds := int(d.Seconds())

	if totalSeconds < 60 {
		return fmt.Sprintf("%ds", totalSeconds)
	} else if totalSeconds < 3600 {
		m := totalSeconds / 60
		s := totalSeconds % 60
		return fmt.Sprintf("%dm%ds", m, s)
	} else if totalSeconds < 86400 {
		h := totalSeconds / 3600
		m := (totalSeconds % 3600) / 60
		return fmt.Sprintf("%dh%dm", h, m)
	}
	days := totalSeconds / 86400
	h := (totalSeconds % 86400) / 3600
	return fmt.Sprintf("%dd%dh", days, h)
}

// QuitChan 返回退出 channel
func (t *App) QuitChan() <-chan struct{} {
	return t.quitCh
}

// DisconnectChan 返回断开连接 channel
func (t *App) DisconnectChan() <-chan struct{} {
	return t.disconnectCh
}

// ServerSelectChan 返回服务器选择 channel
func (t *App) ServerSelectChan() <-chan config.ServerConfig {
	return t.serverSelectCh
}

// SetConnecting 设置正在连接状态
func (t *App) SetConnecting(msg string) {
	t.setIcon("disconnected")
	systray.SetTooltip("Claude Code Status Monitor - 正在连接...\n" + msg)
	t.mStatus.SetTitle("正在连接 - " + msg)
}

// SetConnected 设置连接状态
func (t *App) SetConnected(connected bool, msg string) {
	if connected {
		t.setIcon("input-needed")
		systray.SetTooltip("Claude Code Status Monitor - 已连接\n" + msg)
		t.mStatus.SetTitle("已连接 - " + msg)
		t.connectedServer = msg
		t.updateServerMenus()
	} else {
		t.setIcon("disconnected")
		systray.SetTooltip("Claude Code Status Monitor - " + msg)
		t.mStatus.SetTitle(msg)
	}
}

// SetDisconnected 设置用户主动断开状态
func (t *App) SetDisconnected() {
	t.setIcon("disconnected")
	t.mStatus.SetTitle("已断开连接")
	systray.SetTooltip("Claude Code Status Monitor - 已断开连接")
	t.connectedServer = ""
	t.updateServerMenus()
}

// SetError 设置错误状态
func (t *App) SetError(errType string, msg string) {
	t.setIcon("disconnected")

	var statusMsg string
	switch errType {
	case "not_configured":
		statusMsg = "服务端未配置"
		systray.SetTooltip("Claude Code Status Monitor\n\n服务端未配置，请在服务器上运行:\ncd server && ./install.sh")
	case "connection_failed":
		statusMsg = "连接失败"
		systray.SetTooltip("Claude Code Status Monitor - 连接失败\n" + msg)
	case "session_error":
		statusMsg = "会话错误"
		systray.SetTooltip("Claude Code Status Monitor - 会话错误\n" + msg)
	case "no_config":
		statusMsg = "未配置"
		systray.SetTooltip("Claude Code Status Monitor\n\n请选择要连接的服务器，或创建 config.yaml")
	default:
		statusMsg = msg
		systray.SetTooltip("Claude Code Status Monitor - " + msg)
	}

	t.mStatus.SetTitle(statusMsg)
}

// ShowServerSelection 显示服务器选择提示
func (t *App) ShowServerSelection() {
	t.setIcon("disconnected")
	t.mStatus.SetTitle("请选择服务器")
	systray.SetTooltip("Claude Code Status Monitor\n\n请从菜单选择要连接的服务器")
}
