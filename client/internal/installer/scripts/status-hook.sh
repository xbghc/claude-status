#!/bin/bash
# Claude Code Status Hook Script
# 用法: status-hook.sh <working|idle|stopped>
# 由 Claude Code Hook 调用，更新状态文件
#
# 性能优化：使用 bash 内置命令，减少 fork 次数

STATUS_DIR="$HOME/.claude-status"
STATUS="${1:-idle}"

# 确保状态目录存在（只在不存在时创建）
[[ -d "$STATUS_DIR" ]] || mkdir -p "$STATUS_DIR"

# 获取项目目录（从 Claude Code 环境变量，静默回退到 PWD）
PROJECT_DIR="${CLAUDE_PROJECT_DIR:-$PWD}"

# 使用 bash 内置的字符串操作替代 basename
PROJECT_NAME="${PROJECT_DIR##*/}"

# 使用简单的哈希替代 md5sum（基于字符串长度和部分字符）
# 这足够区分不同项目路径，且速度快很多
hash_str="${#PROJECT_DIR}_${PROJECT_DIR//[^a-zA-Z0-9]/_}"
# 截取前64字符作为文件名
PROJECT_HASH="${hash_str:0:64}"
STATUS_FILE="$STATUS_DIR/${PROJECT_HASH}.json"

# 使用 printf 获取时间戳（bash 内置，无需 fork）
printf -v TIMESTAMP '%(%s)T' -1

# JSON 转义函数（转义 \ 和 "）
json_escape() {
    local s="$1"
    s="${s//\\/\\\\}"  # 先转义反斜杠
    s="${s//\"/\\\"}"  # 再转义双引号
    printf '%s' "$s"
}

# 转义 JSON 字符串值
PROJECT_DIR_ESCAPED=$(json_escape "$PROJECT_DIR")
PROJECT_NAME_ESCAPED=$(json_escape "$PROJECT_NAME")

# 直接写入文件（使用 printf 替代 cat，无需 fork）
printf '{
  "project": "%s",
  "project_name": "%s",
  "status": "%s",
  "updated_at": %s
}
' "$PROJECT_DIR_ESCAPED" "$PROJECT_NAME_ESCAPED" "$STATUS" "$TIMESTAMP" > "$STATUS_FILE"

exit 0
