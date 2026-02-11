package app

import (
	"testing"
)

func TestStateString(t *testing.T) {
	tests := []struct {
		state State
		want  string
	}{
		{StateUnconfigured, "Unconfigured"},
		{StateConnecting, "Connecting"},
		{StateInstalling, "Installing"},
		{StateReinstalling, "Reinstalling"},
		{StateConnected, "Connected"},
		{StateDisconnected, "Disconnected"},
		{StateError, "Error"},
		{StateQuitting, "Quitting"},
		{State(99), "Unknown"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("State(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestEventString(t *testing.T) {
	tests := []struct {
		event Event
		want  string
	}{
		{EventConfigLoaded, "ConfigLoaded"},
		{EventServerSelected, "ServerSelected"},
		{EventConnectSuccess, "ConnectSuccess"},
		{EventConnectFailed, "ConnectFailed"},
		{EventVersionMismatch, "VersionMismatch"},
		{EventNotConfigured, "NotConfigured"},
		{EventInstallSuccess, "InstallSuccess"},
		{EventInstallFailed, "InstallFailed"},
		{EventSessionError, "SessionError"},
		{EventSessionClosed, "SessionClosed"},
		{EventStatusUpdate, "StatusUpdate"},
		{EventUserDisconnect, "UserDisconnect"},
		{EventUserQuit, "UserQuit"},
		{EventSwitchServer, "SwitchServer"},
		{Event(99), "Unknown"},
	}

	for _, tt := range tests {
		if got := tt.event.String(); got != tt.want {
			t.Errorf("Event(%d).String() = %q, want %q", tt.event, got, tt.want)
		}
	}
}

func TestNewStateMachine(t *testing.T) {
	sm := NewStateMachine(StateUnconfigured, nil)
	if sm.Current() != StateUnconfigured {
		t.Errorf("initial state = %v, want %v", sm.Current(), StateUnconfigured)
	}

	sm2 := NewStateMachine(StateConnecting, nil)
	if sm2.Current() != StateConnecting {
		t.Errorf("initial state = %v, want %v", sm2.Current(), StateConnecting)
	}
}

func TestValidTransitions(t *testing.T) {
	// 测试所有转换表中定义的有效转换
	for _, rule := range transitionRules {
		sm := NewStateMachine(rule.From, nil)
		change := sm.Transition(rule.Event)

		if !change.Valid {
			t.Errorf("transition %s + %s should be valid", rule.From, rule.Event)
		}
		if change.From != rule.From {
			t.Errorf("transition %s + %s: From = %s, want %s", rule.From, rule.Event, change.From, rule.From)
		}
		if change.To != rule.To {
			t.Errorf("transition %s + %s: To = %s, want %s", rule.From, rule.Event, change.To, rule.To)
		}
		if change.Event != rule.Event {
			t.Errorf("transition %s + %s: Event = %s, want %s", rule.From, rule.Event, change.Event, rule.Event)
		}
		if sm.Current() != rule.To {
			t.Errorf("after %s + %s: Current() = %s, want %s", rule.From, rule.Event, sm.Current(), rule.To)
		}
	}
}

func TestInvalidTransitions(t *testing.T) {
	tests := []struct {
		from  State
		event Event
	}{
		// Unconfigured 不能接收 ConnectSuccess
		{StateUnconfigured, EventConnectSuccess},
		// Connected 不能接收 ConfigLoaded
		{StateConnected, EventConfigLoaded},
		// Error 不能接收 StatusUpdate
		{StateError, EventStatusUpdate},
		// Disconnected 不能接收 ConnectFailed
		{StateDisconnected, EventConnectFailed},
		// Installing 不能接收 SessionError
		{StateInstalling, EventSessionError},
		// Quitting 不能接收任何事件
		{StateQuitting, EventServerSelected},
	}

	for _, tt := range tests {
		sm := NewStateMachine(tt.from, nil)
		change := sm.Transition(tt.event)

		if change.Valid {
			t.Errorf("transition %s + %s should be invalid", tt.from, tt.event)
		}
		// 无效转换不应改变状态
		if sm.Current() != tt.from {
			t.Errorf("after invalid %s + %s: state changed to %s, should remain %s",
				tt.from, tt.event, sm.Current(), tt.from)
		}
	}
}

func TestTransitionCallback(t *testing.T) {
	var captured []StateChange

	sm := NewStateMachine(StateUnconfigured, func(change StateChange) {
		captured = append(captured, change)
	})

	sm.Transition(EventServerSelected)
	sm.Transition(EventConnectSuccess)
	sm.Transition(EventUserDisconnect)

	if len(captured) != 3 {
		t.Fatalf("expected 3 callbacks, got %d", len(captured))
	}

	// 验证第一次转换
	if captured[0].From != StateUnconfigured || captured[0].To != StateConnecting {
		t.Errorf("callback[0]: %s -> %s, want Unconfigured -> Connecting",
			captured[0].From, captured[0].To)
	}

	// 验证第二次转换
	if captured[1].From != StateConnecting || captured[1].To != StateConnected {
		t.Errorf("callback[1]: %s -> %s, want Connecting -> Connected",
			captured[1].From, captured[1].To)
	}

	// 验证第三次转换
	if captured[2].From != StateConnected || captured[2].To != StateDisconnected {
		t.Errorf("callback[2]: %s -> %s, want Connected -> Disconnected",
			captured[2].From, captured[2].To)
	}
}

func TestInvalidTransitionNoCallback(t *testing.T) {
	callCount := 0
	sm := NewStateMachine(StateUnconfigured, func(change StateChange) {
		callCount++
	})

	// 无效转换不应触发回调
	sm.Transition(EventConnectSuccess)
	if callCount != 0 {
		t.Errorf("callback called %d times for invalid transition, want 0", callCount)
	}
}

func TestSelfTransition(t *testing.T) {
	sm := NewStateMachine(StateConnected, nil)

	// StatusUpdate 是自循环：Connected -> Connected
	change := sm.Transition(EventStatusUpdate)
	if !change.Valid {
		t.Error("StatusUpdate in Connected should be valid")
	}
	if change.From != StateConnected || change.To != StateConnected {
		t.Errorf("StatusUpdate: %s -> %s, want Connected -> Connected", change.From, change.To)
	}
	if sm.Current() != StateConnected {
		t.Errorf("after StatusUpdate: Current() = %s, want Connected", sm.Current())
	}
}

func TestCompleteLifecycle(t *testing.T) {
	// 模拟完整的应用生命周期：启动 -> 连接 -> 工作 -> 断开 -> 重连 -> 退出
	sm := NewStateMachine(StateUnconfigured, nil)

	steps := []struct {
		event    Event
		expected State
	}{
		{EventServerSelected, StateConnecting},
		{EventConnectSuccess, StateConnected},
		{EventStatusUpdate, StateConnected},
		{EventStatusUpdate, StateConnected},
		{EventUserDisconnect, StateDisconnected},
		{EventServerSelected, StateConnecting},
		{EventConnectSuccess, StateConnected},
		{EventUserQuit, StateQuitting},
	}

	for i, step := range steps {
		change := sm.Transition(step.event)
		if !change.Valid {
			t.Fatalf("step %d: %s should be valid from %s", i, step.event, change.From)
		}
		if sm.Current() != step.expected {
			t.Fatalf("step %d: Current() = %s, want %s", i, sm.Current(), step.expected)
		}
	}
}

func TestInstallLifecycle(t *testing.T) {
	// 模拟：连接 -> 未配置 -> 安装 -> 重连 -> 成功
	sm := NewStateMachine(StateConnecting, nil)

	steps := []struct {
		event    Event
		expected State
	}{
		{EventNotConfigured, StateInstalling},
		{EventInstallSuccess, StateConnecting},
		{EventConnectSuccess, StateConnected},
	}

	for i, step := range steps {
		change := sm.Transition(step.event)
		if !change.Valid {
			t.Fatalf("step %d: %s should be valid from %s", i, step.event, change.From)
		}
		if sm.Current() != step.expected {
			t.Fatalf("step %d: Current() = %s, want %s", i, sm.Current(), step.expected)
		}
	}
}

func TestReinstallLifecycle(t *testing.T) {
	// 模拟：连接 -> 版本不匹配 -> 重新安装 -> 重连 -> 成功
	sm := NewStateMachine(StateConnecting, nil)

	steps := []struct {
		event    Event
		expected State
	}{
		{EventVersionMismatch, StateReinstalling},
		{EventInstallSuccess, StateConnecting},
		{EventConnectSuccess, StateConnected},
	}

	for i, step := range steps {
		change := sm.Transition(step.event)
		if !change.Valid {
			t.Fatalf("step %d: %s should be valid from %s", i, step.event, change.From)
		}
		if sm.Current() != step.expected {
			t.Fatalf("step %d: Current() = %s, want %s", i, sm.Current(), step.expected)
		}
	}
}

func TestInstallFailureLifecycle(t *testing.T) {
	// 模拟：连接 -> 未配置 -> 安装失败 -> 用户重选 -> 连接成功
	sm := NewStateMachine(StateConnecting, nil)

	steps := []struct {
		event    Event
		expected State
	}{
		{EventNotConfigured, StateInstalling},
		{EventInstallFailed, StateError},
		{EventServerSelected, StateConnecting},
		{EventConnectSuccess, StateConnected},
	}

	for i, step := range steps {
		change := sm.Transition(step.event)
		if !change.Valid {
			t.Fatalf("step %d: %s should be valid from %s", i, step.event, change.From)
		}
		if sm.Current() != step.expected {
			t.Fatalf("step %d: Current() = %s, want %s", i, sm.Current(), step.expected)
		}
	}
}

func TestSessionErrorRecovery(t *testing.T) {
	// 模拟：已连接 -> 会话错误 -> 用户重选 -> 重连
	sm := NewStateMachine(StateConnected, nil)

	steps := []struct {
		event    Event
		expected State
	}{
		{EventSessionError, StateError},
		{EventServerSelected, StateConnecting},
		{EventConnectSuccess, StateConnected},
	}

	for i, step := range steps {
		change := sm.Transition(step.event)
		if !change.Valid {
			t.Fatalf("step %d: %s should be valid from %s", i, step.event, change.From)
		}
		if sm.Current() != step.expected {
			t.Fatalf("step %d: Current() = %s, want %s", i, sm.Current(), step.expected)
		}
	}
}

func TestEveryStateCanQuit(t *testing.T) {
	// 除了 Quitting 自身，所有状态都应该能通过 UserQuit 退出
	states := []State{
		StateUnconfigured,
		StateConnecting,
		StateInstalling,
		StateReinstalling,
		StateConnected,
		StateDisconnected,
		StateError,
	}

	for _, state := range states {
		sm := NewStateMachine(state, nil)
		change := sm.Transition(EventUserQuit)
		if !change.Valid {
			t.Errorf("state %s should accept UserQuit", state)
		}
		if sm.Current() != StateQuitting {
			t.Errorf("state %s + UserQuit: got %s, want Quitting", state, sm.Current())
		}
	}
}
