package route

import (
	"context"
	"errors"
	"time"
)

var (
	ErrNoLiveConnections = errors.New("no live connections")
	ErrConnectionMissing = errors.New("connection route missing")
)

type ConnectionMeta struct {
	UserID        string `json:"userID"`
	ConnectionID  string `json:"connectionID"`
	DeviceID      string `json:"deviceID,omitempty"`
	GatewayID     string `json:"gatewayID,omitempty"`
	GatewayAddr   string `json:"gatewayAddr"`
	Platform      string `json:"platform,omitempty"`
	ClientVersion string `json:"clientVersion,omitempty"`
	LoginAt       int64  `json:"loginAt"`
	LastActiveAt  int64  `json:"lastActiveAt"`
}

func (m ConnectionMeta) Touch(now time.Time) ConnectionMeta {
	if m.LoginAt == 0 {
		m.LoginAt = now.Unix()
	}
	m.LastActiveAt = now.Unix()
	return m
}

// Store 是长连接路由缓存的唯一入口。
// im:conn:{connectionID} 是连接事实源；im:user:{uid}:conns 只是可修复索引。
type Store interface {
	Register(ctx context.Context, meta ConnectionMeta) error
	Refresh(ctx context.Context, userID string, connectionID string) error
	Unregister(ctx context.Context, userID string, connectionID string) error
	Get(ctx context.Context, connectionID string) (ConnectionMeta, error)
	ListByUser(ctx context.Context, userID string) ([]ConnectionMeta, error)
	Cleanup(ctx context.Context, userID string, connectionID string) error
	CleanupStale(ctx context.Context, userID string) (int, error)
}
