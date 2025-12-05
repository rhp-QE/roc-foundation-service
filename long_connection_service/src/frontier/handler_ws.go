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
	if msg.Service == "" {
		return ErrServiceNotFound
	}
	if msg.Method == "" {
		return ErrMethodNotFound
	}

	// TODO: 这里应该根据 service 和 method 转发到后端服务
	// 网关将 msg.Payload（业务数据）和 msg.Metadata（验证信息如token、track_id等）原样转发给后端服务
	// response, err := h.callBackendService(msg.Service, msg.Method, msg.Payload, msg.Metadata)

	// 临时实现：返回一个示例响应
	response := NewResponseMessage(msg.RequestID)
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
