package biz

import (
	"context"
	"fmt"
	"math/rand"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-kratos/kratos/v2/log"
)

// ============================================
// 派单算法（简化版：片区快递员独占）
// ============================================

// DispatchService 派单服务
type DispatchService struct {
	courierRepo CourierRepo // 快递员仓储
	zoneRepo    ZoneRepo    // 片区仓储
	log         *log.Helper // 日志助手
}

// NewDispatchService 创建派单服务
func NewDispatchService(courierRepo CourierRepo, zoneRepo ZoneRepo, logger log.Logger) *DispatchService {
	return &DispatchService{
		courierRepo: courierRepo,
		zoneRepo:    zoneRepo,
		log:         log.NewHelper(logger),
	}
}

// DispatchResult 派单结果
type DispatchResult struct {
	CourierID    string // 分配的快递员ID
	CourierName  string // 快递员姓名
	ShortCode    string // 取件核验码（6位随机）
	AssignedTime string // 派单时间
	MatchLevel   int    // 匹配级别（1=门牌号区间，2=关键字，3=街道，0=无匹配）
}

// Dispatch 执行派单算法
// 简化逻辑：匹配片区后直接取片区快递员，不做负载均衡
// 返回值：派单结果，无匹配时返回 nil（需要人工分配）
func (ds *DispatchService) Dispatch(ctx context.Context, senderAddress string) (*DispatchResult, error) {
	// 1. 解析地址
	street, houseNumber, keywords := ds.ParseAddress(senderAddress)
	ds.log.Infof("Address parsed: street=%s, house=%d, keywords=%v", street, houseNumber, keywords)

	// 2. Level 1: 门牌号区间匹配
	courier := ds.matchByHouseNumber(ctx, street, houseNumber)
	if courier != nil {
		ds.log.Infof("Level 1 match: courierId=%s, street=%s, house=%d", courier.ID, street, houseNumber)
		return ds.createResult(courier, 1), nil
	}

	// 3. Level 2: 关键字模糊匹配
	courier = ds.matchByKeywords(ctx, keywords, street)
	if courier != nil {
		ds.log.Infof("Level 2 match: courierId=%s, keywords=%v", courier.ID, keywords)
		return ds.createResult(courier, 2), nil
	}

	// 4. Level 3: 街道匹配
	courier = ds.matchByStreet(ctx, street)
	if courier != nil {
		ds.log.Infof("Level 3 match: courierId=%s, street=%s", courier.ID, street)
		return ds.createResult(courier, 3), nil
	}

	// 5. 无匹配，需要人工分配
	ds.log.Warnf("No courier matched for address: %s", senderAddress)
	return nil, nil
}

// ============================================
// 地址解析
// ============================================

// ParseAddress 解析地址，提取街道、门牌号、关键字
func (ds *DispatchService) ParseAddress(address string) (street string, houseNumber int, keywords []string) {
	// 北京地址格式示例：
	// "朝阳区建国门外大街28号国贸中心A座"
	// "海淀区中关村大街1号银泰中心"

	// 1. 提取街道名
	streetRegex := regexp.MustCompile(`([^\d]+街[^\d]*|[^街]+路[^\d]*|[^路]+道[^\d]*)`)
	if match := streetRegex.FindString(address); match != "" {
		street = strings.TrimSpace(match)
	}

	// 2. 提取门牌号
	houseRegex := regexp.MustCompile(`(\d+)号`)
	if match := houseRegex.FindString(address); match != "" {
		numStr := strings.TrimSuffix(match, "号")
		houseNumber, _ = strconv.Atoi(numStr)
	}

	// 3. 提取关键字（大厦/小区名）
	keywordRegex := regexp.MustCompile(`(\d+号)?([^街路道号]+(?:大厦|中心|小区|公寓|商务楼|写字楼|花园|院))`)
	if matches := keywordRegex.FindAllStringSubmatch(address, -1); len(matches) > 0 {
		for _, m := range matches {
			if len(m) >= 3 && m[2] != "" {
				keyword := strings.TrimSpace(m[2])
				if keyword != street && keyword != "" {
					keywords = append(keywords, keyword)
				}
			}
		}
	}

	// 4. 补充：如果关键字未提取到，尝试提取地址中的标志性建筑名
	if len(keywords) == 0 {
		buildingRegex := regexp.MustCompile(`([^街路道]+(?:大厦|中心|小区|公寓|商务楼|写字楼|花园|院|号楼|单元))`)
		if match := buildingRegex.FindString(address); match != "" {
			keywords = append(keywords, strings.TrimSpace(match))
		}
	}

	return street, houseNumber, keywords
}

// ============================================
// Level 1: 门牌号区间匹配（简化版）
// ============================================

// matchByHouseNumber 门牌号区间匹配
// 简化逻辑：匹配片区后直接取片区快递员，不做负载均衡
func (ds *DispatchService) matchByHouseNumber(ctx context.Context, street string, houseNumber int) *Courier {
	if street == "" || houseNumber == 0 {
		return nil
	}

	// 查询覆盖该街道的所有片区
	zones, _, err := ds.zoneRepo.FindByStreet(ctx, street, 1, 100)
	if err != nil || len(zones) == 0 {
		return nil
	}

	// 遍历片区，找到门牌号区间匹配的，直接取片区快递员
	for _, zone := range zones {
		if zone.Status != ZoneStatusActive {
			continue
		}
		if houseNumber >= zone.HouseStart && houseNumber <= zone.HouseEnd {
			// 片区快递员独占：直接返回片区快递员
			return ds.getCourierFromZone(ctx, zone)
		}
	}

	return nil
}

