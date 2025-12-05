// Package main 服务上下文，管理全局共享资源
//
// Author: Ruan Huipeng
// Date: 2025-12-01

package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/cloudwego/kitex/client"
	"github.com/google/uuid"
	backbonservice "github.com/rhp-QE/roc-foundation-service/long_connection_service/kitex_gen/backbonservice"
	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/config"
	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/frontier"
	"github.com/roc/roc-foundation-util-go/cache"
	"github.com/roc/roc-foundation-util-go/cache/redis"
	"github.com/roc/roc-foundation-util-go/service_registry/discovery"
	"github.com/roc/roc-foundation-util-go/service_registry/loadbalancer"
	"github.com/roc/roc-foundation-util-go/service_registry/registry"
	"github.com/roc/roc-foundation-util-go/service_registry/registry/etcd"
	"github.com/roc/roc-foundation-util-go/stringutil"
)

// ServiceContext 服务上下文，管理全局共享资源
type ServiceContext struct {
	// 配置
	Config *config.Config

	// Etcd Registry
	Registry *etcd.EtcdRegistry

	// 服务发现
	Discovery discovery.Discovery

	// Redis 客户端
	Redis cache.Cache

	// 远程客户端缓存：machineAddr -> Client
	remoteClients sync.Map // map[string]backbonservice.Client

	// 本机地址（初始化时确定，后续不变）
	LocalAddress string
}

// NewServiceContext 创建服务上下文（内部加载配置）
func NewServiceContext(configPath string) (*ServiceContext, error) {
	// 加载配置文件
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config from %s: %w", configPath, err)
	}

	serviceCtx := &ServiceContext{
		Config: cfg,
	}

	// 初始化资源
	if err := serviceCtx.init(); err != nil {
		return nil, fmt.Errorf("failed to initialize service context: %w", err)
	}

	return serviceCtx, nil
}

// init 初始化所有资源
func (ctx *ServiceContext) init() error {
	var err error

	// 创建 etcd registry
	ctx.Registry, err = etcd.NewEtcdRegistry(
		etcd.WithEndpoints(ctx.Config.Registry.Etcd.Endpoints),
		etcd.WithDialTimeout(5*time.Second),
	)
	if err != nil {
		return fmt.Errorf("failed to create etcd registry: %w", err)
	}

	// 创建服务发现客户端
	lb := loadbalancer.NewRoundRobinLoadBalancer()
	ctx.Discovery = discovery.NewDiscovery(ctx.Registry, lb)

	// 在这里向注册中心注册 redis 服务 ip 是本地IP （先mock 生产环境再改）
	if ctx.Config.Redis.ServiceName != "" {
		if err := ctx.registerRedisService(); err != nil {
			log.Printf("Warning: Failed to register Redis service: %v", err)
			// 注册失败不阻塞启动，但会记录警告
		}
	}

	// 初始化 Redis 客户端
	if ctx.Config.Redis.ServiceName != "" {
		if err := ctx.initRedis(); err != nil {
			log.Printf("Warning: Failed to initialize Redis: %v", err)
			// Redis 初始化失败不阻塞启动，但会记录警告
		}
	}

	// 初始化本机地址
	ctx.LocalAddress = ctx.getLocalAddress()

	return nil
}

// registerRedisService 向注册中心注册 Redis 服务（开发环境 mock 使用）
func (ctx *ServiceContext) registerRedisService() error {
	// 获取本机 IP
	localIP, err := getLocalIP()
	if err != nil {
		return fmt.Errorf("failed to get local IP: %w", err)
	}

	// Redis 默认端口 6379
	redisPort := 6379

	// 生成实例 ID
	instanceID := fmt.Sprintf("redis-service-%s", uuid.New().String()[:8])

	// 创建服务实例
	instance := &registry.ServiceInstance{
		ServiceName: ctx.Config.Redis.ServiceName,
		InstanceID:  instanceID,
		Host:        localIP,
		Port:        redisPort,
		Status:      registry.StatusHealthy,
		Weight:      100,
		Metadata: map[string]string{
			"version": "1.0.0",
		},
	}

	// 注册到 etcd
	regCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := ctx.Registry.Register(regCtx, instance); err != nil {
		return fmt.Errorf("failed to register Redis service instance: %w", err)
	}

	log.Printf("Successfully registered Redis service: %s:%d (instance: %s)", localIP, redisPort, instanceID)
	return nil
}

