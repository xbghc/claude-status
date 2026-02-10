# Claude Status Monitor - Windows 构建脚本
# 用法:
#   .\build.ps1              # 编译 amd64
#   .\build.ps1 -Arch arm64  # 编译 arm64
#   .\build.ps1 -Icons       # 生成图标（需要 ImageMagick）
#   .\build.ps1 -All         # 图标 + winres + 编译 amd64 & arm64
#   .\build.ps1 -Clean       # 清理构建产物

param(
    [ValidateSet("amd64", "arm64", "auto")]
    [string]$Arch = "auto",
    [switch]$Icons,
    [switch]$All,
    [switch]$Clean,
    [switch]$WinRes
)

# 自动检测架构
if ($Arch -eq "auto") {
    $cpuArch = $env:PROCESSOR_ARCHITECTURE
    if ($cpuArch -eq "ARM64") {
        $Arch = "arm64"
    } else {
        $Arch = "amd64"
    }
    Write-Host "检测到架构: $cpuArch -> $Arch" -ForegroundColor Gray
}

$ErrorActionPreference = "Stop"
$AppName = "claude-status"
$BuildDir = "build"
$CmdDir = "cmd\claude-status"
$AssetsDir = "assets"
$IconsDir = "$AssetsDir\icons"
$SvgDir = "$AssetsDir\svg"
$WinResDir = "$CmdDir\winres"

# 确保在项目根目录运行
if (-not (Test-Path "go.mod")) {
    Write-Error "请在 client 目录下运行此脚本"
    exit 1
}

function Require-Tool {
    param([string]$Name, [string]$Description)
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        throw "缺少必要工具: $Name ($Description)，请先安装后再运行"
    }
}

function Build-Icons {
    Write-Host "=== 生成图标 ===" -ForegroundColor Cyan
    Require-Tool "magick" "ImageMagick"

    if (-not (Test-Path $IconsDir)) { New-Item -ItemType Directory -Path $IconsDir -Force | Out-Null }

    $staticIcons = @(
        @{ Name = "disconnected-dark";  Svg = "$SvgDir\icon-disconnected-dark.svg" }
        @{ Name = "disconnected-light"; Svg = "$SvgDir\icon-disconnected-light.svg" }
        @{ Name = "input-needed-dark";  Svg = "$SvgDir\icon-input-needed-dark.svg" }
        @{ Name = "input-needed-light"; Svg = "$SvgDir\icon-input-needed-light.svg" }
        @{ Name = "running-dark";       Svg = "$SvgDir\icon-running-dark.svg" }
        @{ Name = "running-light";      Svg = "$SvgDir\icon-running-light.svg" }
    )

    foreach ($icon in $staticIcons) {
        Write-Host "  - $($icon.Name).ico"
        $tmpPng = "$IconsDir\$($icon.Name)-tmp.png"
        & magick -background none -density 384 $icon.Svg -resize 256x256 $tmpPng
        if ($LASTEXITCODE -ne 0) { throw "magick SVG->PNG 失败: $($icon.Name)" }
        & magick $tmpPng -define icon:auto-resize="256,48,32,24,20,16" "$IconsDir\$($icon.Name).ico"
        if ($LASTEXITCODE -ne 0) { throw "magick PNG->ICO 失败: $($icon.Name)" }
        Remove-Item $tmpPng -Force -ErrorAction SilentlyContinue
    }

    # 生成动画帧
    $frames = 24
    $frameAngle = 5

    foreach ($theme in @(
        @{ Name = "dark";  Stroke = "#a3a3a3"; Circle = "#e5e5e5" }
        @{ Name = "light"; Stroke = "#525252"; Circle = "#171717" }
    )) {
        Write-Host "  - running-$($theme.Name) 动画帧 ($frames 帧)"
        for ($i = 0; $i -lt $frames; $i++) {
            $angle = $i * $frameAngle
            $frameSvg = "$IconsDir\frame.svg"
            $framePng = "$IconsDir\frame.png"
            @"
<svg width="100" height="100" viewBox="0 0 100 100" fill="none" xmlns="http://www.w3.org/2000/svg">
  <path d="M38 32 L62 50 L38 68" stroke="$($theme.Stroke)" stroke-width="8" stroke-linecap="round" stroke-linejoin="round"/>
  <g transform="rotate($angle 50 50)">
    <circle cx="50" cy="50" r="40" stroke="$($theme.Circle)" stroke-width="8" stroke-linecap="round" stroke-dasharray="57 26"/>
  </g>
</svg>
"@ | Set-Content -Path $frameSvg -Encoding UTF8
            & magick -background none -density 384 $frameSvg -resize 256x256 $framePng
            if ($LASTEXITCODE -ne 0) { throw "magick SVG->PNG 失败: running-$($theme.Name)-frame${i}" }
            & magick $framePng -define icon:auto-resize="256,48,32,24,20,16" "$IconsDir\running-$($theme.Name)-frame${i}.ico"
            if ($LASTEXITCODE -ne 0) { throw "magick PNG->ICO 失败: running-$($theme.Name)-frame${i}" }
        }
        Remove-Item "$IconsDir\frame.svg", "$IconsDir\frame.png" -Force -ErrorAction SilentlyContinue
    }

    Write-Host "图标生成完成" -ForegroundColor Green
}

