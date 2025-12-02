package frontier

import (
	"context"
	"fmt"
	"net/http"
	"time"
)

// Server WebSocket服务器
type Server struct {
	config     *Config
	hub        *Hub
	httpServer *http.Server
}

// NewServer 创建新的WebSocket服务器
func NewServer(config *Config) *Server {
	if config == nil {
		config = DefaultConfig()
	}

	if err := config.Validate(); err != nil {
		panic(err)
	}

	hub := NewHub()
	hub.heartbeatInterval = config.HeartbeatInterval
	hub.connectionTimeout = config.ConnectionTimeout

	return &Server{
		config: config,
		hub:    hub,
	}
}

// Start 启动服务器
func (s *Server) Start() error {
	// 启动Hub
	go s.hub.Run()

	// 设置路由
	mux := http.NewServeMux()

	// WebSocket连接处理
	wsHandler := NewWebSocketHandler(s.hub)
	mux.Handle("/ws", wsHandler)

	// 注册消息处理器
	s.registerMessageHandlers(wsHandler)

	// 统计信息
	statsHandler := NewStatsHandler(s.hub)
	mux.Handle("/stats", statsHandler)

	// 健康检查
	healthHandler := NewHealthHandler(s.hub)
	mux.Handle("/health", healthHandler)

	// 创建HTTP服务器
	addr := fmt.Sprintf("%s:%s", s.config.Host, s.config.Port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  s.config.ReadTimeout,
		WriteTimeout: s.config.WriteTimeout,
	}

	fmt.Printf("WebSocket server starting on %s\n", addr)

	// 启动服务器
	if s.config.EnableTLS {
		return s.httpServer.ListenAndServeTLS(s.config.CertFile, s.config.KeyFile)
	}
	return s.httpServer.ListenAndServe()
}

// Stop 停止服务器
func (s *Server) Stop() error {
	// 停止Hub
	s.hub.Stop()

	// 停止HTTP服务器
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return s.httpServer.Shutdown(ctx)
}

// registerMessageHandlers 注册消息处理器
func (s *Server) registerMessageHandlers(wsHandler *WebSocketHandler) {
	s.hub.RegisterMessageHandler(MessageTypeAuth, wsHandler.HandleAuth)
	s.hub.RegisterMessageHandler(MessageTypeRequest, wsHandler.HandleMessage)
}

// GetHub 获取Hub实例
func (s *Server) GetHub() *Hub {
	return s.hub
}

// GetConfig 获取配置
func (s *Server) GetConfig() *Config {
	return s.config
}
