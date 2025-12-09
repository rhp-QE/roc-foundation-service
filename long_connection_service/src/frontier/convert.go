// Package frontier 提供数据转换工具函数
//
// Author: Ruan Huipeng
// Date: 2025-12-01

package frontier

import (
	"time"

	back "github.com/rhp-QE/roc-foundation-service/long_connection_service/kitex_gen/back"
	backbon "github.com/rhp-QE/roc-foundation-service/long_connection_service/kitex_gen/backbon"
)

// PushMessageToFrontierMessage 将 PushMessage 转换为 frontier.Message
func PushMessageToFrontierMessage(pushMsg *backbon.PushMessage) *Message {
	msg := NewMessage(MessageTypePush)
	if pushMsg != nil {
		msg.RequestID = pushMsg.GetRequestID()
		msg.Type = pushMsg.GetType()
		if pushMsg.GetType() == "" {
			msg.Type = MessageTypePush
		}
		msg.Service = pushMsg.GetService()
		msg.Method = pushMsg.GetMethod()
		msg.Payload = pushMsg.GetPayload()
		msg.Error = pushMsg.GetError()
		msg.Timestamp = pushMsg.GetTimestamp()
		if pushMsg.GetTimestamp() == 0 {
			msg.Timestamp = time.Now().Unix()
		}
		msg.Metadata = pushMsg.GetMetadata()
		if msg.Metadata == nil {
			msg.Metadata = make(map[string]string)
		}
	}
	return msg
}

// CallResponseToFontierMessage 将 CallResponse 转换为 frontier.Message
func CallResponseToFontierMessage(callResp *back.CallResponse, requestID string) *Message {
	response := &Message{
		RequestID: callResp.GetRequestID(),
		Type:      callResp.GetType(),
		Payload:   callResp.GetPayload(),
		Error:     callResp.GetError(),
		Metadata:  callResp.GetMetadata(),
		Timestamp: callResp.GetTimestamp(),
	}

	if !callResp.GetSuccess() && response.Error == "" {
		response.Error = "service call failed"
	}

	return response
}

// MessageToCallRequest 将 frontier.Message 和 Connection 转换为 CallRequest
func MessageToCallRequest(msg *Message, conn *Connection) *back.CallRequest {
	return &back.CallRequest{
		RequestID:    msg.RequestID,
		Type:         msg.Type,
		Service:      msg.Service,
		Method:       msg.Method,
		Payload:      msg.Payload,
		Metadata:     msg.Metadata,
		UserID:       conn.UserID,
		ConnectionID: conn.ID,
		Timestamp:    msg.Timestamp,
	}
}
