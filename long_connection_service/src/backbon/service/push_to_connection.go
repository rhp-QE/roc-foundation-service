package service

import (
	"context"
	"fmt"

	"github.com/cloudwego/kitex/pkg/klog"
	backbon "github.com/rhp-QE/roc-foundation-service/long_connection_service/kitex_gen/backbon"
	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/backbon/storage"
	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/frontier"
)

// PushToConnection 向指定连接推送消息
func (s *backbonServiceImpl) PushToConnection(ctx context.Context, req *backbon.PushToConnectionReq) (resp *backbon.PushToConnectionResp, err error) {
	resp = &backbon.PushToConnectionResp{
		Success: false,
	}

	// 验证请求
	connectionID := req.GetConnectionID()
	if connectionID == "" {
		resp.Error = "connectionID is required"
		return resp, fmt.Errorf("connectionID is required")
	}

	pushMsg := req.GetMessage()
	if pushMsg == nil {
		resp.Error = "message is required"
		return resp, fmt.Errorf("message is required")
	}

	// 转换为 frontier.Message
	frontierMsg := frontier.PushMessageToFrontierMessage(pushMsg)

	// 检查连接是否在本机
	hub := s.getHub()
	if hub != nil {
		_, exists := hub.GetConnection(connectionID)
		if exists {
			// 连接在本机，直接推送
			err := hub.SendToConnection(connectionID, frontierMsg)
			if err == nil {
				resp.Success = true
				klog.CtxInfof(ctx, "Pushed message to connection %s on local machine", connectionID)
			} else {
				resp.Error = err.Error()
				klog.CtxErrorf(ctx, "Failed to push message to connection %s on local machine: %v", connectionID, err)
			}
			return resp, err
		}
	}

	// 连接不在本机，从 storage 获取连接所在机器并调用远程服务
	storage := storage.NewBackbonStorage(s.serviceCtx)
	machineAddr, err := storage.GetConnectionMachineAddr(ctx, connectionID)
	if err != nil {
		resp.Error = fmt.Sprintf("connection %s not found", connectionID)
		klog.CtxWarnf(ctx, "Connection %s not found: %v", connectionID, err)
		return resp, fmt.Errorf("connection %s not found", connectionID)
	}

	// 调用远程服务
	return s.callRemotePushToConnectionFromLocal(ctx, connectionID, pushMsg, machineAddr)
}

// callRemotePushToConnectionFromLocal 从本机调用远程服务的 PushToConnection
func (s *backbonServiceImpl) callRemotePushToConnectionFromLocal(
	ctx context.Context,
	connectionID string,
	pushMsg *backbon.PushMessage,
	machineAddr string,
) (*backbon.PushToConnectionResp, error) {
	resp := &backbon.PushToConnectionResp{
		Success: false,
	}

	// 创建远程客户端
	remoteClient, err := s.serviceCtx.GetRemoteClient(machineAddr)
	if err != nil {
		resp.Error = fmt.Sprintf("failed to create remote client: %v", err)
		klog.CtxErrorf(ctx, "Failed to create remote client for %s: %v", machineAddr, err)
		return resp, err
	}

	// 构建远程推送请求
	remoteReq := &backbon.PushToConnectionReq{
		ConnectionID: connectionID,
		Message:      pushMsg,
	}

	// 调用远程服务
	remoteResp, err := remoteClient.PushToConnection(ctx, remoteReq)
	if err != nil {
		resp.Error = fmt.Sprintf("failed to call remote PushToConnection service: %v", err)
		klog.CtxErrorf(ctx, "Failed to call remote PushToConnection at %s for connection %s: %v", machineAddr, connectionID, err)
		return resp, err
	}

	if remoteResp == nil {
		resp.Error = "invalid response from remote service"
		return resp, fmt.Errorf("invalid response from remote service")
	}

	resp.Success = remoteResp.GetSuccess()
	resp.Error = remoteResp.GetError()

	klog.CtxInfof(ctx, "Pushed message to connection %s on remote machine %s: success=%v", connectionID, machineAddr, resp.Success)
	return resp, nil
}
