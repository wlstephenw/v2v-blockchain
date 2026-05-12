# V2V Blockchain 安全文档

## Task 13.10: 安全审计和渗透测试

本文档描述 V2V Blockchain 的安全特性和最佳实践。

## 安全架构

### 威胁模型

| 威胁类型 | 风险等级 | 防护措施 |
|---------|---------|---------|
| 中间人攻击 | 高 | TLS + 证书验证 |
| 重放攻击 | 高 | 序列号 + 时间戳 |
| 女巫攻击 | 中 | 身份认证 + 准入控制 |
| DDoS 攻击 | 中 | 限流 + 资源隔离 |
| 拜占庭节点 | 中 | PBFT 共识 (f < n/3) |
| 密钥泄露 | 高 | 密钥轮换 + HSM |

### 安全边界

```
┌─────────────────────────────────────────────────────────┐
│                    外部网络 (不可信)                      │
├─────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐  │
│  │   TLS 终止   │  │   限流      │  │   WAF/防火墙     │  │
│  └─────────────┘  └─────────────┘  └─────────────────┘  │
├─────────────────────────────────────────────────────────┤
│                   API 网关层                              │
├─────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐  │
│  │  身份认证    │  │  签名验证    │  │   访问控制       │  │
│  │  (JWT/mTLS) │  │  (ECDSA)    │  │   (RBAC)        │  │
│  └─────────────┘  └─────────────┘  └─────────────────┘  │
├─────────────────────────────────────────────────────────┤
│                   应用服务层                              │
├─────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐  │
│  │   P2P 加密   │  │  PBFT 共识   │  │   数据加密       │  │
│  │  (Noise)    │  │  (BFT)      │  │   (AES-256)     │  │
│  └─────────────┘  └─────────────┘  └─────────────────┘  │
├─────────────────────────────────────────────────────────┤
│                   核心协议层                              │
├─────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────┐  │
│  │   密钥管理   │  │  安全存储    │  │   审计日志       │  │
│  │  (KMS/HSM)  │  │  (加密磁盘)  │  │   (不可篡改)     │  │
│  └─────────────┘  └─────────────┘  └─────────────────┘  │
├─────────────────────────────────────────────────────────┤
│                   数据存储层 (可信)                        │
└─────────────────────────────────────────────────────────┘
```

## 加密机制

### 1. 传输加密

#### P2P 网络 (libp2p)
- **协议**: Noise Protocol Framework
- **密钥交换**: X25519
- **认证**: Ed25519 签名
- **加密**: ChaCha20-Poly1305

```go
// 配置加密传输
import "github.com/libp2p/go-libp2p/p2p/security/noise"

host, err := libp2p.New(
    libp2p.Security(noise.ID, noise.New),
)
```

#### API 通信
- **协议**: TLS 1.3
- **证书**: X.509 v3
- **密码套件**: TLS_AES_256_GCM_SHA384

### 2. 数据加密

#### 静态数据
- **算法**: AES-256-GCM
- **密钥管理**: 硬件安全模块 (HSM) 或 KMS
- **密钥轮换**: 每 90 天

#### 敏感字段
```go
// 加密存储私钥
encryptedKey, err := aesgcm.Seal(nonce, nonce, privateKey, nil)
```

### 3. 端到端加密

#### 车辆间通信
- **算法**: ECIES (Elliptic Curve Integrated Encryption Scheme)
- **曲线**: secp256k1
- **对称加密**: AES-256-GCM

#### 群组通信
- **群组密钥**: 每次编队创建生成新密钥
- **密钥分发**: Leader 使用成员公钥加密分发
- **密钥轮换**: Leader 变更时轮换

## 身份认证

### 1. 车辆身份

#### 证书结构
```
Certificate:
  Subject: Vehicle ID
  Issuer: V2V CA
  Public Key: secp256k1
  Validity: 90 days
  Extensions:
    - Vehicle Type
    - Manufacturer
    - VIN (Vehicle Identification Number)
```

#### 验证流程
```
1. 提取证书中的 Vehicle ID
2. 验证证书签名链 (Root CA -> Intermediate CA -> Vehicle)
3. 检查证书有效期
4. 检查 CRL (证书吊销列表)
5. 验证公钥与 Vehicle ID 匹配
```

### 2. 节点准入

