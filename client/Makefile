# Claude Status Monitor - Windows 客户端构建脚本

APP_NAME := claude-status
BUILD_DIR := build
CMD_DIR := cmd/claude-status
LDFLAGS := -ldflags "-H windowsgui"

.PHONY: all amd64 arm64 icons winres clean

all: icons winres amd64 arm64

# 生成图标
icons:
	@echo "=== 生成图标 ==="
	@chmod +x assets/generate.sh
	@cd assets && ./generate.sh

# 生成 Windows 资源（exe 图标）
winres:
	@echo "=== 生成 Windows 资源 ==="
	@mkdir -p $(CMD_DIR)/winres
	@convert -background none -density 384 assets/svg/icon-running-dark.svg -resize 256x256 -depth 8 PNG32:$(CMD_DIR)/winres/icon.png
	@cd $(CMD_DIR) && go-winres make --arch amd64,arm64

# 编译 Windows amd64
amd64:
	@echo "=== 编译 Windows amd64 ==="
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-amd64.exe ./$(CMD_DIR)

# 编译 Windows arm64
arm64:
	@echo "=== 编译 Windows arm64 ==="
	@mkdir -p $(BUILD_DIR)
	GOOS=windows GOARCH=arm64 go build $(LDFLAGS) -o $(BUILD_DIR)/$(APP_NAME)-arm64.exe ./$(CMD_DIR)

# 清理
clean:
	@echo "=== 清理 ==="
	rm -rf $(BUILD_DIR)
	rm -f assets/icons/*.ico
	rm -f $(CMD_DIR)/*.syso
	rm -f $(CMD_DIR)/winres/icon.png
