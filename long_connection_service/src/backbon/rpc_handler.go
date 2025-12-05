// Package backbon 实现 Backbon RPC 服务
//
// Author: Ruan Huipeng
// Date: 2025-12-01

package backbon

import (
	"context"

	"github.com/rhp-QE/roc-foundation-service/long_connection_service/kitex_gen"
	backbonservice "github.com/rhp-QE/roc-foundation-service/long_connection_service/kitex_gen/backbonservice"
	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/frontier"
	"github.com/roc/roc-foundation-util-go/cache"
)

// ServiceContext 服务上下文接口（依赖倒置，解耦具体实现）
type ServiceContext interface {
	GetLocalAddress() string
	GetRemoteClient(machineAddr string) (backbonservice.Client, error)
	GetRedis() cache.Cache
	GetUserConnectionKey(userID string) string
	GetConnectionKey(connectionID string) string
	GetServiceKey(serviceName string) string
}

// BackbonServiceImpl implements the last service interface defined in the IDL.
type BackbonServiceImpl struct {
	hub        *frontier.Hub
	serviceCtx ServiceContext
}

// NewBackbonServiceImpl creates a new BackbonServiceImpl.
func NewBackbonServiceImpl(hub *frontier.Hub, serviceCtx ServiceContext) *BackbonServiceImpl {
	if serviceCtx == nil {
		panic("ServiceContext cannot be nil")
	}

	return &BackbonServiceImpl{
		hub:        hub,
		serviceCtx: serviceCtx,
	}
}

// CheckUserOnline implements the BackbonServiceImpl interface.
func (s *BackbonServiceImpl) CheckUserOnline(ctx context.Context, req *kitex_gen.CheckUserOnlineReq) (resp *kitex_gen.CheckUserOnlineResp, err error) {
	// TODO: Your code here...
	return
}
