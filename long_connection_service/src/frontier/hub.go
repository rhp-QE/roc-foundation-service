package frontier

import (
	"sync"
	"time"
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
func NewHub() *Hub {
	return &Hub{
		connections:       make(map[string]*Connection),
		userConnections:   make(map[string]map[string]*Connection),
		Register:          make(chan *Connection),
		Unregister:        make(chan *Connection),
		messageHandlers:   make(map[string]MessageHandler),
		heartbeatInterval: 30 * time.Second,
		connectionTimeout: 90 * time.Second,
		stopChan:          make(chan struct{}),
		stats:             &HubStats{},
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
}

// unregisterConnection 注销连接
func (h *Hub) unregisterConnection(conn *Connection) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// 从连接映射中删除
	delete(h.connections, conn.ID)

	// 从用户连接映射中删除
	if conn.UserID != "" {
		if userConns, ok := h.userConnections[conn.UserID]; ok {
			delete(userConns, conn.ID)
			if len(userConns) == 0 {
				delete(h.userConnections, conn.UserID)
			}
		}
	}

	// 更新统计
	h.stats.mu.Lock()
	h.stats.ActiveConnections--
	h.stats.mu.Unlock()
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
		response.SetPayload("status", "ok")
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
