#!/bin/bash

# 设置颜色输出
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

echo -e "${BLUE}===== 开始测试 RocketMQ 配置 =====${NC}"
echo -e "${BLUE}当前工作目录: $(pwd)${NC}"
echo ""

# 确保依赖已安装
echo -e "${BLUE}检查依赖...${NC}"
if ! go mod tidy; then
    echo -e "${RED}依赖检查失败!${NC}"
    exit 1
else
    echo -e "${GREEN}依赖检查完成.${NC}"
fi

# 编译测试程序
echo -e "${BLUE}编译测试程序...${NC}"
if ! go build -o rocket_test test_rocket.go; then
    echo -e "${RED}编译失败!${NC}"
    exit 1
else
    echo -e "${GREEN}编译成功.${NC}"
fi

# 运行测试
echo -e "${BLUE}运行 RocketMQ 测试...${NC}"
echo -e "${BLUE}注意: 使用 Ctrl+C 终止测试${NC}"
echo ""

./rocket_test

# 清理编译产物
echo -e "${BLUE}清理临时文件...${NC}"
rm -f rocket_test

echo -e "${GREEN}===== 测试完成 =====${NC}" 