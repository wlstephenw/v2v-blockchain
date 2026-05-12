# V2V Blockchain

专为车联网 (Vehicle-to-Vehicle) 场景设计的轻量级区块链系统，支持车辆编队管理和安全通信。

## 特性

- **轻量级区块链核心** - 适配车载资源受限环境
- **PBFT 共识机制** - 支持拜占庭容错，三阶段提交
- **P2P 网络** - 基于 libp2p 和 Gossipsub
- **车辆编队管理** - 创建/加入/离开/解散编队
- **身份认证** - 基于证书的车辆身份验证
- **消息验证** - V2V 消息签名和防重放保护
- **状态追溯** - 完整的审计日志和历史查询

## 快速开始

### 构建

```bash
make build
```

### 启动节点

```bash
./v2v-node start --api-port 8080 --p2p-port 10000 --validator
```

### CLI 命令

```bash
# 创建编队
./v2v-node platoon create --platoon-id platoon-001 --leader 0x1234... --max-size 8

# 查看节点状态
./v2v-node status

# 查询区块
./v2v-node query block --height 100
```

## 架构

```
┌─────────────────────────────────────────────────────────────────┐
│ 应用层: v2v-node CLI │ REST API │ WebSocket                      │
├─────────────────────────────────────────────────────────────────┤
│ 服务层: Platoon │ Identity │ Message │ State                      │
├─────────────────────────────────────────────────────────────────┤
│ 核心层: Blockchain ↔ P2P ↔ TxPool, PBFT Consensus               │
├─────────────────────────────────────────────────────────────────┤
│ 基础设施: LevelDB │ Crypto │ Logger                              │
└─────────────────────────────────────────────────────────────────┘
```

## 技术栈

- **语言**: Go 1.21+
- **P2P**: libp2p
- **存储**: LevelDB
- **共识**: PBFT (自研实现)
- **API**: REST + WebSocket

## 文档

- [架构设计](docs/ARCHITECTURE.md)
- [API 文档](docs/API.md)
- [快速开始](docs/QUICKSTART.md)
- [部署指南](docs/DEPLOYMENT.md)

## License

MIT
