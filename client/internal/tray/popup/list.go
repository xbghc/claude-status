//go:build windows

package popup

import (
	"fmt"

	"claude-status/internal/monitor"

	"github.com/lxn/walk"
)

// SessionItem 会话项
type SessionItem struct {
	ProjectName string
	IsWorking   bool
}

// SessionList 会话列表（极简风格）
type SessionList struct {
	widget *walk.CustomWidget
	items  []*SessionItem
}

// NewSessionList 创建会话列表
func NewSessionList(parent walk.Container) (*SessionList, error) {
	sl := &SessionList{
		items: make([]*SessionItem, 0),
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

	// 绘制列表项
	for i, item := range sl.items {
		sl.paintItem(canvas, item, i)
	}

	// 空状态
	if len(sl.items) == 0 {
		sl.paintEmpty(canvas, bounds)
	}

	return nil
}

// paintItem 绘制单个列表项
func (sl *SessionList) paintItem(canvas *walk.Canvas, item *SessionItem, index int) {
	y := Window.Padding + index*Item.Height

	// 状态颜色
	var dotColor walk.Color
	if item.IsWorking {
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

	// 绘制项目名称
	font, err := walk.NewFont(Fonts.Primary, Fonts.Size, 0)
	if err != nil {
		return
	}
	defer font.Dispose()

	textX := dotX + Item.DotSize + Item.DotMargin
	textRect := walk.Rectangle{
		X:      textX,
		Y:      y,
		Width:  Window.Width - textX - Window.Padding,
		Height: Item.Height,
	}
	canvas.DrawTextPixels(item.ProjectName, font, Colors.TextPrimary, textRect,
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

	// 统计每个项目的会话数量
	projectCount := make(map[string]int)
	for _, s := range filtered {
		projectCount[s.Project]++
	}

	// 构建列表项
	projectIndex := make(map[string]int)
	items := make([]*SessionItem, 0, len(filtered))

	for _, s := range filtered {
		displayName := s.ProjectName
		if projectCount[s.Project] > 1 {
			projectIndex[s.Project]++
			displayName = fmt.Sprintf("%s #%d", s.ProjectName, projectIndex[s.Project])
		}

		items = append(items, &SessionItem{
			ProjectName: displayName,
			IsWorking:   s.Status == "working",
		})
	}

	sl.items = items
	sl.widget.Invalidate()
}

// GetItemCount 获取项目数量
func (sl *SessionList) GetItemCount() int {
	return len(sl.items)
}

// GetHeight 获取列表高度
func (sl *SessionList) GetHeight() int {
	return CalcWindowHeight(len(sl.items))
}

// SetSize 设置控件大小
func (sl *SessionList) SetSize(width, height int) {
	sl.widget.SetMinMaxSizePixels(
		walk.Size{Width: width, Height: height},
		walk.Size{Width: width, Height: height},
	)
}
