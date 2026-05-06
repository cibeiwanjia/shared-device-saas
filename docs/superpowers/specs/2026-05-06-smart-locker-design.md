# 智能快递柜模块设计文档

> **日期**: 2026-05-06
> **状态**: Draft
> **作者**: Sisyphus + 用户协作设计
> **范围**: `app/storage/` 服务完整实现 + `api/storage/` Proto 定义 + `api/device/` 扩展

---

## 1. 架构总览

### 1.1 系统架构图

```
┌─────────────────────────────────────────────────────────────────┐
│                       Client (App / 柜机屏幕)                    │
└────────┬────────────────────────────────────────────┬───────────┘
         │ HTTP                                       │ MQTT
         ▼                                            ▼
┌─────────────────────┐                    ┌──────────────────────┐
│   Storage Service   │                    │    Device Service    │
│                     │                    │                      │
│ biz/                │                    │ biz/                 │
│  delivery_in.go     │   gRPC             │  monitor.go          │
│  delivery_out.go ◄──┼──► OpenCell ──────►│  inventory.go        │
│  storage_uc.go      │   gRPC             │  device.go           │
│                     │◄── ReportDoorEvent │                      │
│  order_fsm.go       │                    │ MQTT Client          │
│  cell_allocator.go  │                    │  ↓ publish/subscribe │
│  pricing_engine.go  │                    │  ↓ topic router      │
│  timeout_manager.go │                    │                      │
│  timeout_handler.go │                    │ pkg/mqtt/            │
│  pickup_code.go     │                    │                      │
│  device_commander.go│                    └──────────┬───────────┘
│  event_publisher.go │                               │
│                     │                               │
│ data/               │                    ┌──────────▼───────────┐
│  MySQL (cabinets,   │                    │   MQTT Broker        │
│   cells, orders,    │                    │   (EMQX/Mosquitto)   │
│   pricing_rules)    │                    └──────────┬───────────┘
│  Redis (pickup      │                               │
│   codes, 分布式锁)  │                               │
└─────────────────────┘                               │
         │                                            │
         │ Async (RabbitMQ)                           │
         ▼                                            │
┌─────────────────────┐                               │
│ Notification Worker │                    ┌──────────▼───────────┐
│ (App WS / SMS /     │                    │   柜机硬件            │
│  微信模板消息)       │                    │   (嵌入式控制器)      │
└─────────────────────┘                    └──────────────────────┘
```

### 1.2 两层真相源

| 真相源 | 服务 | 存储 | 驱动力 | 职责 |
|--------|------|------|--------|------|
| 业务真相 | Storage Service | MySQL cells/orders 表 | 订单状态流转 | 格口分配/释放、计费、取件码、超时检测 |
| 物理真相 | Device Service | Redis `device:slots:{tenant}:{device}` | 硬件 MQTT 上报 | slot 状态缓存、开门指令下发、在线检测 |

**服务间通信**：
- Storage → Device：gRPC（OpenCell、GetDeviceSlotStatus、ForceReleaseCell）
- Device → Storage：gRPC（ReportDoorEvent，Device 收到 MQTT 柜门事件后回调）
- Storage → Notification：RabbitMQ 异步消息

### 1.3 依赖方向

```
delivery_in / delivery_out / storage_uc
    ↓ 单向依赖
timeout_handler → order_fsm / cell_allocator / pricing_engine / timeout_manager / pickup_code / device_commander
    ↓ 依赖
data 层（Repo 接口实现）
```

三个业务 Usecase 依赖共享引擎，共享引擎之间无交叉依赖。Wire 注入自然有序。

---

## 2. 数据模型

### 2.1 cabinets 表（柜机实例）

柜机是 device 在 storage 服务的业务投影。device 表存物理属性（SN、电量、在线状态），cabinet 表存运营属性（名称、位置、格口总数）。

```sql
CREATE TABLE cabinets (
    id              BIGINT PRIMARY KEY AUTO_INCREMENT,
    tenant_id       BIGINT NOT NULL,
    device_id       BIGINT NOT NULL,
    device_sn       VARCHAR(32) NOT NULL,
    name            VARCHAR(64) NOT NULL,
    location_name   VARCHAR(128) NOT NULL,
    total_cells     INT NOT NULL,
    status          TINYINT NOT NULL DEFAULT 1,  -- 1=正常 2=维护中 3=停用
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_tenant_status (tenant_id, status),
    UNIQUE INDEX uk_device_id (device_id)
) ENGINE=InnoDB;
```

### 2.2 cells 表（格口）

