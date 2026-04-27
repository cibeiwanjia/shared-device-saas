package data

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"shared-device-saas/app/user/internal/biz"

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
	data *Data
	log  *log.Helper
}

// NewStationRepo 创建 StationRepo（MySQL 实现）
func NewStationRepo(data *Data, logger log.Logger) biz.StationRepo {
	return &stationRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

// FindNearby 查找附近站点（MySQL ST_Distance_Sphere）
func (r *stationRepo) FindNearby(ctx context.Context, lat, lng float64, radius int, limit int) ([]*biz.Station, error) {
	db := r.data.GetSqlDB()
	if db == nil {
		return nil, fmt.Errorf("mysql not connected")
	}

	query := `SELECT id, name, address, lat, lng, province, city, district, adcode, amap_poi_id, status, distance
	          FROM (
	              SELECT id, name, address, lat, lng, province, city, district, adcode, amap_poi_id, status,
	                     CAST(ST_Distance_Sphere(POINT(?, ?), POINT(lng, lat)) AS UNSIGNED) AS distance
	              FROM stations
	              WHERE status = 1
	          ) AS t
	          WHERE distance <= ?
	          ORDER BY distance ASC
	          LIMIT ?`

	rows, err := db.QueryContext(ctx, query, lng, lat, radius, limit)
	if err != nil {
		r.log.Errorf("FindNearby query error: %v", err)
		return nil, fmt.Errorf("find nearby stations: %w", err)
	}
	defer rows.Close()

	var stations []*biz.Station
	for rows.Next() {
		var row stationRow
		var distance int32
		if err := rows.Scan(
			&row.ID, &row.Name, &row.Address, &row.Lat, &row.Lng,
			&row.Province, &row.City, &row.District, &row.Adcode, &row.AmapPoiID, &row.Status,
			&distance,
		); err != nil {
			r.log.Errorf("FindNearby scan error: %v", err)
			return nil, fmt.Errorf("scan station: %w", err)
		}

		s := rowToStation(&row)
		s.Distance = distance

		// 获取空闲柜格数和最低价格
		avail, total, minPrice := r.getGridStats(ctx, row.ID)
		s.AvailableGrids = avail
		s.TotalGrids = total
		s.MinPrice = minPrice

		stations = append(stations, s)
	}

	return stations, nil
}

// GetByID 根据 ID 获取站点
func (r *stationRepo) GetByID(ctx context.Context, id int64) (*biz.Station, error) {
	db := r.data.GetSqlDB()
	if db == nil {
		return nil, fmt.Errorf("mysql not connected")
	}

	var row stationRow
	err := db.QueryRowContext(ctx,
		`SELECT id, name, address, lat, lng, province, city, district, adcode, amap_poi_id, status
		 FROM stations WHERE id = ?`, id,
	).Scan(&row.ID, &row.Name, &row.Address, &row.Lat, &row.Lng,
		&row.Province, &row.City, &row.District, &row.Adcode, &row.AmapPoiID, &row.Status)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("station %d not found", id)
		}
		return nil, fmt.Errorf("get station: %w", err)
	}

	s := rowToStation(&row)
	avail, total, minPrice := r.getGridStats(ctx, id)
	s.AvailableGrids = avail
	s.TotalGrids = total
	s.MinPrice = minPrice

	return s, nil
}

// GetGridsByStationID 获取站点柜格列表
func (r *stationRepo) GetGridsByStationID(ctx context.Context, stationID int64) ([]*biz.Grid, error) {
	db := r.data.GetSqlDB()
	if db == nil {
		return nil, fmt.Errorf("mysql not connected")
	}

	rows, err := db.QueryContext(ctx,
		`SELECT id, station_id, grid_no, grid_type, status, price_per_hour, current_order_no
		 FROM station_grids WHERE station_id = ?
		 ORDER BY grid_no ASC`, stationID)
	if err != nil {
		return nil, fmt.Errorf("get grids: %w", err)
	}
	defer rows.Close()

	var grids []*biz.Grid
	for rows.Next() {
		var row gridRow
		if err := rows.Scan(&row.ID, &row.StationID, &row.GridNo, &row.GridType,
			&row.Status, &row.PricePerHour, &row.CurrentOrderNo); err != nil {
			return nil, fmt.Errorf("scan grid: %w", err)
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

// getGridStats 获取站点柜格统计（空闲数、总数、最低价）
func (r *stationRepo) getGridStats(ctx context.Context, stationID int64) (int32, int32, int32) {
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
		r.log.Warnf("getGridStats error: station=%d err=%v", stationID, err)
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

// parseIntToInt64 is kept for utility
var _ = strconv.ParseInt
