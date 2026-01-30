//go:build windows

package popup

import (
	"syscall"

	"github.com/lxn/walk"
)

// ============================================================
// Windows 11 风格极简设计
// ============================================================

// Theme 主题类型
type Theme int

const (
	ThemeDark  Theme = iota // 深色主题
	ThemeLight              // 浅色主题
)

// 当前主题
var CurrentTheme Theme = ThemeDark

// ============================================================
// DPI 缩放
// ============================================================

const BaseDPI = 96

var currentDPI = 96

func InitDPI() {
	user32 := syscall.NewLazyDLL("user32.dll")
	getDPI := user32.NewProc("GetDpiForSystem")
	if getDPI.Find() == nil {
		dpi, _, _ := getDPI.Call()
		if dpi > 0 {
			currentDPI = int(dpi)
		}
	}
}

func Scale(value int) int {
	return value * currentDPI / BaseDPI
}

// ============================================================
// 窗口配置
// ============================================================

var baseWindow = struct {
	Width     int
	Padding   int
	MaxHeight int
}{
	Width:     180,
	Padding:   8,
	MaxHeight: 400,
}

var Window struct {
	Width     int
	Padding   int
	MaxHeight int
}

// ============================================================
// 列表项配置
// ============================================================

var baseItem = struct {
	Height    int
	DotSize   int
	DotMargin int
}{
	Height:    36,
	DotSize:   8,
	DotMargin: 8,
}

var Item struct {
	Height    int
	DotSize   int
	DotMargin int
}

// ============================================================
// 字体配置
// ============================================================

var Fonts = struct {
	Primary string
	Size    int
}{
	Primary: "Segoe UI",
	Size:    11,
}

// ============================================================
// 颜色配置
// ============================================================

type ColorScheme struct {
	// 背景
	Background walk.Color

	// 文字
	TextPrimary walk.Color
	TextMuted   walk.Color

	// 状态
	Working walk.Color
	Idle    walk.Color
}

var darkColors = ColorScheme{
	Background:  walk.RGB(32, 32, 36),
	TextPrimary: walk.RGB(245, 245, 247),
	TextMuted:   walk.RGB(120, 120, 130),
	Working:     walk.RGB(52, 211, 153),  // #34D399
	Idle:        walk.RGB(251, 191, 36),  // #FBBF24
}

var lightColors = ColorScheme{
	Background:  walk.RGB(250, 250, 252),
	TextPrimary: walk.RGB(28, 28, 32),
	TextMuted:   walk.RGB(140, 140, 150),
	Working:     walk.RGB(34, 197, 94),
	Idle:        walk.RGB(234, 179, 8),
}

var Colors = darkColors

func SetTheme(theme Theme) {
	CurrentTheme = theme
	if theme == ThemeLight {
		Colors = lightColors
	} else {
		Colors = darkColors
	}
}

// ============================================================
// 初始化
// ============================================================

func UpdateDPIScaling() {
	InitDPI()

	Window.Width = Scale(baseWindow.Width)
	Window.Padding = Scale(baseWindow.Padding)
	Window.MaxHeight = Scale(baseWindow.MaxHeight)

	Item.Height = Scale(baseItem.Height)
	Item.DotSize = Scale(baseItem.DotSize)
	Item.DotMargin = Scale(baseItem.DotMargin)
}

func CalcWindowHeight(itemCount int) int {
	if itemCount == 0 {
		return Item.Height + Window.Padding*2
	}

	contentHeight := itemCount*Item.Height + Window.Padding*2
	if contentHeight > Window.MaxHeight {
		return Window.MaxHeight
	}
	return contentHeight
}