```sql
CREATE TABLE cells (
    id              BIGINT PRIMARY KEY AUTO_INCREMENT,
    tenant_id       BIGINT NOT NULL,
    cabinet_id      BIGINT NOT NULL,
    slot_index      INT NOT NULL,                   -- 对应 device 服务的 slot 编号
    cell_type       TINYINT NOT NULL DEFAULT 1,     -- 1=小 2=中 3=大
    status          TINYINT NOT NULL DEFAULT 1,     -- 1=空闲 2=占用 3=开门中 4=故障 5=停用
    current_order_id BIGINT DEFAULT NULL,
    pending_action  TINYINT DEFAULT NULL,           -- 1=等投递确认 2=等取件确认 3=临时开柜 4=等寄存确认
    opened_at       DATETIME DEFAULT NULL,          -- 开门时间，用于超时回退
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE INDEX uk_cabinet_slot (cabinet_id, slot_index),
    INDEX idx_tenant_status (tenant_id, status),
    INDEX idx_cabinet_status (cabinet_id, status),
    INDEX idx_open_timeout (status, opened_at)
) ENGINE=InnoDB;
```

**关键字段说明**：
- `slot_index`：与 device 服务 Redis slot 的映射键
- `status=3(开门中)`：中间锁定状态，防止开门期间重复分配
- `pending_action`：开门时写入，关门回调时读取并路由到对应 Usecase 方法
- `opened_at`：door_opened 事件到达时记录，5 分钟未收到 door_closed 则触发超时回退

**分配查询**：`SELECT ... WHERE cabinet_id=? AND status=1 AND cell_type<=? FOR UPDATE`（悲观锁）

### 2.3 storage_orders 表

```sql
CREATE TABLE storage_orders (
    id              BIGINT PRIMARY KEY AUTO_INCREMENT,
    tenant_id       BIGINT NOT NULL,
    order_no        VARCHAR(32) NOT NULL,
    order_type      VARCHAR(16) NOT NULL,           -- delivery_in / delivery_out / storage
    status          INT NOT NULL DEFAULT 10,
    user_id         BIGINT NOT NULL,                 -- 主角色
    operator_id     BIGINT DEFAULT NULL,             -- 副角色（快递员/揽收员）
    cabinet_id      BIGINT NOT NULL,
    cell_id         BIGINT NOT NULL,
    device_sn       VARCHAR(32) NOT NULL,
    slot_index      INT NOT NULL,
    pickup_code     VARCHAR(6) DEFAULT NULL,
    deposited_at    DATETIME DEFAULT NULL,
    picked_up_at    DATETIME DEFAULT NULL,
    total_amount    INT NOT NULL DEFAULT 0,          -- 总费用（分）
    paid_amount     INT NOT NULL DEFAULT 0,
    overtime_minutes INT NOT NULL DEFAULT 0,
    remark          VARCHAR(256) DEFAULT NULL,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE INDEX uk_order_no (order_no),
    INDEX idx_tenant_user (tenant_id, user_id),
    INDEX idx_tenant_cabinet (tenant_id, cabinet_id, status),
    INDEX idx_status_timeout (status, deposited_at),
    INDEX idx_tenant_pickup (tenant_id, pickup_code)
) ENGINE=InnoDB;
```

**operator_id 使用规则**：

| order_type | user_id | operator_id |
|---|---|---|
| delivery_in | 快递员 ID | NULL（取件靠 pickup_code） |
| delivery_out | 寄件用户 ID | 揽收快递员 ID |
| storage | 寄存用户 ID | NULL |

### 2.4 pricing_rules 表

```sql
CREATE TABLE pricing_rules (
    id              BIGINT PRIMARY KEY AUTO_INCREMENT,
    tenant_id       BIGINT NOT NULL,
    rule_type       VARCHAR(32) NOT NULL,
    free_hours      INT NOT NULL DEFAULT 24,
    price_per_hour  INT NOT NULL DEFAULT 0,
    price_per_day   INT NOT NULL DEFAULT 0,
    max_fee         INT NOT NULL DEFAULT 0,
    cell_type       TINYINT DEFAULT NULL,            -- NULL=全部类型
    priority        INT NOT NULL DEFAULT 0,
    effective_from  DATETIME DEFAULT NULL,
    effective_until DATETIME DEFAULT NULL,
    enabled         TINYINT NOT NULL DEFAULT 1,
    created_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    INDEX idx_tenant_type (tenant_id, rule_type, enabled),
    INDEX idx_effective (tenant_id, rule_type, effective_from, effective_until)
) ENGINE=InnoDB;
```

### 2.5 FSM 状态定义

**状态码映射**：

| 状态码 | 常量 | 含义 |
|---|---|---|
| 10 | Pending | 待投递/待存入 |
| 11 | Deposited | 已投递/待取件 |
| 12 | Storing | 存放中（仅 storage） |
| 13 | Completed | 已完成（终态） |
| 15 | Timeout | 超时未取 |
| 16 | Cleared | 运维清理（终态） |

