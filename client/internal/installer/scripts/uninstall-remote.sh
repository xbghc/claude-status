#!/bin/bash
# 远程卸载脚本 - 清理 Claude Code Hooks 和 status-hook 相关文件
# 由客户端通过 SSH/WSL 执行
#
# 执行步骤：
#   1. 停止正在运行的 monitor.sh 进程（避免 trap 再次写入 settings.json）
#   2. 从 ~/.claude/settings.json 移除所有指向 status-hook.sh 的 Hook
#   3. 删除 ~/.claude-status 目录（脚本 + 状态文件）
#
# 可选参数：
#   --purge   额外清理 settings.json.backup.* 备份；若清理后
#             settings.json 仅剩 {} 也一并删除，彻底不留痕迹

set -u

STATUS_DIR="$HOME/.claude-status"
CLAUDE_DIR="$HOME/.claude"
CLAUDE_SETTINGS="$CLAUDE_DIR/settings.json"

PURGE=0
for arg in "$@"; do
    case "$arg" in
        --purge) PURGE=1 ;;
    esac
done

if [ "$PURGE" = "1" ]; then
    echo "[uninstall] 开始卸载 Claude Code Status（purge 模式）..."
else
    echo "[uninstall] 开始卸载 Claude Code Status..."
fi

# 1. 停止正在运行的 monitor.sh（若存在）
#    先关闭 monitor.sh 的 trap cleanup，避免它在退出时再次改写 settings.json
if command -v pkill &> /dev/null; then
    if pgrep -f "$STATUS_DIR/monitor.sh" > /dev/null 2>&1; then
        echo "[uninstall] 停止正在运行的 monitor.sh..."
        pkill -f "$STATUS_DIR/monitor.sh" 2>/dev/null || true
        # 给进程一点时间退出（它自身的 cleanup 也会尝试清理 hook，幂等）
        sleep 1
    fi
fi

# 2. 清理 settings.json 中的 status-hook.sh 相关 Hook
#    purge 模式下跳过备份（反正要删除），其他模式下留一份 uninstall 备份以防误操作
if [ -f "$CLAUDE_SETTINGS" ]; then
    if command -v jq &> /dev/null; then
        if [ "$PURGE" != "1" ]; then
            cp "$CLAUDE_SETTINGS" "$CLAUDE_SETTINGS.backup.uninstall.$(date +%Y%m%d%H%M%S)" 2>/dev/null || true
        fi

        if jq '
            def remove_status_hooks:
                if . == null then null
                elif (. | length) == 0 then .
                else [.[] | select(.hooks[0].command | contains("status-hook.sh") | not)]
                end;

            def is_empty: . == null or . == [];

            if .hooks then
                .hooks.UserPromptSubmit  = (.hooks.UserPromptSubmit  | remove_status_hooks) |
                .hooks.PostToolUse       = (.hooks.PostToolUse       | remove_status_hooks) |
                .hooks.Stop              = (.hooks.Stop              | remove_status_hooks) |
                .hooks.PermissionRequest = (.hooks.PermissionRequest | remove_status_hooks) |
                .hooks.SessionStart      = (.hooks.SessionStart      | remove_status_hooks) |
                .hooks.SessionEnd        = (.hooks.SessionEnd        | remove_status_hooks) |

                (if .hooks.UserPromptSubmit  | is_empty then del(.hooks.UserPromptSubmit)  else . end) |
                (if .hooks.PostToolUse       | is_empty then del(.hooks.PostToolUse)       else . end) |
                (if .hooks.Stop              | is_empty then del(.hooks.Stop)              else . end) |
                (if .hooks.PermissionRequest | is_empty then del(.hooks.PermissionRequest) else . end) |
                (if .hooks.SessionStart      | is_empty then del(.hooks.SessionStart)      else . end) |
                (if .hooks.SessionEnd        | is_empty then del(.hooks.SessionEnd)        else . end) |
                (if .hooks == {} then del(.hooks) else . end)
            else . end
        ' "$CLAUDE_SETTINGS" > "$CLAUDE_SETTINGS.tmp" 2>/dev/null; then
            mv -f "$CLAUDE_SETTINGS.tmp" "$CLAUDE_SETTINGS"
            echo "[uninstall] 已从 settings.json 移除 Hook 配置"
        else
            rm -f "$CLAUDE_SETTINGS.tmp" 2>/dev/null || true
            echo "[uninstall] 警告: 更新 settings.json 失败，请手动检查" >&2
        fi
    else
        echo "[uninstall] 警告: 未安装 jq，跳过 settings.json 清理" >&2
        echo "[uninstall] 请手动编辑 $CLAUDE_SETTINGS 移除 command 包含 status-hook.sh 的项" >&2
    fi
else
    echo "[uninstall] $CLAUDE_SETTINGS 不存在，跳过 Hook 清理"
fi

# 3. 删除脚本和状态文件目录
if [ -d "$STATUS_DIR" ]; then
    rm -rf "$STATUS_DIR"
    echo "[uninstall] 已删除目录: $STATUS_DIR"
else
    echo "[uninstall] $STATUS_DIR 不存在，跳过目录清理"
fi

# 4. purge 模式：删除 install/uninstall 产生的所有 settings.json 备份；
#    若 settings.json 本身已退化为空对象 {}，也一并删除（说明是我们当初
#    为了写入 hook 而创建的空文件，用户并没有自己的 Claude Code 配置）。
if [ "$PURGE" = "1" ]; then
    # 用 nullglob 避免未匹配时展开为字面字符串
    shopt -s nullglob 2>/dev/null || true
    backups=("$CLAUDE_DIR"/settings.json.backup.*)
    if [ ${#backups[@]} -gt 0 ]; then
        rm -f "${backups[@]}"
        echo "[uninstall] 已删除 ${#backups[@]} 个 settings.json 备份"
    fi
    shopt -u nullglob 2>/dev/null || true

    if [ -f "$CLAUDE_SETTINGS" ] && command -v jq &> /dev/null; then
        if jq -e 'type == "object" and length == 0' "$CLAUDE_SETTINGS" > /dev/null 2>&1; then
            rm -f "$CLAUDE_SETTINGS"
            echo "[uninstall] settings.json 已为空，已删除"
        fi
    fi

    # 若 ~/.claude 目录此刻已完全为空（用户无任何其他 Claude Code 配置），
    # 顺手清理掉，真正不留痕迹
    if [ -d "$CLAUDE_DIR" ] && [ -z "$(ls -A "$CLAUDE_DIR" 2>/dev/null)" ]; then
        rmdir "$CLAUDE_DIR" 2>/dev/null && echo "[uninstall] $CLAUDE_DIR 已为空，已删除"
    fi
fi

echo "[uninstall] 卸载完成"
