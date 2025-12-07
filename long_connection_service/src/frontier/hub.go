package frontier

import (
	"context"
	"sync"
	"time"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/util"
)

// Hub 维护所有活跃的连接并处理消息分发
type Hub struct {
	// 注册的连接集合
	connections map[string]*Connection

	// 用户ID到连接的映射（一个用户可能有多个连接）
	userConnections map[string]map[string]*Connection

	// 注册连接的channel
	Register chan *Connection

	// 注销连接的channel
	Unregister chan *Connection

	// 消息处理函数
	messageHandlers map[string]MessageHandler

	// 保护连接映射的锁
	mu sync.RWMutex

	// 心跳检查间隔
	heartbeatInterval time.Duration

	// 连接超时时间
	connectionTimeout time.Duration

	// 停止信号
	stopChan chan struct{}

	// 统计信息
	stats *HubStats

	// 服务上下文（可选，用于存储连接状态）
	serviceCtx ServiceContext

	// 实际的 ServiceContext 结构体（用于直接访问 Registry 和 Discovery）
	// 通过类型断言访问实际的 ServiceContext 结构体字段
	actualServiceCtx interface{}
}

// HubStats Hub统计信息
type HubStats struct {
	mu                sync.RWMutex
	TotalConnections  int64
	ActiveConnections int64
	MessagesReceived  int64
	MessagesSent      int64
}

// MessageHandler 消息处理函数
type MessageHandler func(*Connection, *Message) error

// NewHub 创建新的Hub
func NewHub(serviceCtx ServiceContext) *Hub {
	if serviceCtx == nil {
		panic("ServiceContext cannot be nil")
	}

	// 从 ServiceContext 获取配置
	config := serviceCtx.GetFrontierConfig()
	if config == nil {
		config = DefaultConfig()
	}

	if err := config.Validate(); err != nil {
		panic(err)
	}

	heartbeatInterval := 30 * time.Second
	connectionTimeout := 90 * time.Second

	if config.HeartbeatInterval > 0 {
		heartbeatInterval = config.HeartbeatInterval
	}
	if config.ConnectionTimeout > 0 {
		connectionTimeout = config.ConnectionTimeout
	}

	return &Hub{
		connections:       make(map[string]*Connection),
		userConnections:   make(map[string]map[string]*Connection),
		Register:          make(chan *Connection),
		Unregister:        make(chan *Connection),
		messageHandlers:   make(map[string]MessageHandler),
		heartbeatInterval: heartbeatInterval,
		connectionTimeout: connectionTimeout,
		stopChan:          make(chan struct{}),
		stats:             &HubStats{},
		serviceCtx:        serviceCtx,
	}
}

// Run 启动Hub
func (h *Hub) Run() {
	// 启动心跳检查
	go h.checkHeartbeat()

	for {
		select {
		case conn := <-h.Register:
			h.registerConnection(conn)

		case conn := <-h.Unregister:
			h.unregisterConnection(conn)

		case <-h.stopChan:
			return
		}
	}
}

// Stop 停止Hub
func (h *Hub) Stop() {
	close(h.stopChan)

	// 关闭所有连接
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, conn := range h.connections {
		conn.Close()
	}
}

// registerConnection 注册新连接
func (h *Hub) registerConnection(conn *Connection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// 添加到连接映射
	h.connections[conn.ID] = conn

	// 添加到用户连接映射
	if conn.UserID != "" {
		if _, ok := h.userConnections[conn.UserID]; !ok {
			h.userConnections[conn.UserID] = make(map[string]*Connection)
		}
		h.userConnections[conn.UserID][conn.ID] = conn
	}

	// 更新统计
	h.stats.mu.Lock()
	h.stats.TotalConnections++
	h.stats.ActiveConnections++
	h.stats.mu.Unlock()

	// 更新 Redis（如果配置了 ServiceContext）
	if h.serviceCtx != nil && conn.UserID != "" {
		go h.updateRedisOnRegister(context.Background(), conn)
	}
}

