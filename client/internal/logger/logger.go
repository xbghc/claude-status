package logger

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"
)

const maxLogLines = 1000

// truncateLogFile 截断日志文件，只保留最近 maxLines 行
func truncateLogFile(path string, maxLines int) {
	file, err := os.Open(path)
	if err != nil {
		return // 文件不存在或无法读取，跳过
	}
	defer file.Close()

	// 读取所有行
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	// 如果行数未超过限制，无需截断
	if len(lines) <= maxLines {
		return
	}

	// 只保留最后 maxLines 行
	lines = lines[len(lines)-maxLines:]

	// 重写文件
	file.Close()
	outFile, err := os.Create(path)
	if err != nil {
		return
	}
	defer outFile.Close()

	for _, line := range lines {
		outFile.WriteString(line + "\n")
	}
}

var (
	logFile     *os.File
	logLogger   *log.Logger
	debugMode   bool
	logFilePath string
)

// GetLogPath 返回日志文件路径
func GetLogPath() string {
	return logFilePath
}

// Init 初始化日志文件
func Init() error {
	// 使用 %APPDATA%/claude-status 目录存放日志
	logDir := getLogDir()

	// 日志文件路径
	logPath := filepath.Join(logDir, "claude-status.log")
	logFilePath = logPath

	// 截断日志文件，只保留最近 N 行
	truncateLogFile(logPath, maxLogLines)

	// 创建日志文件
	var err error
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

// getLogDir returns the directory for log files under %APPDATA%/claude-status.
func getLogDir() string {
	appData := os.Getenv("APPDATA")
	if appData == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "."
		}
		appData = filepath.Join(home, "AppData", "Roaming")
	}
	dir := filepath.Join(appData, "claude-status")
	os.MkdirAll(dir, 0755)
	return dir
}
