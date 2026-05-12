## ADDED Requirements

### Requirement: 车辆身份注册
系统 SHALL 支持车辆使用PKI证书在区块链上注册身份。

#### Scenario: 新车辆注册
- **WHEN** 新车辆首次加入网络
- **THEN** 提交车辆证书和公钥
- **AND** 经过CA验证后写入身份合约
- **AND** 分配唯一车辆ID（以太坊地址格式）

#### Scenario: 重复注册拒绝
- **WHEN** 已注册车辆尝试重复注册
- **THEN** 系统 SHALL 拒绝重复注册请求
- **AND** 返回错误信息

### Requirement: 节点准入控制
系统 SHALL 实现基于许可的节点准入机制。

#### Scenario: 验证节点身份
- **WHEN** 节点尝试连接网络
- **THEN** 验证其证书链是否由受信CA签发
- **AND** 检查证书是否在吊销列表中
- **AND** 未通过验证的节点 SHALL 被拒绝连接

#### Scenario: 证书吊销检测
- **WHEN** 节点证书被吊销
- **THEN** 吊销信息广播到全网
- **AND** 该节点 SHALL 被立即断开连接
- **AND** 该节点 SHALL 无法参与共识

### Requirement: 证书生命周期管理
系统 SHALL 支持证书自动轮换和过期提醒。

#### Scenario: 证书自动轮换
- **WHEN** 证书有效期剩余7天
- **THEN** 系统自动向CA申请新证书
- **AND** 新旧证书平滑切换，无服务中断

#### Scenario: 证书过期处理
- **WHEN** 节点证书过期
- **THEN** 该节点 SHALL 进入只读模式
- **AND** 无法创建新交易或参与共识
- **AND** 提醒用户更新证书

### Requirement: 身份查询
系统 SHALL 支持查询车辆身份信息。

#### Scenario: 查询车辆公钥
- **WHEN** 系统需要验证某车辆签名
- **THEN** 通过车辆ID查询其注册公钥
- **AND** 返回公钥和证书状态

#### Scenario: 查询节点在线状态
- **WHEN** 编队管理器需要知道成员状态
- **THEN** 查询车辆的最后一次活跃时间
- **AND** 返回在线/离线状态

### Requirement: 匿名性与隐私
系统 SHALL 保护车辆真实身份，使用匿名地址进行通信。

#### Scenario: 地址派生
- **WHEN** 车辆注册完成
- **THEN** 从证书公钥派生区块链地址
- **AND** 地址与真实车辆信息分离存储

### Requirement: 多车身份批量注册
系统 SHALL 支持车队批量注册。

#### Scenario: 车队注册
- **WHEN** 运输公司提交车队证书列表
- **THEN** 批量验证并注册所有车辆
- **AND** 建立车队与车辆的映射关系
