package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/frontier"
)

func main() {
	// 命令行参数
	host := flag.String("host", "0.0.0.0", "Server host")
	port := flag.String("port", "8080", "Server port")
	enableTLS := flag.Bool("tls", false, "Enable TLS")
	certFile := flag.String("cert", "", "TLS certificate file")
	keyFile := flag.String("key", "", "TLS key file")
	flag.Parse()

	// 创建配置
	config := &frontier.Config{
		Host:              *host,
		Port:              *port,
		HeartbeatInterval: 30 * time.Second,
		ConnectionTimeout: 90 * time.Second,
		MaxMessageSize:    512 * 1024,
		SendBufferSize:    256,
		ReceiveBufferSize: 1024,
		EnableTLS:         *enableTLS,
		CertFile:          *certFile,
		KeyFile:           *keyFile,
		MaxConnections:    10000,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      10 * time.Second,
	}

	// 创建服务器
	server := frontier.NewServer(config)

	// 获取Hub并注册自定义消息处理器
	hub := server.GetHub()
	registerCustomHandlers(hub)

	// 启动服务器
	go func() {
		log.Printf("Starting WebSocket server on %s:%s", config.Host, config.Port)
		if err := server.Start(); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// 优雅关闭
	if err := server.Stop(); err != nil {
		log.Printf("Error stopping server: %v", err)
	}

	log.Println("Server stopped")
}

// registerCustomHandlers 注册自定义消息处理器
func registerCustomHandlers(hub *frontier.Hub) {
	// 示例：注册自定义消息处理器
	hub.RegisterMessageHandler("ping", func(conn *frontier.Connection, msg *frontier.Message) error {
		log.Printf("Received ping from user %s", conn.UserID)

		// 回复 pong
		response := frontier.NewMessage("pong")
		response.SetPayload("timestamp", time.Now().Unix())
		return conn.SendMessage(response)
	})

	// 示例：群组消息处理
	hub.RegisterMessageHandler("group_message", func(conn *frontier.Connection, msg *frontier.Message) error {
		groupID, ok := msg.GetPayloadString("group_id")
		if !ok {
			return frontier.ErrInvalidMessageType
		}

		log.Printf("User %s sending message to group %s", conn.UserID, groupID)

		// TODO: 实现群组消息分发逻辑
		// 这里需要维护群组成员列表，然后向所有成员发送消息

		// 发送响应
		response := frontier.NewResponseMessage(msg.ID)
		response.SetPayload("status", "sent_to_group")
		response.SetPayload("group_id", groupID)
		return conn.SendMessage(response)
	})

	// 示例：状态更新处理
	hub.RegisterMessageHandler("status_update", func(conn *frontier.Connection, msg *frontier.Message) error {
		status, ok := msg.GetPayloadString("status")
		if !ok {
			return frontier.ErrInvalidMessageType
		}

		log.Printf("User %s status updated to: %s", conn.UserID, status)

		// 保存状态到连接元数据
		conn.SetMetadata("status", status)

		// 广播状态更新给相关用户
		// TODO: 实现好友列表获取和状态推送

		return nil
	})
}
