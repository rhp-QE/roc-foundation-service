# 长连接服务架构说明

## 架构概述

这是一个**微服务、分布式**的长连接服务，作为网关维护客户端 WebSocket 连接，并向后端服务转发请求，同时支持后端服务向客户端推送数据。

## 核心组件

### 1. Frontier（前端网关）
- **位置**: `src/frontier/`
- **职责**: 管理客户端 WebSocket 连接，接收客户端请求并转发到后端服务
- **关键文件**:
  - `server.go`: WebSocket 服务器启动
  - `hub.go`: 连接管理中心（管理所有连接，用户ID映射）
  - `connection.go`: 单个连接对象（WebSocket 连接封装）
  - `handler_ws.go`: WebSocket 消息处理器（处理认证、请求转发）
  - `message.go`: 消息结构定义（Message 结构，包含 requestID、type、service、method、payload、metadata 等）

### 2. Backbon（后端接口）
- **位置**: `src/backbon/`（待实现）
- **职责**: 为后端服务提供 RPC 接口
- **接口定义**: `idl/backbon.proto`
- **生成代码**: `kitex_gen/backbonservice/`
- **主要功能**:
  - 查询用户连接状态（CheckUserOnline）
  - 推送数据到客户端（PushData、PushToConnection）
  - 服务注册管理（RegisterService、UnregisterService、GetService）

### 3. BackService（网关调用后端服务）
- **位置**: 后端服务需要实现此接口
- **接口定义**: `idl/backservice.proto`
- **生成代码**: `kitex_gen/backservice/`
- **主要功能**: 网关通过此接口将客户端请求转发给后端服务（Call 方法）

## 数据流动方向

### 1. 客户端请求流程

```
客户端 (WebSocket)
  ↓ 发送请求消息（包含 service、method、payload、metadata）
Frontier (handler_ws.go: HandleMessage)
  ↓ 检查服务注册状态
BackbonService.GetService (查询 service+method 是否注册)
  ↓ 从注册中心获取服务实例
Discovery.GetInstance (获取 ip:port)
  ↓ 转发请求
BackService.Call (RPC 调用后端服务)
  ↓ 处理业务逻辑
后端服务
  ↓ 返回响应
BackService.CallResponse
  ↓ 转发响应
Frontier (handler_ws.go)
  ↓ 发送响应消息
客户端 (WebSocket)
```

### 2. 后端服务推送流程

```
后端服务
  ↓ 调用 BackbonService.PushData
BackbonService (src/backbon/)
  ↓ 查找用户连接
Frontier Hub (hub.go: SendToUser)
  ↓ 发送消息
Connection (connection.go: SendMessage)
  ↓ WebSocket 推送
客户端
```

### 3. 连接建立流程

```
客户端
  ↓ WebSocket 握手（URL 参数：token、track_id）
Frontier (handler_ws.go: ServeHTTP)
  ↓ 验证 token，解析 uid
认证服务（待集成）
  ↓ 创建连接对象
Connection (connection.go)
  ↓ 注册到 Hub
Frontier Hub (hub.go: RegisterConnection)
```

## 关键设计原则

1. **网关不解析业务数据**: `payload` 是字节数组，网关原样转发，由后端服务解析
2. **验证信息在 metadata**: token、track_id 等放在 `metadata` 中，网关提取但不解析业务内容
3. **服务注册简化**: 只注册 (service, method) 映射，ip:port 从注册中心获取
4. **字段名称统一**: 所有链路上的消息结构字段名称保持一致（requestID、type、service、method、payload、error、timestamp、metadata）

## 文件位置说明

### 接口定义
- `idl/backbon.proto`: 后端服务调用长连接服务的接口定义
- `idl/backservice.proto`: 网关调用后端服务的接口定义

### 生成代码
- `kitex_gen/backbonservice/`: BackbonService 的客户端和服务端代码
- `kitex_gen/backservice/`: BackService 的客户端和服务端代码
- `kitex_gen/*.pb.go`: Protobuf 消息定义

### 核心实现
- `src/frontier/`: 前端网关实现（连接管理、消息处理、请求转发）
- `src/backbon/`: 后端接口实现（待实现，处理推送、状态查询、服务注册）

## 待实现功能

1. **BackbonService 实现** (`src/backbon/`):
   - 实现 CheckUserOnline（查询 Hub 中的连接状态）
   - 实现 PushData/PushToConnection（通过 Hub 推送消息）
   - 实现服务注册管理（维护 service+method 映射表）

2. **请求转发实现** (`src/frontier/handler_ws.go: HandleMessage`):
   - 检查服务注册状态（调用 BackbonService.GetService）
   - 从注册中心获取服务实例（使用 discovery.GetInstance）
   - 调用 BackService.Call 转发请求

3. **认证集成** (`src/frontier/handler_ws.go: HandleAuth`):
   - 从 metadata 提取 token
   - 验证 token 并解析 uid
   - 保存到连接元数据

## 代码生成命令

```bash
# 生成 backbon 服务代码
kitex -compiler-path /usr/bin/protoc \
  -module github.com/rhp-QE/roc-foundation-service \
  -service backbonservice \
  -I ./idl \
  ./idl/backbon.proto

# 生成 backservice 服务代码
kitex -compiler-path /usr/bin/protoc \
  -module github.com/rhp-QE/roc-foundation-service \
  -service backservice \
  -I ./idl \
  ./idl/backservice.proto
```

## 依赖

- `github.com/cloudwego/kitex`: RPC 框架
- `github.com/gorilla/websocket`: WebSocket 支持
- `roc-foundation-util-go/service_registry`: 服务注册中心（etcd 等）
