#!/bin/bash

# WebSocket 长连接服务运行脚本

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 默认配置
HOST="${WS_HOST:-0.0.0.0}"
PORT="${WS_PORT:-8080}"
ENABLE_TLS="${WS_ENABLE_TLS:-false}"
CERT_FILE="${WS_CERT_FILE:-}"
KEY_FILE="${WS_KEY_FILE:-}"

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}   WebSocket 长连接服务${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""

# 项目根目录
PROJECT_ROOT="/root/project/imcloud/roc-foundation-service"
MAIN_FILE="$PROJECT_ROOT/long_connection_service/src/frontier/main/main.go"

# 检查文件是否存在
if [ ! -f "$MAIN_FILE" ]; then
    echo -e "${RED}错误: 找不到主程序文件: $MAIN_FILE${NC}"
    exit 1
fi

# 显示配置
echo -e "${BLUE}服务配置:${NC}"
echo -e "  Host: ${YELLOW}$HOST${NC}"
echo -e "  Port: ${YELLOW}$PORT${NC}"
echo -e "  TLS:  ${YELLOW}$ENABLE_TLS${NC}"
if [ "$ENABLE_TLS" = "true" ]; then
    echo -e "  Cert: ${YELLOW}$CERT_FILE${NC}"
    echo -e "  Key:  ${YELLOW}$KEY_FILE${NC}"
fi
echo ""

# 构建运行参数
RUN_ARGS="-host $HOST -port $PORT"

if [ "$ENABLE_TLS" = "true" ]; then
    RUN_ARGS="$RUN_ARGS -tls"
    if [ -n "$CERT_FILE" ]; then
        RUN_ARGS="$RUN_ARGS -cert $CERT_FILE"
    fi
    if [ -n "$KEY_FILE" ]; then
        RUN_ARGS="$RUN_ARGS -key $KEY_FILE"
    fi
fi

echo -e "${YELLOW}正在启动服务...${NC}"
echo ""

# 切换到项目目录
cd "$PROJECT_ROOT"

# 运行服务
go run "$MAIN_FILE" $RUN_ARGS

# 如果服务意外退出
echo ""
echo -e "${RED}服务已停止${NC}"

