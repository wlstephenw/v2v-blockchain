## ADDED Requirements

### Requirement: REST API 基础规范
系统 SHALL 提供RESTful API接口。

#### Scenario: API基础路径
- **WHEN** 访问API服务
- **THEN** 基础路径为`/api/v1`
- **AND** 支持HTTPS(生产环境)
- **AND** HTTP端口默认可配置(默认8080)

#### Scenario: API响应格式
- **WHEN** 调用任意API
- **THEN** 响应格式为JSON
- **AND** 包含标准字段: `code`, `message`, `data`
- **AND** 错误时`code`非零，`message`包含错误描述

#### Scenario: API认证
- **WHEN** 调用需要认证的API
- **THEN** 需要在Header中携带`Authorization: Bearer <token>`
- **AND** Token有效期为24小时
- **AND** Token无效时返回401错误

#### Scenario: API速率限制
- **WHEN** 客户端频繁调用API
- **THEN** 限制每IP每分钟最多1000次请求
- **AND** 超过限制返回429错误
- **AND** 响应头包含`X-RateLimit-Remaining`

### Requirement: 区块查询API
系统 SHALL 提供区块查询接口。

#### Scenario: 查询最新区块
- **WHEN** 发送`GET /api/v1/blocks/latest`
- **THEN** 返回最新区块的完整数据
- **AND** 响应时间小于10ms

#### Scenario: 按高度查询区块
- **WHEN** 发送`GET /api/v1/blocks/{height}`
- **THEN** 返回指定高度的区块
- **AND** 区块不存在时返回404

#### Scenario: 按哈希查询区块
- **WHEN** 发送`GET /api/v1/blocks/hash/{hash}`
- **THEN** 返回指定哈希的区块
- **AND** 支持完整哈希或前缀匹配(至少8字符)

#### Scenario: 查询区块范围
- **WHEN** 发送`GET /api/v1/blocks?from={from}&to={to}&limit={limit}`
- **THEN** 返回指定范围的区块列表
- **AND** 默认limit为100，最大1000
- **AND** 按高度降序排列

#### Scenario: 查询区块头(轻客户端)
- **WHEN** 发送`GET /api/v1/headers/{height}`
- **THEN** 只返回区块头(不包含交易)
- **AND** 数据量小于完整区块的10%

### Requirement: 交易查询和提交API
系统 SHALL 提供交易相关接口。

#### Scenario: 提交交易
- **WHEN** 发送`POST /api/v1/transactions`
- **AND** 请求体包含签名后的交易数据
- **THEN** 交易被验证并加入交易池
- **AND** 返回交易哈希
- **AND** 返回202 Accepted(异步处理)

#### Scenario: 查询交易状态
- **WHEN** 发送`GET /api/v1/transactions/{hash}/status`
- **THEN** 返回交易状态(pending/confirmed/failed)
- **AND** 如已确认，返回所在区块高度和确认时间

#### Scenario: 查询交易详情
- **WHEN** 发送`GET /api/v1/transactions/{hash}`
- **THEN** 返回交易的完整信息
- **AND** 包含发送者、接收者、金额、数据、签名

#### Scenario: 查询交易池
- **WHEN** 发送`GET /api/v1/transactions/pending`
- **THEN** 返回当前待处理交易列表
- **AND** 默认返回前100条
- **AND** 按优先级和时间排序

#### Scenario: 查询账户交易历史
- **WHEN** 发送`GET /api/v1/accounts/{address}/transactions?limit={limit}&offset={offset}`
- **THEN** 返回该地址相关的所有交易
- **AND** 支持分页(limit/offset)

### Requirement: 编队管理API
系统 SHALL 提供编队管理接口。

#### Scenario: 创建编队
- **WHEN** 发送`POST /api/v1/platoons`
- **AND** 请求体包含编队参数(最大车辆数、目标速度、车距)
- **THEN** 创建新编队交易被提交
- **AND** 返回编队ID

#### Scenario: 查询编队列表
- **WHEN** 发送`GET /api/v1/platoons?status={status}&limit={limit}`
- **THEN** 返回编队列表
- **AND** 支持按状态过滤(active/dissolved)

#### Scenario: 查询编队详情
- **WHEN** 发送`GET /api/v1/platoons/{platoonId}`
- **THEN** 返回编队详细信息
- **AND** 包含成员列表、Leader信息、编队参数

#### Scenario: 加入编队申请
- **WHEN** 发送`POST /api/v1/platoons/{platoonId}/join`
- **AND** 请求体包含申请人信息和目的地
- **THEN** 加入申请被发送给Leader
- **AND** 返回申请ID用于查询状态

#### Scenario: 审批加入申请
- **WHEN** Leader发送`POST /api/v1/platoons/{platoonId}/join/{requestId}/approve`
- **THEN** 审批交易被提交
- **AND** 新成员被加入编队

#### Scenario: 离开编队
- **WHEN** 发送`POST /api/v1/platoons/{platoonId}/leave`
- **THEN** 离开交易被提交
- **AND** 当前车辆从编队中移除

