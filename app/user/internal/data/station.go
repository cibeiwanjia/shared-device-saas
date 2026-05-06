package data

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"shared-device-saas/app/user/internal/biz"
	redisPkg "shared-device-saas/pkg/redis"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

// stationRow MySQL stations 表行结构
type stationRow struct {
	ID        int64
	Name      string
	Address   string
	Lat       float64
	Lng       float64
	Province  string
	City      string
	District  string
	Adcode    string
	AmapPoiID string `db:"amap_poi_id"`
	Status    int32
	CreatedAt int64
	UpdatedAt int64
}

// gridRow MySQL station_grids 表行结构
type gridRow struct {
	ID             int64
	StationID      int64  `db:"station_id"`
	GridNo         string `db:"grid_no"`
	GridType       string `db:"grid_type"`
	Status         int32
	PricePerHour   int32  `db:"price_per_hour"`
	CurrentOrderNo string `db:"current_order_no"`
	CreatedAt      int64
	UpdatedAt      int64
}

type stationRepo struct {
	data        *Data
	redisClient *redisPkg.Client
	log         *log.Helper
}

// NewStationRepo 创建 StationRepo（Redis GEO 空间索引 + MySQL 详情查询）
func NewStationRepo(data *Data, redisClient *redisPkg.Client, logger log.Logger) biz.StationRepo {
	return &stationRepo{
		data:        data,
		redisClient: redisClient,
		log:         log.NewHelper(logger),
	}
}

// FindNearby 查找附近站点
// 架构：Redis GEORADIUS（空间索引）→ 拿 ID 列表 → MySQL IN 查详情 + 批量统计
func (r *stationRepo) FindNearby(ctx context.Context, lat, lng float64, radius int, limit int) ([]*biz.Station, error) {
	db := r.data.GetSqlDB()
	if db == nil {
		return nil, errors.InternalServer("MYSQL_UNAVAILABLE", "mysql not connected")
	}

	// Step 1: 空间过滤 — Redis GEO 或 MySQL fallback
	stationIDs, distances, err := r.geoSpatialFilter(ctx, lng, lat, float64(radius), int64(limit))
	if err != nil {
		return nil, err
	}
	if len(stationIDs) == 0 {
		return nil, nil
	}

	// Step 2: MySQL IN 查询站点详情
	placeholders := make([]string, len(stationIDs))
	args := make([]interface{}, len(stationIDs))
	for i, id := range stationIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	rows, err := db.QueryContext(ctx,
		fmt.Sprintf(`SELECT id, name, address, lat, lng, province, city, district, adcode, amap_poi_id, status
		              FROM stations WHERE id IN (%s)`, strings.Join(placeholders, ",")),
		args...,
	)
	if err != nil {
		r.log.Errorf("FindNearby MySQL query error: %v", err)
		return nil, errors.InternalServer("DB_ERROR", "query stations failed")
	}
	defer rows.Close()

	stations := make([]*biz.Station, 0, len(stationIDs))
	var queriedIDs []int64
	for rows.Next() {
		var row stationRow
		if err := rows.Scan(&row.ID, &row.Name, &row.Address, &row.Lat, &row.Lng,
			&row.Province, &row.City, &row.District, &row.Adcode, &row.AmapPoiID, &row.Status,
		); err != nil {
			r.log.Errorf("FindNearby scan error: %v", err)
			return nil, errors.InternalServer("DB_ERROR", "scan station failed")
		}
		s := rowToStation(&row)
		if dist, ok := distances[row.ID]; ok {
			s.Distance = dist
		}
		stations = append(stations, s)
		queriedIDs = append(queriedIDs, row.ID)
	}

	// Step 3: 批量查询柜格统计（一次 SQL）
	if len(queriedIDs) > 0 {
		statsMap, err := r.batchGridStats(ctx, queriedIDs)
		if err != nil {
			r.log.Warnf("FindNearby batchGridStats error: %v", err)
		} else {
			for _, s := range stations {
				if st, ok := statsMap[s.ID]; ok {
					s.AvailableGrids = st.available
					s.TotalGrids = st.total
					s.MinPrice = st.minPrice
				}
			}
		}
	}

	// 按距离排序（Redis GEO 已排序，但 MySQL IN 打乱了顺序）
	sortByDistance(stations)

	return stations, nil
}

