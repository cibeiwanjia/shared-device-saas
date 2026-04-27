package biz

import (
	"context"
	"fmt"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

// ========================================
// 库存校验错误定义
// ========================================

var (
	ErrCellNotFound      = errors.NotFound("CELL_NOT_FOUND", "格口不存在")
	ErrCellNotAvailable  = errors.BadRequest("CELL_NOT_AVAILABLE", "格口已被占用或不可用")
	ErrCabinetOffline    = errors.BadRequest("CABINET_OFFLINE", "快递柜当前离线")
	ErrPowerBankNotFound = errors.NotFound("POWER_BANK_NOT_FOUND", "充电宝不存在")
	ErrPowerBankNotAvail = errors.BadRequest("POWER_BANK_NOT_AVAILABLE", "该站点暂无可借充电宝")
	ErrStationOffline    = errors.BadRequest("STATION_OFFLINE", "站点当前离线")
	ErrInventoryConflict = errors.Conflict("INVENTORY_CONFLICT", "库存冲突，请重试")
)

// ========================================
// 格口（cell）状态常量
// ========================================

const (
	CellStatusAvailable int32 = 1 // 空闲
	CellStatusOccupied  int32 = 2 // 已占用
	CellStatusTimeout   int32 = 3 // 超时未取
	CellStatusFault     int32 = 4 // 故障
)

// 格口类型
const (
	CellTypeSmall  int32 = 1 // 小格口
	CellTypeMedium int32 = 2 // 中格口
	CellTypeLarge  int32 = 3 // 大格口
)

// ========================================
// 充电宝（power_bank）状态常量
// ========================================

const (
	PowerBankStatusAvailable int32 = 1 // 在仓可借
	PowerBankStatusRented    int32 = 2 // 借出
	PowerBankStatusFault     int32 = 3 // 故障
	PowerBankStatusRetired   int32 = 4 // 退役
)

// 快递柜/站点状态
const (
	DeviceStatusOnline  int32 = 1 // 在线
	DeviceStatusOffline int32 = 2 // 离线
	DeviceStatusMaint   int32 = 3 // 维护中
)

// ========================================
// 领域实体
// ========================================

// Cell 快递柜格口
type Cell struct {
	ID              int64
	TenantID        int64
	CabinetID       int64
	CellNo          string
	CellType        int32
	Status          int32
	CurrentOrderID  int64
	CabinetName     string // 冗余：所属柜名（查询时 JOIN 填充）
	CabinetStatus   int32  // 冗余：所属柜状态
}

// PowerBank 充电宝
type PowerBank struct {
	ID            int64
	TenantID      int64
	BankNo        string
	StationID     int64
	SlotNo        string
	BatteryLevel  int32
	Status        int32
	ChargeCycles  int32
	StationName   string // 冗余：所属站名
	StationStatus int32  // 冗余：所属站状态
}

// InventoryAllocation 库存分配结果
type InventoryAllocation struct {
	ResourceType string // "cell" 或 "power_bank"
	ResourceID   int64  // 格口ID 或 充电宝ID
	ResourceNo   string // 格口号 或 充电宝编号
	DeviceID     int64  // 所属柜ID 或 站点ID
	DeviceName   string // 柜名 或 站点名
}

// ========================================
// 仓储接口
// ========================================

// InventoryRepo 库存仓储接口
type InventoryRepo interface {
	// AllocateCell 悲观锁分配格口：锁行→校验→创建订单→更新状态（同一事务）
	AllocateCell(ctx context.Context, order *Order, cabinetID int64, cellType int32) (*InventoryAllocation, error)

	// AllocateCellByID 指定格口分配：用户扫码指定某个格口
	AllocateCellByID(ctx context.Context, order *Order, cellID int64) (*InventoryAllocation, error)

	// RentPowerBank 悲观锁借出充电宝：锁行→校验→创建订单→更新状态（同一事务）
	RentPowerBank(ctx context.Context, order *Order, stationID int64) (*InventoryAllocation, error)

	// ReleaseCell 释放格口（订单取消/超时未取后释放）
	ReleaseCell(ctx context.Context, cellID int64) error

	// ReturnPowerBank 归还充电宝
	ReturnPowerBank(ctx context.Context, powerBankID int64, stationID int64, slotNo string) error

	// GetAvailableCells 查询快递柜可用格口（不加锁，纯查询）
	GetAvailableCells(ctx context.Context, cabinetID int64) ([]*Cell, error)

	// GetAvailablePowerBanks 查询站点可借充电宝（不加锁，纯查询）
	GetAvailablePowerBanks(ctx context.Context, stationID int64) ([]*PowerBank, error)
}

// ========================================
// 业务逻辑
// ========================================

// InventoryUsecase 库存业务逻辑
type InventoryUsecase struct {
	repo InventoryRepo
	log  *log.Helper
}

// NewInventoryUsecase 创建 InventoryUsecase
func NewInventoryUsecase(repo InventoryRepo, logger log.Logger) *InventoryUsecase {
	return &InventoryUsecase{repo: repo, log: log.NewHelper(logger)}
}

// AllocateCellForOrder 快递柜存件：分配格口 + 创建订单（事务）
// 调用路径: OrderUsecase.CreateOrder → 当 source=4 时走此方法
func (uc *InventoryUsecase) AllocateCellForOrder(ctx context.Context, order *Order, cabinetID int64, cellType int32) (*InventoryAllocation, error) {
	if cabinetID <= 0 {
		return nil, errors.BadRequest("INVALID_CABINET", "请指定快递柜")
	}
	if cellType < CellTypeSmall || cellType > CellTypeLarge {
		return nil, errors.BadRequest("INVALID_CELL_TYPE", "格口类型无效")
	}

	alloc, err := uc.repo.AllocateCell(ctx, order, cabinetID, cellType)
	if err != nil {
		uc.log.Errorf("AllocateCellForOrder: cabinet=%d cellType=%d err=%v", cabinetID, cellType, err)
		return nil, fmt.Errorf("allocate cell: %w", err)
	}

	uc.log.Infof("Cell allocated: cellID=%d cellNo=%s orderNo=%s", alloc.ResourceID, alloc.ResourceNo, order.OrderNo)
	return alloc, nil
}

// AllocateSpecificCell 快递柜存件：指定格口分配（扫码模式）
func (uc *InventoryUsecase) AllocateSpecificCell(ctx context.Context, order *Order, cellID int64) (*InventoryAllocation, error) {
	if cellID <= 0 {
		return nil, errors.BadRequest("INVALID_CELL", "请指定格口")
	}

	alloc, err := uc.repo.AllocateCellByID(ctx, order, cellID)
	if err != nil {
		uc.log.Errorf("AllocateSpecificCell: cellID=%d err=%v", cellID, err)
		return nil, fmt.Errorf("allocate cell: %w", err)
	}

	uc.log.Infof("Specific cell allocated: cellID=%d cellNo=%s orderNo=%s", alloc.ResourceID, alloc.ResourceNo, order.OrderNo)
	return alloc, nil
}

// RentPowerBankForOrder 充电宝借出：分配充电宝 + 创建订单（事务）
// 调用路径: OrderUsecase.CreateOrder → 当 source=3 时走此方法
func (uc *InventoryUsecase) RentPowerBankForOrder(ctx context.Context, order *Order, stationID int64) (*InventoryAllocation, error) {
	if stationID <= 0 {
		return nil, errors.BadRequest("INVALID_STATION", "请指定充电宝站点")
	}

	alloc, err := uc.repo.RentPowerBank(ctx, order, stationID)
	if err != nil {
		uc.log.Errorf("RentPowerBankForOrder: station=%d err=%v", stationID, err)
		return nil, fmt.Errorf("rent power bank: %w", err)
	}

	uc.log.Infof("Power bank rented: bankID=%d bankNo=%s orderNo=%s", alloc.ResourceID, alloc.ResourceNo, order.OrderNo)
	return alloc, nil
}

// ReleaseInventory 释放库存（订单取消/退款时调用）
func (uc *InventoryUsecase) ReleaseInventory(ctx context.Context, source int32, resourceID int64) error {
	switch source {
	case 4: // 快递柜
		return uc.repo.ReleaseCell(ctx, resourceID)
	case 3: // 充电宝
		// 充电宝归还需要指定归还站点和槽位，这里用空值表示异常释放
		return uc.repo.ReturnPowerBank(ctx, resourceID, 0, "")
	default:
		return nil
	}
}
