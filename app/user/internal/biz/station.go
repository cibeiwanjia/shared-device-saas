package biz

import (
	"context"
	"math"
	"strconv"

	"shared-device-saas/pkg/amap"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

// Station 站点实体
type Station struct {
	ID             int64
	Name           string
	Address        string
	Lat            float64
	Lng            float64
	Province       string
	City           string
	District       string
	Adcode         string
	AmapPoiID      string
	Status         int32
	Distance       int32 // 距离（米）
	AvailableGrids int32
	TotalGrids     int32
	MinPrice       int32
}

// Grid 柜格实体
type Grid struct {
	ID             int64
	StationID      int64
	GridNo         string
	GridType       string
	Status         int32
	PricePerHour   int32
	CurrentOrderNo string
}

// 站点相关错误
var (
	ErrInvalidLngLat  = errors.BadRequest("INVALID_LNGLAT", "经纬度参数无效或超出范围")
	ErrInvalidRadius  = errors.BadRequest("INVALID_RADIUS", "搜索半径无效")
	ErrInvalidLimit   = errors.BadRequest("INVALID_LIMIT", "分页参数无效")
	ErrStationNotFound = errors.NotFound("STATION_NOT_FOUND", "站点不存在")
)

// StationRepo 站点仓储接口
type StationRepo interface {
	FindNearby(ctx context.Context, lat, lng float64, radius int, limit int) ([]*Station, error)
	GetByID(ctx context.Context, id int64) (*Station, error)
	GetGridsByStationID(ctx context.Context, stationID int64) ([]*Grid, error)
}

// StationUsecase 站点业务逻辑
type StationUsecase struct {
	repo       StationRepo
	amapClient *amap.Client
	log        *log.Helper
}

// NewStationUsecase 创建 StationUsecase
// amapClient 可为 nil（高德 API 未配置时不影响本地站点搜索）
func NewStationUsecase(repo StationRepo, amapClient *amap.Client, logger log.Logger) *StationUsecase {
	return &StationUsecase{repo: repo, amapClient: amapClient, log: log.NewHelper(logger)}
}

// SearchNearby 搜索附近站点
func (uc *StationUsecase) SearchNearby(ctx context.Context, lngStr, latStr string, radius, limit int) ([]*Station, int32, error) {
	// 校验经纬度
	lng, lat, err := parseLngLat(lngStr, latStr)
	if err != nil {
		return nil, 0, err
	}

	// 校验并规范化参数
	if radius <= 0 || radius > 50000 {
		radius = 3000
	}
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}

	stations, err := uc.repo.FindNearby(ctx, lat, lng, radius, limit)
	if err != nil {
		return nil, 0, err
	}

	return stations, int32(len(stations)), nil
}

// GetStation 获取站点详情（含柜格）
func (uc *StationUsecase) GetStation(ctx context.Context, stationID int64) (*Station, []*Grid, error) {
	if stationID <= 0 {
		return nil, nil, ErrStationNotFound
	}

	station, err := uc.repo.GetByID(ctx, stationID)
	if err != nil {
		return nil, nil, err
	}

	grids, err := uc.repo.GetGridsByStationID(ctx, stationID)
	if err != nil {
		uc.log.Warnf("GetStation grids error: station=%d err=%v", stationID, err)
		grids = nil
	}

	return station, grids, nil
}

// parseLngLat 解析并校验经纬度字符串
func parseLngLat(lngStr, latStr string) (float64, float64, error) {
	lng, err := strconv.ParseFloat(lngStr, 64)
	if err != nil {
		return 0, 0, ErrInvalidLngLat
	}
	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		return 0, 0, ErrInvalidLngLat
	}
	if lng < -180 || lng > 180 || lat < -90 || lat > 90 {
		return 0, 0, ErrInvalidLngLat
	}
	if math.Abs(lng) < 1e-9 && math.Abs(lat) < 1e-9 {
		return 0, 0, ErrInvalidLngLat
	}
	return lng, lat, nil
}
