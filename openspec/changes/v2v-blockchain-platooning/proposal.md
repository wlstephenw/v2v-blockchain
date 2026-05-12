## Why

传统V2V（Vehicle-to-Vehicle）车车通信面临信任缺失、消息篡改和编队管理中心化的问题。随着自动驾驶编队技术的发展，需要一种去中心化、安全可信且适合车载边缘设备的轻量级解决方案，确保编队车辆间的安全协同和状态可追溯。

## What Changes

- 构建轻量级区块链网络层，适配车载设备资源限制
- 实现基于PKI的车辆身份认证与动态节点管理
- 支持车辆编队（platooning）的创建、加入、离开、解散全生命周期管理
- 实现低延迟的轻量级共识机制（PBFT，支持4-100个节点）
- 提供V2V消息签名验证与防篡改存储
- 实现编队状态的链上可追溯记录
- 支持Leader-Follower编队拓扑动态维护

## Capabilities

### New Capabilities
- `lightweight-blockchain-core`: 轻量级区块链核心网络，区块结构、链存储、P2P网络适配车载设备
- `vehicle-identity-auth`: 车辆身份注册、证书管理、节点准入控制
- `platoon-management`: 编队创建、加入、离开、解散、Leader选举与切换（最少4辆车）
- `v2v-consensus`: 车车通信的轻量级PBFT共识算法，低延迟消息确认
- `message-verification`: V2V消息签名、验证、防重放攻击保护，网络异常处理
- `state-traceability`: 编队状态链上记录、历史查询、审计日志
- `api-interface`: REST API和WebSocket接口，支持区块/交易/编队查询和实时推送
- `non-functional-requirements`: 性能指标、安全要求、可用性、可维护性等非功能需求

### Modified Capabilities
- *(无现有能力需要修改)*

## Impact

- 新增轻量级区块链核心模块，需适配车载Linux/嵌入式环境
- 需要车辆证书颁发机构（CA）集成或自建PKI体系
- V2V通信协议层需要扩展区块链消息类型
- 编队控制算法需要集成区块链状态同步
- 车载存储和网络资源使用增加，需进行资源评估
