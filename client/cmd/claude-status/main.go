//go:build windows

package main

import (
	"flag"
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"claude-status/internal/app"
	"claude-status/internal/config"
	"claude-status/internal/tray"
)

var (
	configPath   = flag.String("config", "", "配置文件路径")
	uninstallCmd = flag.Bool("uninstall", false, "卸载服务器上的脚本和 Hook 配置后退出")
)

func main() {
	flag.Parse()
	cp := *configPath
	if cp == "" {
		cp = config.DefaultConfigPath()
	}

	if *uninstallCmd {
		runUninstallFlow(cp)
		return
	}

	ui := tray.NewApp()
	app.Run(cp, ui)
}

// runUninstallFlow 执行一次性卸载流程：
//   - 尝试附加父进程控制台，便于在 PowerShell/CMD 中看到日志
//   - 执行服务端卸载
//   - 通过 MessageBox 给出结果反馈（双击运行时也能看到结果）
func runUninstallFlow(configPath string) {
	attachParentConsole()

	err := app.RunUninstall(configPath)
	if err != nil {
		msg := "卸载失败: " + err.Error()
		fmt.Fprintln(os.Stderr, msg)
		showMessageBox("Claude Status 卸载", msg, true)
		os.Exit(1)
	}

	msg := "服务端卸载完成\n\n已移除 ~/.claude/settings.json 中的 status-hook\n已删除 ~/.claude-status 目录"
	fmt.Println("服务端卸载完成")
	showMessageBox("Claude Status 卸载", msg, false)
}

// attachParentConsole 将进程附加到父进程的控制台，
// 让 -H windowsgui 构建的二进制在从终端启动时也能输出文本。
// 若没有父控制台（例如双击运行），静默返回，依赖 MessageBox 给用户反馈。
func attachParentConsole() {
	const attachParentProcess = ^uintptr(0) // -1
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	attachConsole := kernel32.NewProc("AttachConsole")

	if r1, _, _ := attachConsole.Call(attachParentProcess); r1 == 0 {
		return
	}

	// 通过 CONOUT$ 重新打开 stdout/stderr，指向刚附加上的控制台
	if f, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
		os.Stdout = f
		os.Stderr = f
	}
}

// showMessageBox 弹出 Windows 系统 MessageBox
func showMessageBox(title, message string, isError bool) {
	titlePtr, err := syscall.UTF16PtrFromString(title)
	if err != nil {
		return
	}
	msgPtr, err := syscall.UTF16PtrFromString(message)
	if err != nil {
		return
	}

	// MB_OK | (MB_ICONERROR | MB_ICONINFORMATION)
	var flags uintptr = 0x40 // MB_ICONINFORMATION
	if isError {
		flags = 0x10 // MB_ICONERROR
	}

	user32 := syscall.NewLazyDLL("user32.dll")
	messageBox := user32.NewProc("MessageBoxW")
	messageBox.Call(
		0,
		uintptr(unsafe.Pointer(msgPtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		flags,
	)
}
