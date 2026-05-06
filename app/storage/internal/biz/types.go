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
	PendingActionDeposit  int32 = 1 // 等待投递/存入确认
	PendingActionPickup   int32 = 2 // 等待取件确认
	PendingActionTempOpen int32 = 3 // 临时开柜
	PendingActionStore    int32 = 4 // 等待寄存确认
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
	ID             int64
	TenantID       int64
	RuleType       string
	FreeHours      int32
	PricePerHour   int32
	PricePerDay    int32
	MaxFee         int32
	CellType       *int32
	Priority       int32
	EffectiveFrom  *time.Time
	EffectiveUntil *time.Time
	Enabled        bool
	CreatedAt      time.Time
	UpdatedAt      time.Time
}
