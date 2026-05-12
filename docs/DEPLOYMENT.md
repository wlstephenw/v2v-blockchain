# V2V Blockchain 部署文档

## Task 13.4: 部署文档

本文档介绍如何部署 V2V Blockchain 节点。

## 系统要求

### 硬件要求

| 节点类型 | CPU | 内存 | 存储 | 网络 |
|---------|-----|------|------|------|
| Validator | 2核+ | 2GB+ | 10GB+ SSD | 100Mbps+ |
| Follower | 1核+ | 1GB+ | 5GB+ SSD | 50Mbps+ |

### 软件要求

- Docker 20.10+
- Docker Compose 2.0+
- Go 1.21+ (如需从源码构建)

## 快速开始

### 使用 Docker Compose 部署 (推荐)

1. **克隆代码**
```bash
git clone https://github.com/your-org/v2v-blockchain.git
cd v2v-blockchain
```

2. **构建镜像**
```bash
docker-compose build
```

3. **启动网络**
```bash
# 启动所有节点
docker-compose up -d

# 查看日志
docker-compose logs -f validator1
```

4. **验证部署**
```bash
# 检查节点健康状态
curl http://localhost:8081/health
curl http://localhost:8082/health

# 查看节点状态
curl http://localhost:8081/api/v1/node/status
```

5. **停止网络**
```bash
docker-compose down

# 清除数据 (谨慎使用)
docker-compose down -v
```

## 生产环境部署

### 单节点部署

```bash
# 拉取镜像
docker pull v2v-blockchain:latest

# 运行节点
docker run -d \
  --name v2v-node \
  -p 8080:8080 \
  -p 10000:10000 \
  -v /data/v2v:/data \
  v2v-blockchain:latest \
  start --validator --api-port 8080
```

### 多节点部署

#### 1. 准备节点配置

创建 `node-config.env`:
```env
V2V_NETWORK_ID=v2v-mainnet
V2V_LOG_LEVEL=info
V2V_VALIDATOR=true
```

#### 2. 启动种子节点

```bash
docker run -d \
  --name v2v-seed \
  --env-file node-config.env \
  -p 8080:8080 \
  -p 10000:10000 \
  -v /data/seed:/data \
  v2v-blockchain:latest
```

#### 3. 启动其他节点

```bash
docker run -d \
  --name v2v-node2 \
  --env-file node-config.env \
  -e V2V_BOOTSTRAP=/ip4/SEED_IP/tcp/10000 \
  -p 8081:8080 \
  -p 10001:10000 \
  -v /data/node2:/data \
  v2v-blockchain:latest
```

## Kubernetes 部署

### 前置条件

- Kubernetes 1.24+
- kubectl 配置完成
- Ingress Controller (nginx 推荐)

### 部署步骤

1. **创建命名空间**
```bash
kubectl apply -f k8s/namespace.yaml
```

2. **应用配置**
```bash
kubectl apply -f k8s/configmap.yaml
```

3. **部署 Validator 节点**
```bash
kubectl apply -f k8s/validator-deployment.yaml
```

4. **部署 Follower 节点**
```bash
kubectl apply -f k8s/follower-deployment.yaml
```

5. **配置 Ingress (可选)**
```bash
kubectl apply -f k8s/ingress.yaml
```

6. **验证部署**
```bash
# 查看 Pod 状态
kubectl get pods -n v2v-blockchain

# 查看服务
kubectl get svc -n v2v-blockchain

# 查看日志
kubectl logs -f v2v-validator-0 -n v2v-blockchain
```

### 扩容

```bash
# 增加 Follower 节点
kubectl scale deployment v2v-follower --replicas=5 -n v2v-blockchain

# 增加 Validator 节点 (需修改 StatefulSet)
kubectl patch statefulset v2v-validator -n v2v-blockchain -p '{"spec":{"replicas":6}}'
```

## 配置说明

### 环境变量

| 变量名 | 说明 | 默认值 |
|--------|------|--------|
| V2V_NODE_ID | 节点唯一标识 | 自动生成 |
| V2V_DATA_DIR | 数据存储目录 | /data |
| V2V_API_PORT | API 服务端口 | 8080 |
| V2V_P2P_PORT | P2P 网络端口 | 10000 |
| V2V_VALIDATOR | 是否为验证者 | false |
| V2V_BOOTSTRAP | 引导节点地址 | 无 |
| V2V_LOG_LEVEL | 日志级别 | info |

### 数据持久化

#### Docker Volume
```yaml
volumes:
  - node-data:/data
```

#### 主机目录挂载
```yaml
volumes:
  - /host/path:/data
```

#### Kubernetes PVC
自动创建 10Gi 持久卷。

## 网络配置

### 端口说明

| 端口 | 协议 | 用途 | 必需 |
|------|------|------|------|
| 8080 | TCP | API/HTTP | 是 |
| 10000 | TCP | P2P 网络 | 是 |

### 防火墙规则

```bash
# 允许节点间通信
iptables -A INPUT -p tcp --dport 10000 -j ACCEPT

# 允许 API 访问
iptables -A INPUT -p tcp --dport 8080 -s TRUSTED_IP -j ACCEPT
```

## 监控检查

### 健康检查端点

- `GET /health` - 存活检查
- `GET /ready` - 就绪检查

### 常用命令

```bash
# 查看节点状态
curl http://localhost:8080/api/v1/node/status

# 查看最新区块
curl http://localhost:8080/api/v1/blockchain/blocks/latest

# 查看节点数量
curl http://localhost:8080/api/v1/network/peers | jq '.peers | length'
```

## 故障排除

### 常见问题

1. **节点无法连接**
   - 检查防火墙设置
   - 验证引导节点地址
   - 查看网络日志

2. **数据目录权限错误**
   ```bash
   chown -R 1000:1000 /data/v2v
   ```

3. **端口冲突**
   - 修改 docker-compose.yml 中的端口映射
   - 或使用不同主机端口

### 日志查看

```bash
# Docker
docker logs -f v2v-node

# Kubernetes
kubectl logs -f v2v-validator-0 -n v2v-blockchain

# 查看所有节点日志
docker-compose logs -f
```

## 升级指南

### 滚动升级

```bash
# 1. 拉取新版本
docker-compose pull

# 2. 滚动重启
docker-compose up -d --no-deps --build validator1
docker-compose up -d --no-deps --build validator2
# ... 依次重启其他节点
```

### 数据迁移

```bash
# 备份数据
docker run --rm -v v2v-data:/data -v $(pwd):/backup alpine tar czf /backup/v2v-backup.tar.gz -C /data .

# 恢复数据
docker run --rm -v v2v-data:/data -v $(pwd):/backup alpine tar xzf /backup/v2v-backup.tar.gz -C /data
```

## 安全建议

1. **使用 TLS** - 生产环境启用 HTTPS
2. **访问控制** - 限制 API 访问 IP
3. **密钥管理** - 使用密钥管理服务
4. **定期备份** - 设置自动备份策略
5. **监控告警** - 配置异常检测

## 参考

- [快速入门指南](QUICKSTART.md)
- [API 文档](API.md)
- [运维手册](OPERATIONS.md)
