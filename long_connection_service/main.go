package main

import (
	"log"
	"os"

	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/start"
)

func main() {
	// 获取配置文件路径（支持环境变量）
	configPath := os.Getenv("LONG_CONNECTION_CONFIG")
	if configPath == "" {
		configPath = "config.yaml"
	}

	// 启动服务
	if err := start.Start(configPath); err != nil {
		log.Fatalf("Failed to start long connection service: %v", err)
	}
}
