# V2V Blockchain API 文档

## Task 13.5: API 接口文档 (Swagger/OpenAPI)

本文档描述 V2V Blockchain 提供的 RESTful API 接口。

## 基础信息

- **Base URL**: `http://localhost:8080/api/v1`
- **Content-Type**: `application/json`
- **认证方式**: 暂无 (后续支持 JWT)

## 通用响应格式

### 成功响应
```json
{
  "code": 200,
  "message": "success",
  "data": { ... }
}
```

### 错误响应
```json
{
  "code": 400,
  "message": "error description",
  "error": "DETAILED_ERROR_CODE"
}
```

## API 端点

### 健康检查

#### GET /health
检查节点健康状态。

**响应示例**:
```json
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:30:00Z"
}
```

#### GET /ready
检查节点是否就绪。

**响应示例**:
```json
{
  "ready": true,
  "synced": true
}
```

---

### 节点管理

#### GET /node/status
获取节点当前状态。

**响应示例**:
```json
{
  "node_id": "node-001",
  "version": "1.0.0",
  "uptime": 3600,
  "role": "validator",
  "network": {
    "peer_count": 8,
    "listening_addresses": ["/ip4/0.0.0.0/tcp/10000"]
  },
  "blockchain": {
    "latest_height": 1000,
    "latest_hash": "0xabc123...",
    "sync_status": "synced"
  },
  "consensus": {
    "view_number": 5,
    "is_primary": true
  }
}
```

---

### 区块链查询

#### GET /blockchain/blocks/latest
获取最新区块。

**响应示例**:
```json
{
  "code": 200,
  "data": {
    "header": {
      "height": 1000,
      "hash": "0xabc123...",
      "prev_hash": "0xdef456...",
      "timestamp": 1705315800,
      "merkle_root": "0x789abc...",
      "validator": "0xvalidator001..."
    },
    "transactions": [
      {
        "hash": "0xtx123...",
        "from": "0xvehicle001...",
        "to": "0xvehicle002...",
        "value": 100,
        "nonce": 5,
        "type": "transfer"
      }
    ],
    "tx_count": 10
  }
}
```

#### GET /blockchain/blocks/{height}
获取指定高度区块。

**路径参数**:
- `height` (integer, required): 区块高度

**响应示例**: 同上

#### GET /blockchain/blocks/range
获取区块范围。

**查询参数**:
- `start` (integer, required): 起始高度
- `end` (integer, required): 结束高度
- `limit` (integer, optional): 最大返回数量，默认 10

**响应示例**:
```json
{
  "code": 200,
  "data": {
    "blocks": [ ... ],
    "total": 100,
    "has_more": true
  }
}
```

#### GET /blockchain/transactions/{hash}
查询交易详情。

**路径参数**:
- `hash` (string, required): 交易哈希

**响应示例**:
```json
{
  "code": 200,
  "data": {
    "hash": "0xtx123...",
    "from": "0xvehicle001...",
    "to": "0xvehicle002...",
    "value": 100,
    "nonce": 5,
    "type": "transfer",
    "timestamp": 1705315800,
    "block_height": 1000,
    "block_hash": "0xabc123...",
    "signature": "0xsig...",
    "status": "confirmed"
  }
}
```

#### GET /blockchain/transactions/pending
获取待处理交易。

**响应示例**:
```json
{
  "code": 200,
  "data": {
    "transactions": [ ... ],
    "count": 5,
    "total_size": 1024
  }
}
```

---

### 交易提交

#### POST /transactions
提交新交易。

**请求体**:
```json
{
  "from": "0xvehicle001...",
  "to": "0xvehicle002...",
  "value": 100,
  "nonce": 5,
  "type": "transfer",
  "data": "optional_payload",
  "signature": "0xsig..."
}
```

**响应示例**:
```json
{
  "code": 200,
  "data": {
    "tx_hash": "0xtx123...",
    "status": "pending"
  }
}
```

---

### 编队管理

#### GET /platoons
获取所有编队列表。

**查询参数**:
- `status` (string, optional): 过滤状态 (forming/active/dissolved)
- `limit` (integer, optional): 返回数量限制
- `offset` (integer, optional): 分页偏移

**响应示例**:
```json
{
  "code": 200,
  "data": {
    "platoons": [
      {
        "id": "platoon-001",
        "name": "Highway Convoy A",
        "status": "active",
        "leader_id": "0xvehicle001...",
        "member_count": 5,
        "max_size": 8,
        "target_speed": 30.0,
        "created_at": 1705315800,
        "updated_at": 1705316400
      }
    ],
    "total": 10,
    "has_more": false
  }
}
```

#### GET /platoons/{id}
获取编队详情。

**路径参数**:
- `id` (string, required): 编队 ID

**响应示例**:
```json
{
  "code": 200,
  "data": {
    "id": "platoon-001",
    "name": "Highway Convoy A",
    "status": "active",
    "params": {
      "max_vehicles": 8,
      "target_speed": 30.0,
      "safe_distance": 20.0,
      "lane_id": 1,
      "route_id": "route-001"
    },
    "leader_id": "0xvehicle001...",
    "members": [
      {
        "vehicle_id": "0xvehicle001...",
        "role": "leader",
        "position": 0,
        "joined_at": 1705315800,
        "status": "active"
      },
      {
        "vehicle_id": "0xvehicle002...",
        "role": "follower",
        "position": 1,
        "joined_at": 1705315900,
        "status": "active"
      }
    ],
    "validators": ["0xvehicle001...", "0xvehicle003..."],
    "created_at": 1705315800,
    "updated_at": 1705316400,
    "block_height": 500
  }
}
```

