//go:build windows

package main

import (
	"flag"

	"claude-status/internal/app"
)

var configPath = flag.String("config", "config.yaml", "配置文件路径")

func main() {
	flag.Parse()
	app.Run(*configPath)
}
