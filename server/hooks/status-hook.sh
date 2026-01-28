#!/bin/bash
# Claude Code Status Hook Script
# 用法: status-hook.sh <working|idle|stopped>
# 由 Claude Code Hook 调用，更新状态文件
#
# 注意：
# - stdout 会被添加到 UserPromptSubmit/SessionStart 的对话上下文，保持静默
# - exit code 0 = 成功，exit code 2 = 阻断操作，其他 = 非阻断错误

set -e

STATUS_DIR="$HOME/.claude-status"
STATUS="${1:-idle}"

# 确保状态目录存在
mkdir -p "$STATUS_DIR"

# 获取项目目录（从 Claude Code 环境变量，静默回退到 PWD）
PROJECT_DIR="${CLAUDE_PROJECT_DIR:-$PWD}"
PROJECT_NAME=$(basename "$PROJECT_DIR")

# 使用项目路径的 MD5 哈希作为文件名
PROJECT_HASH=$(echo -n "$PROJECT_DIR" | md5sum | cut -d' ' -f1)
STATUS_FILE="$STATUS_DIR/${PROJECT_HASH}.json"

# 获取当前时间戳
TIMESTAMP=$(date +%s)

# 使用临时文件进行原子写入（防并发问题）
TEMP_FILE="${STATUS_FILE}.tmp.$$"
cat > "$TEMP_FILE" << EOF
{
  "project": "$PROJECT_DIR",
  "project_name": "$PROJECT_NAME",
  "status": "$STATUS",
  "updated_at": $TIMESTAMP
}
EOF

# 原子移动
mv "$TEMP_FILE" "$STATUS_FILE"

exit 0
