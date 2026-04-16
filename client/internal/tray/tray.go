//go:build windows

package tray

import (
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"claude-status/assets/icons"
	"claude-status/internal/config"
	"claude-status/internal/logger"
	"claude-status/internal/monitor"
	"claude-status/internal/tray/popup"

	"github.com/lxn/walk"
	"github.com/lxn/win"
)

// 托盘消息 ID
const (
	WM_TRAYICON = win.WM_USER + 1
)

// serverMenuItem 服务器菜单项结构
type serverMenuItem struct {
	server      config.ServerConfig
	menuItem    *walk.Action
	mConnect    *walk.Action
	mDisconnect *walk.Action
	subMenu     *walk.Menu
}

// App 系统托盘应用
type App struct {
	mainWindow    *walk.MainWindow
	notifyIcon    *walk.NotifyIcon
	popupWindow   *popup.PopupWindow
	contextMenu   *walk.Menu
	mStatus       *walk.Action
	mConnection   *walk.Action
	connectionMenu *walk.Menu
	serverMenuItems []*serverMenuItem

	statuses        []monitor.ProjectStatus
	servers         []config.ServerConfig
	quitCh          chan struct{}
	disconnectCh    chan struct{}
	serverSelectCh  chan config.ServerConfig
	currentIcon     string
	connectedServer string

	// 主题相关
	isDarkMode bool

	// 动画相关
	animMu      sync.Mutex
	animWg      sync.WaitGroup
	animRunning bool
	animStopCh  chan struct{}
	animFrame   int

	// 图标缓存（避免每帧重复创建）
	iconCache     map[string]*walk.Icon
	iconCacheMu   sync.Mutex

	// 悬浮检测
	hoverMu       sync.Mutex
	isHovering    bool
	lastCheckTime time.Time
}

// NewApp 创建托盘应用
func NewApp() *App {
	return &App{
		statuses:        make([]monitor.ProjectStatus, 0),
		servers:         make([]config.ServerConfig, 0),
		quitCh:          make(chan struct{}),
		disconnectCh:    make(chan struct{}, 1),
		serverSelectCh:  make(chan config.ServerConfig, 1),
		currentIcon:     "",
		serverMenuItems: make([]*serverMenuItem, 0),
		connectedServer: "",
		isDarkMode:      IsDarkMode(),
		animFrame:       0,
		iconCache:       make(map[string]*walk.Icon),
	}
}

// Run 运行托盘应用
func (t *App) Run(onReady func(), onQuit func()) {
	var err error

	// 创建隐藏的主窗口（用于消息循环）
	t.mainWindow, err = walk.NewMainWindow()
	if err != nil {
		logger.Error("Failed to create main window: %v", err)
		return
	}
	t.mainWindow.SetVisible(false)

	// 创建托盘图标
	t.notifyIcon, err = walk.NewNotifyIcon(t.mainWindow)
	if err != nil {
		logger.Error("Failed to create notify icon: %v", err)
		return
	}

	// 创建悬浮窗口
	t.popupWindow, err = popup.NewPopupWindow()
	if err != nil {
		logger.Error("Failed to create popup window: %v", err)
		// 继续运行，只是没有悬浮窗口功能
	} else {
		// 设置初始主题
		t.popupWindow.SetDarkMode(t.isDarkMode)
	}

	// 初始化托盘
	t.onReady()

	// 启动悬浮检测
	t.startHoverDetection()

	if onReady != nil {
		go onReady()
	}

	// 运行消息循环
	t.mainWindow.Run()

	// 清理
	t.stopAnimation()
	if t.popupWindow != nil {
		t.popupWindow.Dispose()
	}
	t.notifyIcon.Dispose()

	// 清理图标缓存
	t.iconCacheMu.Lock()
	for _, icon := range t.iconCache {
		icon.Dispose()
	}
	t.iconCache = nil
	t.iconCacheMu.Unlock()

	if onQuit != nil {
		onQuit()
	}
}

// onReady 托盘就绪回调
func (t *App) onReady() {
	logger.Info("Tray onReady called, isDarkMode=%v", t.isDarkMode)

	// 设置图标
	t.SetIcon("disconnected")
	t.notifyIcon.SetToolTip("Claude Code Status - 未连接")
	t.notifyIcon.SetVisible(true)

	// 创建右键菜单
	t.setupContextMenu()

	// 设置鼠标事件（右键菜单由 walk 自动处理）
	t.notifyIcon.MouseDown().Attach(func(x, y int, button walk.MouseButton) {
		if button == walk.LeftButton {
			// 左键切换悬浮窗口
			t.togglePopup()
		}
	})

	// 监听系统主题变化
	MonitorThemeChange(func(isDark bool) {
		logger.Info("Theme changed: isDarkMode=%v", isDark)
		t.isDarkMode = isDark
		t.refreshIcon()
		// 同步弹窗主题
		if t.popupWindow != nil {
			t.popupWindow.SetDarkMode(isDark)
		}
	})

	logger.Info("Tray initialized")
}