// updateRedisOnRegister 在 Redis 中注册连接
func (h *Hub) updateRedisOnRegister(ctx context.Context, conn *Connection) {
	if h.serviceCtx == nil {
		return
	}

	redis := h.serviceCtx.GetRedis()
	if redis == nil {
		return
	}

	localAddress := h.serviceCtx.GetLocalAddress()

	// 1. 在用户连接 Hash 中添加 connectionID -> 机器地址映射
	userKey := util.GetUserConnectionKeyInCache(conn.UserID)
	err := redis.HSet(ctx, userKey, conn.ID, localAddress)
	if err != nil {
		klog.Errorf("Failed to update user connection in Redis for user %s: %v", conn.UserID, err)
		return
	}

	// 2. 在连接 key 中存储连接所在的机器地址
	connectionKey := util.GetConnectionKeyInCache(conn.ID)
	err = redis.Set(ctx, connectionKey, localAddress, 24*time.Hour)
	if err != nil {
		klog.Errorf("Failed to set connection address in Redis for connection %s: %v", conn.ID, err)
		return
	}

	klog.Infof("Registered connection %s (user: %s) in Redis at %s", conn.ID, conn.UserID, localAddress)
}

// unregisterConnection 注销连接
func (h *Hub) unregisterConnection(conn *Connection) {
	h.mu.Lock()
	userID := conn.UserID
	connectionID := conn.ID
	h.mu.Unlock()

	// 先更新 Redis（如果配置了 ServiceContext），避免并发问题
	if h.serviceCtx != nil && userID != "" {
		go h.updateRedisOnUnregister(context.Background(), userID, connectionID)
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// 从连接映射中删除
	delete(h.connections, conn.ID)

	// 从用户连接映射中删除
	if userID != "" {
		if userConns, ok := h.userConnections[userID]; ok {
			delete(userConns, conn.ID)
			if len(userConns) == 0 {
				delete(h.userConnections, userID)
			}
		}
	}

	// 更新统计
	h.stats.mu.Lock()
	h.stats.ActiveConnections--
	h.stats.mu.Unlock()
}

// updateRedisOnUnregister 在 Redis 中注销连接
func (h *Hub) updateRedisOnUnregister(ctx context.Context, userID, connectionID string) {
	if h.serviceCtx == nil {
		return
	}

	redis := h.serviceCtx.GetRedis()
	if redis == nil {
		return
	}

	// 1. 从用户连接 Hash 中删除 connectionID
	userKey := util.GetUserConnectionKeyInCache(userID)
	_, err := redis.HDel(ctx, userKey, connectionID)
	if err != nil {
		klog.Errorf("Failed to remove connection from user hash in Redis for user %s: %v", userID, err)
	}

	// 2. 删除连接的机器地址 key
	connectionKey := util.GetConnectionKeyInCache(connectionID)
	err = redis.Delete(ctx, connectionKey)
	if err != nil {
		klog.Errorf("Failed to delete connection address from Redis for connection %s: %v", connectionID, err)
		return
	}

	klog.Infof("Unregistered connection %s (user: %s) from Redis", connectionID, userID)
}

// GetConnection 根据连接ID获取连接
func (h *Hub) GetConnection(connID string) (*Connection, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	conn, ok := h.connections[connID]
	return conn, ok
}

// GetUserConnections 获取用户的所有连接
func (h *Hub) GetUserConnections(userID string) []*Connection {
	h.mu.RLock()
	defer h.mu.RUnlock()

	userConns, ok := h.userConnections[userID]
	if !ok {
		return nil
	}

	conns := make([]*Connection, 0, len(userConns))
	for _, conn := range userConns {
		conns = append(conns, conn)
	}
	return conns
}

// SendToUser 向指定用户的所有连接发送消息
func (h *Hub) SendToUser(userID string, msg *Message) error {
	conns := h.GetUserConnections(userID)
	if len(conns) == 0 {
		return ErrConnectionClosed
	}

	h.stats.mu.Lock()
	h.stats.MessagesSent += int64(len(conns))
	h.stats.mu.Unlock()

	for _, conn := range conns {
		if err := conn.SendMessage(msg); err != nil {
			// 记录错误但继续发送给其他连接
			continue
		}
	}
	return nil
}

// SendToConnection 向指定连接发送消息
func (h *Hub) SendToConnection(connID string, msg *Message) error {
	conn, ok := h.GetConnection(connID)
	if !ok {
		return ErrConnectionClosed
	}

	h.stats.mu.Lock()
	h.stats.MessagesSent++
	h.stats.mu.Unlock()

	return conn.SendMessage(msg)
}

// Broadcast 广播消息给所有连接
func (h *Hub) Broadcast(msg *Message) {
	h.mu.RLock()
	conns := make([]*Connection, 0, len(h.connections))
	for _, conn := range h.connections {
		conns = append(conns, conn)
	}
	h.mu.RUnlock()

	h.stats.mu.Lock()
	h.stats.MessagesSent += int64(len(conns))
	h.stats.mu.Unlock()

	for _, conn := range conns {
		conn.SendMessage(msg)
	}
}

// RegisterMessageHandler 注册消息处理函数
func (h *Hub) RegisterMessageHandler(msgType string, handler MessageHandler) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.messageHandlers[msgType] = handler
}

