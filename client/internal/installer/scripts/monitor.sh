#!/bin/bash
# Claude Code Status Monitor Script
# 版本: __VERSION__
# 监听状态文件变化并输出 JSON 到 stdout
# 供 SSH 客户端读取
#
# 依赖: inotify-tools (sudo apt install inotify-tools)

SCRIPT_VERSION="__VERSION__"
STATUS_DIR="$HOME/.claude-status"

# 检查 inotifywait 是否可用
if ! command -v inotifywait &> /dev/null; then
    echo '{"type":"error","message":"inotifywait not found. Please install: sudo apt install inotify-tools"}'
    exit 1
fi

# 确保状态目录存在
mkdir -p "$STATUS_DIR"

# 输出所有状态的 JSON 函数
output_status() {
    local data="[]"
    local first=true
    local file_count=0

    # 调试：列出状态目录中的文件
    echo "[output_status] Scanning $STATUS_DIR" >&2

    # 读取所有状态文件
    for file in "$STATUS_DIR"/*.json; do
        [ -f "$file" ] || continue
        file_count=$((file_count + 1))
        echo "[output_status] Found: $file" >&2

        # 读取并压缩成单行（移除换行符）
        content=$(tr -d '\n' < "$file" 2>/dev/null || echo "{}")

        if [ "$first" = true ]; then
            data="[$content"
            first=false
        else
            data="$data,$content"
        fi
    done

    if [ "$first" = false ]; then
        data="$data]"
    fi

    echo "[output_status] Sending $file_count files" >&2
    echo "{\"type\":\"status\",\"data\":$data}"
}

# 清理过期状态文件（超过 1 小时未更新的视为过期）
cleanup_stale() {
    local now=$(date +%s)
    local max_age=3600  # 1 小时

    for file in "$STATUS_DIR"/*.json; do
        [ -f "$file" ] || continue

        # 修复：匹配 "updated_at": 1234567890 或 "updated_at":1234567890 两种格式
        updated_at=$(grep -oE '"updated_at"[[:space:]]*:[[:space:]]*[0-9]+' "$file" | grep -oE '[0-9]+' || echo "0")

        # 如果提取失败，跳过此文件（不删除）
        if [ "$updated_at" = "0" ] || [ -z "$updated_at" ]; then
            echo "[cleanup_stale] Warning: Failed to extract updated_at from $file, skipping" >&2
            continue
        fi

        age=$((now - updated_at))

        if [ "$age" -gt "$max_age" ]; then
            echo "[cleanup_stale] Removing stale file: $file (age=${age}s)" >&2
            rm -f "$file"
        fi
    done
}

# 禁用 stdout 缓冲
exec 1> >(cat)

# 首先输出版本信息
echo "{\"type\":\"version\",\"version\":\"$SCRIPT_VERSION\"}"

# 启动时清理并输出初始状态
cleanup_stale
output_status

# 使用 inotifywait 监听文件变化
# 使用 --monitor 持续监听，每次变化输出状态
while true; do
    inotifywait -qq -e modify -e create -e delete "$STATUS_DIR" 2>/dev/null
    output_status
done
