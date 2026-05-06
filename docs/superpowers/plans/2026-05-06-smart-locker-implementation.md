# Smart Locker Module Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the smart locker (智能快递柜) module — a complete Storage microservice with MQTT-driven device interaction, order FSM, cell allocation, timeout management, and multi-tenant billing.

**Architecture:** Two-layer truth source pattern — Storage service owns business state (MySQL cells/orders), Device service owns physical state (Redis slots). Storage calls Device via gRPC for door operations; Device calls back via gRPC on MQTT door events. Three business usecases (delivery-in, delivery-out, storage) share 6 core engines.

**Tech Stack:** Go 1.22+, Kratos v2, Wire DI, MySQL 8.0, Redis, MQTT (paho.golang), gRPC, RabbitMQ (notifications), golang-migrate

**Spec:** `docs/superpowers/specs/2026-05-06-smart-locker-design.md`

**Existing Patterns to Follow:**
- DDD layers: `server/ → service/ → biz/ ← data/` (biz defines interfaces, data implements)
- Wire ProviderSet in each layer's `biz.go`/`data.go`/`service.go`/`server.go`
- `pkg/redis.Client` wrapper for Redis operations
- `pkg/errx` for error codes
- `pkg/middleware/tenant` for multi-tenant context
- Standard `testing` package for tests (see `app/device/internal/biz/inventory_test.go`)
- Sequential golang-migrate files (`migrations/000007_*.sql`)

---

## Chunk 1: Foundation — Migrations, Proto, Domain Types

### Task 1: Database Migrations

**Files:**
- Create: `migrations/000007_locker_tables.up.sql`
- Create: `migrations/000007_locker_tables.down.sql`

- [ ] **Step 1: Create migration files**

```bash
make migrate-create NAME=locker_tables
```

- [ ] **Step 2: Write the up migration**

Edit `migrations/000007_locker_tables.up.sql`:

