//go:build windows

package popup

import (
	"sync"
	"syscall"
	"time"
	"unsafe"

	"claude-status/internal/monitor"

	"github.com/lxn/walk"
	"github.com/lxn/win"
)



// PopupWindow 悬浮窗口
type PopupWindow struct {
	window   *walk.MainWindow
	list     *SessionList
	statuses []monitor.ProjectStatus

	mu        sync.Mutex
	isVisible bool

	// 悬浮检测
	isHovered     bool
	hideTimer     *time.Timer
	hideTimerLock sync.Mutex
}

// NewPopupWindow 创建悬浮窗口
func NewPopupWindow() (*PopupWindow, error) {
	// 初始化 DPI 缩放
	UpdateDPIScaling()

	pw := &PopupWindow{}

	// 创建主窗口
	var err error
	pw.window, err = walk.NewMainWindow()
	if err != nil {
		return nil, err
	}

	// 设置窗口属性
	pw.window.SetName("ClaudeStatusPopup")
	pw.window.SetTitle("Claude Code Status")
	pw.window.SetSize(walk.Size{Width: Window.Width, Height: Window.MaxHeight})
	pw.window.SetVisible(false)

	// 应用弹出窗口样式
	pw.applyPopupStyle()

	// 使用 VBox 布局
	layout := walk.NewVBoxLayout()
	layout.SetMargins(walk.Margins{HNear: 0, VNear: 0, HFar: 0, VFar: 0})
	layout.SetSpacing(0)
	pw.window.SetLayout(layout)

	// 创建会话列表
	pw.list, err = NewSessionList(pw.window)
	if err != nil {
		pw.window.Dispose()
		return nil, err
	}

	// 监听窗口鼠标事件
	pw.setupMouseTracking()

	return pw, nil
}

// applyPopupStyle 应用弹出窗口样式
func (pw *PopupWindow) applyPopupStyle() {
	hwnd := pw.window.Handle()

	// 设置无边框弹出窗口样式
	const WS_POPUP = 0x80000000
	style := win.GetWindowLong(hwnd, win.GWL_STYLE)
	style &^= win.WS_CAPTION | win.WS_THICKFRAME | win.WS_BORDER | win.WS_SYSMENU
	// 使用 uint32 转换避免溢出问题
	win.SetWindowLong(hwnd, win.GWL_STYLE, int32(uint32(style)|WS_POPUP))

	// 设置扩展样式：工具窗口 + 置顶 + 不抢焦点
	exStyle := win.GetWindowLong(hwnd, win.GWL_EXSTYLE)
	exStyle |= win.WS_EX_TOOLWINDOW | win.WS_EX_TOPMOST | win.WS_EX_NOACTIVATE
	win.SetWindowLong(hwnd, win.GWL_EXSTYLE, exStyle)

	// 刷新窗口框架
	win.SetWindowPos(hwnd, win.HWND_TOPMOST, 0, 0, 0, 0,
		win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_FRAMECHANGED)

	// 应用圆角（Windows 11）
	pw.applyRoundedCorners(hwnd)
}

// applyRoundedCorners 应用圆角效果 (Windows 11)
func (pw *PopupWindow) applyRoundedCorners(hwnd win.HWND) {
	dwmapi := syscall.NewLazyDLL("dwmapi.dll")
	if dwmapi.Load() != nil {
		return
	}

	setWindowAttribute := dwmapi.NewProc("DwmSetWindowAttribute")
	if setWindowAttribute.Find() != nil {
		return
	}

	const DWMWA_WINDOW_CORNER_PREFERENCE = 33
	const DWMWCP_ROUND = 2

	preference := int32(DWMWCP_ROUND)
	setWindowAttribute.Call(
		uintptr(hwnd),
		DWMWA_WINDOW_CORNER_PREFERENCE,
		uintptr(unsafe.Pointer(&preference)),
		unsafe.Sizeof(preference),
	)
}

// setupMouseTracking 设置鼠标追踪
func (pw *PopupWindow) setupMouseTracking() {
	// 监听鼠标进入/离开窗口
	pw.window.MouseMove().Attach(func(x, y int, button walk.MouseButton) {
		pw.onMouseMove()
	})
}

// onMouseMove 鼠标移动到窗口上
func (pw *PopupWindow) onMouseMove() {
	pw.mu.Lock()
	pw.isHovered = true
	pw.mu.Unlock()

	pw.cancelHideTimer()
}