#### POST /platoons
创建新编队。

**请求体**:
```json
{
  "name": "New Convoy",
  "leader_id": "0xvehicle001...",
  "params": {
    "max_vehicles": 8,
    "target_speed": 30.0,
    "safe_distance": 20.0,
    "lane_id": 1,
    "route_id": "route-001"
  }
}
```

**响应示例**:
```json
{
  "code": 200,
  "data": {
    "platoon_id": "platoon-002",
    "status": "forming",
    "created_at": 1705317000
  }
}
```

#### POST /platoons/{id}/join
申请加入编队。

**路径参数**:
- `id` (string, required): 编队 ID

**请求体**:
```json
{
  "vehicle_id": "0xvehicle005...",
  "destination": "Destination A"
}
```

#### POST /platoons/{id}/leave
离开编队。

**路径参数**:
- `id` (string, required): 编队 ID

**请求体**:
```json
{
  "vehicle_id": "0xvehicle005...",
  "reason": "Reached destination"
}
```

#### POST /platoons/{id}/dissolve
解散编队 (仅 Leader)。

**路径参数**:
- `id` (string, required): 编队 ID

**请求体**:
```json
{
  "reason": "End of journey"
}
```

---

### 身份认证

#### GET /identity/vehicles/{id}
查询车辆身份信息。

**路径参数**:
- `id` (string, required): 车辆 ID

**响应示例**:
```json
{
  "code": 200,
  "data": {
    "vehicle_id": "0xvehicle001...",
    "public_key": "0xpubkey...",
    "certificate": "0xcert...",
    "issued_at": 1705315800,
    "expires_at": 1707907800,
    "status": "active",
    "platoon_id": "platoon-001"
  }
}
```

#### POST /identity/vehicles
注册新车辆。

**请求体**:
```json
{
  "vehicle_id": "0xvehicle010...",
  "public_key": "0xpubkey...",
  "certificate": "0xcert...",
  "metadata": {
    "model": "Tesla Model 3",
    "year": 2024
  }
}
```

---

### 网络信息

#### GET /network/peers
获取连接的节点列表。

**响应示例**:
```json
{
  "code": 200,
  "data": {
    "peers": [
      {
        "id": "12D3KooW...",
        "addresses": ["/ip4/192.168.1.10/tcp/10000"],
        "latency_ms": 15,
        "direction": "outbound",
        "protocols": ["/v2v/1.0.0"]
      }
    ],
    "count": 8,
    "inbound": 3,
    "outbound": 5
  }
}
```

#### GET /network/stats
获取网络统计信息。

**响应示例**:
```json
{
  "code": 200,
  "data": {
    "total_bytes_in": 1024000,
    "total_bytes_out": 512000,
    "messages_in": 1000,
    "messages_out": 500,
    "avg_latency_ms": 20
  }
}
```

---

### 状态查询

#### GET /state/current
获取当前系统状态。

**响应示例**:```json
{
  "code": 200,
  "data": {
    "timestamp": 1705317000,
    "blockchain": {
      "height": 1000,
      "hash": "0xabc123...",
      "total_transactions": 5000
    },
    "network": {
      "peers": 8,
      "bandwidth_in": "1.5 MB/s",
      "bandwidth_out": "0.8 MB/s"
    },
    "platoons": {
      "total": 10,
      "active": 8,
      "total_vehicles": 45
    }
  }
}
```

#### GET /state/history
获取历史状态变更。

**查询参数**:
- `start_time` (integer, required): 起始时间戳
- `end_time` (integer, required): 结束时间戳
- `type` (string, optional): 事件类型过滤

**响应示例**:
```json
{
  "code": 200,
  "data": {
    "events": [
      {
        "type": "platoon_created",
        "timestamp": 1705315800,
        "data": {
          "platoon_id": "platoon-001",
          "leader_id": "0xvehicle001..."
        }
      }
    ],
    "total": 100
  }
}
```

---

## WebSocket API

### 连接
```
ws://localhost:8080/ws
```

### 订阅事件

发送订阅消息:
```json
{
  "action": "subscribe",
  "topics": ["blocks", "transactions", "platoons"]
}
```

### 事件格式

#### 新区块事件
```json
{
  "topic": "blocks",
  "data": {
    "height": 1001,
    "hash": "0xabc123...",
    "tx_count": 10,
    "timestamp": 1705317000
  }
}
```

#### 新交易事件
```json
{
  "topic": "transactions",
  "data": {
    "hash": "0xtx123...",
    "from": "0xvehicle001...",
    "to": "0xvehicle002...",
    "type": "transfer",
    "timestamp": 1705317000
  }
}
```

#### 编队事件
```json
{
  "topic": "platoons",
  "data": {
    "event_type": "member_joined",
    "platoon_id": "platoon-001",
    "vehicle_id": "0xvehicle005...",
    "timestamp": 1705317000
  }
}
```

---

## 错误码

| 错误码 | HTTP 状态 | 说明 |
|--------|-----------|------|
| 400 | Bad Request | 请求参数错误 |
| 401 | Unauthorized | 未授权 |
| 404 | Not Found | 资源不存在 |
| 409 | Conflict | 资源冲突 |
| 429 | Too Many Requests | 请求过于频繁 |
| 500 | Internal Server Error | 服务器内部错误 |
| 503 | Service Unavailable | 服务不可用 |

## 限流策略

- 默认限制: 100 请求/分钟/IP
- 超出限制返回 429 状态码

## 版本历史

| 版本 | 日期 | 变更 |
|------|------|------|
| 1.0.0 | 2024-01-15 | 初始版本 |
