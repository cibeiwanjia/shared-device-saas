package biz

import (
	"context"
	"time"

	"shared-device-saas/pkg/auth"
	"shared-device-saas/pkg/userclient"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

// ============================================
// 快递员状态定义
// ============================================

const (
	CourierStatusPendingAudit = "pending_audit" // 待审核
	CourierStatusActive       = "active"        // 已通过（可接单）
	CourierStatusRejected     = "rejected"      // 已拒绝
	CourierStatusBanned       = "banned"        // 已封禁
	CourierStatusResign       = "resign"        // 已离职
)

// CourierStatusText 状态文本
func CourierStatusText(status string) string {
	switch status {
	case CourierStatusPendingAudit:
		return "待审核"
	case CourierStatusActive:
		return "已通过"
	case CourierStatusRejected:
		return "已拒绝"
	case CourierStatusBanned:
		return "已封禁"
	case CourierStatusResign:
		return "已离职"
	default:
		return "未知状态"
	}
}

// ============================================
// Courier 快递员实体
// ============================================

// Courier 快递员实体
type Courier struct {
	ID           string   // MongoDB ObjectID.Hex()
	UserID       string   // 关联 User 服务
	RealName     string   // 真实姓名
	IdCard       string   // 身份证号
	Phone        string   // 手机号（快递员联系方式）
	IntentAreas  []string // 意向商圈（申请时填写）
	Status       string   // 状态
	ZoneIds      []string // 已分配片区ID列表
	RejectReason string   // 拒绝原因

	// 统计指标（仅用于展示，不参与派单选择）
	PendingCount int // 当前待揽收订单数（用于快递员查看"我的待揽收"）

	CreateTime string // RFC3339
	UpdateTime string // RFC3339
}

// Zone 片区实体
type Zone struct {
	ID         string   // MongoDB ObjectID.Hex()
	Name       string   // 片区名称
	Street     string   // 街道名
	HouseStart int      // 门牌号起始
	HouseEnd   int      // 门牌号结束
	Keywords   []string // 大厦/小区关键字
	CourierId  string   // 关联快递员ID（片区快递员独占）
	Status     string   // 片区状态（active/disabled）

	CreateTime string // RFC3339
	UpdateTime string // RFC3339
}

// ZoneStatus 片区状态
const (
	ZoneStatusActive   = "active"   // 活跃
	ZoneStatusDisabled = "disabled" // 禁用
)

// ============================================
// Station 驿站实体
// ============================================

// Station 驿站实体
type Station struct {
	ID      string // MongoDB ObjectID.Hex()
	Name    string // 驿站名称
	Address string // 详细地址
	Lng     float64 // 经度
	Lat     float64 // 纬度
	Status  string // 状态：active/closed

	CreateTime string // RFC3339
	UpdateTime string // RFC3339
}

// StationStatus 驿站状态
const (
	StationStatusActive = "active" // 营业
	StationStatusClosed = "closed" // 停业
)

// StationStatusText 驿站状态文本
func StationStatusText(status string) string {
	switch status {
	case StationStatusActive:
		return "营业"
	case StationStatusClosed:
		return "停业"
	default:
		return "未知状态"
	}
}

// ============================================
// Cabinet 快递柜实体
// ============================================

// Cabinet 快递柜实体
type Cabinet struct {
	ID      string // MongoDB ObjectID.Hex()
	Name    string // 快递柜名称
	Address string // 详细地址
	Lng     float64 // 经度
	Lat     float64 // 纬度
	Status  string // 状态：online/offline/fault

	CreateTime string // RFC3339
	UpdateTime string // RFC3339
}

// CabinetStatus 快递柜状态
const (
	CabinetStatusOnline = "online" // 在线
	CabinetStatusOffline = "offline" // 离线
	CabinetStatusFault = "fault" // 故障
)

// CabinetStatusText 快递柜状态文本
func CabinetStatusText(status string) string {
	switch status {
	case CabinetStatusOnline:
		return "在线"
	case CabinetStatusOffline:
		return "离线"
	case CabinetStatusFault:
		return "故障"
	default:
		return "未知状态"
	}
}

// ============================================
// CabinetGrid 柜格实体
// ============================================

// CabinetGrid 柜格实体
type CabinetGrid struct {
	ID        string // MongoDB ObjectID.Hex()
	CabinetID string // 所属快递柜ID
	GridNo    string // 格口号（如 A1/B1）
	Size      string // 格口大小：small/medium/large
	Status    string // 状态：idle/occupied/fault
	OrderID   string // 占用订单ID（空闲时为空）

	CreateTime string // RFC3339
	UpdateTime string // RFC3339
}

// CabinetGridStatus 柜格状态
const (
	GridStatusIdle     = "idle"     // 空闲
	GridStatusOccupied = "occupied" // 已占用
	GridStatusFault    = "fault"    // 故障
)

// GridStatusText 柜格状态文本
func GridStatusText(status string) string {
	switch status {
	case GridStatusIdle:
		return "空闲"
	case GridStatusOccupied:
		return "已占用"
	case GridStatusFault:
		return "故障"
	default:
		return "未知状态"
	}
}

// ============================================
// CourierRepo 仓储接口
// ============================================

// CourierRepo 快递员仓储接口
type CourierRepo interface {
	Create(context.Context, *Courier) (*Courier, error)
	FindByID(context.Context, string) (*Courier, error)
	FindByUserID(context.Context, string) (*Courier, error)                        // 防止重复申请
	FindByStatus(context.Context, string, int32, int32) ([]*Courier, int64, error) // 查询待审核列表
	Update(context.Context, *Courier) (*Courier, error)
	UpdateZoneIds(context.Context, string, []string) error            // 更新片区关联
	ListAll(context.Context, int32, int32) ([]*Courier, int64, error) // 全量列表（管理后台）
}

// ZoneRepo 片区仓储接口
type ZoneRepo interface {
	Create(context.Context, *Zone) (*Zone, error)
	FindByID(context.Context, string) (*Zone, error)
	FindByIDs(context.Context, []string) ([]*Zone, error)                       // 批量查询片区详情
	FindByStreet(context.Context, string, int32, int32) ([]*Zone, int64, error) // 按街道查询
	ListAll(context.Context, int32, int32) ([]*Zone, int64, error)              // 全量列表
	Update(context.Context, *Zone) (*Zone, error)
	UpdateCourierId(context.Context, string, string) error // 更新快递员关联（单快递员）
}

// StationRepo 驿站仓储接口
type StationRepo interface {
	Create(context.Context, *Station) (*Station, error)
	FindByID(context.Context, string) (*Station, error)
	ListAll(context.Context, int32, int32) ([]*Station, int64, error)
	Update(context.Context, *Station) (*Station, error)
}

// CabinetRepo 快递柜仓储接口
type CabinetRepo interface {
	Create(context.Context, *Cabinet) (*Cabinet, error)
	FindByID(context.Context, string) (*Cabinet, error)
	ListAll(context.Context, int32, int32) ([]*Cabinet, int64, error)
	Update(context.Context, *Cabinet) (*Cabinet, error)
}

// CabinetGridRepo 柜格仓储接口
type CabinetGridRepo interface {
	Create(context.Context, *CabinetGrid) (*CabinetGrid, error)
	FindByID(context.Context, string) (*CabinetGrid, error)
	FindByCabinet(context.Context, string) ([]*CabinetGrid, error)      // 查询快递柜的所有柜格
	FindAvailable(context.Context, string) ([]*CabinetGrid, error)      // 查询快递柜的空闲柜格
	LockGrid(context.Context, string, string) (*CabinetGrid, error)     // 锁定柜格（原子操作）
	ReleaseGrid(context.Context, string) (*CabinetGrid, error)         // 释放柜格（原子操作）
	Update(context.Context, *CabinetGrid) (*CabinetGrid, error)
}

// ============================================
// CourierUsecase 快递员用例
// ============================================

// CourierUsecase 快递员用例
type CourierUsecase struct {
	courierRepo CourierRepo            // 快递员仓储
	zoneRepo    ZoneRepo               // 片区仓储
	userClient  *userclient.UserClient // 用户客户端
	log         *log.Helper            // 日志助手工具
}

// NewCourierUsecase 创建快递员用例
func NewCourierUsecase(courierRepo CourierRepo, zoneRepo ZoneRepo, userClient *userclient.UserClient, logger log.Logger) *CourierUsecase {
	return &CourierUsecase{
		courierRepo: courierRepo,           // 快递员仓储
		zoneRepo:    zoneRepo,              // 片区仓储
		userClient:  userClient,            // 用户客户端
		log:         log.NewHelper(logger), // 日志助手工具
	}
}

// ============================================
// 1. 用户申请成为快递员
// ============================================

func (uc *CourierUsecase) ApplyCourier(ctx context.Context, userID string, realName, idCard, phone string, intentAreas []string) (*Courier, error) {
	// 1. 防止重复申请
	existing, err := uc.courierRepo.FindByUserID(ctx, userID)
	if err == nil && existing != nil {
		// 已有申请记录，检查状态
		if existing.Status == CourierStatusPendingAudit {
			return nil, errors.New(400, "COURIER_ALREADY_APPLIED", "已提交申请，请等待审核")
		}
		if existing.Status == CourierStatusActive {
			return nil, errors.New(400, "COURIER_ALREADY_APPROVED", "您已是快递员")
		}
		// rejected/banned 可以重新申请
	}

	// 2. 字段校验
	if realName == "" || len(realName) < 2 || len(realName) > 20 {
		return nil, errors.New(400, "INVALID_REAL_NAME", "姓名长度不合规（2-20位）")
	}
	if !validateIdCard(idCard) {
		return nil, errors.New(400, "COURIER_INVALID_ID_CARD", "身份证号格式错误（需要18位）")
	}
	if !validatePhone(phone) {
		return nil, errors.New(400, "COURIER_INVALID_PHONE", "手机号格式错误")
	}
	if len(intentAreas) == 0 {
		return nil, errors.New(400, "EMPTY_INTENT_AREAS", "意向商圈列表不能为空")
	}

	// 3. 创建 Courier 实体
	now := time.Now().Format(time.RFC3339) // 格式化当前时间
	courier := &Courier{
		UserID:      userID,                    // 用户ID
		RealName:    realName,                  // 真实姓名
		IdCard:      idCard,                    // 身份证号
		Phone:       phone,                     // 手机号
		IntentAreas: intentAreas,               // 意向商圈列表
		Status:      CourierStatusPendingAudit, // 待审核
		ZoneIds:     []string{},                // 关联片区ID列表
		PendingCount: 0,                        // 待揽收统计初始为0
		CreateTime:  now,                       // 创建时间
		UpdateTime:  now,                       // 更新时间
	}

	// 4. 存入 MongoDB
	created, err := uc.courierRepo.Create(ctx, courier)
	if err != nil {
		uc.log.Errorf("Create courier failed: %v", err)
		return nil, err
	}

	uc.log.Infof("Courier applied: id=%s, userId=%s, status=%s", created.ID, userID, created.Status)
	return created, nil
}

// validateIdCard 校验身份证号（18位）
func validateIdCard(idCard string) bool {
	if len(idCard) != 18 {
		return false
	}
	// 简单校验：17位数字 + 1位数字或X
	for i := 0; i < 17; i++ {
		if idCard[i] < '0' || idCard[i] > '9' {
			return false
		}
	}
	last := idCard[17]
	return (last >= '0' && last <= '9') || last == 'X' || last == 'x'
}

// validatePhone 校验手机号（11位）
func validatePhone(phone string) bool {
	if len(phone) != 11 {
		return false
	}
	for i := 0; i < 11; i++ {
		if phone[i] < '0' || phone[i] > '9' {
			return false
		}
	}
	return phone[0] == '1'
}

// ============================================
// 2. 快递员查看自身信息
// ============================================

func (uc *CourierUsecase) GetCourierInfo(ctx context.Context, userID string) (*Courier, []*Zone, error) {
	// 1. 查询快递员记录
	courier, err := uc.courierRepo.FindByUserID(ctx, userID)
	if err != nil || courier == nil {
		return nil, nil, errors.New(404, "COURIER_NOT_FOUND", "快递员不存在")
	}

	// 2. 查询关联片区
	zones := []*Zone{}
	if len(courier.ZoneIds) > 0 {
		zones, err = uc.zoneRepo.FindByIDs(ctx, courier.ZoneIds)
		if err != nil {
			uc.log.Warnf("Find zones failed: %v", err)
		}
	}

	return courier, zones, nil
}

// ============================================
// 3. 管理员查询快递员列表
// ============================================

func (uc *CourierUsecase) GetCourierList(ctx context.Context, adminID string, status string, page, pageSize int32) ([]*Courier, int64, error) {
	// 1. 权限校验
	if err := uc.checkAdminPermission(ctx, adminID); err != nil {
		return nil, 0, err
	}

	// 2. 查询列表
	if status != "" {
		return uc.courierRepo.FindByStatus(ctx, status, page, pageSize)
	}
	return uc.courierRepo.ListAll(ctx, page, pageSize)
}

// ============================================
// 4. 管理员审核快递员申请
// ============================================

func (uc *CourierUsecase) ApproveCourier(ctx context.Context, adminID, courierID string, approved bool, reason string) (*Courier, error) {
	// 1. 权限校验
	if err := uc.checkAdminPermission(ctx, adminID); err != nil {
		return nil, err
	}

	// 2. 查询快递员记录
	courier, err := uc.courierRepo.FindByID(ctx, courierID)
	if err != nil || courier == nil {
		return nil, errors.New(404, "COURIER_NOT_FOUND", "快递员不存在")
	}

	// 3. 校验状态（只有 pending_audit 可审核）
	if courier.Status != CourierStatusPendingAudit {
		return nil, errors.New(400, "COURIER_ALREADY_APPROVED", "该申请已审核")
	}

	// 4. 更新状态
	if approved {
		courier.Status = CourierStatusActive
	} else {
		courier.Status = CourierStatusRejected
		courier.RejectReason = reason
	}
	courier.UpdateTime = time.Now().Format(time.RFC3339)

	// 5. 保存
	updated, err := uc.courierRepo.Update(ctx, courier)
	if err != nil {
		uc.log.Errorf("Update courier failed: %v", err)
		return nil, err
	}

	// 6. 通知用户（TODO: 调用消息服务）
	uc.log.Infof("Courier approved: id=%s, status=%s, by=%s", courierID, updated.Status, adminID)
	return updated, nil
}

// ============================================
// 5. 管理员分配片区
// ============================================

func (uc *CourierUsecase) AssignZone(ctx context.Context, adminID, courierID string, zoneIds []string) (*Courier, []*Zone, error) {
	// 1. 权限校验
	if err := uc.checkAdminPermission(ctx, adminID); err != nil {
		return nil, nil, err
	}

	// 2. 查询快递员记录
	courier, err := uc.courierRepo.FindByID(ctx, courierID)
	if err != nil || courier == nil {
		return nil, nil, errors.New(404, "COURIER_NOT_FOUND", "快递员不存在")
	}

	// 3. 校验快递员状态（只有 active 可分配片区）
	if courier.Status != CourierStatusActive {
		return nil, nil, errors.New(400, "COURIER_NOT_ACTIVE", "快递员状态不是active，无法分配片区")
	}

	// 4. 校验片区是否存在、未被封禁
	zones, err := uc.zoneRepo.FindByIDs(ctx, zoneIds)
	if err != nil || len(zones) == 0 {
		return nil, nil, errors.New(404, "COURIER_ZONE_NOT_FOUND", "片区不存在")
	}
	for _, zone := range zones {
		if zone.Status == ZoneStatusDisabled {
			return nil, nil, errors.New(400, "COURIER_ZONE_DISABLED", "片区已禁用")
		}
	}

	// 5. 更新快递员片区关联
	courier.ZoneIds = zoneIds
	courier.UpdateTime = time.Now().Format(time.RFC3339)
	updated, err := uc.courierRepo.Update(ctx, courier)
	if err != nil {
		uc.log.Errorf("Update courier zones failed: %v", err)
		return nil, nil, err
	}

	// 6. 批量更新片区快递员关联（覆盖赋值）
	for _, zoneId := range zoneIds {
		zone, err := uc.zoneRepo.FindByID(ctx, zoneId)
		if err != nil || zone == nil {
			continue
		}
		// 片区快递员独占：覆盖赋值
		zone.CourierId = courierID
		zone.UpdateTime = time.Now().Format(time.RFC3339)
		uc.zoneRepo.Update(ctx, zone)
	}

	uc.log.Infof("Zones assigned: courierId=%s, zones=%v, by=%s", courierID, zoneIds, adminID)
	return updated, zones, nil
}

// ============================================
// 6. 管理员创建片区
// ============================================

func (uc *CourierUsecase) CreateZone(ctx context.Context, adminID string, name, street string, houseStart, houseEnd int, keywords []string) (*Zone, error) {
	// 1. 权限校验
	if err := uc.checkAdminPermission(ctx, adminID); err != nil {
		return nil, err
	}

	// 2. 创建片区实体
	now := time.Now().Format(time.RFC3339)
	zone := &Zone{
		Name:       name,             // 片区名称
		Street:     street,           // 街道名称
		HouseStart: houseStart,       // 房号范围开始
		HouseEnd:   houseEnd,         // 房号范围结束
		Keywords:   keywords,         // 关键词列表
		CourierId:  "",               // 关联快递员ID（创建时为空，待分配）
		Status:     ZoneStatusActive, // 状态
		CreateTime: now,              // 创建时间
		UpdateTime: now,              // 更新时间
	}

	// 3. 存入 MongoDB
	created, err := uc.zoneRepo.Create(ctx, zone)
	if err != nil {
		uc.log.Errorf("Create zone failed: %v", err)
		return nil, err
	}

	uc.log.Infof("Zone created: id=%s, name=%s, street=%s", created.ID, name, street)
	return created, nil
}

// ============================================
// 7. 管理员查询片区列表
// ============================================

func (uc *CourierUsecase) GetZoneList(ctx context.Context, adminID string, street string, page, pageSize int32) ([]*Zone, int64, error) {
	// 1. 权限校验
	if err := uc.checkAdminPermission(ctx, adminID); err != nil {
		return nil, 0, err
	}

	// 2. 查询列表
	if street != "" {
		return uc.zoneRepo.FindByStreet(ctx, street, page, pageSize)
	}
	return uc.zoneRepo.ListAll(ctx, page, pageSize)
}

// ============================================
// 权限校验辅助方法
// ============================================

// checkAdminPermission 校验管理员权限
// 直接从 Context 获取角色（JWT 中间件已注入），不再调用 User 服务
func (uc *CourierUsecase) checkAdminPermission(ctx context.Context, userID string) error {
	// 直接从 Context 获取角色（JWT 中间件已注入）
	roles := auth.GetRoles(ctx)

	// 检查是否有 admin 角色
	for _, role := range roles {
		if role == "admin" {
			uc.log.Infof("Admin permission granted: userID=%s", userID)
			return nil // 有 admin 角色，允许访问
		}
	}

	// 没有 admin 角色，拒绝访问
	uc.log.Warnf("Admin permission denied: userID=%s, roles=%v", userID, roles)
	return errors.New(403, "COURIER_PERMISSION_DENIED", "无管理员权限")
}

// containsString 检查字符串是否在数组中
func containsString(arr []string, target string) bool {
	for _, s := range arr {
		if s == target {
			return true
		}
	}
	return false
}
