#!/bin/bash

# WebSocket 长连接服务构建脚本

set -e

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}   WebSocket 长连接服务构建脚本${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""

# 项目根目录
PROJECT_ROOT="/root/project/imcloud/roc-foundation-service"
cd "$PROJECT_ROOT"

# 检查 Go 环境
if ! command -v go &> /dev/null; then
    echo -e "${RED}错误: 未找到 Go 环境${NC}"
    exit 1
fi

echo -e "${YELLOW}Go 版本:${NC}"
go version
echo ""

# 整理依赖
echo -e "${YELLOW}正在整理依赖...${NC}"
go mod tidy
echo -e "${GREEN}✓ 依赖整理完成${NC}"
echo ""

# 构建服务
echo -e "${YELLOW}正在构建服务...${NC}"
OUTPUT_DIR="$PROJECT_ROOT/bin"
mkdir -p "$OUTPUT_DIR"

# 构建主程序
go build -o "$OUTPUT_DIR/ws-server" \
    "$PROJECT_ROOT/long_connection_service/src/frontier/main/main.go"

if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ 构建成功！${NC}"
    echo -e "${GREEN}可执行文件位置: $OUTPUT_DIR/ws-server${NC}"
else
    echo -e "${RED}✗ 构建失败${NC}"
    exit 1
fi

echo ""
echo -e "${GREEN}========================================${NC}"
echo -e "${GREEN}   构建完成！${NC}"
echo -e "${GREEN}========================================${NC}"
echo ""
echo -e "运行服务:"
echo -e "  ${YELLOW}$OUTPUT_DIR/ws-server${NC}"
echo ""
echo -e "或使用自定义参数:"
echo -e "  ${YELLOW}$OUTPUT_DIR/ws-server -host 0.0.0.0 -port 8080${NC}"
echo ""