function Build-WinRes {
    Write-Host "=== 生成 Windows 资源 ===" -ForegroundColor Cyan
    Require-Tool "go-winres" "go-winres (go install github.com/tc-hib/go-winres@latest)"

    # 生成 winres 所需的 icon.png（从 ICO 转换）
    $iconPng = "$WinResDir\icon.png"
    if (-not (Test-Path $iconPng)) {
        $icoPath = "$IconsDir\running-light.ico"
        if (Test-Path $icoPath) {
            Write-Host "  从 ICO 生成 icon.png ..."
            Add-Type -AssemblyName System.Drawing
            $ico = New-Object System.Drawing.Icon($icoPath, 256, 256)
            $bmp = $ico.ToBitmap()
            $bmp.Save($iconPng, [System.Drawing.Imaging.ImageFormat]::Png)
            $bmp.Dispose()
            $ico.Dispose()
        } else {
            Write-Error "缺少图标文件: $icoPath，请先运行 .\build.ps1 -Icons"
            exit 1
        }
    }

    Push-Location $CmdDir
    try {
        & go-winres make --arch amd64,arm64
        if ($LASTEXITCODE -ne 0) { throw "go-winres 失败" }
    }
    finally {
        Pop-Location
    }
    Write-Host "Windows 资源生成完成" -ForegroundColor Green
}

function Build-Exe {
    param([string]$TargetArch)

    # 检查 .syso 文件是否存在
    $sysoPattern = "$CmdDir\*.syso"
    if (-not (Test-Path $sysoPattern)) {
        Write-Host ".syso 文件不存在，先生成 Windows 资源..." -ForegroundColor Yellow
        Build-WinRes
    }

    Write-Host "=== 编译 Windows $TargetArch ===" -ForegroundColor Cyan
    if (-not (Test-Path $BuildDir)) { New-Item -ItemType Directory -Path $BuildDir -Force | Out-Null }

    $env:GOOS = "windows"
    $env:GOARCH = $TargetArch
    & go build -ldflags "-H windowsgui" -o "$BuildDir\$AppName-$TargetArch.exe" ".\$CmdDir"
    if ($LASTEXITCODE -ne 0) { throw "编译失败" }

    # 清理环境变量
    Remove-Item Env:GOOS -ErrorAction SilentlyContinue
    Remove-Item Env:GOARCH -ErrorAction SilentlyContinue

    Write-Host "输出: $BuildDir\$AppName-$TargetArch.exe" -ForegroundColor Green
}

function Clean-Build {
    Write-Host "=== 清理 ===" -ForegroundColor Cyan
    if (Test-Path $BuildDir) { Remove-Item $BuildDir -Recurse -Force }
    Get-ChildItem "$CmdDir\*.syso" -ErrorAction SilentlyContinue | Remove-Item -Force
    Write-Host "清理完成" -ForegroundColor Green
}

# 主逻辑
if ($Clean) {
    Clean-Build
    exit 0
}

if ($All) {
    Build-Icons
    Build-WinRes
    Build-Exe -TargetArch "amd64"
    Build-Exe -TargetArch "arm64"
    exit 0
}

if ($Icons) {
    Build-Icons
    exit 0
}

if ($WinRes) {
    Build-WinRes
    exit 0
}

# 默认：编译指定架构
Build-Exe -TargetArch $Arch
