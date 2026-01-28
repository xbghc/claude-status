package logger

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

var (
	logFile    *os.File
	logLogger  *log.Logger
	debugMode  bool
)

// Init 初始化日志文件
func Init() error {
	// 获取可执行文件目录
	exePath, err := os.Executable()
	if err != nil {
		exePath = "."
	}
	exeDir := filepath.Dir(exePath)

	// 创建日志文件
	logPath := filepath.Join(exeDir, "claude-status.log")
	logFile, err = os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("failed to open log file: %w", err)
	}

	logLogger = log.New(logFile, "", log.LstdFlags)
	Info("=== Claude Status Monitor Started ===")
	Info("Log file: %s", logPath)
	Info("Time: %s", time.Now().Format(time.RFC3339))

	return nil
}

// Close 关闭日志文件
func Close() {
	if logFile != nil {
		Info("=== Claude Status Monitor Stopped ===")
		logFile.Close()
	}
}

// Info 记录信息
func Info(format string, args ...interface{}) {
	if logLogger != nil {
		logLogger.Printf("[INFO] "+format, args...)
	}
}

// Error 记录错误
func Error(format string, args ...interface{}) {
	if logLogger != nil {
		logLogger.Printf("[ERROR] "+format, args...)
	}
}

// SetDebug 设置调试模式
func SetDebug(enabled bool) {
	debugMode = enabled
	if enabled {
		Info("Debug mode enabled")
	}
}

// Debug 记录调试信息（仅在 debug 模式下输出）
func Debug(format string, args ...interface{}) {
	if logLogger != nil && debugMode {
		logLogger.Printf("[DEBUG] "+format, args...)
	}
}
