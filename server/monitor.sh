#!/bin/bash
# Claude Code Status Monitor Script
# 监听状态文件变化并输出 JSON 到 stdout
# 供 SSH 客户端读取
#
# 依赖: inotify-tools (sudo apt install inotify-tools)

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

    # 读取所有状态文件
    for file in "$STATUS_DIR"/*.json; do
        [ -f "$file" ] || continue

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

    echo "{\"type\":\"status\",\"data\":$data}"
}

# 清理过期状态文件（超过 1 小时未更新的视为过期）
cleanup_stale() {
    local now=$(date +%s)
    local max_age=3600  # 1 小时

    for file in "$STATUS_DIR"/*.json; do
        [ -f "$file" ] || continue

        updated_at=$(grep -o '"updated_at":[0-9]*' "$file" | grep -o '[0-9]*' || echo "0")
        age=$((now - updated_at))

        if [ "$age" -gt "$max_age" ]; then
            rm -f "$file"
        fi
    done
}

# 禁用 stdout 缓冲
exec 1> >(cat)

# 启动时清理并输出初始状态
cleanup_stale
output_status

# 使用 inotifywait 监听文件变化
# 使用 --monitor 持续监听，每次变化输出状态
while true; do
    inotifywait -qq -e modify -e create -e delete "$STATUS_DIR" 2>/dev/null
    output_status
done