// ShowAt 在指定位置显示窗口
func (pw *PopupWindow) ShowAt(x, y int) {
	pw.mu.Lock()
	if pw.isVisible {
		pw.mu.Unlock()
		return
	}
	pw.isVisible = true
	statuses := pw.statuses
	pw.mu.Unlock()

	// 先更新内容（在显示之前）
	pw.list.Update(statuses)

	// 获取窗口高度（根据项目数量）
	windowHeight := pw.list.GetHeight()
	windowWidth := Window.Width

	// 设置 CustomWidget 大小（强制精确尺寸）
	pw.list.SetSize(windowWidth, windowHeight)

	// 调整位置，确保窗口在托盘图标上方
	adjustedX := x - windowWidth/2
	adjustedY := y - windowHeight - Window.Padding

	// 确保窗口不超出屏幕
	screenWidth := int(win.GetSystemMetrics(win.SM_CXSCREEN))
	screenHeight := int(win.GetSystemMetrics(win.SM_CYSCREEN))

	if adjustedX < 0 {
		adjustedX = 0
	}
	if adjustedX+windowWidth > screenWidth {
		adjustedX = screenWidth - windowWidth
	}
	if adjustedY < 0 {
		adjustedY = y + Window.Padding // 显示在图标下方
	}
	if adjustedY+windowHeight > screenHeight {
		adjustedY = screenHeight - windowHeight
	}

	// 设置窗口大小约束（固定尺寸）
	pw.window.SetMinMaxSize(
		walk.Size{Width: windowWidth, Height: windowHeight},
		walk.Size{Width: windowWidth, Height: windowHeight},
	)

	// 先设置窗口边界
	pw.window.SetBoundsPixels(walk.Rectangle{
		X: adjustedX, Y: adjustedY, Width: windowWidth, Height: windowHeight,
	})

	// 使用 SetWindowPos 确保置顶并显示
	win.SetWindowPos(pw.window.Handle(), win.HWND_TOPMOST,
		int32(adjustedX), int32(adjustedY), int32(windowWidth), int32(windowHeight),
		win.SWP_SHOWWINDOW|win.SWP_FRAMECHANGED)

	pw.window.SetVisible(true)
	pw.window.Invalidate()
}

// Hide 隐藏窗口
func (pw *PopupWindow) Hide() {
	pw.mu.Lock()
	if !pw.isVisible {
		pw.mu.Unlock()
		return
	}
	pw.isVisible = false
	pw.isHovered = false
	pw.mu.Unlock()

	pw.cancelHideTimer()
	pw.window.SetVisible(false)
}

// ScheduleHide 延迟隐藏（给用户移动鼠标的时间）
func (pw *PopupWindow) ScheduleHide(delay time.Duration) {
	pw.hideTimerLock.Lock()
	defer pw.hideTimerLock.Unlock()

	if pw.hideTimer != nil {
		pw.hideTimer.Stop()
	}

	pw.hideTimer = time.AfterFunc(delay, func() {
		// 使用 IsHovered() 检查鼠标是否在窗口上
		if !pw.IsHovered() {
			pw.Hide()
		}
	})
}

// cancelHideTimer 取消隐藏定时器
func (pw *PopupWindow) cancelHideTimer() {
	pw.hideTimerLock.Lock()
	defer pw.hideTimerLock.Unlock()

	if pw.hideTimer != nil {
		pw.hideTimer.Stop()
		pw.hideTimer = nil
	}
}

// UpdateSessions 更新会话列表
func (pw *PopupWindow) UpdateSessions(statuses []monitor.ProjectStatus) {
	pw.mu.Lock()
	defer pw.mu.Unlock()

	pw.statuses = statuses

	// 如果窗口可见，更新显示
	if pw.isVisible {
		pw.list.Update(statuses)
	}
}

// IsVisible 返回窗口是否可见
func (pw *PopupWindow) IsVisible() bool {
	pw.mu.Lock()
	defer pw.mu.Unlock()
	return pw.isVisible
}

// IsHovered 返回鼠标是否在窗口上（通过检查鼠标坐标）
func (pw *PopupWindow) IsHovered() bool {
	pw.mu.Lock()
	visible := pw.isVisible
	pw.mu.Unlock()

	if !visible {
		return false
	}

	// 获取窗口矩形
	var rect win.RECT
	if !win.GetWindowRect(pw.window.Handle(), &rect) {
		return false
	}

	// 获取鼠标位置
	var pt win.POINT
	win.GetCursorPos(&pt)

	// 检查鼠标是否在窗口范围内
	return pt.X >= rect.Left && pt.X <= rect.Right &&
		pt.Y >= rect.Top && pt.Y <= rect.Bottom
}

// SetHovered 设置悬浮状态
func (pw *PopupWindow) SetHovered(hovered bool) {
	pw.mu.Lock()
	pw.isHovered = hovered
	pw.mu.Unlock()
}

// Dispose 释放资源
func (pw *PopupWindow) Dispose() {
	pw.cancelHideTimer()
	if pw.window != nil {
		pw.window.Dispose()
	}
}

// SetDarkMode 设置深色/浅色模式
func (pw *PopupWindow) SetDarkMode(isDark bool) {
	if isDark {
		SetTheme(ThemeDark)
	} else {
		SetTheme(ThemeLight)
	}
	// 刷新窗口显示
	if pw.isVisible {
		pw.window.Invalidate()
	}
}
