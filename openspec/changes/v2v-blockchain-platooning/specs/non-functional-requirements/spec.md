## ADDED Requirements

### Requirement: 性能指标-区块确认延迟
系统 SHALL 满足区块确认延迟指标。

#### Scenario: 正常负载下确认延迟
- **WHEN** 交易被提交到网络且网络负载正常
- **THEN** 交易在100ms内被确认(P99)
- **AND** 平均确认延迟不超过50ms

#### Scenario: 高负载下确认延迟
- **WHEN** 网络TPS达到峰值100
- **THEN** 交易确认延迟不超过500ms(P99)
- **AND** 系统不丢包、不崩溃

#### Scenario: 极端负载降级
- **WHEN** 网络TPS超过系统容量
- **THEN** 交易进入队列等待
- **AND** 队列满时返回"系统繁忙"错误
- **AND** 已提交交易仍能正常处理

### Requirement: 性能指标-存储限制
系统 SHALL 控制存储使用在车载设备限制内。

#### Scenario: 全节点存储增长
- **WHEN** 节点作为全节点运行30天
- **AND** 平均每日产生1000个区块
- **THEN** 本地存储使用不超过1GB
- **AND** 自动归档触发后释放空间

#### Scenario: 轻客户端存储
- **WHEN** 节点以轻客户端模式运行
- **THEN** 只存储最近1000个区块头
- **AND** 存储使用不超过100MB

#### Scenario: 存储不足处理
- **WHEN** 磁盘空间不足10%
- **THEN** 触发紧急归档
- **AND** 删除最旧的区块数据
- **AND** 向用户发出警告

### Requirement: 性能指标-网络吞吐
系统 SHALL 支持足够的交易吞吐量。

#### Scenario: 正常吞吐
- **WHEN** 网络有4个Validator节点
- **THEN** 系统支持每秒处理100笔交易
- **AND** 每笔交易平均大小为1KB

#### Scenario: 批量交易处理
- **WHEN** 一次性提交批量交易(最多100笔)
- **THEN** 系统在1个区块内打包完成
- **AND** 批量交易的原子性得到保证

### Requirement: 性能指标-节点启动时间
系统 SHALL 快速完成节点启动和同步。

#### Scenario: 冷启动
- **WHEN** 节点首次启动(无本地数据)
- **THEN** 在30秒内完成初始化
- **AND** 在5分钟内完成全量同步(假设10000个区块)

#### Scenario: 热启动
- **WHEN** 节点重启(有本地数据)
- **THEN** 在10秒内恢复服务
- **AND** 自动同步离线期间缺失的区块

#### Scenario: 快速同步模式
- **WHEN** 节点选择快速同步模式
- **THEN** 只同步区块头
- **AND** 同步速度不低于1000 headers/秒

### Requirement: 可用性-系统可用时间
系统 SHALL 保持高可用性。

#### Scenario: 正常运行时间
- **WHEN** 系统部署运行
- **THEN** 月度可用性达到99.9%
- **AND** 计划内维护时间不超过每月4小时

#### Scenario: Leader故障恢复
- **WHEN** Leader节点故障
- **THEN** 系统在6秒内检测到故障（3个出块周期）
- **AND** 在15秒内完成PBFT View Change选举新Leader
- **AND** 服务不中断（由其他Validator继续服务）

### Requirement: 可用性-网络分区容忍
系统 SHALL 容忍网络分区。

#### Scenario: 短暂分区恢复
- **WHEN** 车辆进入信号盲区(不超过30秒)
- **THEN** 节点进入分区模式继续运行
- **AND** 恢复连接后自动同步
- **AND** 解决分区期间的分叉

#### Scenario: 长期分区处理
- **WHEN** 分区时间超过5分钟
- **THEN** 节点标记为"离线"
- **AND** 从Validator集合中临时移除
- **AND** 恢复后需要重新加入共识

### Requirement: 安全性-加密强度
系统 SHALL 使用足够强度的加密算法。

#### Scenario: 签名算法强度
- **WHEN** 系统执行数字签名
- **THEN** 使用ECDSA with secp256k1曲线
- **AND** 私钥长度256位
- **AND** 哈希算法使用Keccak-256

