package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/cloudwego/kitex/pkg/klog"
	"github.com/cloudwego/kitex/pkg/rpcinfo"
	"github.com/cloudwego/kitex/server"
	"github.com/google/uuid"
	"github.com/kitex-contrib/obs-opentelemetry/tracing"
	"github.com/roc/roc-foundation-util-go/log/otel"
	foundationregistry "github.com/roc/roc-foundation-util-go/service_registry/registry"

	backbon "github.com/rhp-QE/roc-foundation-service/long_connection_service/kitex_gen/backbonservice"
	backbonImpl "github.com/rhp-QE/roc-foundation-service/long_connection_service/src/backbon"
	frontier "github.com/rhp-QE/roc-foundation-service/long_connection_service/src/frontier"
)

func main() {
	// 创建服务上下文（内部加载配置并统一管理所有共享资源）
	serviceCtx, err := NewServiceContext("config.yaml")
	if err != nil {
		log.Fatalf("Failed to create service context: %v", err)
	}
	defer serviceCtx.Close()


	instanceID := fmt.Sprintf("backbon-service-%s", uuid.New().String()[:8])
	instance := &foundationregistry.ServiceInstance{
		ServiceName: "backbon-service",
		InstanceID:  instanceID,
		Host:        serviceCtx.GetLocalAddress(),
		Port:        8956,
		Weight:      1,
		Metadata: map[string]string{
			"version": "1.0.0",
		},
	}

	if err := serviceCtx.Registry.Register(context.Background(), instance); err != nil {
		log.Fatalf("Failed to register service instance: %v", err)
	}

	// 使用 foundation-util-go 统一初始化 OTEL
	kit, err := otel.InitOTEL(context.Background(),
		otel.WithServiceName("backbon-service"),
		otel.WithEndpoint("localhost:4317"),
		otel.WithInsecure(true),
	)
	if err != nil {
		log.Fatalf("Failed to init OTEL: %v", err)
	}
	defer kit.Shutdown(context.Background())

	// 创建 Frontier 服务器（从 ServiceContext 获取配置）
	frontierServer := frontier.NewServer(serviceCtx)

	// 创建 Backbon 服务，传入服务上下文
	backbonService := backbonImpl.NewBackbonServiceImpl(frontierServer.GetHub(), serviceCtx)

	// 创建 Backbon RPC 服务器，配置服务地址、OTEL tracing 等
	backbonServer := backbon.NewServer(
		backbonService,
		server.WithServiceAddr(&net.TCPAddr{IP: net.ParseIP(serviceCtx.GetLocalAddress()), Port: 8956}),
		server.WithSuite(tracing.NewServerSuite()),
		server.WithServerBasicInfo(&rpcinfo.EndpointBasicInfo{ServiceName: "backbon-service"}),
	)

	// 设置日志级别为 DEBUG
	kit.Logger.SetLevel(klog.LevelDebug)

	// 优雅关闭处理
	defer func() {
		if err := serviceCtx.Registry.Deregister(context.Background(), instance); err != nil {
			log.Printf("Failed to deregister service instance: %v", err)
		}
	}()

	log.Printf("Backbon service starting - instance_id: %s, address: %s:%d", instanceID, serviceCtx.GetLocalAddress(), 8956)

	// 启动 Frontier 服务器
	go func() {
		err := frontierServer.Start()
		if err != nil {
			log.Fatalf("Failed to start frontier server: %v", err)
		}
	}()

	// 启动 Backbon RPC 服务器
	go func() {
		err := backbonServer.Run()
		if err != nil {
			log.Fatalf("Failed to start backbon server: %v", err)
		}
	}()

	// 优雅关闭
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	log.Println("Shutting down...")
}
