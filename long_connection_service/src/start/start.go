// Package start 提供长连接服务的启动函数
//
// Author: Ruan Huipeng
// Date: 2025-12-01

package start

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/server"
	"github.com/google/uuid"
	"github.com/kitex-contrib/obs-opentelemetry/tracing"
	"github.com/roc/roc-foundation-util-go/log/otel"
	"github.com/roc/roc-foundation-util-go/network"
	foundationregistry "github.com/roc/roc-foundation-util-go/service_registry/registry"

	backbon "github.com/rhp-QE/roc-foundation-service/long_connection_service/kitex_gen/backbonservice"
	backbonImpl "github.com/rhp-QE/roc-foundation-service/long_connection_service/src/backbon"
	frontier "github.com/rhp-QE/roc-foundation-service/long_connection_service/src/frontier"
	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/servicecontext"
)

// Start 启动长连接服务
// configPath: 配置文件路径，如果为空则使用默认路径 "config.yaml"
func Start(configPath string) error {
	if configPath == "" {
		configPath = "config.yaml"
	}

	// 初始化 OTEL
	kit := initOTEL()
	defer kit.Shutdown(context.Background())

	// 创建服务上下文
	serviceCtx := createServiceContext(configPath)
	defer serviceCtx.Close()

	// 获取本机地址
	host := getLocalIP()

	// 创建服务实例
	instance := createServiceInstance(host)

	// 创建服务器
	frontierServer, backbonServer := createServers(serviceCtx, host)

	// 启动服务器
	startServers(frontierServer, backbonServer)

	// 注册服务到 etcd
	registerService(serviceCtx, instance)

	// 设置优雅关闭处理
	defer setupGracefulShutdown(serviceCtx, instance)

	klog.Infof("Backbon service starting - instance_id: %s, address: %s:%d", instance.InstanceID, host, 8956)

	// 等待关闭信号
	waitForShutdown()

	return nil
}

// initOTEL 初始化 OTEL 并设置日志级别
func initOTEL() *otel.OTELKit {
	kit, err := otel.InitOTEL(context.Background(),
		otel.WithServiceName("frontier"),
		otel.WithEndpoint("localhost:4317"),
		otel.WithInsecure(true),
	)
	if err != nil {
		// 注意：OTEL 初始化失败时，klog 还未初始化，使用标准库 log
		log.Fatalf("Failed to init OTEL: %v", err)
	}
	kit.Logger.SetLevel(klog.LevelDebug)
	return kit
}

// createServiceContext 创建服务上下文
func createServiceContext(configPath string) *servicecontext.ServiceContext {
	serviceCtx, err := servicecontext.NewServiceContext(configPath)
	if err != nil {
		klog.Fatalf("Failed to create service context: %v", err)
	}
	return serviceCtx
}

// getLocalIP 获取本机IP
func getLocalIP() string {
	host, err := network.GetLocalIP()
	if err != nil {
		klog.Fatalf("Failed to get local IP: %v", err)
	}
	return host
}

// createServiceInstance 创建服务实例信息
func createServiceInstance(host string) *foundationregistry.ServiceInstance {
	instanceID := fmt.Sprintf("backbon-service-%s", uuid.New().String()[:8])
	return &foundationregistry.ServiceInstance{
		ServiceName: "backbon-service",
		InstanceID:  instanceID,
		Host:        host,
		Port:        8956,
		Weight:      1,
		Metadata: map[string]string{
			"version": "1.0.0",
		},
	}
}

// createServers 创建 Frontier 和 Backbon 服务器
func createServers(serviceCtx *servicecontext.ServiceContext, host string) (*frontier.Server, server.Server) {
	// 创建 Frontier 服务器
	frontierServer := frontier.NewServer(serviceCtx)

	// 创建 Backbon 服务
	backbonService := backbonImpl.NewBackbonServiceImpl(frontierServer.GetHub(), serviceCtx)

	// 创建 Backbon RPC 服务器
	backbonServer := backbon.NewServer(
		backbonService,
		server.WithServiceAddr(&net.TCPAddr{IP: net.ParseIP(host), Port: 8956}),
		server.WithSuite(tracing.NewServerSuite()),
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: "backbon-service"}),
	)

	return frontierServer, backbonServer
}

// startServers 启动服务器
func startServers(frontierServer *frontier.Server, backbonServer server.Server) {
	// 启动 Frontier 服务器
	go func() {
		if err := frontierServer.Start(); err != nil {
			klog.Fatalf("Failed to start frontier server: %v", err)
		}
	}()

	// 启动 Backbon RPC 服务器
	go func() {
		if err := backbonServer.Run(); err != nil {
			klog.Fatalf("Failed to start backbon server: %v", err)
		}
	}()

	// 等待 1 秒确保服务已经启动并监听端口
	time.Sleep(1 * time.Second)
}

// registerService 注册服务到 etcd
func registerService(serviceCtx *servicecontext.ServiceContext, instance *foundationregistry.ServiceInstance) {
	// 服务启动后再注册到 etcd，避免保活机制在服务未就绪时删除注册
	if err := serviceCtx.Registry.Register(context.Background(), instance); err != nil {
		klog.Fatalf("Failed to register service instance: %v", err)
	}
	klog.Infof("Successfully registered backbon-service to etcd")
}

// setupGracefulShutdown 设置优雅关闭处理
func setupGracefulShutdown(serviceCtx *servicecontext.ServiceContext, instance *foundationregistry.ServiceInstance) {
	if err := serviceCtx.Registry.Deregister(context.Background(), instance); err != nil {
		klog.Errorf("Failed to deregister service instance: %v", err)
	}
}

// waitForShutdown 等待关闭信号
func waitForShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
	klog.Info("Shutting down...")
}