// ============================================
// Level 2: 关键字模糊匹配（简化版）
// ============================================

// matchByKeywords 关键字模糊匹配
// 简化逻辑：匹配片区后直接取片区快递员，不做负载均衡
func (ds *DispatchService) matchByKeywords(ctx context.Context, keywords []string, fallbackStreet string) *Courier {
	if len(keywords) == 0 {
		return nil
	}

	// 查询所有活跃片区（不限制街道，关键字可能跨街道）
	zones, _, err := ds.zoneRepo.ListAll(ctx, 1, 100)
	if err != nil || len(zones) == 0 {
		return nil
	}

	// 遍历片区，检查关键字是否匹配
	for _, zone := range zones {
		if zone.Status != ZoneStatusActive {
			continue
		}
		// 检查片区关键字是否包含用户地址关键字
		for _, zoneKeyword := range zone.Keywords {
			for _, inputKeyword := range keywords {
				if strings.Contains(zoneKeyword, inputKeyword) || strings.Contains(inputKeyword, zoneKeyword) {
					// 片区快递员独占：直接返回片区快递员
					return ds.getCourierFromZone(ctx, zone)
				}
			}
		}
	}

	return nil
}

// ============================================
// Level 3: 街道匹配（简化版）
// ============================================

// matchByStreet 街道匹配
// 简化逻辑：匹配片区后直接取片区快递员，不做负载均衡
func (ds *DispatchService) matchByStreet(ctx context.Context, street string) *Courier {
	if street == "" {
		return nil
	}

	// 查询覆盖该街道的所有片区
	zones, _, err := ds.zoneRepo.FindByStreet(ctx, street, 1, 100)
	if err != nil || len(zones) == 0 {
		return nil
	}

	// 遍历活跃片区，直接取片区快递员
	for _, zone := range zones {
		if zone.Status == ZoneStatusActive {
			// 片区快递员独占：直接返回片区快递员
			return ds.getCourierFromZone(ctx, zone)
		}
	}

	return nil
}

// ============================================
// 片区快递员获取（核心简化逻辑）
// ============================================

// getCourierFromZone 从片区获取快递员
// 片区快递员独占模式：直接取片区快递员，验证活跃状态
func (ds *DispatchService) getCourierFromZone(ctx context.Context, zone *Zone) *Courier {
	// 片区无快递员
	if zone.CourierId == "" {
		ds.log.Warnf("Zone has no courier: zoneId=%s", zone.ID)
		return nil
	}

	// 查询快递员详情
	courier, err := ds.courierRepo.FindByID(ctx, zone.CourierId)
	if err != nil || courier == nil {
		ds.log.Warnf("Courier not found: courierId=%s", zone.CourierId)
		return nil
	}

	// 校验快递员状态（只有活跃状态可派单）
	if courier.Status != CourierStatusActive {
		ds.log.Warnf("Courier not active: courierId=%s, status=%s", courier.ID, courier.Status)
		return nil
	}

	return courier
}

// ============================================
// 核验码生成
// ============================================

// GenerateShortCode 生成6位随机取件码（寄件核验码）
func GenerateShortCode() string {
	return fmt.Sprintf("%06d", rand.Intn(1000000))
}

// ============================================
// 结果构建
// ============================================

// createResult 创建派单结果
func (ds *DispatchService) createResult(courier *Courier, matchLevel int) *DispatchResult {
	return &DispatchResult{
		CourierID:    courier.ID,                      // 快递员ID
		CourierName:  courier.RealName,                // 快递员姓名
		ShortCode:    GenerateShortCode(),             // 寄件核验码
		AssignedTime: time.Now().Format(time.RFC3339), // 派单时间
		MatchLevel:   matchLevel,                      // 匹配等级（1-2-3）
	}
}

// ============================================
// 区域覆盖检查
// ============================================

// CheckCoverage 检查地址是否有快递员覆盖
func (ds *DispatchService) CheckCoverage(ctx context.Context, address string) (hasCoverage bool, nearestStation string) {
	street, houseNumber, keywords := ds.ParseAddress(address)

	// Level 1: 门牌号区间匹配
	if street != "" && houseNumber != 0 {
		zones, _, _ := ds.zoneRepo.FindByStreet(ctx, street, 1, 100)
		for _, zone := range zones {
			if zone.Status == ZoneStatusActive && houseNumber >= zone.HouseStart && houseNumber <= zone.HouseEnd {
				if zone.CourierId != "" {
					return true, ""
				}
			}
		}
	}

	// Level 2: 关键字匹配
	if len(keywords) > 0 {
		zones, _, _ := ds.zoneRepo.ListAll(ctx, 1, 100)
		for _, zone := range zones {
			if zone.Status != ZoneStatusActive {
				continue
			}
			for _, zoneKeyword := range zone.Keywords {
				for _, inputKeyword := range keywords {
					if strings.Contains(zoneKeyword, inputKeyword) || strings.Contains(inputKeyword, zoneKeyword) {
						if zone.CourierId != "" {
							return true, ""
						}
					}
				}
			}
		}
	}

	// Level 3: 街道匹配
	if street != "" {
		zones, _, _ := ds.zoneRepo.FindByStreet(ctx, street, 1, 100)
		for _, zone := range zones {
			if zone.Status == ZoneStatusActive && zone.CourierId != "" {
				return true, ""
			}
		}
	}

	// 无覆盖
	// TODO: 返回最近驿站（需要驿站数据）
	return false, ""
}