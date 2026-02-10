package version

// Version 是客户端和脚本的协议版本
// 当以下内容变更时需要递增版本号：
// - monitor.sh 脚本逻辑
// - status-hook.sh 脚本逻辑
// - install-remote.sh Hook 配置
// - 通信协议（StatusMessage 结构）
const Version = "1.1.1"
