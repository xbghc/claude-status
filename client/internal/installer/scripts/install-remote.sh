#!/bin/bash
# 远程安装脚本 - 配置 Claude Code Hooks
# 由客户端通过 SSH 执行

HOOK_CMD="$HOME/.claude-status/hooks/status-hook.sh"
CLAUDE_SETTINGS="$HOME/.claude/settings.json"

# 确保 settings.json 存在
if [ ! -f "$CLAUDE_SETTINGS" ]; then
    echo '{}' > "$CLAUDE_SETTINGS"
fi

# 检查是否已配置
if grep -q "$HOOK_CMD" "$CLAUDE_SETTINGS" 2>/dev/null; then
    echo "Hook 已配置"
    exit 0
fi

# 备份
cp "$CLAUDE_SETTINGS" "$CLAUDE_SETTINGS.backup.$(date +%Y%m%d%H%M%S)" 2>/dev/null || true

# 配置 hooks
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

echo "Hook 配置完成"
