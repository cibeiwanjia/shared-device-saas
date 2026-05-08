package data

import (
	"context"
	"errors"
	"time"

	"shared-device-saas/app/storage/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// ============================================
// CourierRepo 实现
// ============================================

type courierRepo struct {
	data *Data
	log  *log.Helper
}

// NewCourierRepo 创建快递员仓储
func NewCourierRepo(data *Data, logger log.Logger) biz.CourierRepo {
	return &courierRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

// Create 创建快递员
func (r *courierRepo) Create(ctx context.Context, courier *biz.Courier) (*biz.Courier, error) {
	collection := r.data.mongoDatabase.Collection("couriers")
	doc := courierToDoc(courier)
	result, err := collection.InsertOne(ctx, doc)
	if err != nil {
		r.log.Errorf("Insert courier failed: %v", err)
		return nil, err
	}
	courier.ID = result.InsertedID.(primitive.ObjectID).Hex()
	return courier, nil
}

// FindByID 根据ID查询
func (r *courierRepo) FindByID(ctx context.Context, id string) (*biz.Courier, error) {
	collection := r.data.mongoDatabase.Collection("couriers")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	filter := bson.M{"_id": objID}
	var doc courierDoc
	err = collection.FindOne(ctx, filter).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return docToCourier(&doc), nil
}

// FindByUserID 根据UserID查询（防止重复申请）
func (r *courierRepo) FindByUserID(ctx context.Context, userID string) (*biz.Courier, error) {
	collection := r.data.mongoDatabase.Collection("couriers")
	filter := bson.M{"userId": userID}
	var doc courierDoc
	err := collection.FindOne(ctx, filter).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return docToCourier(&doc), nil
}

// FindByStatus 根据状态查询列表
func (r *courierRepo) FindByStatus(ctx context.Context, status string, page, pageSize int32) ([]*biz.Courier, int64, error) {
	filter := bson.M{"status": status}
	return r.findList(ctx, filter, page, pageSize)
}

// ListAll 全量列表
func (r *courierRepo) ListAll(ctx context.Context, page, pageSize int32) ([]*biz.Courier, int64, error) {
	filter := bson.M{}
	return r.findList(ctx, filter, page, pageSize)
}

// findList 查询列表通用方法
func (r *courierRepo) findList(ctx context.Context, filter bson.M, page, pageSize int32) ([]*biz.Courier, int64, error) {
	collection := r.data.mongoDatabase.Collection("couriers")
	
	// 计算总数
	total, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	skip := int64((page - 1) * pageSize)
	opts := options.Find().
		SetSkip(skip).
		SetLimit(int64(pageSize)).
		SetSort(bson.D{{Key: "createTime", Value: -1}})

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var docs []courierDoc
	if err = cursor.All(ctx, &docs); err != nil {
		return nil, 0, err
	}

	couriers := make([]*biz.Courier, len(docs))
	for i, doc := range docs {
		couriers[i] = docToCourier(&doc)
	}
	return couriers, total, nil
}

// Update 更新快递员
func (r *courierRepo) Update(ctx context.Context, courier *biz.Courier) (*biz.Courier, error) {
	collection := r.data.mongoDatabase.Collection("couriers")
	objID, err := primitive.ObjectIDFromHex(courier.ID)
	if err != nil {
		return nil, err
	}
	filter := bson.M{"_id": objID}
	update := bson.M{"$set": courierToDoc(courier)}
	_, err = collection.UpdateOne(ctx, filter, update)
	if err != nil {
		r.log.Errorf("Update courier failed: %v", err)
		return nil, err
	}
	return courier, nil
}

// UpdateZoneIds 更新片区关联
func (r *courierRepo) UpdateZoneIds(ctx context.Context, courierID string, zoneIds []string) error {
	collection := r.data.mongoDatabase.Collection("couriers")
	objID, err := primitive.ObjectIDFromHex(courierID)
	if err != nil {
		return err
	}
	filter := bson.M{"_id": objID}
	update := bson.M{
		"$set": bson.M{
			"zoneIds":    zoneIds,
			"updateTime": time.Now().Format(time.RFC3339),
		},
	}
	_, err = collection.UpdateOne(ctx, filter, update)
	return err
}

// ============================================
// Courier 文档结构
// ============================================

type courierDoc struct {
	ID           primitive.ObjectID `bson:"_id"`
	UserID       string             `bson:"userId"`
	RealName     string             `bson:"realName"`
	IdCard       string             `bson:"idCard"`
	Phone        string             `bson:"phone"`
	IntentAreas  []string           `bson:"intentAreas"`
	Status       string             `bson:"status"`
	ZoneIds      []string           `bson:"zoneIds"`
	RejectReason string             `bson:"rejectReason"`
	PendingCount int                `bson:"pendingCount"` // 待揽收统计（仅展示用）
	CreateTime   string             `bson:"createTime"`
	UpdateTime   string             `bson:"updateTime"`
}

func courierToDoc(c *biz.Courier) bson.M {
	return bson.M{
		"userId":       c.UserID,
		"realName":     c.RealName,
		"idCard":       c.IdCard,
		"phone":        c.Phone,
		"intentAreas":  c.IntentAreas,
		"status":       c.Status,
		"zoneIds":      c.ZoneIds,
		"rejectReason": c.RejectReason,
		"pendingCount": c.PendingCount,
		"createTime":   c.CreateTime,
		"updateTime":   c.UpdateTime,
	}
}

func docToCourier(d *courierDoc) *biz.Courier {
	return &biz.Courier{
		ID:           d.ID.Hex(),
		UserID:       d.UserID,
		RealName:     d.RealName,
		IdCard:       d.IdCard,
		Phone:        d.Phone,
		IntentAreas:  d.IntentAreas,
		Status:       d.Status,
		ZoneIds:      d.ZoneIds,
		RejectReason: d.RejectReason,
		PendingCount: d.PendingCount,
		CreateTime:   d.CreateTime,
		UpdateTime:   d.UpdateTime,
	}
}

// ============================================
// ZoneRepo 实现
// ============================================

type zoneRepo struct {
	data *Data
	log  *log.Helper
}

// NewZoneRepo 创建片区仓储
func NewZoneRepo(data *Data, logger log.Logger) biz.ZoneRepo {
	return &zoneRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

// Create 创建片区
func (r *zoneRepo) Create(ctx context.Context, zone *biz.Zone) (*biz.Zone, error) {
	collection := r.data.mongoDatabase.Collection("zones")
	doc := zoneToDoc(zone)
	result, err := collection.InsertOne(ctx, doc)
	if err != nil {
		r.log.Errorf("Insert zone failed: %v", err)
		return nil, err
	}
	zone.ID = result.InsertedID.(primitive.ObjectID).Hex()
	return zone, nil
}

// FindByID 根据ID查询
func (r *zoneRepo) FindByID(ctx context.Context, id string) (*biz.Zone, error) {
	collection := r.data.mongoDatabase.Collection("zones")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	filter := bson.M{"_id": objID}
	var doc zoneDoc
	err = collection.FindOne(ctx, filter).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return docToZone(&doc), nil
}

// FindByIDs 批量查询片区详情
func (r *zoneRepo) FindByIDs(ctx context.Context, ids []string) ([]*biz.Zone, error) {
	collection := r.data.mongoDatabase.Collection("zones")
	objIDs := make([]primitive.ObjectID, len(ids))
	for i, id := range ids {
		objID, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			continue
		}
		objIDs[i] = objID
	}
	filter := bson.M{"_id": bson.M{"$in": objIDs}}
	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var docs []zoneDoc
	if err = cursor.All(ctx, &docs); err != nil {
		return nil, err
	}

	zones := make([]*biz.Zone, len(docs))
	for i, doc := range docs {
		zones[i] = docToZone(&doc)
	}
	return zones, nil
}

// FindByStreet 按街道查询
func (r *zoneRepo) FindByStreet(ctx context.Context, street string, page, pageSize int32) ([]*biz.Zone, int64, error) {
	filter := bson.M{"street": street}
	return r.findZoneList(ctx, filter, page, pageSize)
}

// ListAll 全量列表
func (r *zoneRepo) ListAll(ctx context.Context, page, pageSize int32) ([]*biz.Zone, int64, error) {
	filter := bson.M{}
	return r.findZoneList(ctx, filter, page, pageSize)
}

// findZoneList 查询片区列表通用方法
func (r *zoneRepo) findZoneList(ctx context.Context, filter bson.M, page, pageSize int32) ([]*biz.Zone, int64, error) {
	collection := r.data.mongoDatabase.Collection("zones")
	
	// 计算总数
	total, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	skip := int64((page - 1) * pageSize)
	opts := options.Find().
		SetSkip(skip).
		SetLimit(int64(pageSize)).
		SetSort(bson.D{{Key: "createTime", Value: -1}})

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var docs []zoneDoc
	if err = cursor.All(ctx, &docs); err != nil {
		return nil, 0, err
	}

	zones := make([]*biz.Zone, len(docs))
	for i, doc := range docs {
		zones[i] = docToZone(&doc)
	}
	return zones, total, nil
}

// Update 更新片区
func (r *zoneRepo) Update(ctx context.Context, zone *biz.Zone) (*biz.Zone, error) {
	collection := r.data.mongoDatabase.Collection("zones")
	objID, err := primitive.ObjectIDFromHex(zone.ID)
	if err != nil {
		return nil, err
	}
	filter := bson.M{"_id": objID}
	update := bson.M{"$set": zoneToDoc(zone)}
	_, err = collection.UpdateOne(ctx, filter, update)
	if err != nil {
		r.log.Errorf("Update zone failed: %v", err)
		return nil, err
	}
	return zone, nil
}

// UpdateCourierId 更新快递员关联（单快递员）
func (r *zoneRepo) UpdateCourierId(ctx context.Context, zoneID string, courierId string) error {
	collection := r.data.mongoDatabase.Collection("zones")
	objID, err := primitive.ObjectIDFromHex(zoneID)
	if err != nil {
		return err
	}
	filter := bson.M{"_id": objID}
	update := bson.M{
		"$set": bson.M{
			"courierId":  courierId,
			"updateTime": time.Now().Format(time.RFC3339),
		},
	}
	_, err = collection.UpdateOne(ctx, filter, update)
	return err
}

// ============================================
// Zone 文档结构
// ============================================

type zoneDoc struct {
	ID         primitive.ObjectID `bson:"_id"`
	Name       string             `bson:"name"`
	Street     string             `bson:"street"`
	HouseStart int                `bson:"houseStart"`
	HouseEnd   int                `bson:"houseEnd"`
	Keywords   []string           `bson:"keywords"`
	CourierId  string             `bson:"courierId"` // 单快递员ID
	Status     string             `bson:"status"`
	CreateTime string             `bson:"createTime"`
	UpdateTime string             `bson:"updateTime"`
}

func zoneToDoc(z *biz.Zone) bson.M {
	return bson.M{
		"name":       z.Name,
		"street":     z.Street,
		"houseStart": z.HouseStart,
		"houseEnd":   z.HouseEnd,
		"keywords":   z.Keywords,
		"courierId":  z.CourierId,
		"status":     z.Status,
		"createTime": z.CreateTime,
		"updateTime": z.UpdateTime,
	}
}

func docToZone(d *zoneDoc) *biz.Zone {
	return &biz.Zone{
		ID:         d.ID.Hex(),
		Name:       d.Name,
		Street:     d.Street,
		HouseStart: d.HouseStart,
		HouseEnd:   d.HouseEnd,
		Keywords:   d.Keywords,
		CourierId:  d.CourierId,
		Status:     d.Status,
		CreateTime: d.CreateTime,
		UpdateTime: d.UpdateTime,
	}
}

// ============================================
// StationRepo 实现
// ============================================

type stationDoc struct {
	ID        primitive.ObjectID `bson:"_id"`
	Name      string             `bson:"name"`
	Address   string             `bson:"address"`
	Lng       float64            `bson:"lng"`
	Lat       float64            `bson:"lat"`
	Status    string             `bson:"status"`
	CreateTime string             `bson:"createTime"`
	UpdateTime string             `bson:"updateTime"`
}

type stationRepo struct {
	data *Data
	log  *log.Helper
}

func NewStationRepo(data *Data, logger log.Logger) biz.StationRepo {
	return &stationRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *stationRepo) Create(ctx context.Context, station *biz.Station) (*biz.Station, error) {
	collection := r.data.mongoDatabase.Collection("stations")
	doc := stationToDoc(station)
	result, err := collection.InsertOne(ctx, doc)
	if err != nil {
		r.log.Errorf("Insert station failed: %v", err)
		return nil, err
	}
	station.ID = result.InsertedID.(primitive.ObjectID).Hex()
	return station, nil
}

func (r *stationRepo) FindByID(ctx context.Context, id string) (*biz.Station, error) {
	collection := r.data.mongoDatabase.Collection("stations")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var doc stationDoc
	err = collection.FindOne(ctx, bson.M{"_id": objID}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return docToStation(&doc), nil
}

func (r *stationRepo) ListAll(ctx context.Context, page, pageSize int32) ([]*biz.Station, int64, error) {
	collection := r.data.mongoDatabase.Collection("stations")
	skip := int64((page - 1) * pageSize)
	limit := int64(pageSize)

	findOptions := options.Find().SetSkip(skip).SetLimit(limit)
	cursor, err := collection.Find(ctx, bson.M{}, findOptions)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var docs []stationDoc
	if err = cursor.All(ctx, &docs); err != nil {
		return nil, 0, err
	}

	total, err := collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, 0, err
	}

	stations := make([]*biz.Station, len(docs))
	for i, doc := range docs {
		stations[i] = docToStation(&doc)
	}
	return stations, total, nil
}

func (r *stationRepo) Update(ctx context.Context, station *biz.Station) (*biz.Station, error) {
	collection := r.data.mongoDatabase.Collection("stations")
	objID, err := primitive.ObjectIDFromHex(station.ID)
	if err != nil {
		return nil, err
	}
	station.UpdateTime = time.Now().Format(time.RFC3339)
	update := bson.M{"$set": stationToDoc(station)}
	_, err = collection.UpdateByID(ctx, objID, update)
	if err != nil {
		return nil, err
	}
	return station, nil
}

func stationToDoc(s *biz.Station) bson.M {
	return bson.M{
		"name":      s.Name,
		"address":   s.Address,
		"lng":       s.Lng,
		"lat":       s.Lat,
		"status":    s.Status,
		"createTime": s.CreateTime,
		"updateTime": s.UpdateTime,
	}
}

func docToStation(d *stationDoc) *biz.Station {
	return &biz.Station{
		ID:        d.ID.Hex(),
		Name:      d.Name,
		Address:   d.Address,
		Lng:       d.Lng,
		Lat:       d.Lat,
		Status:    d.Status,
		CreateTime: d.CreateTime,
		UpdateTime: d.UpdateTime,
	}
}

// ============================================
// CabinetRepo 实现
// ============================================

type cabinetDoc struct {
	ID        primitive.ObjectID `bson:"_id"`
	Name      string             `bson:"name"`
	Address   string             `bson:"address"`
	Lng       float64            `bson:"lng"`
	Lat       float64            `bson:"lat"`
	Status    string             `bson:"status"`
	CreateTime string             `bson:"createTime"`
	UpdateTime string             `bson:"updateTime"`
}

type cabinetRepo struct {
	data *Data
	log  *log.Helper
}

func NewCabinetRepo(data *Data, logger log.Logger) biz.CabinetRepo {
	return &cabinetRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *cabinetRepo) Create(ctx context.Context, cabinet *biz.Cabinet) (*biz.Cabinet, error) {
	collection := r.data.mongoDatabase.Collection("cabinets")
	doc := cabinetToDoc(cabinet)
	result, err := collection.InsertOne(ctx, doc)
	if err != nil {
		r.log.Errorf("Insert cabinet failed: %v", err)
		return nil, err
	}
	cabinet.ID = result.InsertedID.(primitive.ObjectID).Hex()
	return cabinet, nil
}

func (r *cabinetRepo) FindByID(ctx context.Context, id string) (*biz.Cabinet, error) {
	collection := r.data.mongoDatabase.Collection("cabinets")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var doc cabinetDoc
	err = collection.FindOne(ctx, bson.M{"_id": objID}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return docToCabinet(&doc), nil
}

func (r *cabinetRepo) ListAll(ctx context.Context, page, pageSize int32) ([]*biz.Cabinet, int64, error) {
	collection := r.data.mongoDatabase.Collection("cabinets")
	skip := int64((page - 1) * pageSize)
	limit := int64(pageSize)

	findOptions := options.Find().SetSkip(skip).SetLimit(limit)
	cursor, err := collection.Find(ctx, bson.M{}, findOptions)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var docs []cabinetDoc
	if err = cursor.All(ctx, &docs); err != nil {
		return nil, 0, err
	}

	total, err := collection.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, 0, err
	}

	cabinets := make([]*biz.Cabinet, len(docs))
	for i, doc := range docs {
		cabinets[i] = docToCabinet(&doc)
	}
	return cabinets, total, nil
}

func (r *cabinetRepo) Update(ctx context.Context, cabinet *biz.Cabinet) (*biz.Cabinet, error) {
	collection := r.data.mongoDatabase.Collection("cabinets")
	objID, err := primitive.ObjectIDFromHex(cabinet.ID)
	if err != nil {
		return nil, err
	}
	cabinet.UpdateTime = time.Now().Format(time.RFC3339)
	update := bson.M{"$set": cabinetToDoc(cabinet)}
	_, err = collection.UpdateByID(ctx, objID, update)
	if err != nil {
		return nil, err
	}
	return cabinet, nil
}

func cabinetToDoc(c *biz.Cabinet) bson.M {
	return bson.M{
		"name":      c.Name,
		"address":   c.Address,
		"lng":       c.Lng,
		"lat":       c.Lat,
		"status":    c.Status,
		"createTime": c.CreateTime,
		"updateTime": c.UpdateTime,
	}
}

func docToCabinet(d *cabinetDoc) *biz.Cabinet {
	return &biz.Cabinet{
		ID:        d.ID.Hex(),
		Name:      d.Name,
		Address:   d.Address,
		Lng:       d.Lng,
		Lat:       d.Lat,
		Status:    d.Status,
		CreateTime: d.CreateTime,
		UpdateTime: d.UpdateTime,
	}
}

// ============================================
// CabinetGridRepo 实现
// ============================================

type cabinetGridDoc struct {
	ID        primitive.ObjectID `bson:"_id"`
	CabinetID string             `bson:"cabinetId"`
	GridNo    string             `bson:"gridNo"`
	Size      string             `bson:"size"`
	Status    string             `bson:"status"`
	OrderID   string             `bson:"orderId"`
	CreateTime string             `bson:"createTime"`
	UpdateTime string             `bson:"updateTime"`
}

type cabinetGridRepo struct {
	data *Data
	log  *log.Helper
}

func NewCabinetGridRepo(data *Data, logger log.Logger) biz.CabinetGridRepo {
	return &cabinetGridRepo{
		data: data,
		log:  log.NewHelper(logger),
	}
}

func (r *cabinetGridRepo) Create(ctx context.Context, grid *biz.CabinetGrid) (*biz.CabinetGrid, error) {
	collection := r.data.mongoDatabase.Collection("cabinet_grids")
	doc := gridToDoc(grid)
	result, err := collection.InsertOne(ctx, doc)
	if err != nil {
		r.log.Errorf("Insert cabinet_grid failed: %v", err)
		return nil, err
	}
	grid.ID = result.InsertedID.(primitive.ObjectID).Hex()
	return grid, nil
}

func (r *cabinetGridRepo) FindByID(ctx context.Context, id string) (*biz.CabinetGrid, error) {
	collection := r.data.mongoDatabase.Collection("cabinet_grids")
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var doc cabinetGridDoc
	err = collection.FindOne(ctx, bson.M{"_id": objID}).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}
	return docToGrid(&doc), nil
}

func (r *cabinetGridRepo) FindByCabinet(ctx context.Context, cabinetID string) ([]*biz.CabinetGrid, error) {
	collection := r.data.mongoDatabase.Collection("cabinet_grids")
	cursor, err := collection.Find(ctx, bson.M{"cabinetId": cabinetID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var docs []cabinetGridDoc
	if err = cursor.All(ctx, &docs); err != nil {
		return nil, err
	}

	grids := make([]*biz.CabinetGrid, len(docs))
	for i, doc := range docs {
		grids[i] = docToGrid(&doc)
	}
	return grids, nil
}

func (r *cabinetGridRepo) FindAvailable(ctx context.Context, cabinetID string) ([]*biz.CabinetGrid, error) {
	collection := r.data.mongoDatabase.Collection("cabinet_grids")
	cursor, err := collection.Find(ctx, bson.M{
		"cabinetId": cabinetID,
		"status":    biz.GridStatusIdle,
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var docs []cabinetGridDoc
	if err = cursor.All(ctx, &docs); err != nil {
		return nil, err
	}

	grids := make([]*biz.CabinetGrid, len(docs))
	for i, doc := range docs {
		grids[i] = docToGrid(&doc)
	}
	return grids, nil
}

func (r *cabinetGridRepo) LockGrid(ctx context.Context, gridID, orderID string) (*biz.CabinetGrid, error) {
	collection := r.data.mongoDatabase.Collection("cabinet_grids")
	objID, err := primitive.ObjectIDFromHex(gridID)
	if err != nil {
		return nil, err
	}

	now := time.Now().Format(time.RFC3339)
	filter := bson.M{
		"_id":    objID,
		"status": biz.GridStatusIdle, // 只能锁定空闲柜格
	}
	update := bson.M{
		"$set": bson.M{
			"status":     biz.GridStatusOccupied,
			"orderId":    orderID,
			"updateTime": now,
		},
	}

	findOneAndUpdateOptions := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var doc cabinetGridDoc
	err = collection.FindOneAndUpdate(ctx, filter, update, findOneAndUpdateOptions).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("grid not available")
		}
		return nil, err
	}
	return docToGrid(&doc), nil
}

func (r *cabinetGridRepo) ReleaseGrid(ctx context.Context, gridID string) (*biz.CabinetGrid, error) {
	collection := r.data.mongoDatabase.Collection("cabinet_grids")
	objID, err := primitive.ObjectIDFromHex(gridID)
	if err != nil {
		return nil, err
	}

	now := time.Now().Format(time.RFC3339)
	filter := bson.M{
		"_id":    objID,
		"status": biz.GridStatusOccupied, // 只能释放已占用柜格
	}
	update := bson.M{
		"$set": bson.M{
			"status":     biz.GridStatusIdle,
			"orderId":    "",
			"updateTime": now,
		},
	}

	findOneAndUpdateOptions := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var doc cabinetGridDoc
	err = collection.FindOneAndUpdate(ctx, filter, update, findOneAndUpdateOptions).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("grid not occupied")
		}
		return nil, err
	}
	return docToGrid(&doc), nil
}

func (r *cabinetGridRepo) Update(ctx context.Context, grid *biz.CabinetGrid) (*biz.CabinetGrid, error) {
	collection := r.data.mongoDatabase.Collection("cabinet_grids")
	objID, err := primitive.ObjectIDFromHex(grid.ID)
	if err != nil {
		return nil, err
	}
	grid.UpdateTime = time.Now().Format(time.RFC3339)
	update := bson.M{"$set": gridToDoc(grid)}
	_, err = collection.UpdateByID(ctx, objID, update)
	if err != nil {
		return nil, err
	}
	return grid, nil
}

func gridToDoc(g *biz.CabinetGrid) bson.M {
	return bson.M{
		"cabinetId": g.CabinetID,
		"gridNo":    g.GridNo,
		"size":      g.Size,
		"status":    g.Status,
		"orderId":   g.OrderID,
		"createTime": g.CreateTime,
		"updateTime": g.UpdateTime,
	}
}

func docToGrid(d *cabinetGridDoc) *biz.CabinetGrid {
	return &biz.CabinetGrid{
		ID:        d.ID.Hex(),
		CabinetID: d.CabinetID,
		GridNo:    d.GridNo,
		Size:      d.Size,
		Status:    d.Status,
		OrderID:   d.OrderID,
		CreateTime: d.CreateTime,
		UpdateTime: d.UpdateTime,
	}
}