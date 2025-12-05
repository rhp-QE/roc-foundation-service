// Package backbon 实现 Backbon RPC 服务
//
// Author: Ruan Huipeng
// Date: 2025-12-01

package backbon

import (
	"time"

	"github.com/rhp-QE/roc-foundation-service/long_connection_service/kitex_gen"
	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/frontier"
)

// getServiceKey 获取服务在 Redis 中的 key
func (s *BackbonServiceImpl) getServiceKey(serviceName string) string {
	return s.serviceCtx.GetServiceKey(serviceName)
}

// getConnectionKey 获取连接在 Redis 中的 key
func (s *BackbonServiceImpl) getConnectionKey(connectionID string) string {
	return s.serviceCtx.GetConnectionKey(connectionID)
}

// getUserConnectionKey 获取用户在 Redis 中的 key
func (s *BackbonServiceImpl) getUserConnectionKey(userID string) string {
	return s.serviceCtx.GetUserConnectionKey(userID)
}

// convertPushMessageToFrontierMessage 将 PushMessage 转换为 frontier.Message
func convertPushMessageToFrontierMessage(pushMsg *kitex_gen.PushMessage) *frontier.Message {
	msg := frontier.NewMessage(frontier.MessageTypePush)
	if pushMsg != nil {
		msg.RequestID = pushMsg.GetRequestID()
		msg.Type = pushMsg.GetType()
		if pushMsg.GetType() == "" {
			msg.Type = frontier.MessageTypePush
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
