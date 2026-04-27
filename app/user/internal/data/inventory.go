package data

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"time"

	"shared-device-saas/app/user/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

// ========================================
// 数据库行结构
// ========================================

// cellRow 格口表行结构
type cellRow struct {
	ID             int64  `db:"id"`
	TenantID       int64  `db:"tenant_id"`
	CabinetID      int64  `db:"cabinet_id"`
	CellNo         string `db:"cell_no"`
	CellType       int32  `db:"cell_type"`
	Status         int32  `db:"status"`
	CurrentOrderID int64  `db:"current_order_id"`
	CabinetName    string `db:"cabinet_name"`   // JOIN 查询填充
	CabinetStatus  int32  `db:"cabinet_status"`  // JOIN 查询填充
}

// powerBankRow 充电宝表行结构
type powerBankRow struct {
	ID            int64  `db:"id"`
	TenantID      int64  `db:"tenant_id"`
	BankNo        string `db:"bank_no"`
	StationID     int64  `db:"station_id"`
	SlotNo        string `db:"slot_no"`
	BatteryLevel  int32  `db:"battery_level"`
	Status        int32  `db:"status"`
	ChargeCycles  int32  `db:"charge_cycles"`
	StationName   string `db:"station_name"`   // JOIN 查询填充
	StationStatus int32  `db:"station_status"` // JOIN 查询填充
}

// ========================================
// InventoryRepo 实现
// ========================================

type inventoryRepo struct {
	data *Data
	log  *log.Helper
}

