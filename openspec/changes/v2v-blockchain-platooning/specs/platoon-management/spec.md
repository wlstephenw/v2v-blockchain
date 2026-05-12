## ADDED Requirements

### Requirement: 编队创建
系统 SHALL 支持车辆创建新的编队。

#### Scenario: Leader创建编队
- **WHEN** 车辆发起创建编队请求
- **THEN** 验证该车辆有资格成为Leader
- **AND** 在链上创建编队智能合约
- **AND** 生成唯一编队ID
- **AND** 设置编队参数（最大车辆数、目标速度、车距等）

#### Scenario: 创建参数验证
- **WHEN** 编队参数不合理（如车距小于安全距离）
- **THEN** 系统 SHALL 拒绝创建请求
- **AND** 返回参数错误信息

#### Scenario: 创建编队规模检查
- **WHEN** 车辆尝试创建编队
- **THEN** 检查是否至少有4辆车（包括创建者及已邀请车辆）
- **AND** 不足4辆时提示"需要至少4辆车才能形成安全编队"
- **AND** 建议等待更多车辆加入或加入现有编队

### Requirement: 加入编队
系统 SHALL 支持车辆申请加入现有编队。

#### Scenario: Follower申请加入
- **WHEN** 车辆发送加入编队请求
- **THEN** Leader节点收到申请
- **AND** Leader验证申请车辆身份
- **AND** 批准后更新编队成员列表到链上
- **AND** 新成员收到编队配置信息

#### Scenario: 编队已满拒绝
- **WHEN** 编队已达到最大车辆数
- **THEN** 新加入请求 SHALL 被拒绝
- **AND** 返回"编队已满"错误

#### Scenario: 黑名单车辆加入
- **WHEN** 被该编队拉黑过的车辆申请加入
- **THEN** 系统 SHALL 自动拒绝
- **AND** 通知Leader该尝试

### Requirement: 离开编队
系统 SHALL 支持车辆主动离开编队。

#### Scenario: Follower主动离开
- **WHEN** Follower发送离开编队请求
- **THEN** 记录离开交易到链上
- **AND** 更新编队成员列表
- **AND** 触发编队重组（如有必要）

#### Scenario: Leader主动离开
- **WHEN** Leader发送离开请求
- **THEN** 触发Leader切换流程
- **AND** 新Leader接管后原Leader才能离开

### Requirement: 编队解散
系统 SHALL 支持编队解散操作。

#### Scenario: Leader解散编队
- **WHEN** Leader发起解散编队
- **THEN** 广播解散消息到所有成员
- **AND** 在链上标记编队状态为"已解散"
- **AND** 释放所有成员与编队的关联

#### Scenario: 成员不足解散
- **WHEN** 编队成员减少到3辆或以下
- **AND** 超过10分钟未找到新成员
- **THEN** 系统自动解散编队
- **AND** 记录解散原因"成员不足"到链上

#### Scenario: 超时自动解散
- **WHEN** 编队所有成员离线超过5分钟
- **THEN** 系统自动解散编队
- **AND** 记录解散原因到链上

### Requirement: Leader选举与切换
系统 SHALL 实现Leader故障时的自动切换。

#### Scenario: Leader故障检测
- **WHEN** Leader连续3个出块周期未出块（约6-15秒）
- **THEN** Validator节点检测到Leader故障
- **AND** 广播怀疑Leader故障的消息

#### Scenario: PBFT视图变更选举Leader
- **WHEN** 超过f+1个Validator确认Leader故障
- **THEN** 触发PBFT View Change流程
- **AND** 按预定顺序选择新Leader（如按节点ID轮询）
- **AND** 链上记录Leader变更交易
- **AND** 新Leader开始出块

#### Scenario: Leader降级为Follower
- **WHEN** 原Leader恢复连接
- **THEN** 识别新Leader并降级为Follower
- **AND** 同步缺失区块

### Requirement: 编队状态查询
系统 SHALL 支持查询编队实时状态。

#### Scenario: 查询编队成员
- **WHEN** 系统查询某编队
- **THEN** 返回所有成员车辆ID和角色
- **AND** 返回每个成员的加入时间

#### Scenario: 查询编队历史
- **WHEN** 查询编队历史记录
- **THEN** 返回编队创建、成员变更、Leader切换历史
- **AND** 所有记录可追溯且不可篡改

### Requirement: Validator管理
系统 SHALL 管理Validator集合（PBFT共识节点）。

#### Scenario: 任命Validator
- **WHEN** 编队需要增加Validator（如从4车增加到5车）
- **THEN** Leader提议新Validator加入共识集合
- **AND** 现有Validator投票批准（PBFT配置变更）
- **AND** 新Validator开始参与PBFT共识

#### Scenario: Validator降级为Follower
- **WHEN** 编队规模缩减需要减少Validator数量
- **THEN** 指定某Validator降级为Follower
- **AND** 更新PBFT共识集合
- **AND** 被降级节点转为轻客户端模式

#### Scenario: 创建时Validator随机分配
- **WHEN** 编队创建时有4-6辆车
- **THEN** 所有车都成为Validator
- **AND** 随机选择1辆车作为Leader
- **AND** 其余车辆作为Validators

#### Scenario: 创建时Follower随机分配
- **WHEN** 编队创建时有7辆以上车
- **THEN** 随机选择7辆车作为Validators（1 Leader + 6 Validators）
- **AND** 其余车辆成为轻客户端Follower

#### Scenario: Validator故障补充
- **WHEN** 某个Validator故障离线超过1分钟
- **THEN** 从Followers中随机选择1辆补充为Validator
- **AND** 更新PBFT共识集合
- **AND** 通知所有节点新的Validator列表

#### Scenario: Validator恢复重新分配
- **WHEN** 故障Validator恢复连接
- **THEN** 根据当前Validator数量决定是否恢复其Validator身份
- **AND** 如果Validator已满7个，恢复的节点作为Follower