// geoSpatialFilter 空间过滤：优先 Redis GEO，不可用时降级 MySQL
func (r *stationRepo) geoSpatialFilter(ctx context.Context, lng, lat, radius float64, limit int64) ([]int64, map[int64]int32, error) {
	if r.redisClient != nil {
		return r.redisGeoFilter(ctx, lng, lat, radius, limit)
	}
	return r.mysqlGeoFilter(ctx, lng, lat, radius, limit)
}

// redisGeoFilter 使用 Redis GEORADIUS 做空间过滤
func (r *stationRepo) redisGeoFilter(ctx context.Context, lng, lat, radius float64, limit int64) ([]int64, map[int64]int32, error) {
	results, err := r.redisClient.GeoRadius(ctx, redisPkg.StationsGeoKey, lng, lat, radius, limit)
	if err != nil {
		r.log.Warnf("Redis GEORADIUS failed, falling back to MySQL: %v", err)
		return r.mysqlGeoFilter(ctx, lng, lat, radius, limit)
	}

	// Redis 返回空结果时降级 MySQL（可能 GEO key 尚未初始化）
	if len(results) == 0 {
		return r.mysqlGeoFilter(ctx, lng, lat, radius, limit)
	}

	ids := make([]int64, 0, len(results))
	distances := make(map[int64]int32, len(results))
	for _, loc := range results {
		id, err := strconv.ParseInt(loc.Name, 10, 64)
		if err != nil {
			continue
		}
		ids = append(ids, id)
		distances[id] = int32(loc.Dist)
	}
	return ids, distances, nil
}

// mysqlGeoFilter MySQL ST_Distance_Sphere 降级方案（不可走索引，仅少量站点可用）
func (r *stationRepo) mysqlGeoFilter(ctx context.Context, lng, lat, radius float64, limit int64) ([]int64, map[int64]int32, error) {
	db := r.data.GetSqlDB()
	if db == nil {
		return nil, nil, errors.InternalServer("MYSQL_UNAVAILABLE", "mysql not connected")
	}

	rows, err := db.QueryContext(ctx,
		`SELECT id, CAST(ST_Distance_Sphere(POINT(?, ?), POINT(lng, lat)) AS UNSIGNED) AS distance
		 FROM stations WHERE status = 1
		 HAVING distance <= ?
		 ORDER BY distance ASC LIMIT ?`,
		lng, lat, radius, limit,
	)
	if err != nil {
		return nil, nil, errors.InternalServer("DB_ERROR", "geo query failed")
	}
	defer rows.Close()

	ids := make([]int64, 0)
	distances := make(map[int64]int32)
	for rows.Next() {
		var id int64
		var dist int32
		if err := rows.Scan(&id, &dist); err != nil {
			continue
		}
		ids = append(ids, id)
		distances[id] = dist
	}
	return ids, distances, nil
}

// gridStats 柜格统计结果
type gridStats struct {
	available int32
	total     int32
	minPrice  int32
}

// batchGridStats 批量查询多个站点的柜格统计
func (r *stationRepo) batchGridStats(ctx context.Context, stationIDs []int64) (map[int64]gridStats, error) {
	db := r.data.GetSqlDB()
	if db == nil {
		return nil, fmt.Errorf("mysql not connected")
	}

	placeholders := make([]string, len(stationIDs))
	args := make([]interface{}, len(stationIDs))
	for i, id := range stationIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	rows, err := db.QueryContext(ctx,
		fmt.Sprintf(`SELECT station_id,
		                    COUNT(*) AS total,
		                    SUM(CASE WHEN status = 1 THEN 1 ELSE 0 END) AS available,
		                    MIN(price_per_hour) AS min_price
		             FROM station_grids
		             WHERE station_id IN (%s)
		             GROUP BY station_id`, strings.Join(placeholders, ",")),
		args...,
	)
	if err != nil {
		return nil, fmt.Errorf("batch grid stats: %w", err)
	}
	defer rows.Close()

	result := make(map[int64]gridStats, len(stationIDs))
	for rows.Next() {
		var stationID int64
		var total, available int32
		var minPrice sql.NullInt64
		if err := rows.Scan(&stationID, &total, &available, &minPrice); err != nil {
			return nil, fmt.Errorf("scan grid stats: %w", err)
		}
		mp := int32(0)
		if minPrice.Valid {
			mp = int32(minPrice.Int64)
		}
		result[stationID] = gridStats{available: available, total: total, minPrice: mp}
	}
	return result, nil
}

