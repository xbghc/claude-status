//go:build windows

package tray

import (
	"syscall"

	"golang.org/x/sys/windows/registry"
)

const (
	themeRegKey  = `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`
	themeRegName = `AppsUseLightTheme`
)

// IsDarkMode 检测当前是否为暗色模式
func IsDarkMode() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, themeRegKey, registry.QUERY_VALUE)
	if err != nil {
		return true // 默认暗色
	}
	defer k.Close()

	val, _, err := k.GetIntegerValue(themeRegName)
	if err != nil {
		return true
	}
	return val == 0
}

// MonitorThemeChange 监听主题变化，调用回调函数
func MonitorThemeChange(onChange func(isDark bool)) {
	advapi32, err := syscall.LoadDLL("Advapi32.dll")
	if err != nil {
		return
	}

	regNotifyChangeKeyValue, err := advapi32.FindProc("RegNotifyChangeKeyValue")
	if err != nil {
		return
	}

	go func() {
		k, err := registry.OpenKey(registry.CURRENT_USER, themeRegKey, syscall.KEY_NOTIFY|registry.QUERY_VALUE)
		if err != nil {
			return
		}
		defer k.Close()

		var lastVal uint64 = 2 // 初始无效值
		for {
			// REG_NOTIFY_CHANGE_LAST_SET = 0x00000004
			regNotifyChangeKeyValue.Call(uintptr(k), 0, 0x00000004, 0, 0)

			val, _, err := k.GetIntegerValue(themeRegName)
			if err != nil {
				continue
			}

			if val != lastVal {
				lastVal = val
				onChange(val == 0)
			}
		}
	}()
}