// HandleMessage 处理消息
func (h *Hub) HandleMessage(conn *Connection, msg *Message) {
	h.stats.mu.Lock()
	h.stats.MessagesReceived++
	h.stats.mu.Unlock()

	// 验证消息
	if err := msg.Validate(); err != nil {
		conn.SendError(err.Error())
		return
	}

	// 获取消息处理函数
	h.mu.RLock()
	handler, ok := h.messageHandlers[msg.Type]
	h.mu.RUnlock()

	if !ok {
		// 默认处理
		h.defaultHandler(conn, msg)
		return
	}

	// 执行处理函数
	if err := handler(conn, msg); err != nil {
		conn.SendError(err.Error())
	}
}

// defaultHandler 默认消息处理
func (h *Hub) defaultHandler(conn *Connection, msg *Message) {
	switch msg.Type {
	case MessageTypeHeartbeat:
		// 心跳响应
		response := NewMessage(MessageTypeHeartbeat)
		conn.SendMessage(response)

	default:
		conn.SendError("unsupported message type: " + msg.Type)
	}
}

// checkHeartbeat 检查连接心跳
func (h *Hub) checkHeartbeat() {
	ticker := time.NewTicker(h.heartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			h.mu.RLock()
			conns := make([]*Connection, 0)
			for _, conn := range h.connections {
				if !conn.IsAlive(h.connectionTimeout) {
					conns = append(conns, conn)
				}
			}
			h.mu.RUnlock()

			// 关闭超时的连接
			for _, conn := range conns {
				conn.Close()
			}

		case <-h.stopChan:
			return
		}
	}
}

// GetStats 获取统计信息
func (h *Hub) GetStats() HubStats {
	h.stats.mu.RLock()
	defer h.stats.mu.RUnlock()
	return HubStats{
		TotalConnections:  h.stats.TotalConnections,
		ActiveConnections: h.stats.ActiveConnections,
		MessagesReceived:  h.stats.MessagesReceived,
		MessagesSent:      h.stats.MessagesSent,
	}
}

// GetActiveConnectionCount 获取活跃连接数
func (h *Hub) GetActiveConnectionCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.connections)
}

// GetServiceContext 获取服务上下文
func (h *Hub) GetServiceContext() ServiceContext {
	return h.serviceCtx
}

// GetActualServiceContext 获取实际的 ServiceContext 结构体（用于直接访问字段）
func (h *Hub) GetActualServiceContext() interface{} {
	return h.actualServiceCtx
}
