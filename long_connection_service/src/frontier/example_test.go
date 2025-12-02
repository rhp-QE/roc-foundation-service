package frontier

import (
	"fmt"
	"testing"
	"time"
)

// 这是一个示例，展示如何使用WebSocket长连接服务

func ExampleServer() {
	// 创建配置
	config := DefaultConfig()
	config.Host = "0.0.0.0"
	config.Port = "8080"
	config.HeartbeatInterval = 30 * time.Second
	config.ConnectionTimeout = 90 * time.Second

	// 创建服务器
	server := NewServer(config)

	// 获取Hub实例，可以注册自定义消息处理器
	hub := server.GetHub()

	// 注册自定义消息处理器
	hub.RegisterMessageHandler("custom_type", func(conn *Connection, msg *Message) error {
		fmt.Printf("Received custom message from user %s: %v\n", conn.UserID, msg.Payload)

		// 回复消息
		response := NewMessage("custom_response")
		response.SetPayload("status", "received")
		return conn.SendMessage(response)
	})

	// 启动服务器
	if err := server.Start(); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}

func TestMessageCreation(t *testing.T) {
	// 创建请求消息
	msg := NewRequestMessage("chat", "sendMessage")
	msg.ID = "msg_123"
	msg.SetPayload("text", "Hello, World!")
	msg.SetPayload("to", "user_2")

	// 验证消息
	if err := msg.Validate(); err != nil {
		t.Errorf("Message validation failed: %v", err)
	}

	// 验证 service 和 method
	if msg.Service != "chat" {
		t.Errorf("Expected service 'chat', got '%s'", msg.Service)
	}
	if msg.Method != "sendMessage" {
		t.Errorf("Expected method 'sendMessage', got '%s'", msg.Method)
	}

	// 获取载荷数据
	text, ok := msg.GetPayloadString("text")
	if !ok || text != "Hello, World!" {
		t.Error("Failed to get message text")
	}
}

func TestHub(t *testing.T) {
	// 创建Hub
	hub := NewHub()

	// 验证初始状态
	if hub.GetActiveConnectionCount() != 0 {
		t.Error("New hub should have 0 connections")
	}

	// 测试统计信息
	stats := hub.GetStats()
	if stats.ActiveConnections != 0 {
		t.Error("Initial active connections should be 0")
	}
}

func TestConfig(t *testing.T) {
	// 测试默认配置
	config := DefaultConfig()

	if config.Port != "8080" {
		t.Errorf("Expected port 8080, got %s", config.Port)
	}

	// 测试配置验证
	if err := config.Validate(); err != nil {
		t.Errorf("Config validation failed: %v", err)
	}

	// 测试空配置的验证
	emptyConfig := &Config{}
	if err := emptyConfig.Validate(); err != nil {
		t.Errorf("Empty config validation failed: %v", err)
	}

	if emptyConfig.Port != "8080" {
		t.Error("Empty config should default to port 8080")
	}
}