**按 order_type 的合法路径**：

```
delivery_in:
  10(Pending) → 11(Deposited) → 13(Completed)
                    ↓ 超时
                15(Timeout) → 13(Completed, 补缴后)
                            → 16(Cleared, 运维清理)

delivery_out:
  10(Pending) → 11(Deposited) → 13(Completed, 快递员揽收)

storage:
  10(Pending) → 12(Storing) → 13(Completed)
                    ↓ 超时
                15(Timeout) → 13(Completed, 补缴后)
                            → 16(Cleared)
  12(Storing) → 12(Storing)   // 临时开柜（有每日次数上限）
```

**FSM 转移表（代码）**：

```go
var storageTransitions = map[string]map[int32][]int32{
    "delivery_in": {
        10: {11},
        11: {13, 15},
        15: {13, 16},
    },
    "delivery_out": {
        10: {11},
        11: {13},
    },
    "storage": {
        10: {12},
        12: {13, 15, 12},
        15: {13, 16},
    },
}
```

---

## 3. 代码组织

### 3.1 biz/ 层文件清单（12 个文件）

```
biz/
├── biz.go                 ← Wire ProviderSet
│
│   // ── 共享引擎（纯逻辑/基础设施，引擎间无交叉依赖）──
├── order_fsm.go           ← 纯函数：CanTransition + Transition（只改内存 struct，无 repo）
├── cell_allocator.go      ← CellRepo → AllocateCell(悲观锁) / ReleaseCell / MarkOpening / ConfirmOccupied
├── pricing_engine.go      ← PricingRepo → MatchRule / CalculateFee
├── timeout_manager.go     ← Go timer + DB 轮询 + open timeout 直接处理（依赖 cellRepo/deviceCommander/cellAllocator/eventPublisher）
├── timeout_handler.go     ← 统一路由 order timeout，按 order_type 分发到对应 Usecase
├── pickup_code.go         ← Redis only → Generate / Verify(返回 orderID)
├── device_commander.go    ← gRPC client → OpenCell(10s 应用层超时) / GetSlotStatus / ForceRelease
├── event_publisher.go     ← interface：PublishPickupReady / PublishOrderTimeout / PublishOvertimeFee / PublishOpenTimeout
│
│   // ── 业务 Usecase（依赖共享引擎）──
├── delivery_in.go         ← InitiateDelivery(sync) + HandleDepositClosed(async) + HandlePickupClosed(async) + OnOrderTimeout
├── delivery_out.go        ← InitiateShipment(sync) + HandleDepositClosed(async) + OnOrderTimeout
└── storage_uc.go          ← InitiateStorage(sync) + HandleDepositClosed(async) + HandleTempOpenClosed(async) + OnOrderTimeout
```

### 3.2 关键接口

```go
// biz/order_fsm.go — 纯逻辑，无 repo
type OrderFSM struct { transitions map[string]map[int32][]int32 }
func (f *OrderFSM) CanTransition(orderType string, from, to int32) bool
func (f *OrderFSM) Transition(orderType string, order *StorageOrder, to int32) error

// biz/cell_allocator.go
type CellAllocator struct { cellRepo CellRepo }
func (a *CellAllocator) AllocateCell(ctx, cabinetID, cellType) (*Cell, error)
func (a *CellAllocator) ReleaseCell(ctx, cellID) error
func (a *CellAllocator) MarkOpening(ctx, cellID, pendingAction int32, orderID int64) error
func (a *CellAllocator) ConfirmOccupied(ctx, cellID) error

// biz/pricing_engine.go
type PricingEngine struct { pricingRepo PricingRepo }
func (e *PricingEngine) MatchRule(ctx, tenantID, ruleType, cellType) (*PricingRule, error)
func (e *PricingEngine) CalculateFee(ctx, rule, overtimeMinutes) (int32, error)

// biz/timeout_manager.go
type TimeoutManager struct {
    orderRepo       OrderRepo
    cellRepo        CellRepo
    deviceCommander *DeviceCommander
    cellAllocator   *CellAllocator
    eventPublisher  EventPublisher
    orderTimeoutCb  func(ctx context.Context, orderID int64) error  // → TimeoutHandler
}
func (m *TimeoutManager) RegisterOrderTimeout(orderID int64, duration time.Duration)
func (m *TimeoutManager) RegisterOpenTimeout(cellID int64, duration time.Duration)
func (m *TimeoutManager) HandleOpenTimeout(ctx, cellID) error
func (m *TimeoutManager) StartDBPolling(ctx context.Context)

// biz/timeout_handler.go — 统一路由 order timeout
type TimeoutHandler struct {
    deliveryIn  *DeliveryInUsecase
    deliveryOut *DeliveryOutUsecase
    storageUc   *StorageUsecase
    orderRepo   OrderRepo
}
func (h *TimeoutHandler) HandleOrderTimeout(ctx, orderID) error

// biz/pickup_code.go — Redis only
type PickupCodeManager struct { redis *redis.Client }
func (m *PickupCodeManager) Generate(ctx, tenantID, cabinetID, orderID) (string, error)
func (m *PickupCodeManager) Verify(ctx, tenantID, code) (int64, error)

// biz/device_commander.go
type DeviceCommander struct { deviceClient pb.DeviceCommandServiceClient }
func (dc *DeviceCommander) OpenCell(ctx, tenantID, deviceSN, slotIndex, refOrderNo) (msgID string, err error)
func (dc *DeviceCommander) GetSlotStatus(ctx, tenantID, deviceSN) (*SlotOverview, error)
func (dc *DeviceCommander) ForceReleaseCell(ctx, tenantID, deviceSN, slotIndex, operator, reason) error

// biz/event_publisher.go — 接口
type EventPublisher interface {
    PublishPickupReady(ctx context.Context, evt *PickupReadyEvent) error
    PublishOrderTimeout(ctx context.Context, evt *OrderTimeoutEvent) error
    PublishOvertimeFeeCalculated(ctx context.Context, evt *OvertimeFeeEvent) error
    PublishOpenTimeout(ctx context.Context, evt *OpenTimeoutEvent) error
}
```

