#!/bin/bash
# Claude Code Status Hook Script
# 用法: status-hook.sh <working|idle|stopped>
# 由 Claude Code Hook 调用，更新状态文件
#
# 性能优化：后台执行，立即返回，不阻塞 Claude Code
# 性能优化：状态相同时跳过写入

# 捕获参数和环境变量
_STATUS="${1:-idle}"
_PROJECT_DIR="${CLAUDE_PROJECT_DIR:-$PWD}"

# 从 stdin 读取 JSON（带超时避免阻塞）
_INPUT_JSON=$(timeout 0.1 cat 2>/dev/null || echo '{}')

# 提取 session_id
_SESSION_ID=$(echo "$_INPUT_JSON" | grep -o '"session_id"[[:space:]]*:[[:space:]]*"[^"]*"' | head -1 | sed 's/.*"\([^"]*\)"$/\1/')

# 如果没有 session_id，输出警告并使用项目哈希作为后备
if [[ -z "$_SESSION_ID" ]]; then
    echo "[claude-status] Warning: No session_id in hook input, using project hash fallback" >&2
    hash_str="${#_PROJECT_DIR}_${_PROJECT_DIR//[^a-zA-Z0-9]/_}"
    _SESSION_ID="${hash_str:0:64}"
fi

# 后台执行主逻辑
(
    STATUS_DIR="$HOME/.claude-status"

    # 确保状态目录存在
    [[ -d "$STATUS_DIR" ]] || mkdir -p "$STATUS_DIR"

    # 使用 session_id 作为状态文件名
    STATUS_FILE="$STATUS_DIR/${_SESSION_ID}.json"

    # 检查当前状态，相同则跳过写入
    if [[ -f "$STATUS_FILE" ]]; then
        # 简单提取 status 字段值（避免依赖 jq）
        current_status=$(grep -o '"status"[[:space:]]*:[[:space:]]*"[^"]*"' "$STATUS_FILE" | head -1 | sed 's/.*"\([^"]*\)"$/\1/')
        [[ "$current_status" == "$_STATUS" ]] && exit 0
    fi

    # 使用 bash 内置的字符串操作替代 basename
    PROJECT_NAME="${_PROJECT_DIR##*/}"

    # 使用 printf 获取时间戳
    printf -v TIMESTAMP '%(%s)T' -1

    # JSON 转义函数
    json_escape() {
        local s="$1"
        s="${s//\\/\\\\}"
        s="${s//\"/\\\"}"
        printf '%s' "$s"
    }

    PROJECT_DIR_ESCAPED=$(json_escape "$_PROJECT_DIR")
    PROJECT_NAME_ESCAPED=$(json_escape "$PROJECT_NAME")
    SESSION_ID_ESCAPED=$(json_escape "$_SESSION_ID")

    # 原子写入状态文件（先写临时文件，再 mv）
    TMP_FILE="$STATUS_FILE.tmp.$$"
    printf '{
  "project": "%s",
  "project_name": "%s",
  "session_id": "%s",
  "status": "%s",
  "updated_at": %s
}
' "$PROJECT_DIR_ESCAPED" "$PROJECT_NAME_ESCAPED" "$SESSION_ID_ESCAPED" "$_STATUS" "$TIMESTAMP" > "$TMP_FILE" && mv -f "$TMP_FILE" "$STATUS_FILE"
) &

exit 0