// setupContextMenu 设置右键菜单
func (t *App) setupContextMenu() {
	// 获取托盘图标的上下文菜单
	t.contextMenu = t.notifyIcon.ContextMenu()

	// 状态菜单项
	t.mStatus = walk.NewAction()
	t.mStatus.SetText("未配置")
	t.mStatus.SetEnabled(false)
	t.contextMenu.Actions().Add(t.mStatus)

	// 分隔符
	t.contextMenu.Actions().Add(walk.NewSeparatorAction())

	// 连接菜单
	t.connectionMenu, _ = walk.NewMenu()
	t.mConnection = walk.NewMenuAction(t.connectionMenu)
	t.mConnection.SetText("连接")
	t.contextMenu.Actions().Add(t.mConnection)

	// 分隔符
	t.contextMenu.Actions().Add(walk.NewSeparatorAction())

	// 关于子菜单
	aboutMenu, _ := walk.NewMenu()
	aboutAction := walk.NewMenuAction(aboutMenu)
	aboutAction.SetText("关于")
	t.contextMenu.Actions().Add(aboutAction)

	// GitHub 仓库
	githubAction := walk.NewAction()
	githubAction.SetText("GitHub 仓库")
	githubAction.Triggered().Attach(func() {
		exec.Command("cmd", "/c", "start", "https://github.com/xbghc/claude-status").Start()
	})
	aboutMenu.Actions().Add(githubAction)

	// 日志文件
	logAction := walk.NewAction()
	logPath := logger.GetLogPath()
	logAction.SetText("日志: " + logPath)
	logAction.Triggered().Attach(func() {
		exec.Command("explorer", "/select,", logPath).Start()
	})
	aboutMenu.Actions().Add(logAction)

	// 分隔符
	t.contextMenu.Actions().Add(walk.NewSeparatorAction())

	// 退出菜单项
	quitAction := walk.NewAction()
	quitAction.SetText("退出")
	quitAction.Triggered().Attach(func() {
		t.stopAnimation() // 先停止动画，等待 goroutine 退出
		close(t.quitCh)
		walk.App().Exit(0)
	})
	t.contextMenu.Actions().Add(quitAction)
}

// startHoverDetection 启动悬浮检测
func (t *App) startHoverDetection() {
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				t.checkHover()
			case <-t.quitCh:
				return
			}
		}
	}()
}

// checkHover 检查鼠标是否悬浮在托盘图标上
func (t *App) checkHover() {
	// 未连接时不显示弹窗，由 tooltip 显示状态
	if t.connectedServer == "" {
		return
	}

	iconRect := t.getTrayIconRect()
	if iconRect == nil {
		return
	}

	var pt win.POINT
	win.GetCursorPos(&pt)

	// 检查鼠标是否在图标范围内
	isInIcon := pt.X >= iconRect.Left && pt.X <= iconRect.Right &&
		pt.Y >= iconRect.Top && pt.Y <= iconRect.Bottom

	// 检查是否在弹窗范围内
	isInPopup := false
	if t.popupWindow != nil && t.popupWindow.IsVisible() {
		isInPopup = t.popupWindow.IsHovered()
	}

	t.hoverMu.Lock()
	wasHovering := t.isHovering
	t.isHovering = isInIcon || isInPopup
	t.hoverMu.Unlock()

	if isInIcon && !wasHovering {
		t.showPopupAtTrayIcon(iconRect)
	} else if !isInIcon && !isInPopup && wasHovering {
		if t.popupWindow != nil {
			t.popupWindow.ScheduleHide(300 * time.Millisecond)
		}
	}
}

// getTrayIconRect 获取托盘图标位置
func (t *App) getTrayIconRect() *win.RECT {
	type NOTIFYICONIDENTIFIER struct {
		CbSize   uint32
		HWnd     win.HWND
		UID      uint32
		GuidItem [16]byte
	}

	shell32 := syscall.NewLazyDLL("shell32.dll")
	getRect := shell32.NewProc("Shell_NotifyIconGetRect")

	nii := NOTIFYICONIDENTIFIER{
		CbSize: uint32(unsafe.Sizeof(NOTIFYICONIDENTIFIER{})),
		HWnd:   t.mainWindow.Handle(),
		UID:    0,
	}

	var rect win.RECT
	ret, _, _ := getRect.Call(
		uintptr(unsafe.Pointer(&nii)),
		uintptr(unsafe.Pointer(&rect)),
	)

	if ret != 0 {
		return t.getTrayIconRectFallback()
	}
	return &rect
}

