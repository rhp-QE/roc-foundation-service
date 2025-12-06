package frontier

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/roc/roc-foundation-util-go/cache"
)

// Server WebSocket服务器
type Server struct {
	serviceCtx ServiceContext
	hub        *Hub
	httpServer *http.Server
}

// ServiceContext 服务上下文接口（依赖倒置，解耦具体实现）
type ServiceContext interface {
	GetLocalAddress() string
	GetRedis() cache.Cache
	GetUserConnectionKey(userID string) string
	GetConnectionKey(connectionID string) string
	GetFrontierConfig() *Config
}

// NewServer 创建新的WebSocket服务器（从 ServiceContext 获取配置）
func NewServer(serviceCtx ServiceContext) *Server {
	if serviceCtx == nil {
		panic("ServiceContext cannot be nil")
	}

	hub := NewHub(serviceCtx)

	return &Server{
		serviceCtx: serviceCtx,
		hub:        hub,
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

	// 获取配置
	config := s.serviceCtx.GetFrontierConfig()
	if config == nil {
		config = DefaultConfig()
	}

	// 创建HTTP服务器
	addr := fmt.Sprintf("%s:%s", config.Host, config.Port)
	s.httpServer = &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  config.ReadTimeout,
		WriteTimeout: config.WriteTimeout,
	}

	klog.Infof("WebSocket server starting on %s", addr)

	// 启动服务器（阻塞调用，会一直运行直到调用 Shutdown）
	// ListenAndServe 只在以下情况返回：
	// 1. 调用 Shutdown() 时返回 http.ErrServerClosed（正常关闭）
	// 2. 启动失败时返回其他错误（如端口被占用）
	if config.EnableTLS {
		err := s.httpServer.ListenAndServeTLS(config.CertFile, config.KeyFile)
		if err != nil && err != http.ErrServerClosed {
			return fmt.Errorf("failed to start TLS server: %w", err)
		}
		// 正常关闭时，返回 nil 而不是错误
		return nil
	}

	err := s.httpServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("failed to start server: %w", err)
	}
	// 正常关闭时（Shutdown 被调用），返回 nil
	return nil
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
	config := s.serviceCtx.GetFrontierConfig()
	if config == nil {
		return DefaultConfig()
	}
	return config
}