### 3.3 同步/异步拆分模式

每个 Usecase 有两个入口方法：

```go
// 1. 同步入口（HTTP 请求触发）— 不等关门
func (uc *DeliveryInUsecase) InitiateDelivery(ctx context.Context, req *DeliveryRequest) (*StorageOrder, error) {
    // CreateOrder(status=10) → AllocateCell → MarkOpening(pending_action=1) → OpenCell → return
    // 注册订单超时 timer（基于 pricing_rules.free_hours）
    // 订单停在 Pending，等 MQTT 关门回调
}

// 2. 异步回调（Device 服务 gRPC ReportDoorEvent 触发）
func (uc *DeliveryInUsecase) HandleDepositClosed(ctx context.Context, order *StorageOrder, cell *Cell) error {
    // FSM 10→11 → ConfirmOccupied → GeneratePickupCode → PublishPickupReady
}
```

### 3.4 Repo 接口

```go
type CellRepo interface {
    FindByDeviceAndSlot(ctx context.Context, deviceSN string, slotIndex int32) (*Cell, error)
    AllocateForUpdate(ctx context.Context, cabinetID int64, cellType int32) (*Cell, error)  // SELECT FOR UPDATE
    UpdateStatus(ctx context.Context, id int64, status int32) error
    UpdateStatusCAS(ctx context.Context, id int64, expectedFrom, newStatus int32) (int64, error)
    UpdateOpenedAt(ctx context.Context, id int64, t time.Time) error
    FindOpenTimeoutCells(ctx context.Context, threshold time.Duration) ([]*Cell, error)
}

type OrderRepo interface {
    Create(ctx context.Context, order *StorageOrder) (*StorageOrder, error)
    GetByID(ctx context.Context, id int64) (*StorageOrder, error)
    GetByOrderNo(ctx context.Context, orderNo string) (*StorageOrder, error)
    UpdateStatusCAS(ctx context.Context, id int64, expectedFrom, newStatus int32) (int64, error)
    UpdateAmount(ctx context.Context, id int64, totalAmount int32) error
    FindPossiblyTimeoutOrders(ctx context.Context, threshold time.Duration) ([]*StorageOrder, error)
    ListByUser(ctx context.Context, tenantID, userID int64, page, pageSize int32) ([]*StorageOrder, int32, error)
}
```

### 3.5 Wire ProviderSet

```go
var ProviderSet = wire.NewSet(
    NewOrderFSM,
    NewCellAllocator,
    NewPricingEngine,
    NewTimeoutManager,
    NewTimeoutHandler,
    NewPickupCodeManager,
    NewDeviceCommander,
    NewDeliveryInUsecase,
    NewDeliveryOutUsecase,
    NewStorageUsecase,
)
```

---

## 4. 核心流程

### 4.1 Delivery-In（快递员投递 → 用户取件）

