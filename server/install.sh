#!/bin/bash
# Claude Code Status Monitor - 服务器端安装脚本

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INSTALL_DIR="$HOME/.claude-status"
HOOKS_DIR="$INSTALL_DIR/hooks"
CLAUDE_SETTINGS="$HOME/.claude/settings.json"
HOOK_CMD="$HOOKS_DIR/status-hook.sh"

echo "=== Claude Code Status Monitor 安装脚本 ==="
echo ""

# 检查是否已安装
if [ -f "$INSTALL_DIR/monitor.sh" ]; then
    echo "检测到已安装的版本。"
    echo ""
    read -p "是否重新安装？(y/N) " -n 1 -r
    echo ""
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "取消安装。"
        exit 0
    fi
    echo ""
fi

# 1. 检查依赖
echo "[1/4] 检查依赖..."
if ! command -v inotifywait &> /dev/null; then
    echo "  ✗ inotify-tools 未安装"
    echo ""
    echo "请先安装 inotify-tools:"
    echo "  Debian/Ubuntu: sudo apt install inotify-tools"
    echo "  CentOS/RHEL:   sudo yum install inotify-tools"
    echo "  Arch:          sudo pacman -S inotify-tools"
    exit 1
fi
echo "  ✓ inotify-tools 已安装"

if ! command -v jq &> /dev/null; then
    echo "  ✗ jq 未安装"
    echo ""
    echo "请先安装 jq:"
    echo "  Debian/Ubuntu: sudo apt install jq"
    echo "  CentOS/RHEL:   sudo yum install jq"
    echo "  Arch:          sudo pacman -S jq"
    exit 1
fi
echo "  ✓ jq 已安装"

# 2. 创建目录
echo "[2/4] 创建目录..."
mkdir -p "$INSTALL_DIR"
mkdir -p "$HOOKS_DIR"
mkdir -p "$HOME/.claude"

# 3. 复制脚本
echo "[3/4] 复制脚本..."
cp "$SCRIPT_DIR/hooks/status-hook.sh" "$HOOKS_DIR/"
cp "$SCRIPT_DIR/monitor.sh" "$INSTALL_DIR/"
chmod +x "$HOOKS_DIR/status-hook.sh"
chmod +x "$INSTALL_DIR/monitor.sh"

# 4. 配置 Claude Code Hook
echo "[4/4] 配置 Claude Code Hook..."

# 初始化 settings.json
if [ ! -f "$CLAUDE_SETTINGS" ]; then
    echo '{}' > "$CLAUDE_SETTINGS"
fi

# 检查是否已配置过此 hook
if grep -q "$HOOK_CMD" "$CLAUDE_SETTINGS" 2>/dev/null; then
    echo "  - Hook 已配置，跳过"
else
    # 备份现有配置
    cp "$CLAUDE_SETTINGS" "$CLAUDE_SETTINGS.backup.$(date +%Y%m%d%H%M%S)"
    echo "  - 已备份现有配置"

    # 使用 jq 添加 hooks 配置
    # - UserPromptSubmit: 用户提交输入，开始工作
    # - PostToolUse: 工具执行完成，继续工作（包括权限批准后）
    # - Stop: Claude 完成处理，等待用户输入
    # - PermissionRequest: 请求权限，等待用户操作
    # - SessionStart: 会话开始，初始化状态
    # - SessionEnd: 会话结束，移除实例
    jq --arg hook "$HOOK_CMD" '
        .hooks.UserPromptSubmit = (.hooks.UserPromptSubmit // []) + [{
            "matcher": "*",
            "hooks": [{"type": "command", "command": ($hook + " working")}]
        }] |
        .hooks.PostToolUse = (.hooks.PostToolUse // []) + [{
            "matcher": "*",
            "hooks": [{"type": "command", "command": ($hook + " working")}]
        }] |
        .hooks.Stop = (.hooks.Stop // []) + [{
            "matcher": "*",
            "hooks": [{"type": "command", "command": ($hook + " idle")}]
        }] |
        .hooks.PermissionRequest = (.hooks.PermissionRequest // []) + [{
            "matcher": "*",
            "hooks": [{"type": "command", "command": ($hook + " idle")}]
        }] |
        .hooks.SessionStart = (.hooks.SessionStart // []) + [{
            "matcher": "*",
            "hooks": [{"type": "command", "command": ($hook + " idle")}]
        }] |
        .hooks.SessionEnd = (.hooks.SessionEnd // []) + [{
            "matcher": "*",
            "hooks": [{"type": "command", "command": ($hook + " stopped")}]
        }]
    ' "$CLAUDE_SETTINGS" > "$CLAUDE_SETTINGS.tmp" && mv "$CLAUDE_SETTINGS.tmp" "$CLAUDE_SETTINGS"
    echo "  - 已添加 Hook 配置"
fi

echo ""
echo "=== 安装完成 ==="
echo ""
echo "监听脚本位置: $INSTALL_DIR/monitor.sh"
echo "客户端连接命令: ssh <user>@<host> '$INSTALL_DIR/monitor.sh'"
echo ""
echo "测试方法:"
echo "  1. 运行: $HOOKS_DIR/status-hook.sh idle     # 模拟会话开始"
echo "  2. 运行: $HOOKS_DIR/status-hook.sh working  # 模拟用户提交输入"
echo "  3. 运行: $HOOKS_DIR/status-hook.sh idle     # 模拟 Claude 完成处理"
echo "  4. 运行: $HOOKS_DIR/status-hook.sh stopped  # 模拟会话结束"
echo "  5. 检查: cat $INSTALL_DIR/*.json"
