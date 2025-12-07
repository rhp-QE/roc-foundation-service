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

	// 使用 foundation-util-go 统一初始化 OTEL（初始化 klog）
	kit, err := otel.InitOTEL(context.Background(),
		otel.WithServiceName("frontier"),
		otel.WithEndpoint("localhost:4317"),
		otel.WithInsecure(true),
	)
	if err != nil {
		// 注意：OTEL 初始化失败时，klog 还未初始化，使用标准库 log
		log.Fatalf("Failed to init OTEL: %v", err)
	}
	defer kit.Shutdown(context.Background())

	// 设置日志级别为 DEBUG
	kit.Logger.SetLevel(klog.LevelDebug)

	// 创建服务上下文（内部加载配置并统一管理所有共享资源）
	serviceCtx, err := servicecontext.NewServiceContext(configPath)
	if err != nil {
		klog.Fatalf("Failed to create service context: %v", err)
	}
	defer serviceCtx.Close()

	// 解析本机地址（格式：host:port）
	localAddress := serviceCtx.GetLocalAddress()
	host, _, err := net.SplitHostPort(localAddress)
	if err != nil {
		// 如果解析失败，尝试获取本机IP
		host, err = network.GetLocalIP()
		if err != nil {
			klog.Fatalf("Failed to get local IP: %v", err)
		}
	}
	if host == "" || host == "0.0.0.0" {
		host, err = network.GetLocalIP()
		if err != nil {
			klog.Fatalf("Failed to get local IP: %v", err)
		}
	}

	instanceID := fmt.Sprintf("backbon-service-%s", uuid.New().String()[:8])
	instance := &foundationregistry.ServiceInstance{
		ServiceName: "backbon-service",
		InstanceID:  instanceID,
		Host:        host,
		Port:        8956,
		Weight:      1,
		Metadata: map[string]string{
			"version": "1.0.0",
		},
	}

	// 创建 Frontier 服务器（从 ServiceContext 获取配置）
	frontierServer := frontier.NewServer(serviceCtx)

	// 创建 Backbon 服务，传入服务上下文
	backbonService := backbonImpl.NewBackbonServiceImpl(frontierServer.GetHub(), serviceCtx)

	// 创建 Backbon RPC 服务器，配置服务地址、OTEL tracing 等
	backbonServer := backbon.NewServer(
		backbonService,
		server.WithServiceAddr(&net.TCPAddr{IP: net.ParseIP(host), Port: 8956}),
		server.WithSuite(tracing.NewServerSuite()),
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: "backbon-service"}),
	)

	klog.Infof("Backbon service starting - instance_id: %s, address: %s:%d", instanceID, host, 8956)

	// 启动 Frontier 服务器
	go func() {
		err := frontierServer.Start()
		if err != nil {
			klog.Fatalf("Failed to start frontier server: %v", err)
		}
	}()

	// 启动 Backbon RPC 服务器
	go func() {
		err := backbonServer.Run()
		if err != nil {
			klog.Fatalf("Failed to start backbon server: %v", err)
		}
	}()

	// 等待 1 秒确保服务已经启动并监听端口
	time.Sleep(1 * time.Second)

	// 服务启动后再注册到 etcd，避免保活机制在服务未就绪时删除注册
	if err := serviceCtx.Registry.Register(context.Background(), instance); err != nil {
		klog.Fatalf("Failed to register service instance: %v", err)
	}
	klog.Infof("Successfully registered backbon-service to etcd")
	// 优雅关闭处理
	defer func() {
		if err := serviceCtx.Registry.Deregister(context.Background(), instance); err != nil {
			klog.Errorf("Failed to deregister service instance: %v", err)
		}
	}()

	// 优雅关闭
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	klog.Info("Shutting down...")
	return nil
}
