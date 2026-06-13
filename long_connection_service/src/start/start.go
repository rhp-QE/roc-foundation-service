// Package start 提供长连接服务的启动函数
//
// Author: Ruan Huipeng
// Date: 2025-12-01

package start

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloudwego/kitex/pkg/klog"
	kitexregistry "github.com/cloudwego/kitex/pkg/registry"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/server"
	"github.com/kitex-contrib/obs-opentelemetry/tracing"
	"github.com/rhp-QE/roc-foundation-util-go/log/otel"
	"github.com/rhp-QE/roc-foundation-util-go/network"

	backbon "github.com/rhp-QE/roc-foundation-service/long_connection_service/kitex_gen/backbon/backbonservice"
	backbonapi "github.com/rhp-QE/roc-foundation-service/long_connection_service/src/backbon/api"
	frontier "github.com/rhp-QE/roc-foundation-service/long_connection_service/src/frontier"
	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/kitexinfra"
	"github.com/rhp-QE/roc-foundation-service/long_connection_service/src/servicecontext"
)

const backbonServiceName = "backbon-service"

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

	// 创建服务器
	frontierServer, backbonServer := createServers(serviceCtx, host)

	// 启动服务器
	startServers(frontierServer, backbonServer)

	klog.Infof("Backbon service started, address: %s:%d", host, 8956)

	// 等待关闭信号
	waitForShutdown()

	if err := frontierServer.Stop(); err != nil {
		klog.Errorf("Frontier server stop failed: %v", err)
	}
	if err := backbonServer.Stop(); err != nil {
		klog.Errorf("Backbon server stop failed: %v", err)
	}

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

// createServers 创建 Frontier 和 Backbon 服务器
func createServers(serviceCtx *servicecontext.ServiceContext, host string) (*frontier.Server, server.Server) {
	kitexRegistry, err := kitexinfra.NewEtcdRegistry(serviceCtx.Config.Registry.Etcd.Endpoints)
	if err != nil {
		klog.Fatalf("Failed to create kitex etcd registry: %v", err)
	}

	// 创建 Frontier 服务器
	frontierServer := frontier.NewServer(serviceCtx)

	// 创建 Backbon 服务（通过 api 层）
	backbonService := backbonapi.NewBackbonServiceImpl(frontierServer.GetHub(), serviceCtx)

	// 创建 Backbon RPC 服务器
	backbonServer := backbon.NewServer(
		backbonService,
		server.WithServiceAddr(&net.TCPAddr{IP: net.ParseIP(host), Port: 8956}),
		server.WithSuite(tracing.NewServerSuite()),
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: backbonServiceName}),
		server.WithRegistry(kitexRegistry),
		server.WithRegistryInfo(&kitexregistry.Info{
			Weight: 1,
			Tags: map[string]string{
				"version": "1.0.0",
			},
		}),
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

// waitForShutdown 等待关闭信号
func waitForShutdown() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan
	klog.Info("Shutting down...")
}
