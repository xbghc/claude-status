#!/bin/bash
# 从 SVG 生成 Windows ICO 图标
# 需要 ImageMagick (convert)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# Windows 托盘图标：使用 icon:auto-resize 生成标准 ICO
FRAMES=24
FRAME_ANGLE=5
OUT_DIR="icons"

echo "生成图标到 ${OUT_DIR}/ ..."

# 创建输出目录
mkdir -p "$OUT_DIR"

# 生成静态图标
generate_icon() {
    local name=$1
    local svg=$2

    echo "  - ${name}.ico"
    convert -background none -density 384 "$svg" -resize 256x256 "$OUT_DIR/${name}-tmp.png"
    convert "$OUT_DIR/${name}-tmp.png" -define icon:auto-resize="256,48,32,24,20,16" "$OUT_DIR/${name}.ico"
    rm -f "$OUT_DIR/${name}-tmp.png"
}

generate_icon "disconnected-dark" "svg/icon-disconnected-dark.svg"
generate_icon "disconnected-light" "svg/icon-disconnected-light.svg"
generate_icon "input-needed-light" "svg/icon-input-needed-light.svg"
generate_icon "input-needed-dark" "svg/icon-input-needed-dark.svg"
generate_icon "running-light" "svg/icon-running-light.svg"
generate_icon "running-dark" "svg/icon-running-dark.svg"

# 生成 running 动画帧（暗色模式）
echo "  - running-dark 动画帧 (${FRAMES} 帧)"
for i in $(seq 0 $((FRAMES - 1))); do
    angle=$((i * FRAME_ANGLE))
    cat > "$OUT_DIR/frame.svg" << EOF
<svg width="100" height="100" viewBox="0 0 100 100" fill="none" xmlns="http://www.w3.org/2000/svg">
  <path d="M38 32 L62 50 L38 68" stroke="#a3a3a3" stroke-width="8" stroke-linecap="round" stroke-linejoin="round"/>
  <g transform="rotate(${angle} 50 50)">
    <circle cx="50" cy="50" r="40" stroke="#e5e5e5" stroke-width="8" stroke-linecap="round" stroke-dasharray="57 26"/>
  </g>
</svg>
EOF
    convert -background none -density 384 "$OUT_DIR/frame.svg" -resize 256x256 "$OUT_DIR/frame.png"
    convert "$OUT_DIR/frame.png" -define icon:auto-resize="256,48,32,24,20,16" "$OUT_DIR/running-dark-frame${i}.ico"
done

# 生成 running 动画帧（亮色模式）
echo "  - running-light 动画帧 (${FRAMES} 帧)"
for i in $(seq 0 $((FRAMES - 1))); do
    angle=$((i * FRAME_ANGLE))
    cat > "$OUT_DIR/frame.svg" << EOF
<svg width="100" height="100" viewBox="0 0 100 100" fill="none" xmlns="http://www.w3.org/2000/svg">
  <path d="M38 32 L62 50 L38 68" stroke="#525252" stroke-width="8" stroke-linecap="round" stroke-linejoin="round"/>
  <g transform="rotate(${angle} 50 50)">
    <circle cx="50" cy="50" r="40" stroke="#171717" stroke-width="8" stroke-linecap="round" stroke-dasharray="57 26"/>
  </g>
</svg>
EOF
    convert -background none -density 384 "$OUT_DIR/frame.svg" -resize 256x256 "$OUT_DIR/frame.png"
    convert "$OUT_DIR/frame.png" -define icon:auto-resize="256,48,32,24,20,16" "$OUT_DIR/running-light-frame${i}.ico"
done

rm -f "$OUT_DIR/frame.svg" "$OUT_DIR/frame.png"

echo ""
echo "完成！生成的文件："
ls -la "$OUT_DIR"/*.ico | awk '{print "  " $9 " (" $5 " bytes)"}'
