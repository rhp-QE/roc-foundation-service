package frontier

import (
	"errors"
	"time"
)

// 消息类型常量
const (
	MessageTypeRequest   = "request"   // 请求消息（RPC调用）
	MessageTypeResponse  = "response"  // 响应消息
	MessageTypeAuth      = "auth"      // 认证消息
	MessageTypeHeartbeat = "heartbeat" // 心跳消息
	MessageTypePush      = "push"      // 服务端主动推送消息
	MessageTypeError     = "error"     // 错误消息
)

// Message WebSocket消息结构（网关模式）
type Message struct {
	ID        string                 `json:"id,omitempty"`      // 消息ID（用于请求-响应匹配）
	Type      string                 `json:"type"`              // 消息类型
	Service   string                 `json:"service,omitempty"` // 目标服务名称
	Method    string                 `json:"method,omitempty"`  // 服务方法名称
	Payload   map[string]interface{} `json:"payload,omitempty"` // 消息有效载荷/参数
	Error     string                 `json:"error,omitempty"`   // 错误信息（响应时使用）
	Timestamp int64                  `json:"timestamp"`         // 时间戳
}

// NewMessage 创建新消息
func NewMessage(msgType string) *Message {
	return &Message{
		Type:      msgType,
		Payload:   make(map[string]interface{}),
		Timestamp: time.Now().Unix(),
	}
}

// NewRequestMessage 创建请求消息
func NewRequestMessage(service, method string) *Message {
	return &Message{
		Type:      MessageTypeRequest,
		Service:   service,
		Method:    method,
		Payload:   make(map[string]interface{}),
		Timestamp: time.Now().Unix(),
	}
}

// NewResponseMessage 创建响应消息
func NewResponseMessage(requestID string) *Message {
	return &Message{
		ID:        requestID,
		Type:      MessageTypeResponse,
		Payload:   make(map[string]interface{}),
		Timestamp: time.Now().Unix(),
	}
}

// NewErrorMessage 创建错误消息
func NewErrorMessage(requestID, errorMsg string) *Message {
	return &Message{
		ID:        requestID,
		Type:      MessageTypeError,
		Error:     errorMsg,
		Timestamp: time.Now().Unix(),
	}
}

// Validate 验证消息
func (m *Message) Validate() error {
	if m.Type == "" {
		return errors.New("message type is required")
	}

	// 请求消息需要 service 和 method
	if m.Type == MessageTypeRequest {
		if m.Service == "" {
			return errors.New("service is required for request message")
		}
		if m.Method == "" {
			return errors.New("method is required for request message")
		}
	}

	return nil
}

// IsAuthMessage 是否为认证消息
func (m *Message) IsAuthMessage() bool {
	return m.Type == MessageTypeAuth
}

// IsHeartbeatMessage 是否为心跳消息
func (m *Message) IsHeartbeatMessage() bool {
	return m.Type == MessageTypeHeartbeat
}

// IsRequestMessage 是否为请求消息
func (m *Message) IsRequestMessage() bool {
	return m.Type == MessageTypeRequest
}

// IsResponseMessage 是否为响应消息
func (m *Message) IsResponseMessage() bool {
	return m.Type == MessageTypeResponse
}

// GetPayloadString 获取字符串类型的载荷数据
func (m *Message) GetPayloadString(key string) (string, bool) {
	val, ok := m.Payload[key]
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}

// GetPayloadInt 获取整数类型的载荷数据
func (m *Message) GetPayloadInt(key string) (int, bool) {
	val, ok := m.Payload[key]
	if !ok {
		return 0, false
	}

	// 处理不同的数字类型
	switch v := val.(type) {
	case int:
		return v, true
	case float64:
		return int(v), true
	case int64:
		return int(v), true
	default:
		return 0, false
	}
}

// SetPayload 设置载荷数据
func (m *Message) SetPayload(key string, value interface{}) {
	if m.Payload == nil {
		m.Payload = make(map[string]interface{})
	}
	m.Payload[key] = value
}

// 错误定义
var (
	ErrInvalidMessageType = errors.New("invalid message type")
	ErrMessageTimeout     = errors.New("message timeout")
	ErrSendBufferFull     = errors.New("send buffer full")
	ErrConnectionClosed   = errors.New("connection closed")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrServiceNotFound    = errors.New("service not found")
	ErrMethodNotFound     = errors.New("method not found")
	ErrInvalidRequest     = errors.New("invalid request")
)
