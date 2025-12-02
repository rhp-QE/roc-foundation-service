# WebSocket 长连接服务

这是一个功能完整的 WebSocket 长连接服务实现，提供了实时消息推送、连接管理、心跳检测等功能。

## 功能特性

- ✅ WebSocket 长连接管理
- ✅ 用户连接映射和管理
- ✅ 消息路由和分发
- ✅ 心跳检测机制
- ✅ 连接状态监控
- ✅ 消息确认机制
- ✅ 广播消息支持
- ✅ 自定义消息处理器
- ✅ 统计信息API
- ✅ 健康检查API

## 架构设计

```
┌─────────────────────────────────────────────┐
│                  Client                     │
└───────────────────┬─────────────────────────┘
                    │ WebSocket
                    ▼
┌─────────────────────────────────────────────┐
│              HTTP Server                    │
│  - /ws        (WebSocket连接)               │
│  - /health    (健康检查)                    │
│  - /stats     (统计信息)                    │
└───────────────────┬─────────────────────────┘
                    │
                    ▼
┌─────────────────────────────────────────────┐
│                  Hub                        │
│  - 连接注册/注销                            │
│  - 消息分发                                 │
│  - 心跳检查                                 │
│  - 消息处理器路由                           │
└───────────────────┬─────────────────────────┘
                    │
            ┌───────┴────────┐
            ▼                ▼
    ┌──────────────┐  ┌──────────────┐
    │ Connection 1 │  │ Connection N │
    │  - ReadPump  │  │  - ReadPump  │
    │  - WritePump │  │  - WritePump │
    └──────────────┘  └──────────────┘
```

## 核心组件

### 1. Server (`server.go`)
服务器主入口，负责：
- HTTP 服务器的创建和管理
- 路由注册
- 服务启动和停止

### 2. Hub (`hub.go`)
消息分发中心，负责：
- 管理所有活跃连接
- 路由消息到目标连接
- 心跳检查
- 统计信息收集

### 3. Connection (`connection.go`)
单个 WebSocket 连接的封装，负责：
- 消息的读取和写入
- 连接状态管理
- 元数据存储

### 4. Message (`message.go`)
消息结构定义，包括：
- 消息类型定义
- 消息验证
- 数据访问方法

### 5. Handler (`handler.go`)
HTTP 处理器，包括：
- WebSocket 连接升级
- 认证处理
- 消息处理
- 统计信息和健康检查

## 快速开始

### 1. 安装依赖

```bash
cd /root/project/imcloud/roc-foundation-service
go mod tidy
```

### 2. 基本使用

```go
package main

import (
    "log"
    "github.com/rhp-QE/roc-foundation-service/long_connection_service/src/frontier"
)

func main() {
    // 使用默认配置创建服务器
    server := frontier.NewServer(nil)
    
    // 启动服务器
    if err := server.Start(); err != nil {
        log.Fatal(err)
    }
}
```

### 3. 自定义配置

```go
config := &frontier.Config{
    Host:              "0.0.0.0",
    Port:              "8080",
    HeartbeatInterval: 30 * time.Second,
    ConnectionTimeout: 90 * time.Second,
    MaxMessageSize:    512 * 1024,
    EnableTLS:         false,
}

server := frontier.NewServer(config)
```

### 4. 注册自定义消息处理器

```go
hub := server.GetHub()

hub.RegisterMessageHandler("custom_message", func(conn *frontier.Connection, msg *frontier.Message) error {
    // 处理自定义消息
    log.Printf("Received custom message: %v", msg.Data)
    
    // 发送响应
    response := frontier.NewMessage("custom_response")
    response.SetData("status", "ok")
    return conn.SendMessage(response)
})
```

## API 端点

### WebSocket 连接
```
ws://localhost:8080/ws?user_id={user_id}
```

### 健康检查
```
GET http://localhost:8080/health
```

响应示例：
```json
{
    "status": "ok",
    "active_connections": 42,
    "timestamp": 1701234567
}
```

### 统计信息
```
GET http://localhost:8080/stats
```

响应示例：
```json
{
    "total_connections": 1000,
    "active_connections": 42,
    "messages_received": 5000,
    "messages_sent": 4800,
    "timestamp": 1701234567
}
```

## 消息格式

### 基本消息结构

```json
{
    "id": "msg_unique_id",
    "type": "chat",
    "from": "user_123",
    "to": "user_456",
    "data": {
        "text": "Hello, World!"
    },
    "timestamp": 1701234567
}
```

