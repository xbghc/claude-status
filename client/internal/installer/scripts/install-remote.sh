#!/bin/bash
# 远程安装脚本 - 配置 Claude Code Hooks
# 由客户端通过 SSH 执行

set -e

HOOK_CMD="$HOME/.claude-status/hooks/status-hook.sh"
CLAUDE_SETTINGS="$HOME/.claude/settings.json"

# 确保目录和 settings.json 存在
mkdir -p "$(dirname "$CLAUDE_SETTINGS")"
if [ ! -f "$CLAUDE_SETTINGS" ]; then
    echo '{}' > "$CLAUDE_SETTINGS"
fi

# 验证 settings.json 是有效 JSON
if ! jq empty "$CLAUDE_SETTINGS" 2>/dev/null; then
    echo "Error: $CLAUDE_SETTINGS is not valid JSON, resetting to {}" >&2
    echo '{}' > "$CLAUDE_SETTINGS"
fi

# 备份
cp "$CLAUDE_SETTINGS" "$CLAUDE_SETTINGS.backup.$(date +%Y%m%d%H%M%S)" 2>/dev/null || true

# 先删除旧的 claude-status Hook（如果存在），再添加新的
# 这样可以确保版本更新时 Hook 配置也被更新
jq --arg hook "$HOOK_CMD" '
    def remove_old_hooks:
        if . == null then []
        else [.[] | select(.hooks[0].command | contains("status-hook.sh") | not)]
        end;

    .hooks.UserPromptSubmit = (.hooks.UserPromptSubmit | remove_old_hooks) |
    .hooks.PostToolUse = (.hooks.PostToolUse | remove_old_hooks) |
    .hooks.Stop = (.hooks.Stop | remove_old_hooks) |
    .hooks.PermissionRequest = (.hooks.PermissionRequest | remove_old_hooks) |
    .hooks.SessionStart = (.hooks.SessionStart | remove_old_hooks) |
    .hooks.SessionEnd = (.hooks.SessionEnd | remove_old_hooks) |

    .hooks.UserPromptSubmit = (.hooks.UserPromptSubmit // []) + [{
        "hooks": [{"type": "command", "command": ($hook + " working")}]
    }] |
    .hooks.PostToolUse = (.hooks.PostToolUse // []) + [{
        "matcher": "*",
        "hooks": [{"type": "command", "command": ($hook + " working")}]
    }] |
    .hooks.Stop = (.hooks.Stop // []) + [{
        "hooks": [{"type": "command", "command": ($hook + " idle")}]
    }] |
    .hooks.PermissionRequest = (.hooks.PermissionRequest // []) + [{
        "matcher": "*",
        "hooks": [{"type": "command", "command": ($hook + " idle")}]
    }] |
    .hooks.SessionStart = (.hooks.SessionStart // []) + [{
        "hooks": [{"type": "command", "command": ($hook + " idle")}]
    }] |
    .hooks.SessionEnd = (.hooks.SessionEnd // []) + [{
        "hooks": [{"type": "command", "command": ($hook + " stopped")}]
    }]
' "$CLAUDE_SETTINGS" > "$CLAUDE_SETTINGS.tmp" && mv "$CLAUDE_SETTINGS.tmp" "$CLAUDE_SETTINGS"

echo "Hook 配置完成"
