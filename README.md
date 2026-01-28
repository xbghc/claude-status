# Claude Code Status Monitor

Windows 系统托盘工具，通过 SSH 实时监控 Linux 服务器上 Claude Code 的运行状态。

<p align="center">
  <img src="client/assets/svg/icon-disconnected.svg" width="48" alt="disconnected" />
  <img src="client/assets/svg/icon-input-needed.svg" width="48" alt="idle" />
  <img src="client/assets/svg/icon-running.svg" width="48" alt="running" />
</p>

<p align="center">
  <em>断开连接 · 等待输入 · 正在运行</em>
</p>

## 功能特性

- **实时状态监控**：系统托盘图标动态显示 Claude Code 运行状态
- **多项目支持**：同时监控多个 Claude Code 实例
- **SSH 安全连接**：复用现有 `~/.ssh/config` 配置
- **事件驱动**：基于 Claude Code Hook，低延迟更新
- **自动清理**：会话结束后自动移除，支持超时清理

## 状态图标

| 图标 | 状态 | 说明 |
|:----:|------|------|
| <img src="client/assets/svg/icon-disconnected.svg" width="24" /> | 断开连接 | SSH 连接失败或未连接 |
| <img src="client/assets/svg/icon-input-needed.svg" width="24" /> | 等待输入 | Claude Code 等待用户操作 |
| <img src="client/assets/svg/icon-running.svg" width="24" /> | 正在运行 | Claude Code 正在执行任务（动画） |

## 架构

```
┌─────────────────────────────────────────────────────────────────┐
│                        Linux 服务器                              │
│  ┌─────────────┐     ┌──────────────┐     ┌─────────────────┐  │
│  │ Claude Code │────►│status-hook.sh│────►│~/.claude-status/│  │
│  │   (Hook)    │     │  更新状态文件  │     │   *.json        │  │
│  └─────────────┘     └──────────────┘     └────────┬────────┘  │
│                                                     │           │
│                                           ┌─────────▼────────┐  │
│                                           │   monitor.sh     │  │
│                                           │ (inotify 监听)   │  │
│                                           └─────────┬────────┘  │
└─────────────────────────────────────────────────────┼───────────┘
                                                      │ SSH stdout
┌─────────────────────────────────────────────────────┼───────────┐
│                       Windows 客户端                 │           │
│                                           ┌─────────▼────────┐  │
│                                           │    SSH 连接       │  │
│                                           │   JSON 解析       │  │
│  ┌─────────────┐                          └─────────┬────────┘  │
│  │ 系统托盘图标 │◄────────────────────────────────────┘           │
│  │  状态显示    │                                                │
│  └─────────────┘                                                │
└─────────────────────────────────────────────────────────────────┘
```

## 快速开始

### 1. 服务器端安装（Linux）

```bash
# 安装依赖
sudo apt install inotify-tools jq  # Debian/Ubuntu
# 或
sudo yum install inotify-tools jq  # CentOS/RHEL

# 运行安装脚本
cd server && ./install.sh
```

安装脚本会自动配置 Claude Code Hook。

### 2. 客户端安装（Windows）

**下载预编译版本**：
- `claude-status-amd64.exe`（x64）
- `claude-status-arm64.exe`（ARM64）

**创建配置文件** `config.yaml`：
```yaml
server:
  host: "your-server.com"  # 或 ~/.ssh/config 中的别名
```

程序会自动从 `~/.ssh/config` 读取用户名、端口、密钥等配置。

### 3. 运行

双击 `claude-status.exe` 即可。图标会出现在系统托盘。

## 配置说明

### 客户端配置 (config.yaml)

```yaml
server:
  host: "example.com"      # SSH 服务器地址（必填）
  port: 22                 # SSH 端口（可选，默认 22）
  user: "username"         # SSH 用户名（可选，从 ssh config 读取）
  identity_file: ""        # 密钥路径（可选）

# 调试模式
debug: false

# 状态超时（秒），超时的项目会从列表移除，0 禁用
status_timeout: 300
```

### Claude Code Hook 配置

安装脚本会自动在 `~/.claude/settings.json` 中添加以下 Hook（不会影响原来的Hook）：

| Hook 事件 | 状态 | 说明 |
|-----------|------|------|
| `SessionStart` | idle | 会话开始 |
| `UserPromptSubmit` | working | 用户提交输入 |
| `PostToolUse` | working | 工具执行完成 |
| `PermissionRequest` | idle | 等待用户授权 |
| `Stop` | idle | Claude 完成响应 |
| `SessionEnd` | stopped | 会话结束（客户端移除） |

## 从源码编译

```bash
cd client

# 生成图标（需要 ImageMagick）
make icons

# 生成 Windows 资源（exe 图标）
make winres

# 编译
make all
```

输出文件在 `client/build/` 目录。

## 目录结构

```
claude-status/
├── client/                      # Windows 客户端
│   ├── cmd/claude-status/       # 入口
│   ├── internal/                # 内部包
│   │   ├── app/                 # 应用逻辑
│   │   ├── config/              # 配置管理
│   │   ├── ssh/                 # SSH 客户端
│   │   ├── tray/                # 系统托盘
│   │   └── logger/              # 日志
│   ├── assets/                  # 图标资源
│   │   ├── svg/                 # 源 SVG
│   │   ├── icons/               # 生成的 ICO
│   │   └── generate.sh          # 生成脚本
│   └── config.example.yaml
│
└── server/                      # Linux 服务端
    ├── install.sh               # 安装脚本
    ├── monitor.sh               # 状态监听
    └── hooks/
        └── status-hook.sh       # Hook 脚本
```

## License

MIT
