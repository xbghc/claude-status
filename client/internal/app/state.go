package app

import (
	"claude-status/internal/logger"
)

// State 应用状态
type State int

const (
	StateUnconfigured State = iota // 无配置，等待用户选择服务器
	StateConnecting                // 正在连接服务器
	StateInstalling                // 首次安装服务端组件
	StateReinstalling              // 版本不匹配，更新服务端组件
	StateConnected                 // 已连接，监控状态中
	StateDisconnected              // 用户主动断开
	StateError                     // 连接失败、会话错误等
	StateQuitting                  // 应用退出
)

// String 返回状态的可读名称
func (s State) String() string {
	switch s {
	case StateUnconfigured:
		return "Unconfigured"
	case StateConnecting:
		return "Connecting"
	case StateInstalling:
		return "Installing"
	case StateReinstalling:
		return "Reinstalling"
	case StateConnected:
		return "Connected"
	case StateDisconnected:
		return "Disconnected"
	case StateError:
		return "Error"
	case StateQuitting:
		return "Quitting"
	default:
		return "Unknown"
	}
}

// Event 状态机事件
type Event int

const (
	EventConfigLoaded    Event = iota // 配置文件加载成功
	EventServerSelected              // 用户选择了服务器
	EventConnectSuccess              // 连接成功，会话已启动
	EventConnectFailed               // 连接失败
	EventVersionMismatch             // 服务端版本不匹配
	EventNotConfigured               // 服务端未安装
	EventInstallSuccess              // 安装/更新完成
	EventInstallFailed               // 安装/更新失败
	EventSessionError                // 活动会话出错
	EventSessionClosed               // 连接断开
	EventStatusUpdate                // 状态数据更新
	EventUserDisconnect              // 用户主动断开
	EventUserQuit                    // 用户退出或收到系统信号
	EventSwitchServer                // 用户切换服务器
)

// String 返回事件的可读名称
func (e Event) String() string {
	switch e {
	case EventConfigLoaded:
		return "ConfigLoaded"
	case EventServerSelected:
		return "ServerSelected"
	case EventConnectSuccess:
		return "ConnectSuccess"
	case EventConnectFailed:
		return "ConnectFailed"
	case EventVersionMismatch:
		return "VersionMismatch"
	case EventNotConfigured:
		return "NotConfigured"
	case EventInstallSuccess:
		return "InstallSuccess"
	case EventInstallFailed:
		return "InstallFailed"
	case EventSessionError:
		return "SessionError"
	case EventSessionClosed:
		return "SessionClosed"
	case EventStatusUpdate:
		return "StatusUpdate"
	case EventUserDisconnect:
		return "UserDisconnect"
	case EventUserQuit:
		return "UserQuit"
	case EventSwitchServer:
		return "SwitchServer"
	default:
		return "Unknown"
	}
}

// transitionRule 定义单条转换规则
type transitionRule struct {
	From  State
	Event Event
	To    State
}

// transitionRules 完整的状态转换表
var transitionRules = []transitionRule{
	// From Unconfigured
	{StateUnconfigured, EventConfigLoaded, StateConnecting},
	{StateUnconfigured, EventServerSelected, StateConnecting},
	{StateUnconfigured, EventUserQuit, StateQuitting},

	// From Connecting
	{StateConnecting, EventConnectSuccess, StateConnected},
	{StateConnecting, EventConnectFailed, StateError},
	{StateConnecting, EventVersionMismatch, StateReinstalling},
	{StateConnecting, EventNotConfigured, StateInstalling},
	{StateConnecting, EventSwitchServer, StateConnecting},
	{StateConnecting, EventUserQuit, StateQuitting},

	// From Installing
	{StateInstalling, EventInstallSuccess, StateConnecting},
	{StateInstalling, EventInstallFailed, StateError},
	{StateInstalling, EventUserQuit, StateQuitting},

	// From Reinstalling
	{StateReinstalling, EventInstallSuccess, StateConnecting},
	{StateReinstalling, EventInstallFailed, StateError},
	{StateReinstalling, EventUserQuit, StateQuitting},

	// From Connected
	{StateConnected, EventStatusUpdate, StateConnected},
	{StateConnected, EventSessionError, StateError},
	{StateConnected, EventSessionClosed, StateError},
	{StateConnected, EventUserDisconnect, StateDisconnected},
	{StateConnected, EventSwitchServer, StateConnecting},
	{StateConnected, EventUserQuit, StateQuitting},

	// From Disconnected
	{StateDisconnected, EventServerSelected, StateConnecting},
	{StateDisconnected, EventUserQuit, StateQuitting},

	// From Error
	{StateError, EventServerSelected, StateConnecting},
	{StateError, EventUserQuit, StateQuitting},
}

// StateChange 状态转换结果
type StateChange struct {
	From  State
	To    State
	Event Event
	Valid bool // false 表示转换被拒绝
}

// StateMachine 应用状态机
type StateMachine struct {
	current      State
	transitions  map[State]map[Event]State
	onTransition func(change StateChange)
}

// NewStateMachine 创建状态机
func NewStateMachine(initial State, onTransition func(change StateChange)) *StateMachine {
	lookup := make(map[State]map[Event]State)
	for _, r := range transitionRules {
		if lookup[r.From] == nil {
			lookup[r.From] = make(map[Event]State)
		}
		lookup[r.From][r.Event] = r.To
	}

	return &StateMachine{
		current:      initial,
		transitions:  lookup,
		onTransition: onTransition,
	}
}

// Current 返回当前状态
func (sm *StateMachine) Current() State {
	return sm.current
}

// Transition 触发状态转换
func (sm *StateMachine) Transition(event Event) StateChange {
	eventMap, ok := sm.transitions[sm.current]
	if !ok {
		change := StateChange{From: sm.current, Event: event, Valid: false}
		logger.Error("State machine: no transitions for state %s", sm.current)
		return change
	}

	next, ok := eventMap[event]
	if !ok {
		change := StateChange{From: sm.current, Event: event, Valid: false}
		logger.Error("State machine: invalid transition %s + %s", sm.current, event)
		return change
	}

	change := StateChange{From: sm.current, To: next, Event: event, Valid: true}
	sm.current = next

	if sm.onTransition != nil {
		sm.onTransition(change)
	}

	return change
}
