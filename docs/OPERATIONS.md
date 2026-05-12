# V2V Blockchain 运维手册

## Task 13.8: 运维手册

本文档提供 V2V Blockchain 的日常运维指南。

## 目录

1. [日常监控](#日常监控)
2. [节点管理](#节点管理)
3. [数据管理](#数据管理)
4. [故障处理](#故障处理)
5. [升级维护](#升级维护)
6. [安全配置](#安全配置)

---

## 日常监控

### 健康检查

```bash
# 检查所有节点健康状态
#!/bin/bash
for port in 8081 8082 8083 8084; do
  status=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:$port/health)
  if [ "$status" = "200" ]; then
    echo "✓ Node on port $port: Healthy"
  else
    echo "✗ Node on port $port: Unhealthy (HTTP $status)"
  fi
done
```

### 关键指标监控

| 指标 | 检查命令 | 告警阈值 |
|------|---------|---------|
| 区块高度 | `curl /api/v1/node/status \| jq '.blockchain.latest_height'` | 停滞 > 30s |
| 节点数量 | `curl /api/v1/network/peers \| jq '.count'` | < 4 |
| 内存使用 | `docker stats --no-stream` | > 80% |
| 磁盘使用 | `df -h` | > 85% |
| 交易池大小 | `curl /api/v1/blockchain/transactions/pending \| jq '.count'` | > 1000 |

### 日志监控

```bash
# 实时查看错误日志
docker-compose logs -f | grep -i error

# 查看特定时间段日志
docker-compose logs --since 2024-01-15 validator1

# 导出日志
docker-compose logs validator1 > validator1.log
```

---

## 节点管理

### 查看节点状态

```bash
# 查看所有容器状态
docker-compose ps

# 查看节点详细信息
curl -s http://localhost:8081/api/v1/node/status | jq .
```

### 重启节点

```bash
# 重启单个节点
docker-compose restart validator2

# 滚动重启所有节点
for node in validator1 validator2 validator3 validator4; do
  echo "Restarting $node..."
  docker-compose restart $node
  sleep 10
  # 等待节点就绪
  until curl -s http://localhost:8081/ready > /dev/null; do
    sleep 2
  done
  echo "$node is ready"
done
```

### 添加新节点

1. **编辑 docker-compose.yml**

```yaml
validator5:
  build: .
  container_name: v2v-validator5
  ports:
    - "8087:8080"
    - "10007:10000"
  volumes:
    - validator5-data:/data
  environment:
    - V2V_NODE_ID=validator5
    - V2V_VALIDATOR=true
    - V2V_BOOTSTRAP=/dns4/validator1/tcp/10000
  command: >
    start
    --data-dir /data
    --api-port 8080
    --validator
    --bootstrap /dns4/validator1/tcp/10000
```

2. **启动新节点**

```bash
docker-compose up -d validator5
```

3. **验证加入**

```bash
curl -s http://localhost:8087/api/v1/node/status | jq .
```

### 移除节点

```bash
# 停止并移除节点
docker-compose stop validator5
docker-compose rm validator5

# 清理数据 (可选)
docker volume rm v2v-blockchain_validator5-data
```

---

## 数据管理

### 数据备份

```bash
#!/bin/bash
# backup.sh - 备份脚本

BACKUP_DIR="/backup/v2v/$(date +%Y%m%d_%H%M%S)"
mkdir -p $BACKUP_DIR

# 备份每个节点的数据
for node in validator1 validator2 validator3 validator4; do
  echo "Backing up $node..."
  docker run --rm \
    -v v2v-blockchain_${node}-data:/data \
    -v $BACKUP_DIR:/backup \
    alpine tar czf /backup/${node}.tar.gz -C /data .
done

# 备份配置
cp docker-compose.yml $BACKUP_DIR/
cp -r config/ $BACKUP_DIR/

echo "Backup completed: $BACKUP_DIR"
```

### 数据恢复

```bash
#!/bin/bash
# restore.sh - 恢复脚本

BACKUP_FILE="$1"
NODE_NAME="$2"

if [ -z "$BACKUP_FILE" ] || [ -z "$NODE_NAME" ]; then
  echo "Usage: $0 <backup-file> <node-name>"
  exit 1
fi

# 停止节点
docker-compose stop $NODE_NAME

# 恢复数据
docker run --rm \
  -v v2v-blockchain_${NODE_NAME}-data:/data \
  -v $(pwd):/backup \
  alpine tar xzf /backup/$BACKUP_FILE -C /data

# 启动节点
docker-compose start $NODE_NAME

echo "Restore completed for $NODE_NAME"
```

### 数据清理

```bash
# 清理旧日志
docker system prune --volumes

# 压缩历史数据 (保留最近7天)
find /data/v2v-blockchain -name "*.log" -mtime +7 -exec gzip {} \;

# 归档旧区块 (保留最近10000个区块)
# 注意: 需要实现归档脚本
```

---

## 故障处理

### 节点无法启动

**症状**: 容器不断重启

**排查步骤**:
```bash
# 1. 查看日志
docker-compose logs validator1

# 2. 检查端口冲突
netstat -tlnp | grep 8081

# 3. 检查数据目录权限
ls -la /var/lib/docker/volumes/v2v-blockchain_validator1-data/

# 4. 检查磁盘空间
df -h
```

**解决方案**:
```bash
# 端口冲突: 修改 docker-compose.yml 中的端口映射
# 权限问题: 修复权限
sudo chown -R 1000:1000 /var/lib/docker/volumes/v2v-blockchain_*/

# 磁盘满: 清理空间或扩容
docker system prune -a
```

### 网络分区

**症状**: 节点间无法通信，区块高度不一致

**排查步骤**:
```bash
# 检查节点连接数
curl -s http://localhost:8081/api/v1/network/peers | jq '.count'

# 检查节点间连通性
docker-compose exec validator1 ping validator2

# 查看网络日志
docker-compose logs | grep -i "network\|peer\|connection"
```

**解决方案**:
```bash
# 重启网络服务
docker-compose restart

# 如果问题持续，检查防火墙规则
iptables -L | grep 10000
```

### Leader 故障

**症状**: 区块停止生成

**排查步骤**:
```bash
# 检查当前 Leader
curl -s http://localhost:8081/api/v1/node/status | jq '.consensus'

# 查看 Leader 日志
docker-compose logs validator1 | grep -i "leader\|view\|primary"
```

**解决方案**:
```bash
# PBFT 会自动触发 View Change，等待新 Leader 选举
# 如果长时间未恢复，手动重启 Leader 节点
docker-compose restart validator1
```

### 数据不一致

**症状**: 不同节点区块高度差异大

**排查步骤**:
```bash
# 比较各节点高度
for port in 8081 8082 8083 8084; do
  height=$(curl -s http://localhost:$port/api/v1/node/status | jq '.blockchain.latest_height')
  echo "Port $port: Height $height"
done
```

**解决方案**:
```bash
# 停止落后节点
docker-compose stop validator2

# 删除数据
docker volume rm v2v-blockchain_validator2-data

# 重新启动 (会自动同步)
docker-compose up -d validator2
```

### 性能下降

**症状**: TPS 降低，延迟增加

**排查步骤**:
```bash
# 检查资源使用
docker stats --no-stream

# 检查交易池
curl -s http://localhost:8081/api/v1/blockchain/transactions/pending | jq '.count'

# 检查日志中的慢操作
docker-compose logs | grep -i "slow\|timeout\|latency"
```

**解决方案**:
```bash
# 增加资源限制
docker-compose stop
docker-compose up -d --scale validator1=1  # 使用新的资源限制

# 清理交易池 (谨慎操作)
# 需要实现清理接口
```

---

## 升级维护

### 滚动升级

```bash
#!/bin/bash
# rolling-upgrade.sh

VERSION="$1"
if [ -z "$VERSION" ]; then
  echo "Usage: $0 <version>"
  exit 1
fi

# 1. 拉取新镜像
docker pull v2v-blockchain:$VERSION

# 2. 逐个升级 Validator (保持 quorum)
for node in validator2 validator3 validator4 validator1; do
  echo "Upgrading $node..."
  
  # 停止节点
  docker-compose stop $node
  
  # 更新镜像
  docker-compose up -d $node
  
  # 等待就绪
  until curl -s http://localhost:8081/ready > /dev/null; do
    sleep 2
  done
  
  echo "$node upgraded successfully"
  sleep 10
done

# 3. 升级 Follower
docker-compose up -d follower1 follower2

echo "Upgrade completed to version $VERSION"
```

### 配置更新

```bash
# 1. 修改配置
cp config/config.yaml config/config.yaml.bak
vim config/config.yaml

# 2. 重启节点应用配置
docker-compose restart

# 3. 验证配置
curl -s http://localhost:8081/api/v1/node/status | jq .
```

---

## 安全配置

### 访问控制

```bash
# 限制 API 访问 (使用 Nginx 反向代理)
cat > nginx.conf << 'EOF'
server {
    listen 80;
    server_name v2v-api.example.com;
    
    location / {
        allow 192.168.1.0/24;  # 只允许内网访问
        deny all;
        
        proxy_pass http://localhost:8081;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
EOF
```

### TLS 配置

```bash
# 生成自签名证书 (测试用)
openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
  -keyout v2v.key -out v2v.crt \
  -subj "/C=US/ST=State/L=City/O=Organization/CN=v2v.example.com"

# 或使用 Let's Encrypt
certbot --nginx -d v2v.example.com
```

### 密钥管理

```bash
# 备份密钥
mkdir -p /secure/backup
cp -r /data/v2v-blockchain/*/keys /secure/backup/
chmod 600 /secure/backup/keys/*

# 定期轮换密钥 (每90天)
# 需要实现密钥轮换脚本
```

---

## 常用脚本

### 日常检查脚本

```bash
#!/bin/bash
# daily-check.sh

echo "=== V2V Blockchain Daily Check ==="
echo "Date: $(date)"
echo ""

# 检查节点健康
echo "1. Node Health Check"
for port in 8081 8082 8083 8084; do
  if curl -s http://localhost:$port/health > /dev/null; then
    echo "  ✓ Port $port: OK"
  else
    echo "  ✗ Port $port: FAILED"
  fi
done
echo ""

# 检查区块高度
echo "2. Block Height"
for port in 8081 8082 8083 8084; do
  height=$(curl -s http://localhost:$port/api/v1/node/status | jq -r '.blockchain.latest_height')
  echo "  Port $port: Height $height"
done
echo ""

# 检查资源使用
echo "3. Resource Usage"
docker stats --no-stream --format "table {{.Name}}\t{{.CPUPerc}}\t{{.MemUsage}}"
echo ""

# 检查磁盘空间
echo "4. Disk Space"
df -h | grep -E "Filesystem|/dev/"
echo ""

echo "=== Check Complete ==="
```

### 告警脚本

```bash
#!/bin/bash
# alert.sh

WEBHOOK_URL="https://hooks.slack.com/services/..."

send_alert() {
  local message="$1"
  curl -X POST -H 'Content-type: application/json' \
    --data "{\"text\":\"$message\"}" \
    $WEBHOOK_URL
}

# 检查节点健康
for port in 8081 8082 8083 8084; do
  if ! curl -s http://localhost:$port/health > /dev/null; then
    send_alert "🚨 ALERT: Node on port $port is down!"
  fi
done

# 检查区块高度停滞
# 需要实现历史高度比较逻辑
```

---

## 参考

- [部署文档](DEPLOYMENT.md)
- [API 文档](API.md)
- [架构设计](ARCHITECTURE.md)
- [快速入门](QUICKSTART.md)

## 获取支持

- GitHub Issues: https://github.com/your-org/v2v-blockchain/issues
- 邮件: ops@v2v-blockchain.io
- 紧急联系: +86-xxx-xxxx-xxxx
