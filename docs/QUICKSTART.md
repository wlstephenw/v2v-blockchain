# V2V Blockchain 快速入门指南

## Task 13.7: 快速入门指南

本指南帮助你在 5 分钟内启动并运行 V2V Blockchain 网络。

## 前置要求

- Docker 20.10+
- Docker Compose 2.0+
- curl 或 wget (用于测试)

## 快速开始

### 1. 克隆项目

```bash
git clone https://github.com/your-org/v2v-blockchain.git
cd v2v-blockchain
```

### 2. 一键启动

```bash
# 构建并启动 4 个 Validator + 2 个 Follower
docker-compose up -d --build
```

等待约 30 秒让网络初始化完成。

### 3. 验证网络

```bash
# 检查所有节点健康状态
for port in 8081 8082 8083 8084 8085 8086; do
  echo "Node on port $port:"
  curl -s http://localhost:$port/health | jq .
done
```

预期输出:
```json
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

### 4. 查看网络状态

```bash
# 查看 Validator 1 的状态
curl -s http://localhost:8081/api/v1/node/status | jq .
```

预期输出:
```json
{
  "node_id": "validator1",
  "version": "1.0.0",
  "role": "validator",
  "blockchain": {
    "latest_height": 10,
    "sync_status": "synced"
  },
  "network": {
    "peer_count": 5
  }
}
```

## 基本操作

### 创建编队

```bash
# 创建新编队
curl -X POST http://localhost:8081/api/v1/platoons \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Highway Convoy A",
    "leader_id": "0x1234567890abcdef...",
    "params": {
      "max_vehicles": 8,
      "target_speed": 30.0,
      "safe_distance": 20.0
    }
  }'
```

### 查询编队

```bash
# 获取所有编队
curl -s http://localhost:8081/api/v1/platoons | jq .

# 获取特定编队详情
curl -s http://localhost:8081/api/v1/platoons/platoon-001 | jq .
```

### 查看最新区块

```bash
curl -s http://localhost:8081/api/v1/blockchain/blocks/latest | jq .
```

### 提交交易

```bash
# 提交转账交易
curl -X POST http://localhost:8081/api/v1/transactions \
  -H "Content-Type: application/json" \
  -d '{
    "from": "0xvehicle001...",
    "to": "0xvehicle002...",
    "value": 100,
    "nonce": 1,
    "type": "transfer",
    "signature": "0xsig..."
  }'
```

## 使用 v2v-node CLI

### 构建

```bash
# 构建节点
make build

# 或者使用 go build
go build -o v2v-node ./cmd/v2v-node
```

### 常用命令

```bash
# 查看帮助
./v2v-node --help

# 启动本地节点（验证者模式）
./v2v-node start --data-dir ./data --api-port 8080 --validator

# 查看节点状态
./v2v-node status

# 创建编队
./v2v-node platoon create \
  --leader 0xvehicle001... \
  --name "Convoy A" \
  --max-size 8

# 列出所有编队
./v2v-node platoon list

# 查询区块
./v2v-node query block --height 100

# 查询交易
./v2v-node query tx --hash 0xabc123...
```

## 开发模式

### 从源码运行

```bash
# 1. 安装依赖
go mod download

# 2. 运行测试
go test ./...

# 3. 启动节点
go run ./cmd/v2v-node start \
  --data-dir /tmp/v2v-node \
  --api-port 8080 \
  --validator \
  --log-level debug
```

### 本地多节点测试

```bash
# 使用测试脚本
./scripts/test-multi-node.sh
```

## 监控网络

### 实时查看日志

```bash
# 查看所有节点日志
docker-compose logs -f

# 查看特定节点
docker-compose logs -f validator1
```

### 检查网络连接

```bash
# 查看连接的节点
curl -s http://localhost:8081/api/v1/network/peers | jq '.peers | length'

# 查看网络统计
curl -s http://localhost:8081/api/v1/network/stats | jq .
```

## 清理环境

```bash
# 停止所有节点
docker-compose down

# 停止并删除数据 (谨慎使用!)
docker-compose down -v

# 清理构建缓存
docker-compose down --rmi all
```

## 故障排除

### 节点无法启动

```bash
# 检查端口占用
netstat -tlnp | grep 808

# 查看日志
docker-compose logs validator1
```

### 节点无法连接

```bash
# 检查防火墙
docker-compose exec validator1 wget -O- http://validator2:8080/health

# 重启网络
docker-compose restart
```

### 数据损坏

```bash
# 删除数据重新同步
docker-compose down -v
docker-compose up -d
```

## 下一步

- 阅读 [架构设计文档](ARCHITECTURE.md) 了解系统原理
- 查看 [API 文档](API.md) 了解完整接口
- 参考 [部署文档](DEPLOYMENT.md) 进行生产部署
- 学习 [运维手册](OPERATIONS.md) 掌握运维技巧

## 获取帮助

- GitHub Issues: https://github.com/your-org/v2v-blockchain/issues
- 文档: https://docs.v2v-blockchain.io
- 社区: https://discord.gg/v2v-blockchain

## 常见问题

**Q: 最低系统要求是什么?**
A: 2核 CPU, 2GB 内存, 10GB 存储 (Validator 节点)

**Q: 支持多少节点?**
A: 建议 4-7 个 Validator, Follower 无限制

**Q: 如何添加新节点?**
A: 修改 docker-compose.yml 添加新服务，或使用 Kubernetes

**Q: 数据存储在哪里?**
A: Docker Volume 中，可通过 `docker volume ls` 查看

**Q: 如何备份数据?**
A: 参考 [部署文档](DEPLOYMENT.md) 中的数据迁移章节