```sql
-- 柜机实例表（device 的业务投影）
CREATE TABLE IF NOT EXISTS `cabinets` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '租户/园区ID',
  `device_id` BIGINT UNSIGNED NOT NULL COMMENT '关联 device 表',
  `device_sn` VARCHAR(32) NOT NULL COMMENT '柜机序列号',
  `name` VARCHAR(64) NOT NULL COMMENT '柜机名称',
  `location_name` VARCHAR(128) NOT NULL COMMENT '安装位置',
  `total_cells` INT NOT NULL COMMENT '格口总数',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '1=正常 2=维护中 3=停用',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_device_id` (`device_id`),
  KEY `idx_tenant_status` (`tenant_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='柜机实例表';

-- 格口表
CREATE TABLE IF NOT EXISTS `cells` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '租户ID',
  `cabinet_id` BIGINT UNSIGNED NOT NULL COMMENT '所属柜机',
  `slot_index` INT NOT NULL COMMENT '对应 device 服务的 slot 编号',
  `cell_type` TINYINT NOT NULL DEFAULT 1 COMMENT '1=小 2=中 3=大',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '1=空闲 2=占用 3=开门中 4=故障 5=停用',
  `current_order_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '当前占用订单',
  `pending_action` TINYINT DEFAULT NULL COMMENT '1=等投递确认 2=等取件确认 3=临时开柜 4=等寄存确认',
  `opened_at` DATETIME DEFAULT NULL COMMENT '开门时间',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_cabinet_slot` (`cabinet_id`, `slot_index`),
  KEY `idx_tenant_status` (`tenant_id`, `status`),
  KEY `idx_cabinet_status` (`cabinet_id`, `status`),
  KEY `idx_open_timeout` (`status`, `opened_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='格口表';

-- 快递柜订单表
CREATE TABLE IF NOT EXISTS `storage_orders` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '租户ID',
  `order_no` VARCHAR(32) NOT NULL COMMENT '业务订单号',
  `order_type` VARCHAR(16) NOT NULL COMMENT 'delivery_in / delivery_out / storage',
  `status` INT NOT NULL DEFAULT 10 COMMENT 'FSM 状态码',
  `user_id` BIGINT UNSIGNED NOT NULL COMMENT '主角色',
  `operator_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '副角色',
  `cabinet_id` BIGINT UNSIGNED NOT NULL,
  `cell_id` BIGINT UNSIGNED NOT NULL,
  `device_sn` VARCHAR(32) NOT NULL,
  `slot_index` INT NOT NULL,
  `pickup_code` VARCHAR(6) DEFAULT NULL COMMENT '取件码',
  `deposited_at` DATETIME DEFAULT NULL COMMENT '物品放入时间',
  `picked_up_at` DATETIME DEFAULT NULL COMMENT '取出时间',
  `total_amount` INT NOT NULL DEFAULT 0 COMMENT '总费用（分）',
  `paid_amount` INT NOT NULL DEFAULT 0 COMMENT '已付金额（分）',
  `overtime_minutes` INT NOT NULL DEFAULT 0 COMMENT '超时分钟数',
  `remark` VARCHAR(256) DEFAULT NULL,
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_order_no` (`order_no`),
  KEY `idx_tenant_user` (`tenant_id`, `user_id`),
  KEY `idx_tenant_cabinet` (`tenant_id`, `cabinet_id`, `status`),
  KEY `idx_status_timeout` (`status`, `deposited_at`),
  KEY `idx_tenant_pickup` (`tenant_id`, `pickup_code`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='快递柜订单表';

-- 计费规则表
CREATE TABLE IF NOT EXISTS `pricing_rules` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
  `tenant_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '租户ID',
  `rule_type` VARCHAR(32) NOT NULL COMMENT 'storage_overtime / storage_daily / delivery_fee',
  `free_hours` INT NOT NULL DEFAULT 24 COMMENT '免费时长（小时）',
  `price_per_hour` INT NOT NULL DEFAULT 0 COMMENT '每小时费用（分）',
  `price_per_day` INT NOT NULL DEFAULT 0 COMMENT '每天封顶（分）',
  `max_fee` INT NOT NULL DEFAULT 0 COMMENT '总封顶（分）',
  `cell_type` TINYINT DEFAULT NULL COMMENT 'NULL=全部类型',
  `priority` INT NOT NULL DEFAULT 0 COMMENT '优先级，越大越优先',
  `effective_from` DATETIME DEFAULT NULL COMMENT '生效起始',
  `effective_until` DATETIME DEFAULT NULL COMMENT '生效截止',
  `enabled` TINYINT NOT NULL DEFAULT 1 COMMENT '1=启用 0=禁用',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`),
  KEY `idx_tenant_type` (`tenant_id`, `rule_type`, `enabled`),
  KEY `idx_effective` (`tenant_id`, `rule_type`, `effective_from`, `effective_until`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='计费规则表';
```

- [ ] **Step 3: Write the down migration**

Edit `migrations/000007_locker_tables.down.sql`:

```sql
DROP TABLE IF EXISTS `pricing_rules`;
DROP TABLE IF EXISTS `storage_orders`;
DROP TABLE IF EXISTS `cells`;
DROP TABLE IF EXISTS `cabinets`;
```

- [ ] **Step 4: Commit**

```bash
git add migrations/000007_locker_tables.up.sql migrations/000007_locker_tables.down.sql
git commit -m "feat(storage): add locker tables migration — cabinets, cells, storage_orders, pricing_rules"
```

---

### Task 2: Domain Types and Constants

**Files:**
- Create: `app/storage/internal/biz/types.go`

- [ ] **Step 1: Write domain type definitions**

```go
package biz

import "time"

// Cell status constants
const (
	CellStatusFree     int32 = 1 // 空闲
	CellStatusOccupied int32 = 2 // 占用
	CellStatusOpening  int32 = 3 // 开门中
	CellStatusFault    int32 = 4 // 故障
	CellStatusDisabled int32 = 5 // 停用
)

// Cell type constants
const (
	CellTypeSmall  int32 = 1
	CellTypeMedium int32 = 2
	CellTypeLarge  int32 = 3
)

// Pending action constants
const (
	PendingActionDeposit    int32 = 1 // 等待投递/存入确认
	PendingActionPickup     int32 = 2 // 等待取件确认
	PendingActionTempOpen   int32 = 3 // 临时开柜
	PendingActionStore      int32 = 4 // 等待寄存确认
)

// Order status constants (FSM states)
const (
	OrderStatusPending   int32 = 10 // 待投递/待存入
	OrderStatusDeposited int32 = 11 // 已投递/待取件
	OrderStatusStoring   int32 = 12 // 存放中
	OrderStatusCompleted int32 = 13 // 已完成（终态）
	OrderStatusTimeout   int32 = 15 // 超时未取
	OrderStatusCleared   int32 = 16 // 运维清理（终态）
)

// Order type constants
const (
	OrderTypeDeliveryIn  = "delivery_in"
	OrderTypeDeliveryOut = "delivery_out"
	OrderTypeStorage     = "storage"
)

// Cabinet — device 的业务投影
type Cabinet struct {
	ID           int64
	TenantID     int64
	DeviceID     int64
	DeviceSN     string
	Name         string
	LocationName string
	TotalCells   int32
	Status       int32
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Cell — 格口
type Cell struct {
	ID             int64
	TenantID       int64
	CabinetID      int64
	SlotIndex      int32
	CellType       int32
	Status         int32
	CurrentOrderID *int64
	PendingAction  *int32
	OpenedAt       *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// StorageOrder — 快递柜订单
type StorageOrder struct {
	ID              int64
	TenantID        int64
	OrderNo         string
	OrderType       string
	Status          int32
	UserID          int64
	OperatorID      *int64
	CabinetID       int64
	CellID          int64
	DeviceSN        string
	SlotIndex       int32
	PickupCode      *string
	DepositedAt     *time.Time
	PickedUpAt      *time.Time
	TotalAmount     int32
	PaidAmount      int32
	OvertimeMinutes int32
	Remark          *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// PricingRule — 计费规则
type PricingRule struct {
	ID            int64
	TenantID      int64
	RuleType      string
	FreeHours     int32
	PricePerHour  int32
	PricePerDay   int32
	MaxFee        int32
	CellType      *int32
	Priority      int32
	EffectiveFrom *time.Time
	EffectiveUntil *time.Time
	Enabled       bool
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
```

- [ ] **Step 2: Run go vet**

```bash
go vet ./app/storage/internal/biz/...
```

Expected: no errors

- [ ] **Step 3: Commit**

```bash
git add app/storage/internal/biz/types.go
git commit -m "feat(storage): add domain types and constants for locker module"
```

---

### Task 3: Proto Definitions — Storage Service API

**Files:**
- Create: `api/storage/v1/storage.proto`
- Create: `api/storage/v1/storage_callback.proto`

- [ ] **Step 1: Write storage.proto**

```protobuf
syntax = "proto3";

package api.storage.v1;

option go_package = "shared-device-saas/api/storage/v1;v1";

import "google/api/annotations.proto";
import "google/protobuf/empty.proto";

service StorageService {
  // 投递（快递员）
  rpc InitiateDelivery (InitiateDeliveryRequest) returns (InitiateDeliveryReply) {
    option (google.api.http) = { post: "/v1/storage/delivery/initiate" body: "*" };
  }

  // 取件两步模式 — Step 1: 校验 + 返回费用
  rpc Pickup (PickupRequest) returns (PickupReply) {
    option (google.api.http) = { post: "/v1/storage/pickup" body: "*" };
  }

  // 取件两步模式 — Step 2: 确认取件（触发开门）
  rpc ConfirmPickup (ConfirmPickupRequest) returns (ConfirmPickupReply) {
    option (google.api.http) = { post: "/v1/storage/pickup/confirm" body: "*" };
  }

  // 寄件（用户）
  rpc InitiateShipment (InitiateShipmentRequest) returns (InitiateShipmentReply) {
    option (google.api.http) = { post: "/v1/storage/shipment/initiate" body: "*" };
  }

  // 寄存（用户）
  rpc InitiateStorage (InitiateStorageRequest) returns (InitiateStorageReply) {
    option (google.api.http) = { post: "/v1/storage/store/initiate" body: "*" };
  }
  rpc RetrieveStorage (RetrieveStorageRequest) returns (RetrieveStorageReply) {
    option (google.api.http) = { post: "/v1/storage/store/retrieve" body: "*" };
  }
  rpc ConfirmRetrieve (ConfirmRetrieveRequest) returns (ConfirmRetrieveReply) {
    option (google.api.http) = { post: "/v1/storage/store/retrieve/confirm" body: "*" };
  }
  rpc TempOpenCell (TempOpenCellRequest) returns (TempOpenCellReply) {
    option (google.api.http) = { post: "/v1/storage/store/temp-open" body: "*" };
  }

  // 查询
  rpc GetOrder (GetOrderRequest) returns (GetOrderReply) {
    option (google.api.http) = { get: "/v1/storage/order/{order_no}" };
  }
  rpc ListMyOrders (ListMyOrdersRequest) returns (ListMyOrdersReply) {
    option (google.api.http) = { get: "/v1/storage/orders" };
  }

  // 管理端
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

// ── Request/Reply messages ──

message InitiateDeliveryRequest {
  int64 cabinet_id = 1;
  int32 cell_type = 2;       // 1=小 2=中 3=大
  string tracking_no = 3;    // 快递单号（可选）
  int64 recipient_user_id = 4; // 收件人用户ID（可选）
}

message InitiateDeliveryReply {
  string order_no = 1;
  int64 cell_id = 2;
  int32 slot_index = 3;
}

message PickupRequest {
  string pickup_code = 1;
}

message PickupReply {
  string order_no = 1;
  int32 fee = 2;              // 费用（分）
  string status = 3;          // "FREE" | "PAYMENT_REQUIRED"
  string cabinet_name = 4;
  int32 slot_index = 5;
}

message ConfirmPickupRequest {
  string order_no = 1;
  string pickup_code = 2;
  int32 paid_amount = 3;     // 已付金额（超时费场景）
}

message ConfirmPickupReply {
  bool ok = 1;
}

message InitiateShipmentRequest {
  int64 cabinet_id = 1;
  int32 cell_type = 2;
  string courier_company = 3;
}

message InitiateShipmentReply {
  string order_no = 1;
  int64 cell_id = 2;
  int32 slot_index = 3;
}

message InitiateStorageRequest {
  int64 cabinet_id = 1;
  int32 cell_type = 2;
}

message InitiateStorageReply {
  string order_no = 1;
  int64 cell_id = 2;
  int32 slot_index = 3;
}

message RetrieveStorageRequest {
  string order_no = 1;
}

message RetrieveStorageReply {
  int32 fee = 1;
  string status = 2;
}

message ConfirmRetrieveRequest {
  string order_no = 1;
  int32 paid_amount = 2;
}

message ConfirmRetrieveReply {
  bool ok = 1;
}

message TempOpenCellRequest {
  string order_no = 1;
}

message TempOpenCellReply {
  bool ok = 1;
}

message GetOrderRequest {
  string order_no = 1;
}

message GetOrderReply {
  int64 id = 1;
  string order_no = 2;
  string order_type = 3;
  int32 status = 4;
  int32 slot_index = 5;
  string pickup_code = 6;
  int32 total_amount = 7;
  string created_at = 8;
  string deposited_at = 9;
}

message ListMyOrdersRequest {
  int32 page = 1;
  int32 page_size = 2;
  string order_type = 3;    // filter by type (optional)
}

message OrderItem {
  string order_no = 1;
  string order_type = 2;
  int32 status = 3;
  string cabinet_name = 4;
  int32 slot_index = 5;
  int32 total_amount = 6;
  string created_at = 7;
}

message ListMyOrdersReply {
  repeated OrderItem items = 1;
  int32 total = 2;
}

message ListCabinetsRequest {
  int32 page = 1;
  int32 page_size = 2;
  int32 status = 3;
}

message CabinetItem {
  int64 id = 1;
  string name = 2;
  string location_name = 3;
  int32 total_cells = 4;
  int32 free_cells = 5;
  int32 status = 6;
}

message ListCabinetsReply {
  repeated CabinetItem items = 1;
  int32 total = 2;
}

message GetCabinetDetailRequest {
  int64 id = 1;
}

message GetCabinetDetailReply {
  int64 id = 1;
  string name = 2;
  string device_sn = 3;
  string location_name = 4;
  int32 total_cells = 5;
  int32 free_cells = 6;
  int32 status = 7;
  repeated CellDetail cells = 8;
}

message CellDetail {
  int64 id = 1;
  int32 slot_index = 2;
  int32 cell_type = 3;
  int32 status = 4;
}

message ForceOpenCellRequest {
  int64 cell_id = 1;
  string reason = 2;
}

message ForceOpenCellReply {
  bool ok = 1;
}
```

- [ ] **Step 2: Write storage_callback.proto**

```protobuf
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
  bool   door_closed = 4;
  int32  duration_ms = 5;
  string ref_msg_id = 6;
}

message ReportDoorEventReply {
  bool ok = 1;
}
```

- [ ] **Step 3: Generate proto Go code**

```bash
cd api && buf generate
```

Expected: `api/storage/v1/storage.pb.go`, `storage_grpc.pb.go`, `storage_http.pb.go`, `storage_callback.pb.go`, `storage_callback_grpc.pb.go` generated

- [ ] **Step 4: Commit**

```bash
git add api/storage/v1/
git commit -m "feat(storage): add proto definitions for storage service and callback"
```

---

### Task 4: Proto — Device Command Service Extension

**Files:**
- Create: `api/device/v1/device_command.proto`

- [ ] **Step 1: Write device_command.proto**

```protobuf
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
  string operator = 5;
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
  string operator = 4;
  string reason = 5;
}

message ForceReleaseCellReply {
  bool   ok = 1;
  string error = 2;
}
```

- [ ] **Step 2: Generate proto Go code**

```bash
cd api && buf generate
```

- [ ] **Step 3: Commit**

```bash
git add api/device/v1/device_command.proto api/device/v1/device_command*.pb.go
git commit -m "feat(device): add DeviceCommandService proto for locker cell operations"
```

---

## Chunk 2: Core Engines — FSM, CellAllocator, PricingEngine, PickupCode

### Task 5: OrderFSM — Pure Logic State Machine

**Files:**
- Create: `app/storage/internal/biz/order_fsm.go`
- Create: `app/storage/internal/biz/order_fsm_test.go`

- [ ] **Step 1: Write the FSM test**

```go
package biz

import "testing"

func TestCanTransition_DeliveryIn(t *testing.T) {
	fsm := NewOrderFSM()

	tests := []struct {
		from, to int32
		want     bool
	}{
		{OrderStatusPending, OrderStatusDeposited, true},
		{OrderStatusDeposited, OrderStatusCompleted, true},
		{OrderStatusDeposited, OrderStatusTimeout, true},
		{OrderStatusTimeout, OrderStatusCompleted, true},
		{OrderStatusTimeout, OrderStatusCleared, true},
		{OrderStatusPending, OrderStatusCompleted, false},
		{OrderStatusCompleted, OrderStatusPending, false},
	}
	for _, tt := range tests {
		got := fsm.CanTransition(OrderTypeDeliveryIn, tt.from, tt.to)
		if got != tt.want {
			t.Errorf("CanTransition(delivery_in, %d, %d) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestCanTransition_Storage(t *testing.T) {
	fsm := NewOrderFSM()

	tests := []struct {
		from, to int32
		want     bool
	}{
		{OrderStatusPending, OrderStatusStoring, true},
		{OrderStatusStoring, OrderStatusCompleted, true},
		{OrderStatusStoring, OrderStatusStoring, true},   // 临时开柜
		{OrderStatusStoring, OrderStatusTimeout, true},
		{OrderStatusPending, OrderStatusDeposited, false}, // storage 不走 Deposited
	}
	for _, tt := range tests {
		got := fsm.CanTransition(OrderTypeStorage, tt.from, tt.to)
		if got != tt.want {
			t.Errorf("CanTransition(storage, %d, %d) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestCanTransition_DeliveryOut(t *testing.T) {
	fsm := NewOrderFSM()

	tests := []struct {
		from, to int32
		want     bool
	}{
		{OrderStatusPending, OrderStatusDeposited, true},
		{OrderStatusDeposited, OrderStatusCompleted, true},
		{OrderStatusDeposited, OrderStatusTimeout, false}, // delivery_out 无超时
	}
	for _, tt := range tests {
		got := fsm.CanTransition(OrderTypeDeliveryOut, tt.from, tt.to)
		if got != tt.want {
			t.Errorf("CanTransition(delivery_out, %d, %d) = %v, want %v", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestTransition_ModifiesStatus(t *testing.T) {
	fsm := NewOrderFSM()
	order := &StorageOrder{OrderType: OrderTypeDeliveryIn, Status: OrderStatusPending}

	err := fsm.Transition(order.OrderType, order, OrderStatusDeposited)
	if err != nil {
		t.Fatalf("Transition failed: %v", err)
	}
	if order.Status != OrderStatusDeposited {
		t.Errorf("order.Status = %d, want %d", order.Status, OrderStatusDeposited)
	}
}

func TestTransition_InvalidReturnsError(t *testing.T) {
	fsm := NewOrderFSM()
	order := &StorageOrder{OrderType: OrderTypeDeliveryIn, Status: OrderStatusCompleted}

	err := fsm.Transition(order.OrderType, order, OrderStatusPending)
	if err == nil {
		t.Error("expected error for invalid transition, got nil")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./app/storage/internal/biz/... -run TestCanTransition -v
```

Expected: FAIL — `NewOrderFSM` undefined

- [ ] **Step 3: Write FSM implementation**

```go
package biz

import (
	"fmt"

	"github.com/go-kratos/kratos/v2/errors"
)

var (
	ErrInvalidTransition = errors.Forbidden("INVALID_TRANSITION", "非法状态转移")
	ErrUnknownOrderType  = errors.Forbidden("UNKNOWN_ORDER_TYPE", "未知订单类型")
)

type OrderFSM struct {
	transitions map[string]map[int32][]int32
}

func NewOrderFSM() *OrderFSM {
	return &OrderFSM{
		transitions: map[string]map[int32][]int32{
			OrderTypeDeliveryIn: {
				OrderStatusPending:   {OrderStatusDeposited},
				OrderStatusDeposited: {OrderStatusCompleted, OrderStatusTimeout},
				OrderStatusTimeout:   {OrderStatusCompleted, OrderStatusCleared},
			},
			OrderTypeDeliveryOut: {
				OrderStatusPending:   {OrderStatusDeposited},
				OrderStatusDeposited: {OrderStatusCompleted},
			},
			OrderTypeStorage: {
				OrderStatusPending:   {OrderStatusStoring},
				OrderStatusStoring:   {OrderStatusCompleted, OrderStatusTimeout, OrderStatusStoring},
				OrderStatusTimeout:   {OrderStatusCompleted, OrderStatusCleared},
			},
		},
	}
}

func (f *OrderFSM) CanTransition(orderType string, from, to int32) bool {
	states, ok := f.transitions[orderType]
	if !ok {
		return false
	}
	allowed, ok := states[from]
	if !ok {
		return false
	}
	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

func (f *OrderFSM) Transition(orderType string, order *StorageOrder, to int32) error {
	if !f.CanTransition(orderType, order.Status, to) {
		return fmt.Errorf("%w: %s %d→%d", ErrInvalidTransition, orderType, order.Status, to)
	}
	order.Status = to
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./app/storage/internal/biz/... -run "TestCanTransition|TestTransition" -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add app/storage/internal/biz/order_fsm.go app/storage/internal/biz/order_fsm_test.go
git commit -m "feat(storage): add OrderFSM — pure logic state machine with tests"
```

---

### Task 6: PricingEngine

**Files:**
- Create: `app/storage/internal/biz/pricing_engine.go`
- Create: `app/storage/internal/biz/pricing_engine_test.go`

- [ ] **Step 1: Write the pricing test**

```go
package biz

import (
	"context"
	"testing"
	"time"
)

func TestCalculateFee_FreePeriod(t *testing.T) {
	engine := NewPricingEngine(&mockPricingRepo{})
	rule := &PricingRule{FreeHours: 24, PricePerHour: 100, MaxFee: 5000}
	fee, err := engine.CalculateFee(context.Background(), rule, 0)
	if err != nil {
		t.Fatal(err)
	}
	if fee != 0 {
		t.Errorf("fee = %d, want 0 within free period", fee)
	}
}

func TestCalculateFee_Overtime(t *testing.T) {
	engine := NewPricingEngine(&mockPricingRepo{})
	rule := &PricingRule{FreeHours: 24, PricePerHour: 100, MaxFee: 5000}
	fee, err := engine.CalculateFee(context.Background(), rule, 180) // 3 hours overtime
	if err != nil {
		t.Fatal(err)
	}
	if fee != 300 {
		t.Errorf("fee = %d, want 300 (3h * 100)", fee)
	}
}

func TestCalculateFee_MaxFeeCap(t *testing.T) {
	engine := NewPricingEngine(&mockPricingRepo{})
	rule := &PricingRule{FreeHours: 0, PricePerHour: 100, MaxFee: 500}
	fee, err := engine.CalculateFee(context.Background(), rule, 6000) // 100 hours
	if err != nil {
		t.Fatal(err)
	}
	if fee != 500 {
		t.Errorf("fee = %d, want 500 (capped at MaxFee)", fee)
	}
}

// mockPricingRepo for testing
type mockPricingRepo struct{}

func (m *mockPricingRepo) FindByTenantAndType(ctx context.Context, tenantID int64, ruleType string, cellType int32) (*PricingRule, error) {
	return nil, nil
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
go test ./app/storage/internal/biz/... -run TestCalculateFee -v
```

- [ ] **Step 3: Write PricingEngine implementation**

```go
package biz

import (
	"context"
	"math"

	"github.com/go-kratos/kratos/v2/log"
)

type PricingRepo interface {
	FindByTenantAndType(ctx context.Context, tenantID int64, ruleType string, cellType int32) (*PricingRule, error)
}

type PricingEngine struct {
	pricingRepo PricingRepo
	log         *log.Helper
}

func NewPricingEngine(pricingRepo PricingRepo) *PricingEngine {
	return &PricingEngine{pricingRepo: pricingRepo, log: log.NewHelper(log.DefaultLogger)}
}

func (e *PricingEngine) MatchRule(ctx context.Context, tenantID int64, ruleType string, cellType int32) (*PricingRule, error) {
	return e.pricingRepo.FindByTenantAndType(ctx, tenantID, ruleType, cellType)
}

// CalculateFee calculates overtime fee based on pricing rule and overtime minutes.
// Returns fee in cents (分).
func (e *PricingEngine) CalculateFee(ctx context.Context, rule *PricingRule, overtimeMinutes int) (int32, error) {
	if rule == nil || overtimeMinutes <= 0 {
		return 0, nil
	}

	// Round up to hours
	overtimeHours := int(math.Ceil(float64(overtimeMinutes) / 60.0))
	if overtimeHours <= 0 {
		return 0, nil
	}

	fee := int32(overtimeHours) * rule.PricePerHour

	// Cap at MaxFee
	if rule.MaxFee > 0 && fee > rule.MaxFee {
		fee = rule.MaxFee
	}

	return fee, nil
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./app/storage/internal/biz/... -run TestCalculateFee -v
```

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add app/storage/internal/biz/pricing_engine.go app/storage/internal/biz/pricing_engine_test.go
git commit -m "feat(storage): add PricingEngine — overtime fee calculation with tests"
```

---

### Task 7: PickupCodeManager

**Files:**
- Create: `app/storage/internal/biz/pickup_code.go`
- Create: `app/storage/internal/biz/pickup_code_test.go`

- [ ] **Step 1: Write pickup code tests**

```go
package biz

import (
	"context"
	"testing"

	"shared-device-saas/pkg/redis"

	"github.com/go-kratos/kratos/v2/log"
	"github.com/redis/go-redis/v9"
)

func TestGeneratePickupCode_Format(t *testing.T) {
	mgr := NewPickupCodeManager(testRedisClient(), log.DefaultLogger)
	code, err := mgr.Generate(context.Background(), 1, 100, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(code) != 6 {
		t.Errorf("code length = %d, want 6", len(code))
	}
}

func TestVerifyPickupCode_NotFound(t *testing.T) {
	mgr := NewPickupCodeManager(testRedisClient(), log.DefaultLogger)
	_, err := mgr.Verify(context.Background(), 999, "000000")
	if err == nil {
		t.Error("expected error for non-existent code, got nil")
	}
}

func testRedisClient() *redis.Client {
	// Integration test — requires running Redis
	// In unit tests, use a mock
	return nil
}
```

> **Note:** Full integration tests require a running Redis. For CI, use miniredis or skip with `testing.Short()`.

- [ ] **Step 2: Write PickupCodeManager implementation**

```go
package biz

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"shared-device-saas/pkg/redis"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

var (
	ErrPickupCodeNotFound = errors.NotFound("PICKUP_CODE_NOT_FOUND", "取件码无效或已过期")
	ErrPickupCodeGenerate = errors.InternalServer("PICKUP_CODE_GENERATE_FAILED", "取件码生成失败，请重试")
)

const (
	PickupCodeKeyPrefix = "storage:pickup:"
	PickupCodeTTL       = 72 * time.Hour
	PickupCodeOrderKey  = "storage:pickup:order:" // orderID → code reverse lookup
)

type PickupCodeManager struct {
	redis *redis.Client
	log   *log.Helper
}

func NewPickupCodeManager(redisClient *redis.Client, logger log.Logger) *PickupCodeManager {
	return &PickupCodeManager{redis: redisClient, log: log.NewHelper(logger)}
}

func (m *PickupCodeManager) Generate(ctx context.Context, tenantID, cabinetID, orderID int64) (string, error) {
	key := fmt.Sprintf("%s%d:%d", PickupCodeKeyPrefix, tenantID, cabinetID)
	orderKey := fmt.Sprintf("%s%d", PickupCodeOrderKey, orderID)

	for i := 0; i < 10; i++ {
		code := fmt.Sprintf("%06d", rand.Intn(900000)+100000)

		// Check uniqueness within this cabinet
		exists, err := m.redis.GetClient().SIsMember(ctx, key, code).Result()
		if err != nil {
			return "", fmt.Errorf("check pickup code: %w", err)
		}
		if !exists {
			m.redis.GetClient().SAdd(ctx, key, code)
			m.redis.GetClient().Expire(ctx, key, PickupCodeTTL)
			// Store reverse mapping: code → orderID
			codeMapKey := fmt.Sprintf("%s%d:%s", PickupCodeKeyPrefix, tenantID, code)
			m.redis.Set(ctx, codeMapKey, fmt.Sprintf("%d", orderID), PickupCodeTTL)
			return code, nil
		}
	}
	return "", ErrPickupCodeGenerate
}

func (m *PickupCodeManager) Verify(ctx context.Context, tenantID int64, code string) (int64, error) {
	codeMapKey := fmt.Sprintf("%s%d:%s", PickupCodeKeyPrefix, tenantID, code)
	data, err := m.redis.Get(ctx, codeMapKey)
	if err != nil {
		return 0, fmt.Errorf("verify pickup code: %w", err)
	}
	if data == "" {
		return 0, ErrPickupCodeNotFound
	}
	var orderID int64
	fmt.Sscanf(data, "%d", &orderID)
	return orderID, nil
}

func (m *PickupCodeManager) Revoke(ctx context.Context, tenantID int64, cabinetID int64, code string) error {
	key := fmt.Sprintf("%s%d:%d", PickupCodeKeyPrefix, tenantID, cabinetID)
	m.redis.GetClient().SRem(ctx, key, code)
	codeMapKey := fmt.Sprintf("%s%d:%s", PickupCodeKeyPrefix, tenantID, code)
	return m.redis.Del(ctx, codeMapKey)
}
```

- [ ] **Step 3: Commit**

```bash
git add app/storage/internal/biz/pickup_code.go app/storage/internal/biz/pickup_code_test.go
git commit -m "feat(storage): add PickupCodeManager — 6-digit code with Redis dedup"
```

---

### Task 8: EventPublisher Interface + DeviceCommander

**Files:**
- Create: `app/storage/internal/biz/event_publisher.go`
- Create: `app/storage/internal/biz/device_commander.go`

- [ ] **Step 1: Write EventPublisher interface**

```go
package biz

import "context"

type StorageEvent struct {
	EventID   string `json:"event_id"`
	EventType string `json:"event_type"`
	TenantID  int64  `json:"tenant_id"`
	OrderNo   string `json:"order_no"`
	UserID    int64  `json:"user_id"`
	CabinetID int64  `json:"cabinet_id,omitempty"`
	CellID    int64  `json:"cell_id,omitempty"`
	Payload   string `json:"payload"`
	Timestamp int64  `json:"timestamp"`
}

type EventPublisher interface {
	PublishPickupReady(ctx context.Context, tenantID int64, orderNo string, userID int64, pickupCode string, cabinetName string) error
	PublishOrderTimeout(ctx context.Context, tenantID int64, orderNo string, userID int64, fee int32) error
	PublishOpenTimeout(ctx context.Context, tenantID int64, deviceSN string, cellID int64, orderNo string) error
}
```

- [ ] **Step 2: Write DeviceCommander**

```go
package biz

import (
	"context"
	"fmt"
	"time"

	pb "shared-device-saas/api/device/v1"

	"github.com/go-kratos/kratos/v2/log"
)

type SlotOverview struct {
	Online     bool
	TotalSlots int32
	FreeSlots  int32
	SlotStatus map[int32]string
}

type DeviceCommander struct {
	deviceClient pb.DeviceCommandServiceClient
	log          *log.Helper
}

func NewDeviceCommander(deviceClient pb.DeviceCommandServiceClient, logger log.Logger) *DeviceCommander {
	return &DeviceCommander{deviceClient: deviceClient, log: log.NewHelper(logger)}
}

func (dc *DeviceCommander) OpenCell(ctx context.Context, tenantID int64, deviceSN string, slotIndex int32, refOrderNo string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	reply, err := dc.deviceClient.OpenCell(ctx, &pb.OpenCellRequest{
		TenantId:    tenantID,
		DeviceSn:    deviceSN,
		SlotIndex:   slotIndex,
		TimeoutSec:  30,
		Operator:    "system",
		RefOrderNo:  refOrderNo,
	})
	if err != nil {
		return "", fmt.Errorf("gRPC open cell: %w", err)
	}
	if !reply.Ok {
		return "", fmt.Errorf("device rejected: %s", reply.Error)
	}
	return reply.MsgId, nil
}

func (dc *DeviceCommander) GetSlotStatus(ctx context.Context, tenantID int64, deviceSN string) (*SlotOverview, error) {
	reply, err := dc.deviceClient.GetDeviceSlotStatus(ctx, &pb.GetDeviceSlotStatusRequest{
		TenantId: tenantID,
		DeviceSn: deviceSN,
	})
	if err != nil {
		return nil, fmt.Errorf("gRPC get slot status: %w", err)
	}
	return &SlotOverview{
		Online:     reply.Online,
		TotalSlots: reply.TotalSlots,
		FreeSlots:  reply.FreeSlots,
		SlotStatus: reply.SlotStatus,
	}, nil
}

func (dc *DeviceCommander) ForceReleaseCell(ctx context.Context, tenantID int64, deviceSN string, slotIndex int32, operator, reason string) error {
	reply, err := dc.deviceClient.ForceReleaseCell(ctx, &pb.ForceReleaseCellRequest{
		TenantId:  tenantID,
		DeviceSn:  deviceSN,
		SlotIndex: slotIndex,
		Operator:  operator,
		Reason:    reason,
	})
	if err != nil {
		return fmt.Errorf("gRPC force release: %w", err)
	}
	if !reply.Ok {
		return fmt.Errorf("device rejected: %s", reply.Error)
	}
	return nil
}
```

- [ ] **Step 3: Commit**

```bash
git add app/storage/internal/biz/event_publisher.go app/storage/internal/biz/device_commander.go
git commit -m "feat(storage): add EventPublisher interface and DeviceCommander gRPC wrapper"
```

---

## Chunk 3: CellAllocator, Repo Interfaces, Timeout System

### Task 9: Repo Interfaces + CellAllocator

**Files:**
- Create: `app/storage/internal/biz/repos.go`
- Create: `app/storage/internal/biz/cell_allocator.go`
- Create: `app/storage/internal/biz/cell_allocator_test.go`

- [ ] **Step 1: Write repo interfaces**

```go
package biz

import (
	"context"
	"time"
)

type CellRepo interface {
	FindByID(ctx context.Context, id int64) (*Cell, error)
	FindByDeviceAndSlot(ctx context.Context, deviceSN string, slotIndex int32) (*Cell, error)
	AllocateForUpdate(ctx context.Context, cabinetID int64, cellType int32) (*Cell, error)
	UpdateStatus(ctx context.Context, id int64, status int32) error
	UpdateStatusCAS(ctx context.Context, id int64, expectedFrom, newStatus int32) (int64, error)
	UpdateOpenedAt(ctx context.Context, id int64, t time.Time) error
	UpdatePendingAction(ctx context.Context, id int64, action *int32, orderID *int64) error
	FindOpenTimeoutCells(ctx context.Context, threshold time.Duration) ([]*Cell, error)
	ListFreeByCabinet(ctx context.Context, cabinetID int64) ([]*Cell, error)
}

type OrderRepo interface {
	Create(ctx context.Context, order *StorageOrder) (*StorageOrder, error)
	GetByID(ctx context.Context, id int64) (*StorageOrder, error)
	GetByOrderNo(ctx context.Context, orderNo string) (*StorageOrder, error)
	UpdateStatusCAS(ctx context.Context, id int64, expectedFrom, newStatus int32) (int64, error)
	UpdateAmount(ctx context.Context, id int64, totalAmount int32) error
	UpdatePickedUp(ctx context.Context, id int64, status int32) error
	FindPossiblyTimeoutOrders(ctx context.Context, threshold time.Duration) ([]*StorageOrder, error)
	ListByUser(ctx context.Context, tenantID, userID int64, orderType string, page, pageSize int32) ([]*StorageOrder, int32, error)
}

type CabinetRepo interface {
	FindByID(ctx context.Context, id int64) (*Cabinet, error)
	ListByTenant(ctx context.Context, tenantID int64, status int32, page, pageSize int32) ([]*Cabinet, int32, error)
	GetFreeCellCount(ctx context.Context, cabinetID int64) (int32, error)
}
```

- [ ] **Step 2: Write CellAllocator tests**

```go
package biz

import (
	"context"
	"testing"
)

func TestCellAllocator_MarkOpening(t *testing.T) {
	// Unit test with mock repo
	allocator := NewCellAllocator(&mockCellRepo{})
	err := allocator.MarkOpening(context.Background(), 1, PendingActionDeposit, 100)
	if err != nil {
		t.Fatal(err)
	}
}

type mockCellRepo struct{}

func (m *mockCellRepo) FindByID(ctx context.Context, id int64) (*Cell, error)                        { return nil, nil }
func (m *mockCellRepo) FindByDeviceAndSlot(ctx context.Context, deviceSN string, slotIndex int32) (*Cell, error) {
	return nil, nil
}
func (m *mockCellRepo) AllocateForUpdate(ctx context.Context, cabinetID int64, cellType int32) (*Cell, error) {
	return &Cell{ID: 1, CabinetID: cabinetID, SlotIndex: 0, CellType: cellType, Status: CellStatusFree}, nil
}
func (m *mockCellRepo) UpdateStatus(ctx context.Context, id int64, status int32) error                 { return nil }
func (m *mockCellRepo) UpdateStatusCAS(ctx context.Context, id int64, expectedFrom, newStatus int32) (int64, error) {
	return 1, nil
}
func (m *mockCellRepo) UpdateOpenedAt(ctx context.Context, id int64, t interface{}) error              { return nil }
func (m *mockCellRepo) UpdatePendingAction(ctx context.Context, id int64, action *int32, orderID *int64) error {
	return nil
}
func (m *mockCellRepo) FindOpenTimeoutCells(ctx context.Context, threshold interface{}) ([]*Cell, error) {
	return nil, nil
}
func (m *mockCellRepo) ListFreeByCabinet(ctx context.Context, cabinetID int64) ([]*Cell, error)        { return nil, nil }
```

- [ ] **Step 3: Write CellAllocator implementation**

```go
package biz

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

var (
	ErrNoFreeCell    = errors.NotFound("NO_FREE_CELL", "无空闲格口")
	ErrCellNotFree   = errors.Conflict("CELL_NOT_FREE", "格口非空闲状态")
)

type CellAllocator struct {
	cellRepo CellRepo
	log      *log.Helper
}

func NewCellAllocator(cellRepo CellRepo, logger log.Logger) *CellAllocator {
	return &CellAllocator{cellRepo: cellRepo, log: log.NewHelper(logger)}
}

func (a *CellAllocator) AllocateCell(ctx context.Context, cabinetID int64, cellType int32) (*Cell, error) {
	cell, err := a.cellRepo.AllocateForUpdate(ctx, cabinetID, cellType)
	if err != nil {
		return nil, fmt.Errorf("allocate cell: %w", err)
	}
	if cell == nil {
		return nil, ErrNoFreeCell
	}
	return cell, nil
}

func (a *CellAllocator) ReleaseCell(ctx context.Context, cellID int64) error {
	affected, err := a.cellRepo.UpdateStatusCAS(ctx, cellID, CellStatusOccupied, CellStatusFree)
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrCellNotFree
	}
	return nil
}

func (a *CellAllocator) MarkOpening(ctx context.Context, cellID int64, pendingAction int32, orderID int64) error {
	action := int32(pendingAction)
	order := int64(orderID)
	if err := a.cellRepo.UpdatePendingAction(ctx, cellID, &action, &order); err != nil {
		return err
	}
	return a.cellRepo.UpdateStatus(ctx, cellID, CellStatusOpening)
}

func (a *CellAllocator) ConfirmOccupied(ctx context.Context, cellID int64) error {
	return a.cellRepo.UpdateStatus(ctx, cellID, CellStatusOccupied)
}

func (a *CellAllocator) ClearPendingAction(ctx context.Context, cellID int64) error {
	return a.cellRepo.UpdatePendingAction(ctx, cellID, nil, nil)
}
```

- [ ] **Step 4: Run tests**

```bash
go test ./app/storage/internal/biz/... -run TestCellAllocator -v
```

- [ ] **Step 5: Commit**

```bash
git add app/storage/internal/biz/repos.go app/storage/internal/biz/cell_allocator.go app/storage/internal/biz/cell_allocator_test.go
git commit -m "feat(storage): add repo interfaces and CellAllocator with pessimistic lock pattern"
```

---

### Task 10: TimeoutManager + TimeoutHandler

**Files:**
- Create: `app/storage/internal/biz/timeout_manager.go`
- Create: `app/storage/internal/biz/timeout_handler.go`

- [ ] **Step 1: Write TimeoutManager**

```go
package biz

import (
	"context"
	"sync"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

type TimeoutCallback func(ctx context.Context, id int64) error

type TimeoutManager struct {
	cellRepo        CellRepo
	deviceCommander *DeviceCommander
	cellAllocator   *CellAllocator
	eventPublisher  EventPublisher
	orderRepo       OrderRepo
	orderTimeoutCb  TimeoutCallback // → TimeoutHandler.HandleOrderTimeout
	log             *log.Helper

	mu     sync.Mutex
	timers map[int64]*time.Timer // key=orderID or cellID, prefixed by type
}

func NewTimeoutManager(
	cellRepo CellRepo,
	deviceCommander *DeviceCommander,
	cellAllocator *CellAllocator,
	eventPublisher EventPublisher,
	orderRepo OrderRepo,
	logger log.Logger,
) *TimeoutManager {
	return &TimeoutManager{
		cellRepo:       cellRepo,
		deviceCommander: deviceCommander,
		cellAllocator:  cellAllocator,
		eventPublisher: eventPublisher,
		orderRepo:      orderRepo,
		log:            log.NewHelper(logger),
		timers:         make(map[int64]*time.Timer),
	}
}

func (m *TimeoutManager) SetOrderTimeoutCallback(cb TimeoutCallback) {
	m.orderTimeoutCb = cb
}

func (m *TimeoutManager) RegisterOrderTimeout(orderID int64, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := orderID
	m.timers[key] = time.AfterFunc(duration, func() {
		ctx := context.Background()
		if m.orderTimeoutCb != nil {
			if err := m.orderTimeoutCb(ctx, orderID); err != nil {
				m.log.Errorf("order timeout callback failed: orderID=%d err=%v", orderID, err)
			}
		}
		delete(m.timers, key)
	})
}

func (m *TimeoutManager) RegisterOpenTimeout(cellID int64, duration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Negative key space for cell timeouts
	key := -cellID
	m.timers[key] = time.AfterFunc(duration, func() {
		ctx := context.Background()
		if err := m.HandleOpenTimeout(ctx, cellID); err != nil {
			m.log.Errorf("open timeout handler failed: cellID=%d err=%v", cellID, err)
		}
		delete(m.timers, key)
	})
}

func (m *TimeoutManager) HandleOpenTimeout(ctx context.Context, cellID int64) error {
	cell, err := m.cellRepo.FindByID(ctx, cellID)
	if err != nil || cell == nil {
		return err
	}

	// Force release via device service
	_ = m.deviceCommander.ForceReleaseCell(ctx, cell.TenantID, "", cell.SlotIndex, "system", "开门超时自动回退")

	// Assume item is inside → mark as occupied
	if err := m.cellAllocator.ConfirmOccupied(ctx, cellID); err != nil {
		return err
	}

	// Alert admin
	if m.eventPublisher != nil {
		_ = m.eventPublisher.PublishOpenTimeout(ctx, cell.TenantID, "", cellID, "")
	}
	return nil
}

func (m *TimeoutManager) StartDBPolling(ctx context.Context) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			// Phase 1: Scan potentially timed out orders (loose threshold)
			orders, err := m.orderRepo.FindPossiblyTimeoutOrders(ctx, 1*time.Hour)
			if err != nil {
				m.log.Errorf("timeout scan orders: %v", err)
				continue
			}
			for _, o := range orders {
				if m.orderTimeoutCb != nil {
					_ = m.orderTimeoutCb(ctx, o.ID)
				}
			}

			// Phase 2: Scan open-timeout cells (5 min threshold)
			cells, err := m.cellRepo.FindOpenTimeoutCells(ctx, 5*time.Minute)
			if err != nil {
				m.log.Errorf("timeout scan cells: %v", err)
				continue
			}
			for _, c := range cells {
				_ = m.HandleOpenTimeout(ctx, c.ID)
			}

		case <-ctx.Done():
			return
		}
	}
}

func (m *TimeoutManager) CancelTimer(id int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if t, ok := m.timers[id]; ok {
		t.Stop()
		delete(m.timers, id)
	}
}
```

- [ ] **Step 2: Write TimeoutHandler**

```go
package biz

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
)

type TimeoutHandler struct {
	deliveryIn  *DeliveryInUsecase
	deliveryOut *DeliveryOutUsecase
	storageUc   *StorageUsecase
	orderRepo   OrderRepo
	log         *log.Helper
}

func NewTimeoutHandler(
	deliveryIn *DeliveryInUsecase,
	deliveryOut *DeliveryOutUsecase,
	storageUc *StorageUsecase,
	orderRepo OrderRepo,
	logger log.Logger,
) *TimeoutHandler {
	return &TimeoutHandler{
		deliveryIn:  deliveryIn,
		deliveryOut: deliveryOut,
		storageUc:   storageUc,
		orderRepo:   orderRepo,
		log:         log.NewHelper(logger),
	}
}

func (h *TimeoutHandler) HandleOrderTimeout(ctx context.Context, orderID int64) error {
	order, err := h.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return err
	}
	if order == nil {
		return nil
	}

	switch order.OrderType {
	case OrderTypeDeliveryIn:
		return h.deliveryIn.OnOrderTimeout(ctx, order)
	case OrderTypeDeliveryOut:
		return h.deliveryOut.OnOrderTimeout(ctx, order)
	case OrderTypeStorage:
		return h.storageUc.OnOrderTimeout(ctx, order)
	default:
		h.log.Warnf("unknown order type for timeout: %s", order.OrderType)
		return nil
	}
}
```

- [ ] **Step 3: Commit**

```bash
git add app/storage/internal/biz/timeout_manager.go app/storage/internal/biz/timeout_handler.go
git commit -m "feat(storage): add TimeoutManager (Go timer + DB polling) and TimeoutHandler router"
```

---

## Chunk 4: Business Usecases

### Task 11: DeliveryInUsecase

**Files:**
- Create: `app/storage/internal/biz/delivery_in.go`

- [ ] **Step 1: Write DeliveryInUsecase**

```go
package biz

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

var (
	ErrOrderNotFound   = errors.NotFound("ORDER_NOT_FOUND", "订单不存在")
	ErrPickupCodeWrong = errors.Forbidden("PICKUP_CODE_WRONG", "取件码错误")
	ErrOrderNotPickable = errors.Forbidden("ORDER_NOT_PICKABLE", "订单状态不可取件")
	ErrPaymentRequired = errors.PaymentRequired("PAYMENT_REQUIRED", "需先支付超时费")
)

type DeliveryInUsecase struct {
	fsm           *OrderFSM
	allocator     *CellAllocator
	pricing       *PricingEngine
	pickup        *PickupCodeManager
	commander     *DeviceCommander
	publisher     EventPublisher
	timeoutMgr    *TimeoutManager
	orderRepo     OrderRepo
	cellRepo      CellRepo
	cabinetRepo   CabinetRepo
	log           *log.Helper
}

func NewDeliveryInUsecase(
	fsm *OrderFSM,
	allocator *CellAllocator,
	pricing *PricingEngine,
	pickup *PickupCodeManager,
	commander *DeviceCommander,
	publisher EventPublisher,
	timeoutMgr *TimeoutManager,
	orderRepo OrderRepo,
	cellRepo CellRepo,
	cabinetRepo CabinetRepo,
	logger log.Logger,
) *DeliveryInUsecase {
	return &DeliveryInUsecase{
		fsm: fsm, allocator: allocator, pricing: pricing,
		pickup: pickup, commander: commander, publisher: publisher,
		timeoutMgr: timeoutMgr, orderRepo: orderRepo, cellRepo: cellRepo,
		cabinetRepo: cabinetRepo, log: log.NewHelper(logger),
	}
}

// InitiateDelivery — sync entry point for courier delivery
func (uc *DeliveryInUsecase) InitiateDelivery(ctx context.Context, tenantID, userID, cabinetID int64, cellType int32, refOrderNo string) (*StorageOrder, error) {
	// 1. Allocate cell (pessimistic lock)
	cell, err := uc.allocator.AllocateCell(ctx, cabinetID, cellType)
	if err != nil {
		return nil, fmt.Errorf("allocate cell: %w", err)
	}

	// 2. Create order
	cabinet, _ := uc.cabinetRepo.FindByID(ctx, cabinetID)
	order := &StorageOrder{
		TenantID:  tenantID,
		OrderType: OrderTypeDeliveryIn,
		Status:    OrderStatusPending,
		UserID:    userID,
		CabinetID: cabinetID,
		CellID:    cell.ID,
		DeviceSN:  cabinet.DeviceSN,
		SlotIndex: cell.SlotIndex,
	}
	order, err = uc.orderRepo.Create(ctx, order)
	if err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}

	// 3. Mark cell as opening
	if err := uc.allocator.MarkOpening(ctx, cell.ID, PendingActionDeposit, order.ID); err != nil {
		return nil, fmt.Errorf("mark opening: %w", err)
	}

	// 4. Open cell via device service
	_, err = uc.commander.OpenCell(ctx, tenantID, cabinet.DeviceSN, cell.SlotIndex, order.OrderNo)
	if err != nil {
		// Rollback: release cell
		_ = uc.allocator.ReleaseCell(ctx, cell.ID)
		return nil, fmt.Errorf("open cell: %w", err)
	}

	return order, nil
}

// HandleDepositClosed — async callback when door closes after deposit
func (uc *DeliveryInUsecase) HandleDepositClosed(ctx context.Context, order *StorageOrder, cell *Cell) error {
	fromStatus := order.Status

	// 1. FSM transition: Pending → Deposited
	if err := uc.fsm.Transition(order.OrderType, order, OrderStatusDeposited); err != nil {
		return err
	}
	affected, err := uc.orderRepo.UpdateStatusCAS(ctx, order.ID, fromStatus, OrderStatusDeposited)
	if affected == 0 {
		return nil // Already handled
	}

	// 2. Confirm cell occupied
	_ = uc.allocator.ConfirmOccupied(ctx, cell.ID)
	_ = uc.allocator.ClearPendingAction(ctx, cell.ID)

	// 3. Generate pickup code
	code, err := uc.pickup.Generate(ctx, order.TenantID, order.CabinetID, order.ID)
	if err != nil {
		uc.log.Errorf("generate pickup code: %v", err)
	} else {
		order.PickupCode = &code
	}

	// 4. Set deposited_at
	now := time.Now()
	order.DepositedAt = &now

	// 5. Register order timeout timer (based on pricing rule free_hours)
	rule, _ := uc.pricing.MatchRule(ctx, order.TenantID, "storage_overtime", 0)
	freeHours := int32(24)
	if rule != nil {
		freeHours = rule.FreeHours
	}
	uc.timeoutMgr.RegisterOrderTimeout(order.ID, time.Duration(freeHours)*time.Hour)

	// 6. Publish notification
	if uc.publisher != nil {
		cabinet, _ := uc.cabinetRepo.FindByID(ctx, order.CabinetID)
		cabinetName := ""
		if cabinet != nil {
			cabinetName = cabinet.Name
		}
		_ = uc.publisher.PublishPickupReady(ctx, order.TenantID, order.OrderNo, order.UserID, code, cabinetName)
	}

	return nil
}

// HandlePickupClosed — async callback when door closes after pickup
func (uc *DeliveryInUsecase) HandlePickupClosed(ctx context.Context, order *StorageOrder, cell *Cell) error {
	fromStatus := order.Status
	targetStatus := OrderStatusCompleted

	if err := uc.fsm.Transition(order.OrderType, order, targetStatus); err != nil {
		return err
	}
	affected, err := uc.orderRepo.UpdateStatusCAS(ctx, order.ID, fromStatus, targetStatus)
	if err != nil || affected == 0 {
		return err
	}

	// Release cell
	_ = uc.allocator.ReleaseCell(ctx, cell.ID)
	_ = uc.allocator.ClearPendingAction(ctx, cell.ID)

	// Cancel timeout timer
	uc.timeoutMgr.CancelTimer(order.ID)

	// Revoke pickup code
	if order.PickupCode != nil {
		_ = uc.pickup.Revoke(ctx, order.TenantID, order.CabinetID, *order.PickupCode)
	}

	return nil
}

// OnOrderTimeout — called by TimeoutHandler
func (uc *DeliveryInUsecase) OnOrderTimeout(ctx context.Context, order *StorageOrder) error {
	fromStatus := order.Status
	if err := uc.fsm.Transition(order.OrderType, order, OrderStatusTimeout); err != nil {
		return err
	}
	affected, err := uc.orderRepo.UpdateStatusCAS(ctx, order.ID, fromStatus, OrderStatusTimeout)
	if affected == 0 {
		return nil // Already handled by timer
	}

	// Calculate overtime fee
	rule, _ := uc.pricing.MatchRule(ctx, order.TenantID, "storage_overtime", 0)
	if rule != nil && order.DepositedAt != nil {
		overtimeMinutes := int(time.Since(*order.DepositedAt).Minutes()) - int(rule.FreeHours)*60
		if overtimeMinutes > 0 {
			fee, _ := uc.pricing.CalculateFee(ctx, rule, overtimeMinutes)
			_ = uc.orderRepo.UpdateAmount(ctx, order.ID, fee)
		}
	}

	// Notify user
	if uc.publisher != nil {
		_ = uc.publisher.PublishOrderTimeout(ctx, order.TenantID, order.OrderNo, order.UserID, order.TotalAmount)
	}
	return nil
}
```

- [ ] **Step 2: Commit**

```bash
git add app/storage/internal/biz/delivery_in.go
git commit -m "feat(storage): add DeliveryInUsecase — deposit, pickup, timeout handling"
```

---

### Task 12: DeliveryOutUsecase + StorageUsecase

**Files:**
- Create: `app/storage/internal/biz/delivery_out.go`
- Create: `app/storage/internal/biz/storage_uc.go`

- [ ] **Step 1: Write DeliveryOutUsecase**

```go
package biz

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

type DeliveryOutUsecase struct {
	fsm         *OrderFSM
	allocator   *CellAllocator
	commander   *DeviceCommander
	orderRepo   OrderRepo
	cellRepo    CellRepo
	cabinetRepo CabinetRepo
	log         *log.Helper
}

func NewDeliveryOutUsecase(
	fsm *OrderFSM, allocator *CellAllocator, commander *DeviceCommander,
	orderRepo OrderRepo, cellRepo CellRepo, cabinetRepo CabinetRepo, logger log.Logger,
) *DeliveryOutUsecase {
	return &DeliveryOutUsecase{
		fsm: fsm, allocator: allocator, commander: commander,
		orderRepo: orderRepo, cellRepo: cellRepo, cabinetRepo: cabinetRepo,
		log: log.NewHelper(logger),
	}
}

func (uc *DeliveryOutUsecase) InitiateShipment(ctx context.Context, tenantID, userID, cabinetID int64, cellType int32) (*StorageOrder, error) {
	cell, err := uc.allocator.AllocateCell(ctx, cabinetID, cellType)
	if err != nil {
		return nil, fmt.Errorf("allocate cell: %w", err)
	}

	cabinet, _ := uc.cabinetRepo.FindByID(ctx, cabinetID)
	order := &StorageOrder{
		TenantID:  tenantID,
		OrderType: OrderTypeDeliveryOut,
		Status:    OrderStatusPending,
		UserID:    userID,
		CabinetID: cabinetID,
		CellID:    cell.ID,
		DeviceSN:  cabinet.DeviceSN,
		SlotIndex: cell.SlotIndex,
	}
	order, err = uc.orderRepo.Create(ctx, order)
	if err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}

	if err := uc.allocator.MarkOpening(ctx, cell.ID, PendingActionDeposit, order.ID); err != nil {
		return nil, err
	}

	_, err = uc.commander.OpenCell(ctx, tenantID, cabinet.DeviceSN, cell.SlotIndex, order.OrderNo)
	if err != nil {
		_ = uc.allocator.ReleaseCell(ctx, cell.ID)
		return nil, fmt.Errorf("open cell: %w", err)
	}

	return order, nil
}

func (uc *DeliveryOutUsecase) HandleDepositClosed(ctx context.Context, order *StorageOrder, cell *Cell) error {
	fromStatus := order.Status
	if err := uc.fsm.Transition(order.OrderType, order, OrderStatusDeposited); err != nil {
		return err
	}
	affected, _ := uc.orderRepo.UpdateStatusCAS(ctx, order.ID, fromStatus, OrderStatusDeposited)
	if affected == 0 {
		return nil
	}
	_ = uc.allocator.ConfirmOccupied(ctx, cell.ID)
	_ = uc.allocator.ClearPendingAction(ctx, cell.ID)
	now := time.Now()
	order.DepositedAt = &now
	return nil
}

func (uc *DeliveryOutUsecase) OnOrderTimeout(ctx context.Context, order *StorageOrder) error {
	// Delivery-out doesn't have timeout in current FSM
	return nil
}
```

- [ ] **Step 2: Write StorageUsecase**

```go
package biz

import (
	"context"
	"fmt"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

type StorageUsecase struct {
	fsm         *OrderFSM
	allocator   *CellAllocator
	pricing     *PricingEngine
	commander   *DeviceCommander
	publisher   EventPublisher
	timeoutMgr  *TimeoutManager
	orderRepo   OrderRepo
	cellRepo    CellRepo
	cabinetRepo CabinetRepo
	log         *log.Helper
}

func NewStorageUsecase(
	fsm *OrderFSM, allocator *CellAllocator, pricing *PricingEngine,
	commander *DeviceCommander, publisher EventPublisher, timeoutMgr *TimeoutManager,
	orderRepo OrderRepo, cellRepo CellRepo, cabinetRepo CabinetRepo, logger log.Logger,
) *StorageUsecase {
	return &StorageUsecase{
		fsm: fsm, allocator: allocator, pricing: pricing,
		commander: commander, publisher: publisher, timeoutMgr: timeoutMgr,
		orderRepo: orderRepo, cellRepo: cellRepo, cabinetRepo: cabinetRepo,
		log: log.NewHelper(logger),
	}
}

func (uc *StorageUsecase) InitiateStorage(ctx context.Context, tenantID, userID, cabinetID int64, cellType int32) (*StorageOrder, error) {
	cell, err := uc.allocator.AllocateCell(ctx, cabinetID, cellType)
	if err != nil {
		return nil, fmt.Errorf("allocate cell: %w", err)
	}

	cabinet, _ := uc.cabinetRepo.FindByID(ctx, cabinetID)
	order := &StorageOrder{
		TenantID:  tenantID,
		OrderType: OrderTypeStorage,
		Status:    OrderStatusPending,
		UserID:    userID,
		CabinetID: cabinetID,
		CellID:    cell.ID,
		DeviceSN:  cabinet.DeviceSN,
		SlotIndex: cell.SlotIndex,
	}
	order, err = uc.orderRepo.Create(ctx, order)
	if err != nil {
		return nil, fmt.Errorf("create order: %w", err)
	}

	if err := uc.allocator.MarkOpening(ctx, cell.ID, PendingActionStore, order.ID); err != nil {
		return nil, err
	}

	_, err = uc.commander.OpenCell(ctx, tenantID, cabinet.DeviceSN, cell.SlotIndex, order.OrderNo)
	if err != nil {
		_ = uc.allocator.ReleaseCell(ctx, cell.ID)
		return nil, fmt.Errorf("open cell: %w", err)
	}

	return order, nil
}

func (uc *StorageUsecase) HandleDepositClosed(ctx context.Context, order *StorageOrder, cell *Cell) error {
	fromStatus := order.Status
	if err := uc.fsm.Transition(order.OrderType, order, OrderStatusStoring); err != nil {
		return err
	}
	affected, _ := uc.orderRepo.UpdateStatusCAS(ctx, order.ID, fromStatus, OrderStatusStoring)
	if affected == 0 {
		return nil
	}
	_ = uc.allocator.ConfirmOccupied(ctx, cell.ID)
	_ = uc.allocator.ClearPendingAction(ctx, cell.ID)
	now := time.Now()
	order.DepositedAt = &now

	// Register timeout
	rule, _ := uc.pricing.MatchRule(ctx, order.TenantID, "storage_overtime", 0)
	freeHours := int32(24)
	if rule != nil {
		freeHours = rule.FreeHours
	}
	uc.timeoutMgr.RegisterOrderTimeout(order.ID, time.Duration(freeHours)*time.Hour)

	return nil
}

func (uc *StorageUsecase) HandleTempOpenClosed(ctx context.Context, order *StorageOrder, cell *Cell) error {
	// 12 → 12, no state change, just clear pending action
	_ = uc.allocator.ClearPendingAction(ctx, cell.ID)
	return nil
}

func (uc *StorageUsecase) HandleRetrieveClosed(ctx context.Context, order *StorageOrder, cell *Cell) error {
	fromStatus := order.Status
	if err := uc.fsm.Transition(order.OrderType, order, OrderStatusCompleted); err != nil {
		return err
	}
	affected, _ := uc.orderRepo.UpdateStatusCAS(ctx, order.ID, fromStatus, OrderStatusCompleted)
	if affected == 0 {
		return nil
	}
	_ = uc.allocator.ReleaseCell(ctx, cell.ID)
	_ = uc.allocator.ClearPendingAction(ctx, cell.ID)
	uc.timeoutMgr.CancelTimer(order.ID)
	return nil
}

func (uc *StorageUsecase) OnOrderTimeout(ctx context.Context, order *StorageOrder) error {
	fromStatus := order.Status
	if err := uc.fsm.Transition(order.OrderType, order, OrderStatusTimeout); err != nil {
		return err
	}
	affected, _ := uc.orderRepo.UpdateStatusCAS(ctx, order.ID, fromStatus, OrderStatusTimeout)
	if affected == 0 {
		return nil
	}

	rule, _ := uc.pricing.MatchRule(ctx, order.TenantID, "storage_overtime", 0)
	if rule != nil && order.DepositedAt != nil {
		overtimeMinutes := int(time.Since(*order.DepositedAt).Minutes()) - int(rule.FreeHours)*60
		if overtimeMinutes > 0 {
			fee, _ := uc.pricing.CalculateFee(ctx, rule, overtimeMinutes)
			_ = uc.orderRepo.UpdateAmount(ctx, order.ID, fee)
		}
	}

	if uc.publisher != nil {
		_ = uc.publisher.PublishOrderTimeout(ctx, order.TenantID, order.OrderNo, order.UserID, order.TotalAmount)
	}
	return nil
}
```

- [ ] **Step 3: Update biz.go Wire ProviderSet**

```go
package biz

import "github.com/google/wire"

var ProviderSet = wire.NewSet(
	NewOrderFSM,
	NewPricingEngine,
	NewPickupCodeManager,
	NewCellAllocator,
	NewDeviceCommander,
	NewTimeoutManager,
	NewTimeoutHandler,
	NewDeliveryInUsecase,
	NewDeliveryOutUsecase,
	NewStorageUsecase,
)
```

- [ ] **Step 4: Commit**

```bash
git add app/storage/internal/biz/delivery_out.go app/storage/internal/biz/storage_uc.go app/storage/internal/biz/biz.go
git commit -m "feat(storage): add DeliveryOutUsecase, StorageUsecase, and update Wire providers"
```

---

## Chunk 5: Data Layer, Service Layer, Server, Wire Assembly

### Task 13: Data Layer — Repos Implementation

**Files:**
- Modify: `app/storage/internal/data/data.go`
- Create: `app/storage/internal/data/cabinet_repo.go`
- Create: `app/storage/internal/data/cell_repo.go`
- Create: `app/storage/internal/data/order_repo.go`
- Create: `app/storage/internal/data/pricing_repo.go`

- [ ] **Step 1: Update data.go with MySQL + Redis initialization**

Follow existing `app/device/internal/data/` pattern for MySQL connection setup. Update `NewData` to initialize `*sql.DB` and Redis client.

- [ ] **Step 2: Implement CabinetRepo**

SQL queries: `FindByID`, `ListByTenant`, `GetFreeCellCount` (COUNT on cells where status=1).

- [ ] **Step 3: Implement CellRepo**

Key queries:
- `AllocateForUpdate`: `SELECT * FROM cells WHERE cabinet_id=? AND status=1 AND (cell_type=? OR cell_type IS NULL) ORDER BY slot_index LIMIT 1 FOR UPDATE`
- `UpdateStatusCAS`: `UPDATE cells SET status=? WHERE id=? AND status=?`
- `FindOpenTimeoutCells`: `SELECT * FROM cells WHERE status=3 AND opened_at < NOW() - INTERVAL ? MINUTE`
- `FindByDeviceAndSlot`: JOIN cabinets on cabinet_id to resolve device_sn

- [ ] **Step 4: Implement OrderRepo**

Key queries:
- `Create`: INSERT with generated order_no
- `UpdateStatusCAS`: `UPDATE storage_orders SET status=? WHERE id=? AND status=?`
- `FindPossiblyTimeoutOrders`: `SELECT * FROM storage_orders WHERE status IN (11,12) AND deposited_at < NOW() - INTERVAL ? HOUR`

- [ ] **Step 5: Implement PricingRepo**

Key query: `FindByTenantAndType`:
```sql
SELECT * FROM pricing_rules
WHERE tenant_id=? AND rule_type=? AND enabled=1
  AND (cell_type IS NULL OR cell_type=?)
  AND (effective_from IS NULL OR effective_from <= NOW())
  AND (effective_until IS NULL OR effective_until >= NOW())
ORDER BY priority DESC LIMIT 1
```

- [ ] **Step 6: Update data.go ProviderSet**

```go
var ProviderSet = wire.NewSet(
	NewData,
	NewCabinetRepo,
	NewCellRepo,
	NewOrderRepo,
	NewPricingRepo,
)
```

- [ ] **Step 7: Delete greeter.go files**

Remove `app/storage/internal/data/greeter.go` and `app/storage/internal/biz/greeter.go`.

- [ ] **Step 8: Commit**

```bash
git add app/storage/internal/data/
git commit -m "feat(storage): implement data layer — CabinetRepo, CellRepo, OrderRepo, PricingRepo"
```

---

### Task 14: Service Layer + Server + Callback

**Files:**
- Create: `app/storage/internal/service/storage_service.go`
- Modify: `app/storage/internal/server/http.go`
- Modify: `app/storage/internal/server/grpc.go`
- Create: `app/storage/internal/server/callback.go`
- Modify: `app/storage/internal/server/server.go`
- Modify: `app/storage/internal/service/service.go`

- [ ] **Step 1: Implement StorageService**

Wire the generated proto server interface to call biz usecases. Follow pattern from `app/device/internal/service/`.

- [ ] **Step 2: Implement StorageCallbackService (gRPC server)**

Register `StorageCallbackService` in the gRPC server. The `ReportDoorEvent` method:
1. If `door_closed=false` (opened): record `opened_at`, register open timeout timer
2. If `door_closed=true` (closed): look up cell by `(device_sn, slot_index)`, read `pending_action`, route to appropriate usecase method

- [ ] **Step 3: Update Wire providers**

```go
// service/service.go
var ProviderSet = wire.NewSet(NewStorageService, NewStorageCallbackService)

// server/server.go
var ProviderSet = wire.NewSet(NewGRPCServer, NewHTTPServer)
```

- [ ] **Step 4: Commit**

```bash
git add app/storage/internal/service/ app/storage/internal/server/
git commit -m "feat(storage): add service layer, HTTP/gRPC server, and callback handler"
```

---

### Task 15: Wire Assembly + Config + Cleanup

**Files:**
- Modify: `app/storage/cmd/storage/main.go`
- Modify: `app/storage/cmd/storage/wire.go`
- Modify: `app/storage/cmd/storage/wire_gen.go`
- Modify: `app/storage/configs/config.yaml`

- [ ] **Step 1: Update config.yaml**

Add device service gRPC address and RabbitMQ connection config to the storage service config.

- [ ] **Step 2: Update conf.proto if needed**

Add new config fields for device gRPC endpoint.

- [ ] **Step 3: Run wire generation**

```bash
cd app/storage/cmd/storage && wire
```

- [ ] **Step 4: Build and verify**

```bash
go build ./app/storage/...
```

Expected: builds successfully

- [ ] **Step 5: Commit**

```bash
git add app/storage/
git commit -m "feat(storage): complete Wire assembly and config for storage service"
```

---

### Task 16: Device Service — Add DeviceCommandService gRPC Server

**Files:**
- Create: `app/device/internal/service/device_command_service.go`
- Modify: `app/device/internal/server/grpc.go`

- [ ] **Step 1: Implement DeviceCommandService server**

Wire the proto-generated `DeviceCommandServiceServer` interface:
- `OpenCell`: Build MQTT command, publish to `{tenant_id}/device/locker/{device_sn}/command`, return msg_id
- `GetDeviceSlotStatus`: Read from Redis cache `device:slots:{tenant}:{device}`
- `ForceReleaseCell`: Publish force-release MQTT command

- [ ] **Step 2: Register in gRPC server**

- [ ] **Step 3: Commit**

```bash
git add app/device/internal/service/device_command_service.go app/device/internal/server/
git commit -m "feat(device): add DeviceCommandService gRPC server for locker operations"
```

---

## Implementation Notes

### Dependency Order

```
Task 1 (migrations)     ──┐
Task 2 (types)          ──┤── can run in parallel
Task 3 (storage proto)  ──┤
Task 4 (device proto)   ──┘
         ↓
Task 5 (FSM)            ──┐
Task 6 (Pricing)        ──┤── can run in parallel
Task 7 (PickupCode)     ──┤
Task 8 (EventPublisher/ ──┘
        Commander)
         ↓
Task 9 (Repos/Allocator)──┐
Task 10 (Timeout)       ──┘
         ↓
Task 11 (DeliveryIn)    ──┐
Task 12 (DeliveryOut +  ──┤── can run in parallel
         StorageUsecase) ─┘
         ↓
Task 13 (Data layer)    ── must be sequential
Task 14 (Service/Server)──
Task 15 (Wire assembly) ──
Task 16 (Device service)── independent
```

### Testing Strategy

- **Unit tests** (no external deps): FSM, PricingEngine (Tasks 5-6)
- **Integration tests** (need Redis): PickupCodeManager (Task 7)
- **E2E tests** (need MySQL + Redis + MQTT): Full flow through service layer (post-implementation)

### What's NOT in This Plan (Deferred)

- RabbitMQ EventPublisher implementation (use a no-op logger initially)
- Notification Worker service
- Payment integration with `pkg/payment/` (fee calculation is done, actual payment flow deferred)
- Temp open daily limit guard (business-level guard, add during storage usecase refinement)
- Admin dashboard API detailed implementation
- Monitoring/probing endpoints
