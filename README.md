<div align="center">

# Claude Code Status Monitor

### 再也不用盯着终端等 Claude 了

<br />

<p>
  <img src="client/assets/svg/icon-disconnected-dark.svg" width="64" alt="disconnected" />
  &nbsp;&nbsp;&nbsp;&nbsp;
  <img src="client/assets/svg/icon-input-needed-dark.svg" width="64" alt="idle" />
  &nbsp;&nbsp;&nbsp;&nbsp;
  <img src="client/assets/svg/icon-running-dark.svg" width="64" alt="running" />
</p>

**断开连接** · **等待输入** · **正在运行**

<br />

[![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![Platform](https://img.shields.io/badge/Platform-Windows-0078D6?style=flat-square&logo=windows)](https://www.microsoft.com/windows)
[![License](https://img.shields.io/badge/License-MIT-green?style=flat-square)](LICENSE)

</div>

---

## 为什么需要这个？

使用 Claude Code 时，你是否经常：

- 🤔 切到其他窗口干活，忘了 Claude 已经在等你回复
- 👀 反复切换窗口查看 Claude 是否完成
- ⏰ 不确定长任务还要跑多久

**Claude Code Status Monitor** 让你在 Windows 任务栏一眼就能看到 Claude 的状态——无需打断手头的工作。

## 功能特性

| 特性 | 说明 |
|-----|------|
| 🎯 **实时状态** | 系统托盘图标动态显示，运行时带动画 |
| 📡 **SSH 连接** | 直接复用 `~/.ssh/config`，零配置 |
| 🖥️ **WSL 支持** | 本地 WSL 中的 Claude Code 也能监控 |
| 🔌 **即插即用** | 首次连接自动安装服务端，无需手动配置 |
| ⚡ **低延迟** | 基于 inotify + Hook，毫秒级响应 |
| 🧹 **自动清理** | 会话结束自动移除，保持清爽 |

## 快速开始

### 30 秒上手

**1. 服务器安装依赖**（一次性）
```bash
sudo apt install inotify-tools jq   # Debian/Ubuntu
```

**2. 下载客户端** → 创建 `config.yaml`
```yaml
server:
  host: "my-server"   # ~/.ssh/config 中的别名即可
```

**3. 双击运行** — 完成！

> 首次连接会自动安装服务端脚本，无需手动配置。

---

### WSL 用户

本地 WSL 也支持，配置更简单：

```yaml
wsl:
  enabled: true
```

WSL 内同样需要 `sudo apt install inotify-tools jq`

## 工作原理

```
Claude Code ──Hook──► status-hook.sh ──► ~/.claude-status/*.json
                                                    │
                                            monitor.sh (inotify)
                                                    │
                                               SSH stdout
                                                    │
                                         Windows 系统托盘图标
```

- **服务端**：Claude Code Hook 触发时更新状态文件，monitor.sh 监听变化
- **客户端**：通过 SSH 读取 JSON 流，更新托盘图标

## 配置参考

<details>
<summary><b>完整配置选项</b>（点击展开）</summary>

```yaml
# SSH 模式
server:
  host: "example.com"      # 服务器地址或 ssh config 别名（必填）
  port: 22                 # 可选，默认从 ssh config 读取
  user: "username"         # 可选，默认从 ssh config 读取
  identity_file: ""        # 可选，默认自动查找

# WSL 模式
wsl:
  enabled: true
  distro: ""               # 可选，空则使用默认发行版

# 通用
debug: false               # 调试日志
status_timeout: 300        # 超时清理（秒），0 禁用
```

</details>

<details>
<summary><b>自动配置的 Hook 事件</b>（点击展开）</summary>

| Hook 事件 | 触发状态 |
|-----------|---------|
| `SessionStart` | 等待输入 |
| `UserPromptSubmit` | 运行中 |
| `PostToolUse` | 运行中 |
| `PermissionRequest` | 等待输入 |
| `Stop` | 等待输入 |
| `SessionEnd` | 移除 |

</details>

## 从源码编译

```bash
cd client
make icons    # 生成图标（需要 ImageMagick）
make all      # 编译
```

产物在 `client/build/` 目录。

## 故障排除

| 问题 | 解决方案 |
|-----|---------|
| 连接失败 | 先测试 `ssh your-server` 是否正常 |
| 自动安装失败 | 确保服务器已安装 `inotify-tools` 和 `jq` |
| 状态不更新 | 重启 Claude Code 会话以加载 Hook |

日志位置：程序同目录下 `claude-status.log`

## 卸载

### 客户端
正常 Windows 卸载（MSI 或删除 exe）即可。

### 服务端（脚本 + Hook）

使用客户端一键卸载（推荐）：

```powershell
# 常规卸载（保留 settings.json 备份以防误操作）
claude-status.exe --uninstall

# 彻底清理，不留任何痕迹
claude-status.exe --uninstall --purge
```

`--uninstall` 会连接配置中的服务器（SSH 或 WSL），执行：
- 从 `~/.claude/settings.json` 移除所有 `status-hook.sh` 相关的 Hook
- 删除 `~/.claude-status/` 目录（脚本 + 状态文件）
- 将原 `settings.json` 备份为 `settings.json.backup.uninstall.<timestamp>`

`--purge` 会在此基础上额外清理：
- 删除所有 `~/.claude/settings.json.backup.*` 备份
- 若 `settings.json` 此时已为空 `{}`（说明是我们安装时创建的）则删除
- 若 `~/.claude` 目录因此变空则删除

---

<div align="center">

**MIT License**

Made with ❤️ by Claude Code（是的，让它监控它自己）

</div>
