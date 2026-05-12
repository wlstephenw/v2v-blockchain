## ADDED Requirements

### Requirement: 区块结构轻量化
系统 SHALL 实现轻量级区块结构，单个区块大小不超过10KB。

#### Scenario: 创建新区块
- **WHEN** 出块节点打包交易
- **THEN** 生成的区块Header不超过300字节
- **AND** 区块最多包含100笔交易

#### Scenario: 验证区块大小
- **WHEN** 节点收到新区块
- **THEN** 系统 SHALL 验证区块大小
- **AND** 超过10KB的区块 SHALL 被拒绝

### Requirement: 轻客户端支持
系统 SHALL 支持轻客户端模式，轻节点只存储区块头。

#### Scenario: 轻节点启动
- **WHEN** 轻节点加入网络
- **THEN** 只同步区块头数据
- **AND** 存储空间减少90%以上

#### Scenario: 验证交易存在性
- **WHEN** 轻节点需要验证交易
- **THEN** 通过Merkle Proof向全节点请求验证
- **AND** 无需下载完整区块数据

### Requirement: P2P网络连接管理
系统 SHALL 使用Libp2p实现P2P网络，支持NAT穿透和动态节点发现。

#### Scenario: 节点加入网络
- **WHEN** 新车辆启动区块链节点
- **THEN** 自动发现并连接网络中的其他节点
- **AND** 连接数维持在8-12个对等节点

#### Scenario: 网络分区恢复
- **WHEN** 车辆因信号盲区暂时断开
- **THEN** 恢复连接后自动同步缺失区块
- **AND** 检测并解决分叉

### Requirement: 链存储管理
系统 SHALL 使用LevelDB存储区块链数据，支持数据归档。

#### Scenario: 写入区块数据
- **WHEN** 新区块被确认
- **THEN** 数据持久化到LevelDB
- **AND** 写入延迟小于10ms

#### Scenario: 本地数据归档
- **WHEN** 本地区块数超过10000个
- **THEN** 自动归档最旧的5000个区块到云端
- **AND** 本地只保留最近区块

### Requirement: 区块头同步
系统 SHALL 实现快速区块头同步机制。

#### Scenario: 快速同步
- **WHEN** 节点启动且区块高度落后
- **THEN** 优先批量同步区块头
- **AND** 同步速度不低于100 headers/秒

### Requirement: 分叉处理
系统 SHALL 自动检测并解决区块链分叉。

#### Scenario: 检测到分叉
- **WHEN** 节点收到与本地链冲突的区块
- **THEN** 比较链的累计难度（或高度）
- **AND** 切换到最长有效链
- **AND** 回滚本地状态到分叉点

### Requirement: 区块生成间隔
系统 SHALL 保持区块生成间隔在1-5秒之间。

#### Scenario: 正常出块
- **WHEN** 网络负载正常
- **THEN** 平均出块间隔为2秒
- **AND** 单个出块间隔不超过5秒

#### Scenario: 高负载调整
- **WHEN** 交易数量突增
- **THEN** 动态调整出块间隔
- **AND** 优先打包高优先级交易

### Requirement: 节点启动与恢复
系统 SHALL 支持节点正常启动和故障恢复。

#### Scenario: 创世区块初始化
- **WHEN** 网络首次启动（无历史数据）
- **THEN** 创建创世区块（高度0）
- **AND** 创世区块包含初始Validator列表
- **AND** 所有节点使用相同的创世配置

#### Scenario: 节点冷启动（有本地数据）
- **WHEN** 节点启动且本地有区块数据
- **THEN** 加载本地链状态
- **AND** 从最后已知区块继续同步
- **AND** 在30秒内完成启动

#### Scenario: 节点崩溃恢复
- **WHEN** 节点异常崩溃后重启
- **THEN** 检测到最后一次状态
- **AND** 回滚可能未完成的区块
- **AND** 恢复交易池状态
- **AND** 重新建立网络连接

#### Scenario: 节点数据损坏恢复
- **WHEN** 节点检测到本地数据损坏
- **THEN** 尝试从备份恢复
- **AND** 如果无备份，清空数据重新同步
- **AND** 记录数据损坏事件

#### Scenario: 网络分区后重新加入
- **WHEN** 节点因网络分区离线后恢复
- **THEN** 检测本地链与网络链的差异
- **AND** 如果本地链落后，批量同步缺失区块
- **AND** 如果本地链分叉，切换到最长链
- **AND** 恢复参与共识
