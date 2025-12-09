package frontier

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/cloudwego/kitex/client"
	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/kitex-contrib/obs-opentelemetry/tracing"
	back "github.com/rhp-QE/roc-foundation-service/long_connection_service/kitex_gen/back"
	backservice "github.com/rhp-QE/roc-foundation-service/long_connection_service/kitex_gen/back/backservice"
	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/util"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// 在生产环境中应该检查 Origin
		return true
	},
}

// WebSocketHandler WebSocket处理器
type WebSocketHandler struct {
	hub *Hub
}

// NewWebSocketHandler 创建WebSocket处理器
func NewWebSocketHandler(hub *Hub) *WebSocketHandler {
	return &WebSocketHandler{
		hub: hub,
	}
}

// ServeHTTP 处理WebSocket连接请求
func (h *WebSocketHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 升级HTTP连接为WebSocket连接
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Failed to upgrade connection", http.StatusBadRequest)
		return
	}

	// 从请求中获取验证信息（token、track_id等）放到元数据中
	token := r.URL.Query().Get("token")
	trackID := r.URL.Query().Get("track_id")
	userID := r.URL.Query().Get("user_id") // 临时处理，实际应该从token解析

	// 创建连接对象
	connectionID := uuid.New().String()
	connection := NewConnection(connectionID, userID, conn, h.hub)

	// 如果连接时有验证信息，保存到连接的元数据中
	if token != "" {
		connection.SetMetadata("token", token)
	}
	if trackID != "" {
		connection.SetMetadata("track_id", trackID)
	}

	// 注册连接
	h.hub.Register <- connection

	// 发送欢迎消息（网关不解析payload，由业务方处理）
	welcomeMsg := NewMessage(MessageTypePush)
	connection.SendMessage(welcomeMsg)

	// 启动读写协程
	go connection.WritePump()
	go connection.ReadPump()
}

// HandleAuth 处理认证请求
// 注意：网关不解析payload，认证逻辑应由业务方处理
// token、track_id等验证信息应该放在metadata中，payload是业务数据
func (h *WebSocketHandler) HandleAuth(conn *Connection, msg *Message) error {
	// 从metadata中获取验证信息（token、track_id等）
	// 网关只负责转发，不解析payload内容
	// 业务方应该从 msg.GetMetadata() 中获取验证信息，从 msg.Payload 中获取业务数据

	// 将metadata中的验证信息保存到连接的元数据中
	if token, ok := msg.GetMetadata("token"); ok {
		conn.SetMetadata("token", token)
	}
	if trackID, ok := msg.GetMetadata("track_id"); ok {
		conn.SetMetadata("track_id", trackID)
	}
	if userID, ok := msg.GetMetadata("user_id"); ok {
		conn.UserID = userID
	}

	response := NewMessage(MessageTypeAuth)
	return conn.SendMessage(response)
}

// HandleMessage 处理RPC请求消息（网关模式）
func (h *WebSocketHandler) HandleMessage(conn *Connection, msg *Message) error {
	// 验证请求
	if err := h.validateRequest(msg); err != nil {
		return err
	}

	// 获取服务上下文
	serviceCtx := h.hub.GetServiceContext()
	if serviceCtx == nil {
		response := NewErrorMessage(msg.RequestID, "service context not available")
		return conn.SendMessage(response)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. 检查服务是否注册
	if err := h.checkServiceRegistered(ctx, serviceCtx, msg); err != nil {
		response := NewErrorMessage(msg.RequestID, err.Error())
		return conn.SendMessage(response)
	}

	// 2. 获取服务实例并调用后端服务
	callResp, err := h.callBackendService(ctx, serviceCtx, msg, conn)
	if err != nil {
		response := NewErrorMessage(msg.RequestID, err.Error())
		return conn.SendMessage(response)
	}

	// 3. 转换响应并返回
	response := CallResponseToFontierMessage(callResp, msg.RequestID)
	return conn.SendMessage(response)
}

// validateRequest 验证请求消息
func (h *WebSocketHandler) validateRequest(msg *Message) error {
	if msg.Service == "" {
		return ErrServiceNotFound
	}
	if msg.Method == "" {
		return ErrMethodNotFound
	}
	return nil
}

// callBackendService 获取服务实例并调用后端服务
func (h *WebSocketHandler) callBackendService(ctx context.Context, serviceCtx ServiceContext, msg *Message, conn *Connection) (*back.CallResponse, error) {
	// 获取服务实例
	discoveryClient := serviceCtx.GetDiscovery()
	if discoveryClient == nil {
		return nil, fmt.Errorf("service discovery not available")
	}

	instance, err := discoveryClient.GetInstance(ctx, msg.Service)
	if err != nil {
		klog.Errorf("Failed to discover service instance for %s: %v", msg.Service, err)
		return nil, fmt.Errorf("failed to discover service: %v", err)
	}

	hostPort := fmt.Sprintf("%s:%d", instance.Host, instance.Port)

	// 创建 backservice 客户端
	backServiceClient, err := backservice.NewClient(
		msg.Service,
		client.WithHostPorts(hostPort),
		client.WithSuite(tracing.NewClientSuite()),
		client.WithClientBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: msg.Service}),
	)
	if err != nil {
		klog.Errorf("Failed to create backservice client for %s: %v", hostPort, err)
		return nil, fmt.Errorf("failed to create client: %v", err)
	}

	// 构建 CallRequest
	callReq := MessageToCallRequest(msg, conn)

	// 调用后端服务
	callResp, err := backServiceClient.Call(ctx, callReq)
	if err != nil {
		klog.Errorf("Failed to call backservice %s.%s: %v", msg.Service, msg.Method, err)
		return nil, fmt.Errorf("service call failed: %v", err)
	}

	return callResp, nil
}

// checkServiceRegistered 检查服务和方法是否已注册
func (h *WebSocketHandler) checkServiceRegistered(ctx context.Context, serviceCtx ServiceContext, msg *Message) error {
	// 获取 Redis 客户端
	redisCache := serviceCtx.GetRedis()
	if redisCache == nil {
		return fmt.Errorf("redis is not available")
	}

	// 构建 Redis key
	key := util.GetServiceKeyInCache(msg.Service)

	// 检查服务是否存在
	exists, err := redisCache.Exists(ctx, key)
	if err != nil {
		klog.Errorf("Failed to check service existence: %v", err)
		return fmt.Errorf("failed to check service existence: %w", err)
	}

	if exists == 0 {
		return fmt.Errorf("service %s with method %s is not registered", msg.Service, msg.Method)
	}

	// 检查 method 是否在 Set 中
	isMember, err := redisCache.SIsMember(ctx, key, msg.Method)
	if err != nil {
		klog.Errorf("Failed to check method: %v", err)
		return fmt.Errorf("failed to check method: %w", err)
	}

	// 检查是否支持所有方法（"*" 标记）
	allMethods, err := redisCache.SIsMember(ctx, key, "*")
	if err != nil {
		klog.Errorf("Failed to check all methods flag: %v", err)
		return fmt.Errorf("failed to check all methods flag: %w", err)
	}

	// 如果支持所有方法，或者指定的 method 在 Set 中，则返回 nil（已注册）
	if allMethods || isMember {
		return nil
	}

	return fmt.Errorf("service %s with method %s is not registered", msg.Service, msg.Method)
}