#### 准入控制
```go
// 节点连接拦截器
func (s *SecurityInterceptor) InterceptConn(
    ctx context.Context,
    conn net.Conn,
) (net.Conn, error) {
    // 1. 获取对端证书
    cert := conn.(*tls.Conn).ConnectionState().PeerCertificates[0]
    
    // 2. 验证证书
    if err := s.validateCertificate(cert); err != nil {
        return nil, fmt.Errorf("certificate validation failed: %w", err)
    }
    
    // 3. 检查是否在白名单
    vehicleID := extractVehicleID(cert)
    if !s.isWhitelisted(vehicleID) {
        return nil, fmt.Errorf("vehicle not whitelisted")
    }
    
    return conn, nil
}
```

## 消息安全

### 1. 消息签名

#### 签名流程
```
1. 序列化消息 (排除签名字段)
2. 使用发送者私钥签名 (ECDSA)
3. 附加签名到消息
```

#### 验证流程
```
1. 提取消息中的签名
2. 从区块链获取发送者公钥
3. 验证签名
4. 验证时间戳 (±30秒窗口)
5. 验证序列号 (防重放)
```

### 2. 重放防护

#### 机制
- **序列号**: 每个发送者单调递增
- **时间戳**: 30 秒有效期窗口
- **ID 缓存**: 最近 10000 条消息

```go
type MessageValidator struct {
    seenMessages *lru.Cache // 消息ID缓存
    seqTrackers  map[Address]*SequenceTracker
}

func (v *MessageValidator) Validate(msg *V2VMessage) error {
    // 检查重复
    if v.seenMessages.Contains(msg.ID) {
        return ErrDuplicateMessage
    }
    
    // 检查序列号
    tracker := v.seqTrackers[msg.Sender]
    if msg.SeqNum <= tracker.LastSeqNum {
        return ErrInvalidSequence
    }
    
    // 检查时间戳
    if abs(time.Now().Unix()-msg.Timestamp) > 30 {
        return ErrExpiredMessage
    }
    
    return nil
}
```

## 共识安全

### 1. PBFT 安全保证

#### 容错能力
- **最大故障节点**: f < n/3
- **4 个 Validator**: 可容忍 1 个故障
- **7 个 Validator**: 可容忍 2 个故障

#### 安全属性
- **安全性**: 所有诚实节点提交相同区块
- **活性**: 最终总能提交新区块
- **一致性**: 已提交区块不会被修改

### 2. 拜占庭检测

#### 异常行为检测
```go
type ByzantineDetector struct {
    messageCounts map[Address]int
    viewChanges   map[uint64]int
}

func (d *ByzantineDetector) Detect(msg *PBFTMessage) {
    // 检测重复投票
    if d.hasDoubleVoted(msg.Sender, msg.View, msg.SeqNumber) {
        d.reportByzantine(msg.Sender, "double voting")
    }
    
    // 检测无效视图变更
    if msg.Type == MsgViewChange && d.isInvalidViewChange(msg) {
        d.reportByzantine(msg.Sender, "invalid view change")
    }
}
```

## 安全配置

### 1. 生产环境检查清单

#### 网络层
- [ ] 启用 TLS 1.3
- [ ] 配置防火墙规则
- [ ] 禁用不安全的密码套件
- [ ] 启用 HSTS

#### 应用层
- [ ] 启用 API 限流
- [ ] 配置 JWT 认证
- [ ] 启用请求日志
- [ ] 配置 CORS 策略

#### 数据层
- [ ] 启用静态加密
- [ ] 配置密钥轮换
- [ ] 启用审计日志
- [ ] 配置备份加密

### 2. 安全加固脚本

