// Package servicecontext 管理长链服务运行期依赖。
package servicecontext

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/cloudwego/kitex/client"
	kitexdiscovery "github.com/cloudwego/kitex/pkg/discovery"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/kitex-contrib/obs-opentelemetry/tracing"
	backbonservice "github.com/rhp-QE/roc-foundation-service/long_connection_service/kitex_gen/backbon/backbonservice"
	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/config"
	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/frontier"
	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/kitexinfra"
	"github.com/rhp-QE/roc-foundation-util-go/cache"
	"github.com/rhp-QE/roc-foundation-util-go/cache/redis"
)

const backbonServiceName = "backbon-service"

// ServiceContext 管理全局共享资源。
// RPC 服务发现和负载均衡由 Kitex resolver/client 负责，不再持有自研 registry/discovery。
type ServiceContext struct {
	Config *config.Config

	Resolver kitexdiscovery.Resolver
	Redis    cache.Cache

	remoteClients sync.Map // map[string]backbonservice.Client
	LocalAddress  string
}

// NewServiceContext 创建服务上下文（内部加载配置）。
func NewServiceContext(configPath string) (*ServiceContext, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config from %s: %w", configPath, err)
	}

	serviceCtx := &ServiceContext{Config: cfg}
	if err := serviceCtx.init(); err != nil {
		return nil, fmt.Errorf("failed to initialize service context: %w", err)
	}
	return serviceCtx, nil
}

func (ctx *ServiceContext) init() error {
	resolver, err := kitexinfra.NewEtcdResolver(ctx.Config.Registry.Etcd.Endpoints)
	if err != nil {
		return fmt.Errorf("failed to create kitex etcd resolver: %w", err)
	}
	ctx.Resolver = resolver

	if err := ctx.initRedis(); err != nil {
		return err
	}

	ctx.LocalAddress = ctx.getLocalAddress()
	return nil
}

func (ctx *ServiceContext) initRedis() error {
	opts := []redis.Option{
		redis.WithAddress(ctx.Config.Redis.Address),
		redis.WithPassword(ctx.Config.Redis.Password),
	}

	redisCache, err := redis.NewRedisCache(opts)
	if err != nil {
		return fmt.Errorf("failed to connect to Redis at %s: %w", ctx.Config.Redis.Address, err)
	}

	ctx.Redis = redisCache
	return nil
}

func (ctx *ServiceContext) getLocalAddress() string {
	host := ctx.Config.Frontier.Host
	port := ctx.Config.Frontier.Port
	if host == "" || host == "0.0.0.0" {
		localIP, err := getLocalIP()
		if err == nil {
			host = localIP
		} else {
			host = "localhost"
		}
	}
	return fmt.Sprintf("%s:%s", host, port)
}

func getLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
			return ipNet.IP.String(), nil
		}
	}
	return "", fmt.Errorf("no non-loopback IP found")
}

func (ctx *ServiceContext) GetLocalAddress() string {
	return ctx.LocalAddress
}

func (ctx *ServiceContext) GetRedis() cache.Cache {
	return ctx.Redis
}

func (ctx *ServiceContext) GetKitexResolver() kitexdiscovery.Resolver {
	return ctx.Resolver
}

// GetRemoteClient 获取或创建指定网关实例的 backbon 客户端。
// 这里使用明确的 machine address 做跨网关推送，不参与服务发现。
func (ctx *ServiceContext) GetRemoteClient(machineAddr string) (backbonservice.Client, error) {
	if clientImpl, ok := ctx.remoteClients.Load(machineAddr); ok {
		return clientImpl.(backbonservice.Client), nil
	}

	newClient, err := backbonservice.NewClient(
		backbonServiceName,
		client.WithHostPorts(machineAddr),
		client.WithSuite(tracing.NewClientSuite()),
		client.WithClientBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: backbonServiceName}),
		client.WithRPCTimeout(10*time.Second),
	)
	if err != nil {
		return nil, err
	}

	actual, loaded := ctx.remoteClients.LoadOrStore(machineAddr, newClient)
	if loaded {
		if closer, ok := newClient.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
		return actual.(backbonservice.Client), nil
	}

	return newClient, nil
}

func (ctx *ServiceContext) GetFrontierConfig() *frontier.Config {
	return &ctx.Config.Frontier
}

func (ctx *ServiceContext) Close() error {
	var errs []error

	ctx.remoteClients.Range(func(key, value interface{}) bool {
		if clientImpl, ok := value.(interface{ Close() error }); ok {
			if err := clientImpl.Close(); err != nil {
				errs = append(errs, fmt.Errorf("failed to close remote client %s: %w", key, err))
			}
		}
		return true
	})

	if ctx.Redis != nil {
		if closer, ok := ctx.Redis.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				errs = append(errs, fmt.Errorf("failed to close Redis: %w", err))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing resources: %v", errs)
	}
	return nil
}