```
快递员 App            Storage Service              Device Service              柜机
    │                      │                           │                       │
    │ POST /v1/storage/    │                           │                       │
    │ delivery/initiate ──→│                           │                       │
    │                      │─ CreateOrder(status=10)   │                       │
    │                      │─ AllocateCell(FOR UPDATE) │                       │
    │                      │─ MarkOpening(action=1)    │                       │
    │                      │─ gRPC: OpenCell ─────────→│                       │
    │                      │                           │─ MQTT: open_door ────→│
    │  ←── 200 {order_no} │                           │                       │
    │                      │                           │←── MQTT: command_ack │
    │                      │                           │                       │
    │                      │                           │←── MQTT: door_opened │
    │                      │←── gRPC: ReportDoorEvent ─│  (door_closed=false)  │
    │                      │  记录 opened_at +         │                       │
    │                      │  RegisterOpenTimeout(5m)  │                       │
    │                      │                           │                       │
    │                      │                           │←── MQTT: door_closed │
    │                      │←── gRPC: ReportDoorEvent ─│  (door_closed=true)   │
    │                      │  pending_action=1:        │                       │
    │                      │  FSM 10→11                │                       │
    │                      │  ConfirmOccupied(cell)    │                       │
    │                      │  GeneratePickupCode       │                       │
    │                      │  PublishPickupReady ──→ RabbitMQ ──→ Notification │
    │                      │                           │                       │
    │ 用户收到取件码通知    │                           │                       │
    │                      │                           │                       │
    │ POST /v1/storage/    │                           │                       │
    │ pickup ─────────────→│                           │                       │
    │ {pickup_code}        │─ VerifyPickupCode → orderID                     │
    │                      │─ LoadOrder → FSM 校验     │                       │
    │                      │─ CalculateFee             │                       │
    │  ←── 200 {fee,       │                           │                       │
    │    status: "FREE"    │                           │                       │
    │    | "PAYMENT_REQ"}  │                           │                       │
    │                      │                           │                       │
    │ POST /v1/storage/    │                           │                       │
    │ pickup/confirm ─────→│ (fee=0 直接过/fee>0 查支付)│                      │
    │                      │─ MarkOpening(action=2)    │                       │
    │                      │─ gRPC: OpenCell ─────────→│─ MQTT: open_door ────→│
    │                      │                           │←── MQTT: door_closed │
    │                      │←── gRPC: ReportDoorEvent ─│                       │
    │                      │  pending_action=2:        │                       │
    │                      │  FSM 11/15→13             │                       │
    │                      │  ReleaseCell              │                       │
    │  ←── 200 OK          │                           │                       │
```

### 4.2 Storage（物品寄存）

与 delivery-in 类似，区别：
- 用户自己下单存入（`InitiateStorage`）
- `HandleDepositClosed` → FSM 10→12（Storing）
- pending_action=4
- 取出时 `RetrieveStorage`：计算寄存费 → `ConfirmPickup` → 开门 → 关门回调 FSM 12→13 + ReleaseCell
- 支持 12→12 临时开柜（pending_action=3），有每日次数上限
- `HandleTempOpenClosed`：状态不变(12→12)，记录开柜日志

### 4.3 Delivery-Out（用户寄件）

与 delivery-in 对称，区别：
- 用户 App 下单 + 预付运费
- `HandleDepositClosed` → FSM 10→11（Deposited），pending_action=4
- 快递员揽收时通过管理端开门 → 关门回调 FSM 11→13，写入 `operator_id`

### 4.4 关门回调路由逻辑

```go
func (s *StorageCallbackService) ReportDoorEvent(ctx, req) {
    if !req.DoorClosed {
        // 开门事件：记录 opened_at + 注册开门超时 timer
        cell, _ := s.cellRepo.FindByDeviceAndSlot(ctx, req.DeviceSN, req.SlotIndex)
        s.cellRepo.UpdateOpenedAt(ctx, cell.ID, time.Now())
        s.timeoutManager.RegisterOpenTimeout(cell.ID, 5*time.Minute)
        return
    }
    // 关门事件
    cell, _ := s.cellRepo.FindByDeviceAndSlot(ctx, req.DeviceSN, req.SlotIndex)
    if cell.PendingAction == 0 {
        // 无待处理动作，忽略但记录日志
        s.log.Warnf("unexpected door_closed: device=%s slot=%d", req.DeviceSN, req.SlotIndex)
        return
    }
    order := s.orderRepo.GetByID(ctx, cell.CurrentOrderID)
    switch cell.PendingAction {
    case 1: s.deliveryIn.HandleDepositClosed(ctx, order, cell)    // 投递确认
    case 2: s.deliveryIn.HandlePickupClosed(ctx, order, cell)     // 取件确认
    case 3: s.storageUc.HandleTempOpenClosed(ctx, order, cell)    // 临时开柜恢复
    case 4: s.storageUc.HandleDepositClosed(ctx, order, cell)     // 寄存确认
    }
}
```

### 4.5 HTTP API Surface

