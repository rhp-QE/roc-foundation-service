package frontier

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Connection 表示一个WebSocket连接
type Connection struct {
	ID         string                 // 连接唯一标识
	UserID     string                 // 用户ID
	Conn       *websocket.Conn        // WebSocket连接
	Send       chan []byte            // 发送消息的channel
	Hub        *Hub                   // 所属的Hub
	Metadata   map[string]interface{} // 连接元数据
	mu         sync.RWMutex           // 保护元数据的锁
	sendMu     sync.RWMutex           // 保护发送队列关闭状态
	lastActive time.Time              // 最后活跃时间
	loginAt    time.Time              // 登录时间
	closeOnce  sync.Once              // 确保连接只关闭一次
	closed     bool                   // 发送队列是否已关闭
}

// NewConnection 创建新连接
func NewConnection(id, userID string, conn *websocket.Conn, hub *Hub) *Connection {
	return &Connection{
		ID:         id,
		UserID:     userID,
		Conn:       conn,
		Send:       make(chan []byte, 256),
		Hub:        hub,
		Metadata:   make(map[string]interface{}),
		lastActive: time.Now(),
		loginAt:    time.Now(),
	}
}

// ReadPump 处理从客户端接收消息
func (c *Connection) ReadPump() {
	defer func() {
		c.Close()
	}()

	// 设置读取超时
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))

	// 设置 Ping 处理器：当客户端发送 WebSocket Ping 帧时，自动回复 Pong 并更新活跃时间
	c.Conn.SetPingHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		c.UpdateLastActive()
		c.Hub.RefreshConnectionPresence(c)
		// gorilla/websocket 会自动回复 Pong 帧，这里只需要更新活跃时间
		return nil
	})

	// 设置 Pong 处理器：当客户端发送 WebSocket Pong 帧时，更新活跃时间
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		c.UpdateLastActive()
		c.Hub.RefreshConnectionPresence(c)
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				// 记录错误
			}
			break
		}

		c.UpdateLastActive()
		c.Hub.RefreshConnectionPresence(c)

		// 解析消息
		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			// 发送错误响应
			c.SendError("invalid message format")
			continue
		}

		// 处理消息
		go func() {
			c.Hub.HandleMessage(c, &msg)
		}()
	}
}

// WritePump 处理向客户端发送消息
func (c *Connection) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub关闭了channel
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			// 一条 WebSocket message 只承载一个 JSON envelope，避免客户端按单帧解析时失败。
			w.Write(message)

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// SendMessage 发送消息
func (c *Connection) SendMessage(msg *Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// 发送前检查关闭状态，避免 close(Send) 后并发写 channel 导致 panic。
	c.sendMu.RLock()
	defer c.sendMu.RUnlock()
	if c.closed {
		return ErrConnectionClosed
	}

	select {
	case c.Send <- data:
		return nil
	default:
		// channel已满，连接可能已经阻塞
		return ErrSendBufferFull
	}
}

// SendError 发送错误消息
func (c *Connection) SendError(errorMsg string) {
	msg := NewMessage(MessageTypeError)
	msg.Error = errorMsg
	c.SendMessage(msg)
}

// Close 关闭连接
func (c *Connection) Close() {
	c.closeOnce.Do(func() {
		// 先关闭发送队列，再通知 Hub 注销；重复 close 由 closeOnce 保证幂等。
		c.sendMu.Lock()
		c.closed = true
		close(c.Send)
		c.sendMu.Unlock()

		c.Hub.Unregister <- c
		c.Conn.Close()
	})
}

// UpdateLastActive 更新最后活跃时间
func (c *Connection) UpdateLastActive() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastActive = time.Now()
}

// GetLastActive 获取最后活跃时间
func (c *Connection) GetLastActive() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lastActive
}

// GetLoginAt 获取连接登录时间
func (c *Connection) GetLoginAt() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.loginAt
}

// SetMetadata 设置元数据
func (c *Connection) SetMetadata(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Metadata[key] = value
}

// GetMetadata 获取元数据
func (c *Connection) GetMetadata(key string) (interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	val, ok := c.Metadata[key]
	return val, ok
}

func (c *Connection) MetadataString(key string) string {
	val, ok := c.GetMetadata(key)
	if !ok || val == nil {
		return ""
	}
	if s, ok := val.(string); ok {
		return s
	}
	return ""
}

// IsAlive 检查连接是否活跃
func (c *Connection) IsAlive(timeout time.Duration) bool {
	return time.Since(c.GetLastActive()) < timeout
}