// GetByID 根据 ID 获取站点
func (r *stationRepo) GetByID(ctx context.Context, id int64) (*biz.Station, error) {
	db := r.data.GetSqlDB()
	if db == nil {
		return nil, errors.InternalServer("MYSQL_UNAVAILABLE", "mysql not connected")
	}

	var row stationRow
	err := db.QueryRowContext(ctx,
		`SELECT id, name, address, lat, lng, province, city, district, adcode, amap_poi_id, status
		 FROM stations WHERE id = ?`, id,
	).Scan(&row.ID, &row.Name, &row.Address, &row.Lat, &row.Lng,
		&row.Province, &row.City, &row.District, &row.Adcode, &row.AmapPoiID, &row.Status)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.NotFound("STATION_NOT_FOUND", fmt.Sprintf("station %d not found", id))
		}
		return nil, errors.InternalServer("DB_ERROR", fmt.Sprintf("get station: %v", err))
	}

	s := rowToStation(&row)

	// 单个站点用单次查询
	avail, total, minPrice := r.getSingleGridStats(ctx, id)
	s.AvailableGrids = avail
	s.TotalGrids = total
	s.MinPrice = minPrice

	return s, nil
}

// GetGridsByStationID 获取站点柜格列表
func (r *stationRepo) GetGridsByStationID(ctx context.Context, stationID int64) ([]*biz.Grid, error) {
	db := r.data.GetSqlDB()
	if db == nil {
		return nil, errors.InternalServer("MYSQL_UNAVAILABLE", "mysql not connected")
	}

	rows, err := db.QueryContext(ctx,
		`SELECT id, station_id, grid_no, grid_type, status, price_per_hour, current_order_no
		 FROM station_grids WHERE station_id = ?
		 ORDER BY grid_no ASC`, stationID)
	if err != nil {
		return nil, errors.InternalServer("DB_ERROR", fmt.Sprintf("get grids: %v", err))
	}
	defer rows.Close()

	var grids []*biz.Grid
	for rows.Next() {
		var row gridRow
		if err := rows.Scan(&row.ID, &row.StationID, &row.GridNo, &row.GridType,
			&row.Status, &row.PricePerHour, &row.CurrentOrderNo); err != nil {
			return nil, errors.InternalServer("DB_ERROR", fmt.Sprintf("scan grid: %v", err))
		}
		grids = append(grids, &biz.Grid{
			ID:             row.ID,
			StationID:      row.StationID,
			GridNo:         row.GridNo,
			GridType:       row.GridType,
			Status:         row.Status,
			PricePerHour:   row.PricePerHour,
			CurrentOrderNo: row.CurrentOrderNo,
		})
	}

	return grids, nil
}

// getSingleGridStats 单个站点柜格统计
func (r *stationRepo) getSingleGridStats(ctx context.Context, stationID int64) (int32, int32, int32) {
	db := r.data.GetSqlDB()
	if db == nil {
		return 0, 0, 0
	}

	var available, total int32
	var minPrice sql.NullInt64
	err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) AS total,
		        SUM(CASE WHEN status = 1 THEN 1 ELSE 0 END) AS available,
		        MIN(price_per_hour) AS min_price
		 FROM station_grids WHERE station_id = ?`, stationID,
	).Scan(&total, &available, &minPrice)

	if err != nil {
		r.log.Warnf("getSingleGridStats error: station=%d err=%v", stationID, err)
		return 0, 0, 0
	}

	mp := int32(0)
	if minPrice.Valid {
		mp = int32(minPrice.Int64)
	}

	return available, total, mp
}

// rowToStation 将 stationRow 转为 biz.Station
func rowToStation(row *stationRow) *biz.Station {
	return &biz.Station{
		ID:        row.ID,
		Name:      row.Name,
		Address:   row.Address,
		Lat:       row.Lat,
		Lng:       row.Lng,
		Province:  row.Province,
		City:      row.City,
		District:  row.District,
		Adcode:    row.Adcode,
		AmapPoiID: row.AmapPoiID,
		Status:    row.Status,
	}
}

// sortByDistance 按距离排序（Redis GEO 有顺序，但 MySQL IN 可能打乱）
func sortByDistance(stations []*biz.Station) {
	for i := 0; i < len(stations)-1; i++ {
		for j := i + 1; j < len(stations); j++ {
			if stations[i].Distance > stations[j].Distance {
				stations[i], stations[j] = stations[j], stations[i]
			}
		}
	}
}
