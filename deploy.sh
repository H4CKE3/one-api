#!/bin/bash

# 快速更新部署脚本 - One API

echo "================================"
echo "One API 快速更新部署脚本"
echo "================================"
echo ""

echo "1. 拉取最新代码..."
git pull origin

echo ""
echo "2. 切换到 main 分支..."
git checkout build/local-docker

echo ""
echo "3. 停止并删除旧容器..."
docker compose down

echo ""
echo "4. 删除旧镜像（如果存在）..."
# 获取容器使用的镜像ID
OLD_IMAGE=$(docker images --filter=reference="*one-api*" --format "{{.ID}}" | head -1)
if [ ! -z "$OLD_IMAGE" ]; then
    docker rmi -f $OLD_IMAGE 2>/dev/null || true
    echo "已删除旧镜像: $OLD_IMAGE"
else
    echo "未找到旧的 one-api 镜像"
fi

# 也尝试删除可能有项目名前缀的镜像
PROJECT_NAME=$(basename $(pwd))
OLD_PROJECT_IMAGE=$(docker images --filter=reference="${PROJECT_NAME}-one-api*" --format "{{.ID}}" | head -1)
if [ ! -z "$OLD_PROJECT_IMAGE" ]; then
    docker rmi -f $OLD_PROJECT_IMAGE 2>/dev/null || true
    echo "已删除项目镜像: $OLD_PROJECT_IMAGE"
fi

echo ""
echo "5. 清理未使用的镜像..."
docker image prune -f

echo ""
echo "6. 重新构建并启动服务..."
docker compose up -d --build

echo ""
echo "7. 等待服务启动..."
sleep 5

echo ""
echo "8. 查看服务状态..."
docker compose ps

echo ""
echo "================================"
echo "部署完成！"
echo "================================"
echo ""
echo "查看日志: docker compose logs -f one-api"
echo "停止服务: docker compose down"
echo ""

