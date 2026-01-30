package monitor

// 消息类型常量
const (
	MsgTypeStatus  = "status"
	MsgTypeError   = "error"
	MsgTypeVersion = "version"
)

// ProjectStatus 单个项目的状态
type ProjectStatus struct {
	Project     string `json:"project"`
	ProjectName string `json:"project_name"`
	SessionId   string `json:"session_id"`
	Status      string `json:"status"`
	UpdatedAt   int64  `json:"updated_at"`
}

// StatusMessage 状态消息
type StatusMessage struct {
	Type    string          `json:"type"`              // "status" | "error" | "version"
	Data    []ProjectStatus `json:"data,omitempty"`    // type=status 时使用
	Message string          `json:"message,omitempty"` // type=error 时使用
	Version string          `json:"version,omitempty"` // type=version 时使用
}

// Client 监控客户端接口
type Client interface {
	Connect() error
	Start() error
	Close()
	StatusChan() <-chan []ProjectStatus
	ErrorChan() <-chan error
	Done() <-chan struct{}
}

// Installer 安装器接口
type Installer interface {
	Connect() error
	Close()
	CheckDependencies() (bool, string)
	Install() error
}