```protobuf
// api/storage/v1/storage.proto

service StorageService {
  // ── 投递（快递员）──
  rpc InitiateDelivery (InitiateDeliveryRequest) returns (InitiateDeliveryReply) {
    option (google.api.http) = { post: "/v1/storage/delivery/initiate" body: "*" };
  }

  // ── 取件（用户）── 两步模式
  rpc Pickup (PickupRequest) returns (PickupReply) {
    option (google.api.http) = { post: "/v1/storage/pickup" body: "*" };
  }
  rpc ConfirmPickup (ConfirmPickupRequest) returns (ConfirmPickupReply) {
    option (google.api.http) = { post: "/v1/storage/pickup/confirm" body: "*" };
  }

  // ── 寄件（用户）──
  rpc InitiateShipment (InitiateShipmentRequest) returns (InitiateShipmentReply) {
    option (google.api.http) = { post: "/v1/storage/shipment/initiate" body: "*" };
  }

  // ── 寄存（用户）──
  rpc InitiateStorage (InitiateStorageRequest) returns (InitiateStorageReply) {
    option (google.api.http) = { post: "/v1/storage/store/initiate" body: "*" };
  }
  rpc RetrieveStorage (RetrieveStorageRequest) returns (RetrieveStorageReply) {
    option (google.api.http) = { post: "/v1/storage/store/retrieve" body: "*" };
  }
  rpc TempOpenCell (TempOpenCellRequest) returns (TempOpenCellReply) {
    option (google.api.http) = { post: "/v1/storage/store/temp-open" body: "*" };
  }

  // ── 查询 ──
  rpc GetOrder (GetOrderRequest) returns (GetOrderReply) {
    option (google.api.http) = { get: "/v1/storage/order/{order_no}" };
  }
  rpc ListMyOrders (ListMyOrdersRequest) returns (ListMyOrdersReply) {
    option (google.api.http) = { get: "/v1/storage/orders" };
  }

  // ── 管理端 ──
  rpc ListCabinets (ListCabinetsRequest) returns (ListCabinetsReply) {
    option (google.api.http) = { get: "/v1/storage/cabinets" };
  }
  rpc GetCabinetDetail (GetCabinetDetailRequest) returns (GetCabinetDetailReply) {
    option (google.api.http) = { get: "/v1/storage/cabinet/{id}" };
  }
  rpc ForceOpenCell (ForceOpenCellRequest) returns (ForceOpenCellReply) {
    option (google.api.http) = { post: "/v1/storage/cabinet/cell/force-open" body: "*" };
  }
}

// gRPC 内部接口 — Device 服务回调
service StorageCallbackService {
  rpc ReportDoorEvent (ReportDoorEventRequest) returns (ReportDoorEventReply);
}
```

### 4.6 超时检测

#### 三层超时体系

| 超时层 | 触发点 | 时长 | 处理 |
|--------|--------|------|------|
| command_ack 超时 | OpenCell gRPC 调用 | 10s | 返回错误给客户端 |
| 开门超时 | door_opened 回调 | 5min | ConfirmOccupied + 告警 |
| 订单超时 | deposited_at | 由 pricing_rules.free_hours 决定 | FSM→Timeout + 计费 |

#### DB 兜底轮询

```go
func (m *TimeoutManager) StartDBPolling(ctx context.Context) {
    ticker := time.NewTicker(60 * time.Second)
    for {
        select {
        case <-ticker.C:
            // Phase 1: 宽松扫描（默认阈值 1 小时）
            orders, _ := m.orderRepo.FindPossiblyTimeoutOrders(ctx, 1*time.Hour)
            // Phase 2: 逐条查 pricing_rules 精确判断
            for _, o := range orders {
                rule, _ := m.pricingEngine.MatchRule(ctx, o.TenantID, "storage_overtime", o.CellType)
                deadline := o.DepositedAt.Add(time.Duration(rule.FreeHours) * time.Hour)
                if time.Now().After(deadline) {
                    m.orderTimeoutCb(ctx, o.ID)
                }
            }
            // 开门超时扫描（5 分钟阈值）
            cells, _ := m.cellRepo.FindOpenTimeoutCells(ctx, 5*time.Minute)
            for _, c := range cells {
                m.HandleOpenTimeout(ctx, c.ID)
            }
        case <-ctx.Done():
            return
        }
    }
}
```

#### CAS 幂等保护

```go
affected, err := uc.orderRepo.UpdateStatusCAS(ctx, order.ID, fromStatus, StorageOrderTimeout)
if affected == 0 {
    return nil  // 已被 timer 或其他实例处理，静默跳过
}
```

---

## 5. gRPC 契约

### 5.1 Device Command Service（Device 服务新增）