// getTrayDPI 获取系统托盘的 DPI (Windows 11)
func (t *App) getTrayDPI() int {
	shellTray := win.FindWindow(syscall.StringToUTF16Ptr("Shell_TrayWnd"), nil)
	if shellTray == 0 {
		return 96
	}

	user32 := syscall.NewLazyDLL("user32.dll")
	getDpiForWindow := user32.NewProc("GetDpiForWindow")
	dpi, _, _ := getDpiForWindow.Call(uintptr(shellTray))
	if dpi == 0 {
		return 96
	}
	return int(dpi)
}

// getTrayIconRectFallback 备用方案：获取任务栏位置
func (t *App) getTrayIconRectFallback() *win.RECT {
	// 找到任务栏窗口
	shellTray := win.FindWindow(
		syscall.StringToUTF16Ptr("Shell_TrayWnd"),
		nil,
	)
	if shellTray == 0 {
		return nil
	}

	var rect win.RECT
	if !win.GetWindowRect(shellTray, &rect) {
		return nil
	}

	// 估算托盘图标位置（通常在任务栏右侧）
	return &win.RECT{
		Left:   rect.Right - 100,
		Top:    rect.Top,
		Right:  rect.Right - 50,
		Bottom: rect.Bottom,
	}
}

// showPopupAtTrayIcon 在托盘图标位置显示悬浮窗口
func (t *App) showPopupAtTrayIcon(iconRect *win.RECT) {
	if t.popupWindow == nil {
		return
	}

	x := int((iconRect.Left + iconRect.Right) / 2)
	y := int(iconRect.Top)

	t.popupWindow.UpdateSessions(t.statuses)
	t.popupWindow.ShowAt(x, y)
}

// togglePopup 切换悬浮窗口显示状态
func (t *App) togglePopup() {
	// 未连接时不显示弹窗
	if t.connectedServer == "" || t.popupWindow == nil {
		return
	}

	if t.popupWindow.IsVisible() {
		t.popupWindow.Hide()
	} else {
		iconRect := t.getTrayIconRect()
		if iconRect != nil {
			t.showPopupAtTrayIcon(iconRect)
		}
	}
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

	t.animWg.Add(1)
	go func() {
		defer t.animWg.Done()
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
				t.setIconData(frames[frame])
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
	if !t.animRunning {
		t.animMu.Unlock()
		return
	}

	t.animRunning = false
	if t.animStopCh != nil {
		close(t.animStopCh)
		t.animStopCh = nil
	}
	t.animMu.Unlock()

	// 等待动画 goroutine 退出
	t.animWg.Wait()
}

// setIconData 设置图标数据
func (t *App) setIconData(data []byte) {
	// 检查是否正在退出
	select {
	case <-t.quitCh:
		return
	default:
	}

	// 使用数据地址作为缓存 key（动画帧是固定数组）
	cacheKey := fmt.Sprintf("%p", &data[0])

	t.iconCacheMu.Lock()
	icon, cached := t.iconCache[cacheKey]
	if !cached {
		var err error
		icon, err = createIconFromICO(data)
		if err != nil {
			t.iconCacheMu.Unlock()
			logger.Error("Failed to create icon: %v", err)
			return
		}
		t.iconCache[cacheKey] = icon
	}
	t.iconCacheMu.Unlock()

	// 必须在主线程执行 GUI 操作
	t.mainWindow.Synchronize(func() {
		// 再次检查是否正在退出
		select {
		case <-t.quitCh:
			return
		default:
		}

		// 使用缓存的图标，不释放（会被复用）
		if err := t.notifyIcon.SetIcon(icon); err != nil {
			logger.Error("Failed to set icon: %v", err)
		}
	})
}

// SetIcon 设置图标
func (t *App) SetIcon(name string) {
	if t.currentIcon == name {
		return
	}
	logger.Info("SetIcon: %s -> %s (isDark=%v)", t.currentIcon, name, t.isDarkMode)
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
			t.setIconData(icons.DisconnectedDark)
		} else {
			t.setIconData(icons.DisconnectedLight)
		}
	case "input-needed":
		t.stopAnimation()
		if t.isDarkMode {
			t.setIconData(icons.InputNeededDark)
		} else {
			t.setIconData(icons.InputNeededLight)
		}
	case "running":
		t.startAnimation()
	}
}