```bash
#!/bin/bash
# security-hardening.sh

echo "=== V2V Blockchain Security Hardening ==="

# 1. 生成强密钥
echo "1. Generating strong keys..."
openssl genpkey -algorithm EC -pkeyopt ec_paramgen_curve:secp256k1 -out node.key
chmod 600 node.key

# 2. 配置 TLS
echo "2. Configuring TLS..."
cat > tls.conf << 'EOF'
[req]
distinguished_name = req_distinguished_name
x509_extensions = v3_req
prompt = no

[req_distinguished_name]
CN = v2v-node.example.com

[v3_req]
keyUsage = keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = v2v-node.example.com
IP.1 = 192.168.1.100
EOF

openssl req -new -x509 -key node.key -out node.crt -days 365 -config tls.conf

# 3. 配置防火墙
echo "3. Configuring firewall..."
iptables -A INPUT -p tcp --dport 8080 -s 192.168.1.0/24 -j ACCEPT
iptables -A INPUT -p tcp --dport 10000 -s 192.168.1.0/24 -j ACCEPT
iptables -A INPUT -p tcp --dport 8080 -j DROP
iptables -A INPUT -p tcp --dport 10000 -j DROP

# 4. 设置文件权限
echo "4. Setting file permissions..."
chmod 700 /data/v2v-blockchain
chmod 600 /data/v2v-blockchain/*/keys/*

echo "=== Hardening Complete ==="
```

## 安全审计

### 1. 审计日志

#### 记录内容
- 所有交易提交
- 节点加入/离开
- 编队创建/解散
- 权限变更
- 配置修改

#### 日志格式
```json
{
  "timestamp": "2024-01-15T10:30:00Z",
  "level": "INFO",
  "event": "PLATOON_CREATED",
  "actor": "0xvehicle001...",
  "resource": "platoon-001",
  "action": "create",
  "result": "success",
  "details": {
    "leader_id": "0xvehicle001...",
    "initial_members": 4
  },
  "hash": "0xaudit123...",
  "prev_hash": "0xaudit122..."
}
```

### 2. 定期审计检查

```bash
#!/bin/bash
# security-audit.sh

echo "=== Security Audit ==="

# 1. 检查证书过期
echo "1. Checking certificate expiration..."
for cert in /etc/v2v/certs/*.crt; do
  expiry=$(openssl x509 -enddate -noout -in "$cert" | cut -d= -f2)
  echo "  $cert expires: $expiry"
done

# 2. 检查密钥权限
echo "2. Checking key permissions..."
find /data/v2v-blockchain -name "*.key" -exec ls -l {} \;

# 3. 检查异常登录
echo "3. Checking for suspicious activity..."
grep -i "failed\|error\|unauthorized" /var/log/v2v/*.log | tail -20

# 4. 检查节点健康
echo "4. Checking node health..."
for port in 8081 8082 8083 8084; do
  if ! curl -s http://localhost:$port/health > /dev/null; then
    echo "  WARNING: Node on port $port is unhealthy"
  fi
done

# 5. 检查区块高度差异
echo "5. Checking block height consistency..."
heights=()
for port in 8081 8082 8083 8084; do
  h=$(curl -s http://localhost:$port/api/v1/node/status | jq -r '.blockchain.latest_height')
  heights+=($h)
  echo "  Port $port: Height $h"
done

max=${heights[0]}
min=${heights[0]}
for h in "${heights[@]}"; do
  ((h > max)) && max=$h
  ((h < min)) && min=$h
done

diff=$((max - min))
if [ $diff -gt 10 ]; then
  echo "  WARNING: Block height difference is $diff (> 10)"
fi

echo "=== Audit Complete ==="
```

## 应急响应

### 1. 安全事件响应流程

```
1. 检测
   └── 监控告警 / 日志分析 / 用户报告

2. 遏制
   ├── 隔离受影响节点
   ├── 阻断恶意连接
   └── 暂停敏感操作

3. 根除
   ├── 分析攻击向量
   ├── 修复漏洞
   └── 清理恶意数据

4. 恢复
   ├── 从备份恢复
   ├── 验证系统完整性
   └── 逐步恢复服务

5. 总结
   ├── 编写事件报告
   ├── 更新安全措施
   └── 团队培训
```

### 2. 紧急联系人

| 角色 | 职责 | 联系方式 |
|------|------|---------|
| 安全负责人 | 决策和协调 | security@v2v.io |
| 运维团队 | 技术响应 | ops@v2v.io |
| 开发团队 | 漏洞修复 | dev@v2v.io |
| 法务团队 | 合规报告 | legal@v2v.io |

## 参考

- [OWASP Blockchain Security](https://owasp.org/www-project-blockchain-security/)
- [NIST Blockchain Technology Overview](https://nvlpubs.nist.gov/nistpubs/ir/2018/NIST.IR.8202.pdf)
- [libp2p Security](https://docs.libp2p.io/concepts/security/)
