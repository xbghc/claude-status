//go:build windows

package main

import (
	"flag"

	"claude-status/internal/app"
	"claude-status/internal/config"
	"claude-status/internal/tray"
)

var configPath = flag.String("config", "", "配置文件路径")

func main() {
	flag.Parse()
	cp := *configPath
	if cp == "" {
		cp = config.DefaultConfigPath()
	}
	ui := tray.NewApp()
	app.Run(cp, ui)
}
