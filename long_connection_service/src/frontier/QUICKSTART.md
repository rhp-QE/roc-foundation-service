# WebSocket 长连接服务 - 快速开始

## 🚀 5分钟快速启动

### 1. 安装依赖

```bash
cd /root/project/imcloud/roc-foundation-service
go mod tidy
```

### 2. 启动服务

**方式一：使用脚本运行（推荐）**

```bash
cd /root/project/imcloud/roc-foundation-service/long_connection_service/src/frontier
./scripts/run.sh
```

**方式二：直接运行**

```bash
cd /root/project/imcloud/roc-foundation-service
go run long_connection_service/src/frontier/main/main.go
```

**方式三：构建后运行**

```bash
cd /root/project/imcloud/roc-foundation-service/long_connection_service/src/frontier
./scripts/build.sh
../../bin/ws-server
```

### 3. 测试连接

**方式一：使用浏览器客户端（最简单）**

1. 用浏览器打开 `client_example.html` 文件
2. 保持默认设置或修改服务器地址
3. 点击"连接"按钮
4. 开始发送消息测试

**方式二：使用 curl 测试健康检查**

```bash
curl http://localhost:8080/health
```

预期输出：
```json
{
    "status": "ok",
    "active_connections": 0,
    "timestamp": 1701234567
}
```

**方式三：使用 curl 测试统计信息**

```bash
curl http://localhost:8080/stats
```

预期输出：
```json
{
    "total_connections": 0,
    "active_connections": 0,
    "messages_received": 0,
    "messages_sent": 0,
    "timestamp": 1701234567
}
```

## 📱 客户端连接示例

### JavaScript 客户端

```javascript
// 连接到服务器
const ws = new WebSocket('ws://localhost:8080/ws?user_id=user_123');

ws.onopen = () => {
    console.log('连接成功');
    
    // 发送聊天消息
    ws.send(JSON.stringify({
        type: 'chat',
        to: 'user_456',
        data: {
            text: 'Hello, World!'
        }
    }));
};

ws.onmessage = (event) => {
    const message = JSON.parse(event.data);
    console.log('收到消息:', message);
};

ws.onerror = (error) => {
    console.error('错误:', error);
};

ws.onclose = () => {
    console.log('连接关闭');
};
```

### Go 客户端

```go
package main

import (
    "encoding/json"
    "log"
    "github.com/gorilla/websocket"
)

func main() {
    // 连接到服务器
    conn, _, err := websocket.DefaultDialer.Dial(
        "ws://localhost:8080/ws?user_id=user_123", 
        nil,
    )
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()

    // 发送消息
    msg := map[string]interface{}{
        "type": "chat",
        "to":   "user_456",
        "data": map[string]interface{}{
            "text": "Hello, World!",
        },
    }
    
    if err := conn.WriteJSON(msg); err != nil {
        log.Fatal(err)
    }

    // 接收消息
    var received map[string]interface{}
    if err := conn.ReadJSON(&received); err != nil {
        log.Fatal(err)
    }
    
    log.Printf("收到消息: %v", received)
}
```

### Python 客户端

```python
import asyncio
import websockets
import json

async def client():
    uri = "ws://localhost:8080/ws?user_id=user_123"
    
    async with websockets.connect(uri) as websocket:
        # 发送消息
        message = {
            "type": "chat",
            "to": "user_456",
            "data": {
                "text": "Hello, World!"
            }
        }
        await websocket.send(json.dumps(message))
        
        # 接收消息
        response = await websocket.recv()
        print(f"收到消息: {response}")

asyncio.run(client())
```

## 🔧 环境变量配置

可以通过环境变量配置服务：

```bash
# 设置主机地址
export WS_HOST=0.0.0.0

# 设置端口
export WS_PORT=8080

# 启用 TLS
export WS_ENABLE_TLS=true
export WS_CERT_FILE=/path/to/cert.pem
export WS_KEY_FILE=/path/to/key.pem

# 运行服务
./scripts/run.sh
```

## 🎯 常见使用场景

### 1. 发送心跳消息

```json
{
    "type": "heartbeat",
    "data": {}
}
```

### 2. 发送聊天消息

```json
{
    "type": "chat",
    "to": "user_456",
    "data": {
        "text": "Hello!",
        "extra": "any custom data"
    }
}
```

### 3. 广播消息（需要权限）

```json
{
    "type": "broadcast",
    "data": {
        "text": "系统公告：服务器将在30分钟后维护"
    }
}
```

### 4. 自定义消息

```json
{
    "type": "ping",
    "data": {
        "custom_field": "custom_value"
    }
}
```

## 🐛 故障排查

### 问题1：连接失败

**症状**：无法连接到 WebSocket 服务器

**解决方案**：
1. 检查服务是否正在运行：`curl http://localhost:8080/health`
2. 检查防火墙设置
3. 确认端口没有被占用：`lsof -i :8080`

### 问题2：消息发送失败

**症状**：发送消息后没有响应

**解决方案**：
1. 检查消息格式是否正确（必须是有效的 JSON）
2. 确认 `type` 字段是否存在
3. 查看服务器日志

### 问题3：连接频繁断开

**症状**：连接建立后很快断开

**解决方案**：
1. 实现心跳机制，定期发送心跳消息
2. 检查网络稳定性
3. 调整服务器的超时配置

## 📊 性能测试

使用 `websocat` 工具进行压力测试：

```bash
# 安装 websocat
# Linux
wget https://github.com/vi/websocat/releases/download/v1.11.0/websocat_amd64-linux
chmod +x websocat_amd64-linux
sudo mv websocat_amd64-linux /usr/local/bin/websocat

# 连接测试
websocat ws://localhost:8080/ws?user_id=test_user

# 发送消息
echo '{"type":"heartbeat","data":{}}' | websocat ws://localhost:8080/ws?user_id=test_user
```

## 🔐 安全建议

### 开发环境

- 当前配置允许所有 Origin，适合开发测试
- 使用 HTTP 连接（非加密）

### 生产环境

1. **启用 TLS**
```bash
./bin/ws-server -tls -cert /path/to/cert.pem -key /path/to/key.pem
```

2. **限制 Origin**
修改 `handler.go` 中的 `CheckOrigin` 函数：
```go
CheckOrigin: func(r *http.Request) bool {
    origin := r.Header.Get("Origin")
    return origin == "https://yourdomain.com"
}
```

3. **添加认证**
在连接建立时验证 token，参考 `HandleAuth` 函数实现

4. **速率限制**
使用中间件限制消息发送频率

## 📚 更多资源

- 完整文档：[README.md](README.md)
- 代码示例：[example_test.go](example_test.go)
- 测试客户端：[client_example.html](client_example.html)

## 💡 提示

1. 使用浏览器开发者工具（F12）的网络标签可以查看 WebSocket 消息
2. 支持多个客户端同时连接
3. 每个用户可以有多个连接（多设备登录）
4. 按 `Ctrl+C` 可以优雅关闭服务器

---

如有问题，请查看 [README.md](README.md) 获取更多详细信息。