#### Scenario: 解散编队
- **WHEN** Leader发送`POST /api/v1/platoons/{platoonId}/dissolve`
- **THEN** 解散交易被提交
- **AND** 编队状态变为dissolved

#### Scenario: 查询编队历史
- **WHEN** 发送`GET /api/v1/platoons/{platoonId}/history?from={time}&to={time}`
- **THEN** 返回编队历史事件列表
- **AND** 包含创建、成员变更、Leader切换等事件

### Requirement: 身份认证API
系统 SHALL 提供身份相关接口。

#### Scenario: 车辆注册
- **WHEN** 发送`POST /api/v1/identity/register`
- **AND** 请求体包含车辆证书和公钥
- **THEN** 注册交易被提交
- **AND** 返回车辆ID(区块链地址)

#### Scenario: 查询身份
- **WHEN** 发送`GET /api/v1/identity/{vehicleId}`
- **THEN** 返回车辆身份信息
- **AND** 包含公钥、证书状态、注册时间

#### Scenario: 更新证书
- **WHEN** 发送`POST /api/v1/identity/{vehicleId}/certificate`
- **AND** 请求体包含新证书
- **THEN** 证书更新交易被提交
- **AND** 旧证书在宽限期后失效

#### Scenario: 查询证书状态
- **WHEN** 发送`GET /api/v1/identity/{vehicleId}/certificate/status`
- **THEN** 返回证书状态(valid/expiring/expired/revoked)
- **AND** 如即将过期，返回剩余天数

### Requirement: 节点管理API
系统 SHALL 提供节点状态接口。

#### Scenario: 查询节点状态
- **WHEN** 发送`GET /api/v1/node/status`
- **THEN** 返回当前节点状态
- **AND** 包含: 节点ID、角色、区块高度、连接数、运行时间

#### Scenario: 查询网络信息
- **WHEN** 发送`GET /api/v1/node/network`
- **THEN** 返回网络连接信息
- **AND** 包含: 对等节点列表、连接状态、带宽使用

#### Scenario: 查询共识状态
- **WHEN** 发送`GET /api/v1/node/consensus`
- **THEN** 返回共识状态
- **AND** 包含: 当前共识算法、Validator列表、当前Leader、视图编号

#### Scenario: 节点配置查询
- **WHEN** 发送`GET /api/v1/node/config`
- **THEN** 返回当前节点配置(敏感信息脱敏)
- **AND** 包含: 网络参数、共识参数、存储路径

### Requirement: 状态和审计API
系统 SHALL 提供状态查询和审计接口。

#### Scenario: 查询当前状态
- **WHEN** 发送`GET /api/v1/state/{key}`
- **THEN** 返回指定键的当前状态值
- **AND** 支持查询编队状态、车辆状态等

#### Scenario: 查询状态历史
- **WHEN** 发送`GET /api/v1/state/{key}/history?from={time}&to={time}`
- **THEN** 返回该状态的历史变更记录
- **AND** 包含变更时间、旧值、新值、交易哈希

#### Scenario: 查询审计日志
- **WHEN** 发送`GET /api/v1/audit/logs?type={type}&from={time}&limit={limit}`
- **THEN** 返回审计日志列表
- **AND** 支持按类型过滤(identity/platoon/consensus/message)

### Requirement: WebSocket实时推送
系统 SHALL 支持WebSocket实时通知。

#### Scenario: 订阅新区块
- **WHEN** 建立WebSocket连接`ws://host/api/v1/ws/blocks`
- **THEN** 每当新区块产生时推送区块数据
- **AND** 支持只订阅区块头以减少流量

#### Scenario: 订阅交易确认
- **WHEN** 发送订阅请求`{"type":"subscribe", "channel":"transactions", "filter":{"address":"0x..."}}`
- **THEN** 当该地址相关交易被确认时推送通知

#### Scenario: 订阅编队事件
- **WHEN** 发送订阅请求`{"type":"subscribe", "channel":"platoon", "filter":{"platoonId":"..."}}`
- **THEN** 当编队状态变化时推送事件
- **AND** 包含事件类型和详情

#### Scenario: 心跳保持
- **WHEN** 连接建立后
- **THEN** 服务器每30秒发送ping
- **AND** 客户端需在10秒内回复pong
- **AND** 超时未回复则断开连接

### Requirement: API错误处理
系统 SHALL 提供清晰的错误信息。

#### Scenario: 参数错误
- **WHEN** 请求参数不合法
- **THEN** 返回400 Bad Request
- **AND** 响应包含具体错误字段和原因

#### Scenario: 资源不存在
- **WHEN** 请求的资源不存在
- **THEN** 返回404 Not Found
- **AND** 错误消息指明资源类型和ID

#### Scenario: 服务器错误
- **WHEN** 服务器内部错误
- **THEN** 返回500 Internal Server Error
- **AND** 记录详细错误日志
- **AND** 响应只包含通用错误信息(不暴露敏感信息)

#### Scenario: 服务不可用
- **WHEN** 服务暂时不可用(如共识升级中)
- **THEN** 返回503 Service Unavailable
- **AND** 响应头包含`Retry-After`建议重试时间