// initRedis 初始化 Redis 客户端
func (ctx *ServiceContext) initRedis() error {
	appCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	instance, err := ctx.Discovery.GetInstance(appCtx, ctx.Config.Redis.ServiceName)
	if err != nil {
		return fmt.Errorf("failed to discover Redis service instance: %w", err)
	}

	redisAddress := fmt.Sprintf("%s:%d", instance.Host, instance.Port)

	opts := []redis.Option{
		redis.WithAddress(redisAddress),
		redis.WithPassword(ctx.Config.Redis.Password),
	}

	redisCache, err := redis.NewRedisCache(opts)
	if err != nil {
		return fmt.Errorf("failed to connect to Redis at %s: %w", redisAddress, err)
	}

	ctx.Redis = redisCache
	log.Printf("Successfully connected to Redis at %s", redisAddress)
	return nil
}

// getLocalAddress 获取本机地址
func (ctx *ServiceContext) getLocalAddress() string {
	host := ctx.Config.Frontier.Host
	port := ctx.Config.Frontier.Port
	if host == "" || host == "0.0.0.0" {
		// 尝试获取本机 IP
		localIP, err := getLocalIP()
		if err == nil {
			host = localIP
		} else {
			host = "localhost"
		}
	}
	return fmt.Sprintf("%s:%s", host, port)
}

// getLocalIP 获取本机 IP 地址
func getLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				return ipNet.IP.String(), nil
			}
		}
	}
	return "", fmt.Errorf("no non-loopback IP found")
}

// GetLocalAddress 获取本机地址（实现接口）
func (ctx *ServiceContext) GetLocalAddress() string {
	return ctx.LocalAddress
}

// GetRedis 获取 Redis 客户端
func (ctx *ServiceContext) GetRedis() cache.Cache {
	return ctx.Redis
}

// GetRemoteClient 获取或创建远程客户端（复用客户端实例）
func (ctx *ServiceContext) GetRemoteClient(machineAddr string) (backbonservice.Client, error) {
	// 先尝试从缓存获取
	if client, ok := ctx.remoteClients.Load(machineAddr); ok {
		return client.(backbonservice.Client), nil
	}

	// 创建新客户端
	newClient, err := backbonservice.NewClient(
		"backbon-service",
		client.WithHostPorts(machineAddr),
	)
	if err != nil {
		return nil, err
	}

	// 尝试存储到缓存（如果并发创建，使用第一个成功的）
	actual, loaded := ctx.remoteClients.LoadOrStore(machineAddr, newClient)
	if loaded {
		// 如果已经有其他 goroutine 创建了客户端，关闭我们刚创建的，使用已有的
		if closer, ok := newClient.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
		return actual.(backbonservice.Client), nil
	}

	// 成功存储了我们创建的客户端
	return newClient, nil
}

// GetUserConnectionKey 获取用户连接 key
func (ctx *ServiceContext) GetUserConnectionKey(userID string) string {
	return stringutil.FormatKey("fronter", "user", "connection", userID)
}

// GetConnectionKey 获取连接 key
func (ctx *ServiceContext) GetConnectionKey(connectionID string) string {
	return stringutil.FormatKey("fronter", "connection", connectionID)
}

// GetServiceKey 获取服务 key
func (ctx *ServiceContext) GetServiceKey(serviceName string) string {
	return stringutil.FormatKey("fronter", "service", serviceName)
}

// GetFrontierConfig 获取 Frontier 配置（供 frontier 包使用）
func (ctx *ServiceContext) GetFrontierConfig() *frontier.Config {
	return &ctx.Config.Frontier
}

// Close 关闭所有资源
func (ctx *ServiceContext) Close() error {
	var errs []error

	// 关闭所有远程客户端
	ctx.remoteClients.Range(func(key, value interface{}) bool {
		if client, ok := value.(interface{ Close() error }); ok {
			if err := client.Close(); err != nil {
				errs = append(errs, fmt.Errorf("failed to close remote client %s: %w", key, err))
			}
		}
		return true
	})

	// 关闭 Redis
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