#### Scenario: 密钥存储安全
- **WHEN** 私钥存储在车载设备
- **THEN** 使用硬件安全模块(HSM)或TEE
- **AND** 私钥不以明文形式出现在内存中
- **AND** 支持密钥加密存储

### Requirement: 安全性-访问控制
系统 SHALL 实施严格的访问控制。

#### Scenario: API认证
- **WHEN** 调用管理API
- **THEN** 需要有效的身份令牌
- **AND** 令牌有过期时间
- **AND** 支持令牌吊销

#### Scenario: 权限分级
- **WHEN** 不同角色访问系统
- **THEN** Leader有完整权限
- **AND** Follower只有查询和提交交易权限
- **AND** 未认证节点无法加入网络

### Requirement: 安全性-抗攻击能力
系统 SHALL 抵抗常见攻击。

#### Scenario: DDoS防护
- **WHEN** 收到大量恶意请求
- **THEN** 启用速率限制
- **AND** 超过限制的IP被临时封禁
- **AND** 正常节点不受影响

#### Scenario: Sybil攻击防护
- **WHEN** 攻击者试图创建多个虚假身份
- **THEN** 基于PKI的身份验证阻止未认证节点
- **AND** 异常节点行为被检测并告警

### Requirement: 可维护性-日志记录
系统 SHALL 提供完善的日志。

#### Scenario: 操作日志
- **WHEN** 关键操作执行
- **THEN** 记录操作类型、时间、执行者、结果
- **AND** 日志级别可配置(DEBUG/INFO/WARN/ERROR)
- **AND** 支持日志轮转(每日轮转，保留30天)

#### Scenario: 性能指标采集
- **WHEN** 系统运行
- **THEN** 每10秒采集性能指标(TPS、延迟、资源使用)
- **AND** 指标可导出到Prometheus
- **AND** 支持自定义告警规则

### Requirement: 可维护性-配置管理
系统 SHALL 支持灵活的配置。

#### Scenario: 动态配置更新
- **WHEN** 管理员修改配置文件
- **THEN** 部分配置支持热更新(无需重启)
- **AND** 配置变更记录到审计日志
- **AND** 配置错误时拒绝并提示

#### Scenario: 多环境配置
- **WHEN** 部署到不同环境(开发/测试/生产)
- **THEN** 使用环境变量区分配置
- **AND** 敏感配置(私钥)支持从KMS读取

### Requirement: 兼容性-协议版本
系统 SHALL 支持协议版本兼容。

#### Scenario: 版本协商
- **WHEN** 不同版本节点连接
- **THEN** 协商使用共同支持的协议版本
- **AND** 大版本差异时拒绝连接
- **AND** 记录版本不兼容事件

#### Scenario: 向后兼容
- **WHEN** 系统升级到新版本
- **THEN** 旧区块数据仍然可读
- **AND** 历史交易可以验证
- **AND** 平滑升级无停机

### Requirement: 可观测性-监控告警
系统 SHALL 提供全面的监控能力。

#### Scenario: 健康检查
- **WHEN** 监控系统发送健康检查请求
- **THEN** 返回节点状态(健康/警告/故障)
- **AND** 包含关键指标(区块高度、连接数、资源使用)

#### Scenario: 异常告警
- **WHEN** 检测到异常(高延迟、分叉、节点离线)
- **THEN** 发送告警通知(日志/Webhook/短信)
- **AND** 告警包含上下文信息
- **AND** 支持告警收敛(避免告警风暴)

### Requirement: 资源限制-CPU和内存
系统 SHALL 控制资源使用。

#### Scenario: CPU使用限制
- **WHEN** 系统正常运行
- **THEN** CPU使用率平均不超过30%
- **AND** 峰值不超过70%

#### Scenario: 内存使用限制
- **WHEN** 系统运行7天
- **THEN** 内存使用不超过500MB
- **AND** 无内存泄漏(内存增长不超过10%)

#### Scenario: 网络带宽限制
- **WHEN** 系统同步大量数据
- **THEN** 可配置带宽上限(默认10Mbps)
- **AND** 优先保证控制消息传输