```protobuf
// api/device/v1/device_command.proto
syntax = "proto3";
package api.device.v1;
option go_package = "shared-device-saas/api/device/v1;v1";

service DeviceCommandService {
    rpc OpenCell (OpenCellRequest) returns (OpenCellReply);
    rpc GetDeviceSlotStatus (GetDeviceSlotStatusRequest) returns (GetDeviceSlotStatusReply);
    rpc ForceReleaseCell (ForceReleaseCellRequest) returns (ForceReleaseCellReply);
}

message OpenCellRequest {
    int64  tenant_id = 1;
    string device_sn = 2;
    int32  slot_index = 3;
    int32  timeout_sec = 4;
    string operator = 5;          // system / admin
    string ref_order_no = 6;
}

message OpenCellReply {
    bool   ok = 1;
    string msg_id = 2;
    string error = 3;
}

message GetDeviceSlotStatusRequest {
    int64  tenant_id = 1;
    string device_sn = 2;
}

message GetDeviceSlotStatusReply {
    bool   online = 1;
    int32  total_slots = 2;
    int32  free_slots = 3;
    map<int32, string> slot_status = 4;
}

message ForceReleaseCellRequest {
    int64  tenant_id = 1;
    string device_sn = 2;
    int32  slot_index = 3;
    string operator = 4;          // admin(运维) / system(自动超时回退)
    string reason = 5;
}

message ForceReleaseCellReply {
    bool   ok = 1;
    string error = 2;
}
```

### 5.2 Storage Callback Service（Storage 服务暴露）

```protobuf
// api/storage/v1/storage_callback.proto
syntax = "proto3";
package api.storage.v1;
option go_package = "shared-device-saas/api/storage/v1;v1";

service StorageCallbackService {
    rpc ReportDoorEvent (ReportDoorEventRequest) returns (ReportDoorEventReply);
}

message ReportDoorEventRequest {
    int64  tenant_id = 1;
    string device_sn = 2;
    int32  slot_index = 3;
    bool   door_closed = 4;       // true=关门 false=开门
    int32  duration_ms = 5;       // 开门持续时长（仅关门事件）
    string ref_msg_id = 6;
}

message ReportDoorEventReply {
    bool ok = 1;
}
```

---

## 6. MQTT 消息格式

### 6.1 信封格式

```json
{
    "v": 1,
    "ts": 1715012345,
    "msg_id": "uuid-xxx",
    "type": "door_closed",
    "data": { ... }
}
```

`v` 字段为 Protobuf 升级预留：`v=2` 时 `data` 为 base64 编码的 Protobuf 二进制。

### 6.2 Topic 规范

沿用 `pkg/mqtt/topic.go`：

| 方向 | Topic | 说明 |
|------|-------|------|
| 上行 | `{tenant_id}/device/locker/{device_sn}/event` | 心跳 + 柜门事件 |
| 上行 | `{tenant_id}/device/locker/{device_sn}/status` | 状态上报 |
| 下行 | `{tenant_id}/device/locker/{device_sn}/command` | 开门指令 |
| 应答 | `{tenant_id}/device/locker/{device_sn}/response` | command_ack |

### 6.3 消息类型

**上行（event topic）**：

| type | data |
|------|------|
| `heartbeat` | `slots: {index: status}`, `battery_level`, `temperature`, `firmware_version` |
| `door_opened` | `slot_index`, `trigger: command/manual/timeout`, `ref_msg_id` |
| `door_closed` | `slot_index`, `duration_ms`, `ref_msg_id` |
| `fault` | `slot_index`, `fault_code`, `description` |

**下行（command topic）**：

| type | data |
|------|------|
| `open_door` | `slot_index`, `timeout_sec`, `operator` |
| `reboot` | — |

**应答（response topic）**：

| type | data |
|------|------|
| `command_ack` | `ref_msg_id`, `result: ok/failed`, `error` |

### 6.4 Device 服务处理链

```
柜机 → MQTT Broker → Device Service MQTT Handler
                        ↓
                     ParseTopic → {tenant_id, device_sn}
                        ↓
                     ParseEnvelope → {msg_id, type, data}
                        ↓
                 ┌──── type 路由 ────┐
                 │                    │
           heartbeat          door_opened / door_closed / fault
                 │                    │
        UpdateRedisCache       gRPC → Storage.ReportDoorEvent
        UpdateDeviceStatus     (Device 不处理 command_ack，仅返回给调用方)
```

---

## 7. RabbitMQ 消息格式

### 7.1 Exchange + Routing Key

```
Exchange: storage.events (topic type)
Routing Key: storage.{event_type}.{tenant_id}
```

