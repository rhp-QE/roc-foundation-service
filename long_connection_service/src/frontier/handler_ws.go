package frontier

import (
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
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

	// 从请求中获取用户信息（这里简化处理，实际应该从token中解析）
	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		conn.Close()
		return
	}

	// 创建连接对象
	connectionID := uuid.New().String()
	connection := NewConnection(connectionID, userID, conn, h.hub)

	// 注册连接
	h.hub.Register <- connection

	// 发送欢迎消息
	welcomeMsg := NewMessage(MessageTypePush)
	welcomeMsg.SetPayload("message", "Connected successfully")
	welcomeMsg.SetPayload("connection_id", connectionID)
	connection.SendMessage(welcomeMsg)

	// 启动读写协程
	go connection.WritePump()
	go connection.ReadPump()
}

// HandleAuth 处理认证请求
func (h *WebSocketHandler) HandleAuth(conn *Connection, msg *Message) error {
	token, ok := msg.GetPayloadString("token")
	if !ok {
		return ErrUnauthorized
	}

	// TODO: 验证token
	_ = token

	// 更新连接的用户ID
	userID, ok := msg.GetPayloadString("user_id")
	if !ok {
		return ErrUnauthorized
	}

	conn.UserID = userID

	// 发送认证成功消息
	response := NewMessage(MessageTypeAuth)
	response.SetPayload("status", "success")
	response.SetPayload("user_id", userID)
	return conn.SendMessage(response)
}

// HandleMessage 处理RPC请求消息（网关模式）
func (h *WebSocketHandler) HandleMessage(conn *Connection, msg *Message) error {
	// 验证请求
	if msg.Service == "" {
		return ErrServiceNotFound
	}
	if msg.Method == "" {
		return ErrMethodNotFound
	}

	// TODO: 这里应该根据 service 和 method 转发到后端服务
	// 示例：调用后端微服务
	// response, err := h.callBackendService(msg.Service, msg.Method, msg.Data)

	// 临时实现：返回一个示例响应
	response := NewResponseMessage(msg.ID)
	response.SetPayload("service", msg.Service)
	response.SetPayload("method", msg.Method)
	response.SetPayload("status", "success")
	response.SetPayload("message", "Request received and will be processed")

	return conn.SendMessage(response)
}

// callBackendService 调用后端服务（待实现）
// 这里应该集成你的后端服务调用逻辑，比如：
// - gRPC 调用
// - HTTP/REST 调用
// - 消息队列发送
func (h *WebSocketHandler) callBackendService(service, method string, data map[string]interface{}) (*Message, error) {
	// TODO: 实现实际的后端服务调用
	// 示例：
	// switch service {
	// case "chat":
	//     return h.chatService.Call(method, data)
	// case "user":
	//     return h.userService.Call(method, data)
	// default:
	//     return nil, ErrServiceNotFound
	// }

	return nil, nil
}