// NewInventoryRepo 创建 InventoryRepo
func NewInventoryRepo(data *Data, logger log.Logger) biz.InventoryRepo {
	return &inventoryRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

// ========================================
// 快递柜格口：悲观锁分配
// ========================================

// AllocateCell 自动分配格口（按类型找空闲格口）
// 事务内流程：BEGIN → SELECT FOR UPDATE(锁格口+校验) → INSERT orders → UPDATE cells → COMMIT
func (r *inventoryRepo) AllocateCell(ctx context.Context, order *biz.Order, cabinetID int64, cellType int32) (*biz.InventoryAllocation, error) {
	db := r.data.GetSqlDB()
	if db == nil {
		return nil, fmt.Errorf("mysql not connected")
	}

	// 设置事务超时（3秒拿不到锁就放弃，避免用户无限等待）
	txctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	tx, err := db.BeginTx(txctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() // 事务未提交时自动回滚

	// Step 1: 悲观锁 — 锁住一个空闲格口行（排他锁）
	// 走主键索引，InnoDB 行锁，不会锁表
	var cell cellRow
	err = tx.QueryRowContext(txctx,
		`SELECT c.id, c.tenant_id, c.cabinet_id, c.cell_no, c.cell_type, c.status, c.current_order_id,
		        cab.name AS cabinet_name, cab.status AS cabinet_status
		 FROM cells c
		 INNER JOIN cabinets cab ON cab.id = c.cabinet_id
		 WHERE c.cabinet_id = ? AND c.cell_type = ? AND c.status = ?
		 LIMIT 1
		 FOR UPDATE`,
		cabinetID, cellType, biz.CellStatusAvailable,
	).Scan(
		&cell.ID, &cell.TenantID, &cell.CabinetID, &cell.CellNo, &cell.CellType,
		&cell.Status, &cell.CurrentOrderID, &cell.CabinetName, &cell.CabinetStatus,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, biz.ErrCellNotAvailable // "该类型格口已满"
		}
		return nil, fmt.Errorf("lock cell: %w", err)
	}

	// Step 1.5: 校验快递柜状态
	if cell.CabinetStatus != biz.DeviceStatusOnline {
		return nil, biz.ErrCabinetOffline
	}

	// Step 2: 创建订单
	now := time.Now().Unix()
	order.CreatedAt = now
	order.UpdatedAt = now

	result, err := tx.ExecContext(txctx,
		`INSERT INTO orders (tenant_id, user_id, order_no, source, order_type, status, total_amount, currency, payment_method, title, description, extra_json, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		order.TenantID, order.UserID, order.OrderNo, order.Source, order.OrderType, order.Status,
		order.TotalAmount, order.Currency, order.PaymentMethod, order.Title, order.Description, order.ExtraJSON,
		order.CreatedAt, order.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create order in tx: %w", err)
	}
	orderID, _ := result.LastInsertId()
	order.ID = strconv.FormatInt(orderID, 10)

	// Step 3: 扣减库存 — 更新格口状态为已占用
	// WHERE status = ? 是二次校验，防止锁和更新之间状态变化
	affected, err := tx.ExecContext(txctx,
		`UPDATE cells SET status = ?, current_order_id = ?, updated_at = ?
		 WHERE id = ? AND status = ?`,
		biz.CellStatusOccupied, orderID, now, cell.ID, biz.CellStatusAvailable,
	)
	if err != nil {
		return nil, fmt.Errorf("update cell status: %w", err)
	}
	rowsAffected, _ := affected.RowsAffected()
	if rowsAffected == 0 {
		// 二次校验失败 = 被别人抢了，回滚整个事务
		return nil, biz.ErrInventoryConflict
	}

	// Step 4: 更新快递柜可用格口数（冗余计数）
	tx.ExecContext(txctx,
		`UPDATE cabinets SET available_cells = available_cells - 1, updated_at = ?
		 WHERE id = ? AND available_cells > 0`,
		now, cabinetID,
	)

	// Step 5: 提交事务
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	r.log.Infof("AllocateCell: cellID=%d cellNo=%s orderID=%d", cell.ID, cell.CellNo, orderID)

	return &biz.InventoryAllocation{
		ResourceType: "cell",
		ResourceID:   cell.ID,
		ResourceNo:   cell.CellNo,
		DeviceID:     cell.CabinetID,
		DeviceName:   cell.CabinetName,
	}, nil
}

// AllocateCellByID 指定格口分配（用户扫码某个格子）
func (r *inventoryRepo) AllocateCellByID(ctx context.Context, order *biz.Order, cellID int64) (*biz.InventoryAllocation, error) {
	db := r.data.GetSqlDB()
	if db == nil {
		return nil, fmt.Errorf("mysql not connected")
	}

	txctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	tx, err := db.BeginTx(txctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Step 1: 悲观锁 — 锁住指定格口
	var cell cellRow
	err = tx.QueryRowContext(txctx,
		`SELECT c.id, c.tenant_id, c.cabinet_id, c.cell_no, c.cell_type, c.status, c.current_order_id,
		        cab.name AS cabinet_name, cab.status AS cabinet_status
		 FROM cells c
		 INNER JOIN cabinets cab ON cab.id = c.cabinet_id
		 WHERE c.id = ?
		 FOR UPDATE`,
		cellID,
	).Scan(
		&cell.ID, &cell.TenantID, &cell.CabinetID, &cell.CellNo, &cell.CellType,
		&cell.Status, &cell.CurrentOrderID, &cell.CabinetName, &cell.CabinetStatus,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, biz.ErrCellNotFound
		}
		return nil, fmt.Errorf("lock cell: %w", err)
	}

	// Step 1.5: 校验
	if cell.CabinetStatus != biz.DeviceStatusOnline {
		return nil, biz.ErrCabinetOffline
	}
	if cell.Status != biz.CellStatusAvailable {
		return nil, biz.ErrCellNotAvailable
	}

	// Step 2: 创建订单
	now := time.Now().Unix()
	order.CreatedAt = now
	order.UpdatedAt = now

	result, err := tx.ExecContext(txctx,
		`INSERT INTO orders (tenant_id, user_id, order_no, source, order_type, status, total_amount, currency, payment_method, title, description, extra_json, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		order.TenantID, order.UserID, order.OrderNo, order.Source, order.OrderType, order.Status,
		order.TotalAmount, order.Currency, order.PaymentMethod, order.Title, order.Description, order.ExtraJSON,
		order.CreatedAt, order.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create order in tx: %w", err)
	}
	orderID, _ := result.LastInsertId()
	order.ID = strconv.FormatInt(orderID, 10)

	// Step 3: 更新格口状态
	affected, err := tx.ExecContext(txctx,
		`UPDATE cells SET status = ?, current_order_id = ?, updated_at = ?
		 WHERE id = ? AND status = ?`,
		biz.CellStatusOccupied, orderID, now, cellID, biz.CellStatusAvailable,
	)
	if err != nil {
		return nil, fmt.Errorf("update cell status: %w", err)
	}
	rowsAffected, _ := affected.RowsAffected()
	if rowsAffected == 0 {
		return nil, biz.ErrInventoryConflict
	}

	// Step 4: 更新快递柜可用格口数
	tx.ExecContext(txctx,
		`UPDATE cabinets SET available_cells = available_cells - 1, updated_at = ?
		 WHERE id = ? AND available_cells > 0`,
		now, cell.CabinetID,
	)

	// Step 5: 提交事务
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	r.log.Infof("AllocateCellByID: cellID=%d cellNo=%s orderID=%d", cell.ID, cell.CellNo, orderID)

	return &biz.InventoryAllocation{
		ResourceType: "cell",
		ResourceID:   cell.ID,
		ResourceNo:   cell.CellNo,
		DeviceID:     cell.CabinetID,
		DeviceName:   cell.CabinetName,
	}, nil
}

// ========================================
// 充电宝：悲观锁借出
// ========================================

// RentPowerBank 从指定站点借出充电宝
// 事务内流程：BEGIN → SELECT FOR UPDATE(锁充电宝+校验) → INSERT orders → UPDATE power_banks → COMMIT
func (r *inventoryRepo) RentPowerBank(ctx context.Context, order *biz.Order, stationID int64) (*biz.InventoryAllocation, error) {
	db := r.data.GetSqlDB()
	if db == nil {
		return nil, fmt.Errorf("mysql not connected")
	}

	txctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	tx, err := db.BeginTx(txctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Step 1: 悲观锁 — 锁住一个在仓可借的充电宝
	var pb powerBankRow
	err = tx.QueryRowContext(txctx,
		`SELECT pb.id, pb.tenant_id, pb.bank_no, pb.station_id, pb.slot_no,
		        pb.battery_level, pb.status, pb.charge_cycles,
		        st.name AS station_name, st.status AS station_status
		 FROM power_banks pb
		 INNER JOIN stations st ON st.id = pb.station_id
		 WHERE pb.station_id = ? AND pb.status = ?
		 ORDER BY pb.battery_level DESC, pb.id ASC
		 LIMIT 1
		 FOR UPDATE`,
		stationID, biz.PowerBankStatusAvailable,
	).Scan(
		&pb.ID, &pb.TenantID, &pb.BankNo, &pb.StationID, &pb.SlotNo,
		&pb.BatteryLevel, &pb.Status, &pb.ChargeCycles,
		&pb.StationName, &pb.StationStatus,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, biz.ErrPowerBankNotAvail
		}
		return nil, fmt.Errorf("lock power bank: %w", err)
	}

	// Step 1.5: 校验站点状态
	if pb.StationStatus != biz.DeviceStatusOnline {
		return nil, biz.ErrStationOffline
	}

	// Step 2: 创建订单
	now := time.Now().Unix()
	order.CreatedAt = now
	order.UpdatedAt = now

	result, err := tx.ExecContext(txctx,
		`INSERT INTO orders (tenant_id, user_id, order_no, source, order_type, status, total_amount, currency, payment_method, title, description, extra_json, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		order.TenantID, order.UserID, order.OrderNo, order.Source, order.OrderType, order.Status,
		order.TotalAmount, order.Currency, order.PaymentMethod, order.Title, order.Description, order.ExtraJSON,
		order.CreatedAt, order.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("create order in tx: %w", err)
	}
	orderID, _ := result.LastInsertId()
	order.ID = strconv.FormatInt(orderID, 10)

	// Step 3: 更新充电宝状态为借出
	affected, err := tx.ExecContext(txctx,
		`UPDATE power_banks SET status = ?, station_id = NULL, slot_no = NULL, updated_at = ?
		 WHERE id = ? AND status = ?`,
		biz.PowerBankStatusRented, now, pb.ID, biz.PowerBankStatusAvailable,
	)
	if err != nil {
		return nil, fmt.Errorf("update power bank status: %w", err)
	}
	rowsAffected, _ := affected.RowsAffected()
	if rowsAffected == 0 {
		return nil, biz.ErrInventoryConflict
	}

	// Step 4: 更新站点可用数量
	tx.ExecContext(txctx,
		`UPDATE stations SET available_count = available_count - 1, return_slots = return_slots + 1, updated_at = ?
		 WHERE id = ? AND available_count > 0`,
		now, stationID,
	)

	// Step 5: 提交事务
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit tx: %w", err)
	}

	r.log.Infof("RentPowerBank: bankID=%d bankNo=%s orderID=%d", pb.ID, pb.BankNo, orderID)

	return &biz.InventoryAllocation{
		ResourceType: "power_bank",
		ResourceID:   pb.ID,
		ResourceNo:   pb.BankNo,
		DeviceID:     pb.StationID,
		DeviceName:   pb.StationName,
	}, nil
}

// ========================================
// 库存释放
// ========================================

// ReleaseCell 释放格口（订单取消/超时未取后调用）
func (r *inventoryRepo) ReleaseCell(ctx context.Context, cellID int64) error {
	db := r.data.GetSqlDB()
	if db == nil {
		return fmt.Errorf("mysql not connected")
	}

	now := time.Now().Unix()

	// 释放格口，同时清空关联订单
	result, err := db.ExecContext(ctx,
		`UPDATE cells SET status = ?, current_order_id = NULL, updated_at = ?
		 WHERE id = ? AND status = ?`,
		biz.CellStatusAvailable, now, cellID, biz.CellStatusOccupied,
	)
	if err != nil {
		return fmt.Errorf("release cell: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return biz.ErrCellNotFound
	}

	// 更新快递柜可用格口数
	db.ExecContext(ctx,
		`UPDATE cabinets SET available_cells = available_cells + 1, updated_at = ?
		 WHERE id = (SELECT cabinet_id FROM cells WHERE id = ?)`,
		now, cellID,
	)

	r.log.Infof("ReleaseCell: cellID=%d", cellID)
	return nil
}

// ReturnPowerBank 归还充电宝
func (r *inventoryRepo) ReturnPowerBank(ctx context.Context, powerBankID int64, stationID int64, slotNo string) error {
	db := r.data.GetSqlDB()
	if db == nil {
		return fmt.Errorf("mysql not connected")
	}

	now := time.Now().Unix()

	// 更新充电宝状态为在仓
	result, err := db.ExecContext(ctx,
		`UPDATE power_banks SET status = ?, station_id = ?, slot_no = ?, updated_at = ?
		 WHERE id = ? AND status = ?`,
		biz.PowerBankStatusAvailable, stationID, slotNo, now, powerBankID, biz.PowerBankStatusRented,
	)
	if err != nil {
		return fmt.Errorf("return power bank: %w", err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return biz.ErrPowerBankNotFound
	}

	// 更新站点可用数量（只有正常归还才更新站点计数）
	if stationID > 0 {
		db.ExecContext(ctx,
			`UPDATE stations SET available_count = available_count + 1, return_slots = return_slots - 1, updated_at = ?
			 WHERE id = ? AND return_slots > 0`,
			now, stationID,
		)
	}

	r.log.Infof("ReturnPowerBank: bankID=%d stationID=%d slotNo=%s", powerBankID, stationID, slotNo)
	return nil
}

// ========================================
// 库存查询（不加锁，纯查询）
// ========================================

// GetAvailableCells 查询快递柜可用格口
func (r *inventoryRepo) GetAvailableCells(ctx context.Context, cabinetID int64) ([]*biz.Cell, error) {
	db := r.data.GetSqlDB()
	if db == nil {
		return nil, fmt.Errorf("mysql not connected")
	}

	rows, err := db.QueryContext(ctx,
		`SELECT c.id, c.tenant_id, c.cabinet_id, c.cell_no, c.cell_type, c.status, c.current_order_id,
		        cab.name AS cabinet_name, cab.status AS cabinet_status
		 FROM cells c
		 INNER JOIN cabinets cab ON cab.id = c.cabinet_id
		 WHERE c.cabinet_id = ? AND c.status = ?
		 ORDER BY c.cell_type, c.cell_no`,
		cabinetID, biz.CellStatusAvailable,
	)
	if err != nil {
		return nil, fmt.Errorf("query available cells: %w", err)
	}
	defer rows.Close()

	var cells []*biz.Cell
	for rows.Next() {
		var row cellRow
		if err := rows.Scan(
			&row.ID, &row.TenantID, &row.CabinetID, &row.CellNo, &row.CellType,
			&row.Status, &row.CurrentOrderID, &row.CabinetName, &row.CabinetStatus,
		); err != nil {
			return nil, fmt.Errorf("scan cell: %w", err)
		}
		cells = append(cells, &biz.Cell{
			ID:             row.ID,
			TenantID:       row.TenantID,
			CabinetID:      row.CabinetID,
			CellNo:         row.CellNo,
			CellType:       row.CellType,
			Status:         row.Status,
			CurrentOrderID: row.CurrentOrderID,
			CabinetName:    row.CabinetName,
			CabinetStatus:  row.CabinetStatus,
		})
	}
	return cells, nil
}

// GetAvailablePowerBanks 查询站点可借充电宝
func (r *inventoryRepo) GetAvailablePowerBanks(ctx context.Context, stationID int64) ([]*biz.PowerBank, error) {
	db := r.data.GetSqlDB()
	if db == nil {
		return nil, fmt.Errorf("mysql not connected")
	}

	rows, err := db.QueryContext(ctx,
		`SELECT pb.id, pb.tenant_id, pb.bank_no, pb.station_id, pb.slot_no,
		        pb.battery_level, pb.status, pb.charge_cycles,
		        st.name AS station_name, st.status AS station_status
		 FROM power_banks pb
		 INNER JOIN stations st ON st.id = pb.station_id
		 WHERE pb.station_id = ? AND pb.status = ?
		 ORDER BY pb.battery_level DESC, pb.id ASC`,
		stationID, biz.PowerBankStatusAvailable,
	)
	if err != nil {
		return nil, fmt.Errorf("query available power banks: %w", err)
	}
	defer rows.Close()

	var banks []*biz.PowerBank
	for rows.Next() {
		var row powerBankRow
		if err := rows.Scan(
			&row.ID, &row.TenantID, &row.BankNo, &row.StationID, &row.SlotNo,
			&row.BatteryLevel, &row.Status, &row.ChargeCycles,
			&row.StationName, &row.StationStatus,
		); err != nil {
			return nil, fmt.Errorf("scan power bank: %w", err)
		}
		banks = append(banks, &biz.PowerBank{
			ID:            row.ID,
			TenantID:      row.TenantID,
			BankNo:        row.BankNo,
			StationID:     row.StationID,
			SlotNo:        row.SlotNo,
			BatteryLevel:  row.BatteryLevel,
			Status:        row.Status,
			ChargeCycles:  row.ChargeCycles,
			StationName:   row.StationName,
			StationStatus: row.StationStatus,
		})
	}
	return banks, nil
}