| Routing Key | 触发场景 |
|-------------|---------|
| `storage.pickup_ready.{tenant_id}` | 投递成功，生成取件码 |
| `storage.order_timeout.{tenant_id}` | 订单超时 |
| `storage.overtime_fee.{tenant_id}` | 超时费计算完成 |
| `storage.open_timeout.{tenant_id}` | 开门超时异常（管理员告警） |

### 7.2 消息体

```go
type StorageEvent struct {
    EventID   string `json:"event_id"`
    EventType string `json:"event_type"`
    TenantID  int64  `json:"tenant_id"`
    OrderNo   string `json:"order_no"`
    UserID    int64  `json:"user_id"`
    CabinetID int64  `json:"cabinet_id,omitempty"`
    CellID    int64  `json:"cell_id,omitempty"`
    Payload   string `json:"payload"`     // JSON string
    Timestamp int64  `json:"timestamp"`
}
```

### 7.3 通知策略

```go
type NotifyChannel interface {
    Send(ctx context.Context, userID string, title, body string) error
}

type NotifyUsecase struct {
    channels map[string]NotifyChannel  // "app" → wsChannel, "sms" → smsChannel
}
```

---

## 8. 异常场景处理

### 8.1 格口状态一致性（MQTT 消息丢失）

**方案**：超时 + 补偿轮询。
- 开门指令下发后，cell 标记为 `status=3(开门中)`
- door_opened 事件到达后记录 `opened_at` + 注册 5 分钟 timer
- 超时未收到 door_closed → `ConfirmOccupied`（假设有物品）+ 管理员告警
- DB 轮询每分钟扫 `status=3 AND opened_at < NOW()-5min` 兜底

### 8.2 硬件故障（门打不开）

**方案**：自动换柜。
- Device 服务上报 `fault` 事件
- Storage 收到 `command_ack.result=failed` 或超时
- 标记 cell 为 `status=4(故障)`
- 释放原 cell，重新 `AllocateCell` 分配新格口

### 8.3 开门未取走（红外检测）

**方案**：关门事件携带红外传感器状态。
- 红外检测仍有物品 → 状态不变，恢复为 `Deposited/Storing`
- 无红外传感器 → 依赖超时机制（开门 3 分钟需重新扫码认证）

### 8.4 设备离线

**方案**：心跳检测 + 本地降级。
- Device 服务 MQTT 心跳维持，3 个周期未响应置为离线
- 柜机本地保存有效取件码，支持屏幕输入开柜
- 网络恢复后异步批量上报开柜记录

### 8.5 并发安全

- 格口分配：`SELECT FOR UPDATE` 悲观锁
- 状态更新：CAS `UPDATE ... SET status=? WHERE id=? AND status=?`，检查 affected rows
- 取件码去重：Redis SET `SISMEMBER` O(1)

---

## 9. 支付集成

### 9.1 先消费后付款 + 超时补缴

- 免费期内取件：`total_amount=0`，直接开门
- 超时：Go timer 触发 → 查 pricing_rules 计算超时费 → `UpdateAmount`
- 取件时 `Pickup` 返回费用 → `ConfirmPickup` 校验支付状态后开门
- 复用现有 `pkg/payment/` 模块，无需改动

### 9.2 取件码

- 格式：6 位纯数字（100000-999999）
- 存储：Redis SET `storage:pickup:{tenant_id}:{cabinet_id}`
- 有效期：跟订单状态绑定，取件后从 SET 中移除
- 查询过滤：`status NOT IN (13, 16)`

---

## 10. 关键设计决策汇总

| 决策点 | 选择 | 理由 |
|--------|------|------|
| 格口管理归属 | Storage MySQL + Device Redis 两层真相源 | 变化频率和可靠性要求不同 |
| 订单模型 | 统一 orders 表 + order_type | 用户查"我的全部订单"一条 SQL，支付/计费复用 |
| FSM 实现 | 转移表 + 纯函数 | 新增状态只加配置行，不改业务代码 |
| 超时检测 | Go timer + DB 轮询兜底 | 精准触发 + 进程重启不丢 |
| MQTT payload | JSON + 信封格式 | 硬件厂商兼容性好，v 字段预留 Protobuf |
| 代码组织 | 按业务拆 Usecase + 共享引擎 | 每个 <200 行，独立可测试 |
| 回调解耦 | TimeoutHandler 统一路由 | 查一次 DB 即路由，不浪费 |
| 状态更新 | CAS（Compare And Swap） | 防并发冲突，幂等安全 |
| 开门超时 | door_opened 时才注册 timer | 避免门未开就计时的误判 |
| pending_action | cells 表字段 | 开门时写、关门时读，可靠路由 |
| 取件码 | 6 位数字 + Redis SET | 临时数据不进 MySQL，SISMEMBER O(1) |
| 支付 | 先消费后付款，超时补缴 | 共享设备行业惯例 |
