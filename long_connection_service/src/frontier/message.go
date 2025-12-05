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
	RequestID string            `json:"requestID,omitempty"` // 请求ID（用于请求-响应匹配）
	Type      string            `json:"type"`                // 消息类型
	Service   string            `json:"service,omitempty"`   // 目标服务名称
	Method    string            `json:"method,omitempty"`    // 服务方法名称
	Payload   []byte            `json:"payload,omitempty"`   // 消息有效载荷/数据（字节数组，业务数据）
	Error     string            `json:"error,omitempty"`     // 错误信息（响应时使用）
	Timestamp int64             `json:"timestamp"`           // 时间戳
	Metadata  map[string]string `json:"metadata,omitempty"`  // 元数据（用于传递验证信息如token、track_id等）
}

// NewMessage 创建新消息
func NewMessage(msgType string) *Message {
	return &Message{
		Type:      msgType,
		Payload:   []byte{},
		Metadata:  make(map[string]string),
		Timestamp: time.Now().Unix(),
	}
}

// NewRequestMessage 创建请求消息
func NewRequestMessage(service, method string) *Message {
	return &Message{
		Type:      MessageTypeRequest,
		Service:   service,
		Method:    method,
		Payload:   []byte{},
		Metadata:  make(map[string]string),
		Timestamp: time.Now().Unix(),
	}
}

// NewResponseMessage 创建响应消息
func NewResponseMessage(requestID string) *Message {
	return &Message{
		RequestID: requestID,
		Type:      MessageTypeResponse,
		Payload:   []byte{},
		Metadata:  make(map[string]string),
		Timestamp: time.Now().Unix(),
	}
}

// NewErrorMessage 创建错误消息
func NewErrorMessage(requestID, errorMsg string) *Message {
	return &Message{
		RequestID: requestID,
		Type:      MessageTypeError,
		Error:     errorMsg,
		Metadata:  make(map[string]string),
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

// GetMetadata 从元数据中获取值
func (m *Message) GetMetadata(key string) (string, bool) {
	if m.Metadata == nil {
		return "", false
	}
	val, ok := m.Metadata[key]
	return val, ok
}

// SetMetadata 设置元数据值
func (m *Message) SetMetadata(key, value string) {
	if m.Metadata == nil {
		m.Metadata = make(map[string]string)
	}
	m.Metadata[key] = value
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
