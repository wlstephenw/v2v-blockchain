## ADDED Requirements

### Requirement: PBFT共识算法
系统 SHALL 实现PBFT（Practical Byzantine Fault Tolerance）共识算法。

#### Scenario: 正常共识流程
- **WHEN** Leader节点（Primary）收集到足够交易
- **THEN** Leader打包区块并广播PRE-PREPARE消息
- **AND** Validator节点验证后广播PREPARE消息
- **AND** 收到2f+1个PREPARE后广播COMMIT消息
- **AND** 收到2f+1个COMMIT后区块最终确认

#### Scenario: 消息聚合优化
- **WHEN** Validator节点准备广播PREPARE/COMMIT
- **THEN** 使用BLS签名聚合多个验证者签名
- **AND** 减少网络消息数量
- **AND** 验证时解聚合验证签名

### Requirement: Validator集合管理
系统 SHALL 动态管理Validator集合。

#### Scenario: Validator加入
- **WHEN** 新车辆加入编队并成为Validator
- **THEN** 更新Validator集合
- **AND** 广播新的Validator列表
- **AND** 从下一个区块开始使用新集合

#### Scenario: Validator退出
- **WHEN** Validator离开编队
- **THEN** 从Validator集合中移除
- **AND** 重新计算所需的共识阈值(2f+1)
- **AND** 如果剩余Validator不足4个，编队无法继续共识，触发解散流程或降级警告

### Requirement: 共识超时处理
系统 SHALL 实现超时机制处理网络延迟。

#### Scenario: PRE-PREPARE超时
- **WHEN** Validator在5秒内未收到Leader的PRE-PREPARE
- **THEN** 触发视图变更(View Change)流程
- **AND** 广播VIEW-CHANGE消息

#### Scenario: PREPARE超时
- **WHEN** Validator在3秒内未收到足够PREPARE
- **THEN** 请求其他节点重发PREPARE
- **AND** 重试2次后仍未收到则触发视图变更

### Requirement: 视图变更(View Change)
系统 SHALL 支持视图变更以处理Leader故障。

#### Scenario: 发起视图变更
- **WHEN** 超过f+1个Validator怀疑Leader故障
- **THEN** 新视图编号递增
- **AND** 广播VIEW-CHANGE消息包含已确认区块证明

#### Scenario: 视图变更完成
- **WHEN** 新Leader收集到2f+1个VIEW-CHANGE
- **THEN** 广播NEW-VIEW消息
- **AND** 包含新的区块提案
- **AND** 继续正常共识流程

#### Scenario: 新Leader选举
- **WHEN** 视图变更完成
- **THEN** 按确定顺序选择新Leader（如按节点ID轮询）
- **AND** 新Leader开始广播PRE-PREPARE

### Requirement: 交易优先级
系统 SHALL 支持交易优先级设置。

#### Scenario: 高优先级交易
- **WHEN** 编队安全相关交易（如紧急制动）被提交
- **THEN** 标记为高优先级
- **AND** 优先打包进下一个区块
- **AND** 验证者优先处理

#### Scenario: 普通交易
- **WHEN** 状态同步类交易被提交
- **THEN** 按到达顺序处理
- **AND** 高优先级交易未处理完时延迟打包

### Requirement: 快速通道机制
系统 SHALL 支持关键消息的快速通道。

#### Scenario: 紧急消息快速确认
- **WHEN** 紧急消息（如事故警告）发送
- **THEN** 先执行后共识（Execute-First）
- **AND** 异步等待链上确认
- **AND** 收到链上确认后验证执行正确性

### Requirement: 拜占庭容错
系统 SHALL 容忍不超过1/3的拜占庭节点。

#### Scenario: 恶意节点检测
- **WHEN** 节点发送冲突消息
- **THEN** 记录节点行为
- **AND** 超过阈值后标记为可疑节点
- **AND** 向全网广播警告

#### Scenario: 双花攻击防护
- **WHEN** 恶意节点尝试双花
- **THEN** 共识机制 SHALL 只确认一个交易
- **AND** 另一个交易被拒绝

### Requirement: 编队规模约束
系统 SHALL 约束编队最小规模以支持PBFT。

#### Scenario: 创建编队规模检查
- **WHEN** 车辆尝试创建编队
- **THEN** 检查是否至少有4辆车（包括创建者）
- **AND** 不足4辆时提示"需要至少4辆车才能形成安全编队"

#### Scenario: 成员不足警告
- **WHEN** 编队成员减少到3辆或以下
- **THEN** 触发警告"编队安全性降低"
- **AND** 建议寻找新的成员加入或解散编队
- **AND** 链上记录成员不足事件

#### Scenario: 少于4辆车拒绝服务
- **WHEN** 编队成员减少到3辆或以下
- **THEN** 系统停止共识服务
- **AND** 进入"待解散"状态
- **AND** 广播警告信息"成员不足，编队将在10分钟后解散"
- **AND** 建议成员加入其他编队或等待新成员
