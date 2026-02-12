//go:build windows

package main

import (
	"flag"

	"claude-status/internal/app"
	"claude-status/internal/config"
)

var configPath = flag.String("config", "", "配置文件路径")

func main() {
	flag.Parse()
	cp := *configPath
	if cp == "" {
		cp = config.DefaultConfigPath()
	}
	app.Run(cp)
}