// SetServers 设置预设服务器列表
func (t *App) SetServers(servers []config.ServerConfig) {
	t.servers = servers

	if len(servers) == 0 {
		return
	}

	t.mStatus.SetText("请选择服务器")
	t.notifyIcon.SetToolTip("Claude Code Status - 请选择服务器")

	// 创建服务器子菜单项
	t.serverMenuItems = make([]*serverMenuItem, len(servers))
	for i, server := range servers {
		item := &serverMenuItem{server: server}

		// 创建服务器子菜单
		item.subMenu, _ = walk.NewMenu()
		item.menuItem = walk.NewMenuAction(item.subMenu)
		item.menuItem.SetText(server.Name)
		t.connectionMenu.Actions().Add(item.menuItem)

		// 添加操作子菜单
		item.mConnect = walk.NewAction()
		item.mConnect.SetText("连接")
		// 使用 item.server 避免闭包捕获循环变量问题
		item.mConnect.Triggered().Attach(func() {
			t.mStatus.SetText("正在连接...")
			select {
			case t.serverSelectCh <- item.server:
			default:
			}
		})
		item.subMenu.Actions().Add(item.mConnect)

		item.mDisconnect = walk.NewAction()
		item.mDisconnect.SetText("断开连接")
		item.mDisconnect.SetVisible(false)
		item.mDisconnect.Triggered().Attach(func() {
			t.mStatus.SetText("正在断开...")
			select {
			case t.disconnectCh <- struct{}{}:
			default:
			}
		})
		item.subMenu.Actions().Add(item.mDisconnect)

		t.serverMenuItems[i] = item
	}
}

// updateServerMenus 更新服务器菜单状态
func (t *App) updateServerMenus() {
	for _, item := range t.serverMenuItems {
		if item.server.Name == t.connectedServer {
			item.menuItem.SetText("✓ " + item.server.Name + " (已连接)")
			item.mConnect.SetText("重新连接")
			item.mDisconnect.SetVisible(true)
		} else {
			item.menuItem.SetText(item.server.Name)
			item.mConnect.SetText("连接")
			item.mDisconnect.SetVisible(false)
		}
	}
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
	t.SetIcon("disconnected")
	t.notifyIcon.SetToolTip("Claude Code Status - 正在连接...")
	t.mStatus.SetText("正在连接 - " + msg)
}

// SetConnected 设置已连接状态
func (t *App) SetConnected(msg string) {
	t.SetIcon("input-needed")
	t.notifyIcon.SetToolTip("") // 已连接时不显示 tooltip，使用悬浮卡片
	t.mStatus.SetText("已连接 - " + msg)
	t.connectedServer = msg
	t.updateServerMenus()
}

// SetDisconnected 设置用户主动断开状态
func (t *App) SetDisconnected() {
	t.SetIcon("disconnected")
	t.mStatus.SetText("已断开连接")
	t.notifyIcon.SetToolTip("Claude Code Status - 已断开连接")
	t.connectedServer = ""
	t.updateServerMenus()
}

// SetError 设置错误状态
func (t *App) SetError(errType string, msg string) {
	t.SetIcon("disconnected")

	var statusMsg string
	switch errType {
	case "not_configured":
		statusMsg = "服务端未配置"
	case "connection_failed":
		statusMsg = "连接失败"
	case "session_error":
		statusMsg = "会话错误"
	case "no_config":
		statusMsg = "未配置"
	case "version_check_timeout":
		statusMsg = "版本检查超时"
	default:
		statusMsg = msg
	}

	t.notifyIcon.SetToolTip("Claude Code Status - " + statusMsg)
	t.mStatus.SetText(statusMsg)
}

// ShowServerSelection 显示服务器选择提示
func (t *App) ShowServerSelection() {
	t.SetIcon("disconnected")
	t.mStatus.SetText("请选择服务器")
	t.notifyIcon.SetToolTip("Claude Code Status - 请选择服务器")
}

// SetStatusText 设置状态菜单项文本
func (t *App) SetStatusText(text string) {
	t.mStatus.SetText(text)
}

// SetTooltip 设置托盘图标的 tooltip 文本
func (t *App) SetTooltip(text string) {
	if text == "" {
		t.notifyIcon.SetToolTip("")
	} else {
		t.notifyIcon.SetToolTip("Claude Code Status - " + text)
	}
}

// UpdatePopup 更新悬浮窗口的会话状态
func (t *App) UpdatePopup(statuses []monitor.ProjectStatus) {
	t.statuses = statuses
	if t.popupWindow != nil {
		t.popupWindow.UpdateSessions(statuses)
	}
}