### 支持的消息类型

1. **认证消息** (`auth`)
```json
{
    "type": "auth",
    "data": {
        "token": "your_token_here",
        "user_id": "user_123"
    }
}
```

2. **心跳消息** (`heartbeat`)
```json
{
    "type": "heartbeat",
    "data": {}
}
```

3. **聊天消息** (`chat`)
```json
{
    "type": "chat",
    "from": "user_123",
    "to": "user_456",
    "data": {
        "text": "Hello!"
    }
}
```

4. **广播消息** (`broadcast`)
```json
{
    "type": "broadcast",
    "data": {
        "text": "System announcement"
    }
}
```

5. **确认消息** (`ack`)
```json
{
    "type": "ack",
    "id": "msg_id",
    "data": {
        "status": "delivered"
    }
}
```

## 客户端示例

### JavaScript/WebSocket API

```javascript
const ws = new WebSocket('ws://localhost:8080/ws?user_id=user_123');

ws.onopen = () => {
    console.log('Connected');
    
    // 发送消息
    ws.send(JSON.stringify({
        type: 'chat',
        to: 'user_456',
        data: {
            text: 'Hello!'
        }
    }));
};

ws.onmessage = (event) => {
    const message = JSON.parse(event.data);
    console.log('Received:', message);
};

ws.onclose = () => {
    console.log('Disconnected');
};

ws.onerror = (error) => {
    console.error('Error:', error);
};
```

### Go 客户端

```go
import (
    "github.com/gorilla/websocket"
)

// 连接到服务器
conn, _, err := websocket.DefaultDialer.Dial("ws://localhost:8080/ws?user_id=user_123", nil)
if err != nil {
    log.Fatal(err)
}
defer conn.Close()

// 发送消息
msg := map[string]interface{}{
    "type": "chat",
    "to":   "user_456",
    "data": map[string]interface{}{
        "text": "Hello!",
    },
}
conn.WriteJSON(msg)

// 接收消息
for {
    var received map[string]interface{}
    err := conn.ReadJSON(&received)
    if err != nil {
        log.Println("Error:", err)
        break
    }
    log.Printf("Received: %v", received)
}
```

## 配置说明

| 参数 | 类型 | 默认值 | 说明 |
|------|------|--------|------|
| Host | string | "0.0.0.0" | 监听地址 |
| Port | string | "8080" | 监听端口 |
| HeartbeatInterval | time.Duration | 30s | 心跳检查间隔 |
| ConnectionTimeout | time.Duration | 90s | 连接超时时间 |
| MaxMessageSize | int64 | 512KB | 最大消息大小 |
| SendBufferSize | int | 256 | 发送缓冲区大小 |
| ReceiveBufferSize | int | 1024 | 接收缓冲区大小 |
| EnableTLS | bool | false | 是否启用TLS |
| MaxConnections | int | 10000 | 最大连接数 |

## 性能优化建议

1. **调整缓冲区大小**
   - 根据消息频率调整 `SendBufferSize`
   - 高频消息场景建议增大缓冲区

2. **心跳配置**
   - 根据网络环境调整心跳间隔
   - 移动网络建议缩短间隔

3. **连接池**
   - 客户端应实现连接池复用

4. **消息批处理**
   - 利用 WritePump 的批量发送机制

## 安全建议

1. **认证**
   - 在 WebSocket 握手时验证 token
   - 使用 JWT 或类似机制

2. **Origin 检查**
   - 在生产环境中严格检查 Origin

3. **速率限制**
   - 实现消息发送速率限制
   - 防止滥用和攻击

4. **TLS**
   - 生产环境建议启用 TLS

## 监控和日志

建议监控以下指标：

- 活跃连接数
- 消息吞吐量
- 平均消息延迟
- 错误率
- 连接建立/断开频率

## 常见问题

### Q: 如何处理连接断开重连？
A: 客户端应实现指数退避重连策略，服务端会自动清理断开的连接。

### Q: 如何保证消息送达？
A: 实现消息确认机制（ACK），客户端收到 ACK 后才认为消息发送成功。

### Q: 如何处理离线消息？
A: 需要配合消息队列（如 Redis/Kafka）存储离线消息，用户上线后推送。

### Q: 如何实现多实例部署？
A: 使用 Redis Pub/Sub 或消息队列实现实例间消息同步。

## 许可证

MIT License

