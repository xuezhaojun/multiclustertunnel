#!/bin/bash
echo "🔧 重新构建和更新mctunnel-server和mctunnel-agent镜像..."

# 1. 重新构建Server镜像
echo "1. 重新构建服务器镜像..."
docker build -f build/server/Dockerfile -t mctunnel-server:latest .
if [ $? -ne 0 ]; then
    echo "❌ 服务器镜像构建失败"
    exit 1
fi

# 2. 重新构建Agent镜像
echo "2. 重新构建Agent镜像..."
docker build -f build/agent/Dockerfile -t mctunnel-agent:latest .
if [ $? -ne 0 ]; then
    echo "❌ Agent镜像构建失败"
    exit 1
fi

# 3. 加载Server镜像到Kind集群
echo "3. 加载服务器镜像到Kind集群..."
kind load docker-image mctunnel-server:latest --name mctunnel-e2e
if [ $? -ne 0 ]; then
    echo "❌ 服务器镜像加载失败"
    exit 1
fi

# 4. 加载Agent镜像到Kind集群
echo "4. 加载Agent镜像到Kind集群..."
kind load docker-image mctunnel-agent:latest --name mctunnel-e2e
if [ $? -ne 0 ]; then
    echo "❌ Agent镜像加载失败"
    exit 1
fi

# 5. 重启Server部署
echo "5. 重启服务器部署..."
kubectl --context kind-mctunnel-e2e rollout restart deployment mctunnel-server -n mctunnel-hub

# 6. 重启Agent部署
echo "6. 重启Agent部署..."
kubectl --context kind-mctunnel-e2e rollout restart deployment mctunnel-agent -n mctunnel-agent

# 7. 等待Server部署完成
echo "7. 等待服务器部署完成..."
kubectl --context kind-mctunnel-e2e rollout status deployment mctunnel-server -n mctunnel-hub --timeout=120s

# 8. 等待Agent部署完成
echo "8. 等待Agent部署完成..."
kubectl --context kind-mctunnel-e2e rollout status deployment mctunnel-agent -n mctunnel-agent --timeout=120s

if [ $? -eq 0 ]; then
    echo "✅ 所有镜像更新完成！"
    echo ""
    echo "检查Server Pod状态："
    kubectl --context kind-mctunnel-e2e get pods -n mctunnel-hub
    echo ""
    echo "检查Agent Pod状态："
    kubectl --context kind-mctunnel-e2e get pods -n mctunnel-agent
    echo ""
    echo "查看Server最新日志："
    kubectl --context kind-mctunnel-e2e logs -n mctunnel-hub -l app.kubernetes.io/component=server --tail=5
    echo ""
    echo "查看Agent最新日志："
    kubectl --context kind-mctunnel-e2e logs -n mctunnel-agent -l app=mctunnel-agent --tail=5
else
    echo "❌ 部署更新失败"
    exit 1
fi