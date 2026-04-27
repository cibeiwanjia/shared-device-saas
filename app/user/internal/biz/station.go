package biz

import (
	"context"
	"strconv"

	"shared-device-saas/pkg/amap"

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
func NewStationUsecase(repo StationRepo, amapClient *amap.Client, logger log.Logger) *StationUsecase {
	return &StationUsecase{repo: repo, amapClient: amapClient, log: log.NewHelper(logger)}
}

// SearchNearby 搜索附近站点
func (uc *StationUsecase) SearchNearby(ctx context.Context, lng, lat string, radius, limit int) ([]*Station, int32, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 50 {
		limit = 50
	}
	if radius <= 0 {
		radius = 3000
	}

	latFloat, _ := strconv.ParseFloat(lat, 64)
	lngFloat, _ := strconv.ParseFloat(lng, 64)

	stations, err := uc.repo.FindNearby(ctx, latFloat, lngFloat, radius, limit)
	if err != nil {
		return nil, 0, err
	}

	totalCount := int32(len(stations))
	return stations, totalCount, nil
}

// GetStation 获取站点详情（含柜格）
func (uc *StationUsecase) GetStation(ctx context.Context, stationID int64) (*Station, []*Grid, error) {
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
