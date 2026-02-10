//go:build windows

package popup

import (
	"fmt"

	"claude-status/internal/monitor"

	"github.com/lxn/walk"
)

// ProjectGroup 按项目分组的会话统计
type ProjectGroup struct {
	ProjectName string // 项目显示名
	Running     int    // 正在运行（working）的会话数
	Total       int    // 总会话数
}

// SessionList 会话列表（极简风格）
type SessionList struct {
	widget *walk.CustomWidget
	groups []*ProjectGroup
}

// NewSessionList 创建会话列表
func NewSessionList(parent walk.Container) (*SessionList, error) {
	sl := &SessionList{
		groups: make([]*ProjectGroup, 0),
	}

	var err error
	sl.widget, err = walk.NewCustomWidget(parent, 0, sl.paint)
	if err != nil {
		return nil, err
	}

	sl.widget.SetMinMaxSize(
		walk.Size{Width: Window.Width, Height: Item.Height},
		walk.Size{Width: Window.Width, Height: Window.MaxHeight},
	)

	return sl, nil
}

// paint 绘制列表
func (sl *SessionList) paint(canvas *walk.Canvas, updateBounds walk.Rectangle) error {
	bounds := sl.widget.ClientBoundsPixels()

	// 绘制背景
	bgBrush, err := walk.NewSolidColorBrush(Colors.Background)
	if err != nil {
		return err
	}
	defer bgBrush.Dispose()
	canvas.FillRectanglePixels(bgBrush, bounds)

	// 绘制分组列表项
	for i, group := range sl.groups {
		sl.paintGroup(canvas, group, i)
	}

	// 空状态
	if len(sl.groups) == 0 {
		sl.paintEmpty(canvas, bounds)
	}

	return nil
}

// paintGroup 绘制单个分组项
func (sl *SessionList) paintGroup(canvas *walk.Canvas, group *ProjectGroup, index int) {
	y := Window.Padding + index*Item.Height

	// 状态颜色：有 running 会话则绿色，否则黄色
	var dotColor walk.Color
	if group.Running > 0 {
		dotColor = Colors.Working
	} else {
		dotColor = Colors.Idle
	}

	// 绘制状态圆点
	dotBrush, err := walk.NewSolidColorBrush(dotColor)
	if err != nil {
		return
	}
	defer dotBrush.Dispose()

	dotX := Window.Padding + Item.DotMargin
	dotY := y + (Item.Height-Item.DotSize)/2
	canvas.FillEllipsePixels(dotBrush, walk.Rectangle{
		X: dotX, Y: dotY, Width: Item.DotSize, Height: Item.DotSize,
	})

	// 字体
	font, err := walk.NewFont(Fonts.Primary, Fonts.Size, 0)
	if err != nil {
		return
	}
	defer font.Dispose()

	// 右侧计数文本 "a/b"
	countText := fmt.Sprintf("%d/%d", group.Running, group.Total)
	countWidth := Scale(36)
	countRect := walk.Rectangle{
		X:      Window.Width - Window.Padding - countWidth,
		Y:      y,
		Width:  countWidth,
		Height: Item.Height,
	}
	canvas.DrawTextPixels(countText, font, Colors.TextMuted, countRect,
		walk.TextRight|walk.TextVCenter|walk.TextSingleLine)

	// 左侧项目名称
	textX := dotX + Item.DotSize + Item.DotMargin
	textRect := walk.Rectangle{
		X:      textX,
		Y:      y,
		Width:  countRect.X - textX - Item.DotMargin,
		Height: Item.Height,
	}
	canvas.DrawTextPixels(group.ProjectName, font, Colors.TextPrimary, textRect,
		walk.TextLeft|walk.TextVCenter|walk.TextSingleLine|walk.TextEndEllipsis)
}

// paintEmpty 绘制空状态
func (sl *SessionList) paintEmpty(canvas *walk.Canvas, bounds walk.Rectangle) {
	font, err := walk.NewFont(Fonts.Primary, Fonts.Size, 0)
	if err != nil {
		return
	}
	defer font.Dispose()

	canvas.DrawTextPixels("无活动会话", font, Colors.TextMuted, bounds,
		walk.TextCenter|walk.TextVCenter|walk.TextSingleLine)
}

// Update 更新会话列表
func (sl *SessionList) Update(statuses []monitor.ProjectStatus) {
	// 过滤 stopped 状态
	filtered := make([]monitor.ProjectStatus, 0, len(statuses))
	for _, s := range statuses {
		if s.Status != "stopped" {
			filtered = append(filtered, s)
		}
	}

	// 按项目目录分组统计
	type groupStats struct {
		name    string
		running int
		total   int
	}
	groupMap := make(map[string]*groupStats)
	var groupOrder []string

	for _, s := range filtered {
		g, exists := groupMap[s.Project]
		if !exists {
			g = &groupStats{name: s.ProjectName}
			groupMap[s.Project] = g
			groupOrder = append(groupOrder, s.Project)
		}
		g.total++
		if s.Status == "working" {
			g.running++
		}
	}

	// 按出现顺序构建分组列表
	groups := make([]*ProjectGroup, 0, len(groupOrder))
	for _, project := range groupOrder {
		g := groupMap[project]
		groups = append(groups, &ProjectGroup{
			ProjectName: g.name,
			Running:     g.running,
			Total:       g.total,
		})
	}

	sl.groups = groups
	sl.widget.Invalidate()
}

// GetItemCount 获取分组数量
func (sl *SessionList) GetItemCount() int {
	return len(sl.groups)
}

// GetHeight 获取列表高度
func (sl *SessionList) GetHeight() int {
	return CalcWindowHeight(len(sl.groups))
}

// SetSize 设置控件大小
func (sl *SessionList) SetSize(width, height int) {
	sl.widget.SetMinMaxSizePixels(
		walk.Size{Width: width, Height: height},
		walk.Size{Width: width, Height: height},
	)
}
