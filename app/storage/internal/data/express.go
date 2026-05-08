package data

import (
	"context"
	"time"

	"shared-device-saas/app/storage/internal/biz"
	"shared-device-saas/app/storage/internal/conf"
	"shared-device-saas/pkg/redis"

	"github.com/go-kratos/kratos/v2/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// expressRepo 快递订单仓储实现 (MongoDB)
type expressRepo struct {
	data       *Data
	redis      *redis.Client
	collection *mongo.Collection
	log        *log.Helper
}

// expressDocument MongoDB 快递订单文档结构
type expressDocument struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"`
	UserID     string             `bson:"userId"`
	SenderPhone string            `bson:"senderPhone"`

	SenderName    string `bson:"senderName"`
	SenderAddress string `bson:"senderAddress"`

	ReceiverName     string `bson:"receiverName"`
	ReceiverPhone    string `bson:"receiverPhone"`
	ReceiverProvince string `bson:"receiverProvince"`
	ReceiverCity     string `bson:"receiverCity"`
	ReceiverDistrict string `bson:"receiverDistrict"`
	ReceiverAddress  string `bson:"receiverAddress"`

	ItemType   string  `bson:"itemType"`
	ItemWeight float64 `bson:"itemWeight"`
	ItemRemark string  `bson:"itemRemark"`

	ExpressType     int32  `bson:"expressType"`
	PickupTimeStart string `bson:"pickupTimeStart"`
	PickupTimeEnd   string `bson:"pickupTimeEnd"`

	Status     int32  `bson:"status"`
	PickupCode string `bson:"pickupCode"`
	ExpireTime string `bson:"expireTime"`

	// Phase 2 新增字段
	CourierID       string `bson:"courierId"`
	ShortCode       string `bson:"shortCode"`
	ShortCodeUsed   bool   `bson:"shortCodeUsed"`
	AssignedTime    string `bson:"assignedTime"`
	TimeoutNotified bool   `bson:"timeoutNotified"`

	StationID    string        `bson:"stationId"`
	CabinetID    string        `bson:"cabinetId"`
	GridID       string        `bson:"gridId"`       // 柜格ID（快递柜模式）
	DeliveryType int32         `bson:"deliveryType"` // 投递类型：1=驿站 2=快递柜
	Trace        []traceItemDoc `bson:"trace"`        // 物流轨迹数组

	CreateTime string `bson:"createTime"`
	UpdateTime string `bson:"updateTime"`
}

// traceItemDoc 物流轨迹项文档结构
type traceItemDoc struct {
	Status int32  `bson:"status"`
	Time   string `bson:"time"`
	Desc   string `bson:"desc"`
}

// NewExpressRepo 创建快递订单仓储
func NewExpressRepo(data *Data, redisClient *redis.Client, c *conf.Data, logger log.Logger) biz.ExpressRepo {
	helper := log.NewHelper(logger)

	mongoCfg := c.GetMongodb()
	if mongoCfg == nil || data.mongoDatabase == nil {
		helper.Warn("MongoDB not configured, using in-memory storage")
		return newInMemoryExpressRepo(helper, redisClient)
	}

	collection := data.GetCollection(mongoCfg.Collection)
	if collection == nil {
		helper.Warn("MongoDB collection not available, using in-memory storage")
		return newInMemoryExpressRepo(helper, redisClient)
	}

	// 创建唯一索引（pickupCode）
	indexModel := mongo.IndexModel{
		Keys:    bson.D{{"pickupCode", 1}},
		Options: options.Index().SetUnique(true).SetSparse(true), // sparse允许空值
	}
	_, err := collection.Indexes().CreateOne(context.Background(), indexModel)
	if err != nil {
		helper.Warnf("Failed to create pickupCode index: %v", err)
	} else {
		helper.Info("pickupCode unique index created")
	}

	helper.Infof("Using MongoDB storage: collection=%s", mongoCfg.Collection)
	return &expressRepo{
		data:       data,
		redis:      redisClient,
		collection: collection,
		log:        helper,
	}
}

// Create 创建快递订单
func (r *expressRepo) Create(ctx context.Context, order *biz.ExpressOrder) (*biz.ExpressOrder, error) {
	doc := &expressDocument{
		UserID:     order.UserID,
		SenderPhone: order.SenderPhone,

		SenderName:    order.SenderName,
		SenderAddress: order.SenderAddress,

		ReceiverName:     order.ReceiverName,
		ReceiverPhone:    order.ReceiverPhone,
		ReceiverProvince: order.ReceiverProvince,
		ReceiverCity:     order.ReceiverCity,
		ReceiverDistrict: order.ReceiverDistrict,
		ReceiverAddress:  order.ReceiverAddress,

		ItemType:   order.ItemType,
		ItemWeight: order.ItemWeight,
		ItemRemark: order.ItemRemark,

		ExpressType:     order.ExpressType,
		PickupTimeStart: order.PickupTimeStart,
		PickupTimeEnd:   order.PickupTimeEnd,

		Status:     order.Status,
		PickupCode: order.PickupCode,
		ExpireTime: order.ExpireTime,

		// Phase 2 新增字段
		CourierID:       order.CourierID,
		ShortCode:       order.ShortCode,
		ShortCodeUsed:   order.ShortCodeUsed,
		AssignedTime:    order.AssignedTime,
		TimeoutNotified: order.TimeoutNotified,

		StationID:  order.StationID,
		CabinetID:  order.CabinetID,

		CreateTime: order.CreateTime,
		UpdateTime: order.UpdateTime,
	}

	result, err := r.collection.InsertOne(ctx, doc)
	if err != nil {
		r.log.Errorf("Failed to create express order: %v", err)
		return nil, err
	}

	if oid, ok := result.InsertedID.(primitive.ObjectID); ok {
		order.ID = oid.Hex()
	}

	r.log.Infof("Created express order: id=%s, userId=%s, status=%d, courierId=%s", order.ID, order.UserID, order.Status, order.CourierID)
	return order, nil
}

// FindByID 根据ID查找
func (r *expressRepo) FindByID(ctx context.Context, id string) (*biz.ExpressOrder, error) {
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		r.log.Errorf("Invalid order ID format: %s", id)
		return nil, nil
	}

	filter := bson.M{"_id": objectID}

	var doc expressDocument
	err = r.collection.FindOne(ctx, filter).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}

	return r.documentToOrder(&doc), nil
}

// FindByPickupCode 根据取件码查找
func (r *expressRepo) FindByPickupCode(ctx context.Context, pickupCode string) (*biz.ExpressOrder, error) {
	filter := bson.M{"pickupCode": pickupCode}

	var doc expressDocument
	err := r.collection.FindOne(ctx, filter).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}

	return r.documentToOrder(&doc), nil
}

// FindBySenderPhone 根据寄件人电话查找（已寄出）
func (r *expressRepo) FindBySenderPhone(ctx context.Context, senderPhone string, page, pageSize int32) ([]*biz.ExpressOrder, int64, error) {
	filter := bson.M{"senderPhone": senderPhone}

	// 计算总数
	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	skip := int64((page - 1) * pageSize)
	limit := int64(pageSize)
	opts := options.Find().SetSkip(skip).SetLimit(limit).SetSort(bson.D{{"createTime", -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var orders []*biz.ExpressOrder
	for cursor.Next(ctx) {
		var doc expressDocument
		if err := cursor.Decode(&doc); err != nil {
			r.log.Errorf("Failed to decode document: %v", err)
			continue
		}
		orders = append(orders, r.documentToOrder(&doc))
	}

	return orders, total, nil
}

// FindByReceiverPhone 根据收件人电话查找（待收件）
func (r *expressRepo) FindByReceiverPhone(ctx context.Context, receiverPhone string, statuses []int32, page, pageSize int32) ([]*biz.ExpressOrder, int64, error) {
	filter := bson.M{
		"receiverPhone": receiverPhone,
		"status":        bson.M{"$in": statuses},
	}

	// 计算总数
	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	// 分页查询
	skip := int64((page - 1) * pageSize)
	limit := int64(pageSize)
	opts := options.Find().SetSkip(skip).SetLimit(limit).SetSort(bson.D{{"createTime", -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var orders []*biz.ExpressOrder
	for cursor.Next(ctx) {
		var doc expressDocument
		if err := cursor.Decode(&doc); err != nil {
			r.log.Errorf("Failed to decode document: %v", err)
			continue
		}
		orders = append(orders, r.documentToOrder(&doc))
	}

	return orders, total, nil
}

// Update 更新快递订单
func (r *expressRepo) Update(ctx context.Context, order *biz.ExpressOrder) (*biz.ExpressOrder, error) {
	objectID, err := primitive.ObjectIDFromHex(order.ID)
	if err != nil {
		return nil, err
	}

	filter := bson.M{"_id": objectID}
	update := bson.M{
		"$set": bson.M{
			"status":           order.Status,
			"pickupCode":       order.PickupCode,
			"expireTime":       order.ExpireTime,
			"updateTime":       order.UpdateTime,
			// Phase 2 新增字段
			"courierId":        order.CourierID,
			"shortCode":        order.ShortCode,
			"shortCodeUsed":    order.ShortCodeUsed,
			"assignedTime":     order.AssignedTime,
			"timeoutNotified":  order.TimeoutNotified,
		},
	}

	_, err = r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		r.log.Errorf("Failed to update express order: %v", err)
		return nil, err
	}

	// 如果生成取件码，存入Redis
	if order.PickupCode != "" && order.Status == biz.StatusAtStation && r.redis != nil {
		ttl := 7 * 24 * time.Hour // 7天过期
		r.redis.Set(ctx, "pickup:"+order.PickupCode, order.ID, ttl)
		r.log.Infof("Pickup code stored in Redis: code=%s, orderId=%s", order.PickupCode, order.ID)
	}

	// 如果签收，删除Redis中的取件码
	if order.Status == biz.StatusDelivered && r.redis != nil {
		r.redis.Del(ctx, "pickup:"+order.PickupCode)
	}

	return order, nil
}

// documentToOrder 文档转换为实体
func (r *expressRepo) documentToOrder(doc *expressDocument) *biz.ExpressOrder {
	// 转换 Trace
	trace := make([]biz.TraceItem, len(doc.Trace))
	for i, t := range doc.Trace {
		trace[i] = biz.TraceItem{
			Status: t.Status,
			Time:   t.Time,
			Desc:   t.Desc,
		}
	}

	return &biz.ExpressOrder{
		ID:         doc.ID.Hex(),
		UserID:     doc.UserID,
		SenderPhone: doc.SenderPhone,

		SenderName:    doc.SenderName,
		SenderAddress: doc.SenderAddress,

		ReceiverName:     doc.ReceiverName,
		ReceiverPhone:    doc.ReceiverPhone,
		ReceiverProvince: doc.ReceiverProvince,
		ReceiverCity:     doc.ReceiverCity,
		ReceiverDistrict: doc.ReceiverDistrict,
		ReceiverAddress:  doc.ReceiverAddress,

		ItemType:   doc.ItemType,
		ItemWeight: doc.ItemWeight,
		ItemRemark: doc.ItemRemark,

		ExpressType:     doc.ExpressType,
		PickupTimeStart: doc.PickupTimeStart,
		PickupTimeEnd:   doc.PickupTimeEnd,

		Status:     doc.Status,
		PickupCode: doc.PickupCode,
		ExpireTime: doc.ExpireTime,

		// Phase 2 新增字段
		CourierID:       doc.CourierID,
		ShortCode:       doc.ShortCode,
		ShortCodeUsed:   doc.ShortCodeUsed,
		AssignedTime:    doc.AssignedTime,
		TimeoutNotified: doc.TimeoutNotified,

		// Phase 3 新增字段
		StationID:    doc.StationID,
		CabinetID:    doc.CabinetID,
		GridID:       doc.GridID,
		DeliveryType: doc.DeliveryType,
		Trace:        trace,

		CreateTime: doc.CreateTime,
		UpdateTime: doc.UpdateTime,
	}
}

// FindByStatus 查询指定状态的订单列表
func (r *expressRepo) FindByStatus(ctx context.Context, status int32, page, pageSize int32) ([]*biz.ExpressOrder, int64, error) {
	filter := bson.M{"status": status}

	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	skip := int64((page - 1) * pageSize)
	limit := int64(pageSize)
	opts := options.Find().SetSkip(skip).SetLimit(limit).SetSort(bson.D{{"createTime", -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var orders []*biz.ExpressOrder
	for cursor.Next(ctx) {
		var doc expressDocument
		if err := cursor.Decode(&doc); err != nil {
			r.log.Errorf("Failed to decode document: %v", err)
			continue
		}
		orders = append(orders, r.documentToOrder(&doc))
	}

	return orders, total, nil
}

// FindByCourierID 根据快递员ID查询待揽收订单
func (r *expressRepo) FindByCourierID(ctx context.Context, courierID string, page, pageSize int32) ([]*biz.ExpressOrder, int64, error) {
	filter := bson.M{
		"courierId": courierID,
		"status":    bson.M{"$in": []int32{biz.StatusPendingPickup}},
	}

	total, err := r.collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	skip := int64((page - 1) * pageSize)
	limit := int64(pageSize)
	opts := options.Find().SetSkip(skip).SetLimit(limit).SetSort(bson.D{{"assignedTime", 1}}) // 按派单时间升序

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var orders []*biz.ExpressOrder
	for cursor.Next(ctx) {
		var doc expressDocument
		if err := cursor.Decode(&doc); err != nil {
			r.log.Errorf("Failed to decode document: %v", err)
			continue
		}
		orders = append(orders, r.documentToOrder(&doc))
	}

	return orders, total, nil
}

// FindTimeoutOrders 查询超时未取件的订单
func (r *expressRepo) FindTimeoutOrders(ctx context.Context, now string) ([]*biz.ExpressOrder, error) {
	// status=101 AND pickupTimeEnd < now AND timeoutNotified=false
	filter := bson.M{
		"status":           biz.StatusPendingPickup,
		"pickupTimeEnd":    bson.M{"$lt": now},
		"timeoutNotified":  false,
	}

	cursor, err := r.collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var orders []*biz.ExpressOrder
	for cursor.Next(ctx) {
		var doc expressDocument
		if err := cursor.Decode(&doc); err != nil {
			r.log.Errorf("Failed to decode document: %v", err)
			continue
		}
		orders = append(orders, r.documentToOrder(&doc))
	}

	return orders, nil
}

// UpdateStatus 原子更新状态
func (r *expressRepo) UpdateStatus(ctx context.Context, orderID string, status int32) (*biz.ExpressOrder, error) {
	objectID, err := primitive.ObjectIDFromHex(orderID)
	if err != nil {
		return nil, err
	}

	now := time.Now().Format(time.RFC3339)
	filter := bson.M{"_id": objectID}
	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"updateTime": now,
		},
	}

	var doc expressDocument
	err = r.collection.FindOneAndUpdate(ctx, filter, update, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&doc)
	if err != nil {
		r.log.Errorf("Failed to update status: %v", err)
		return nil, err
	}

	return r.documentToOrder(&doc), nil
}

// AppendTrace 原子追加轨迹
func (r *expressRepo) AppendTrace(ctx context.Context, orderID string, traceItem *biz.TraceItem) (*biz.ExpressOrder, error) {
	objectID, err := primitive.ObjectIDFromHex(orderID)
	if err != nil {
		return nil, err
	}

	traceDoc := traceItemDoc{
		Status: traceItem.Status,
		Time:   traceItem.Time,
		Desc:   traceItem.Desc,
	}

	filter := bson.M{"_id": objectID}
	update := bson.M{
		"$push": bson.M{"trace": traceDoc},
		"$set":  bson.M{"updateTime": traceItem.Time},
	}

	var doc expressDocument
	err = r.collection.FindOneAndUpdate(ctx, filter, update, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&doc)
	if err != nil {
		r.log.Errorf("Failed to append trace: %v", err)
		return nil, err
	}

	return r.documentToOrder(&doc), nil
}

// UpdateDeliveryInfo 更新投递信息
func (r *expressRepo) UpdateDeliveryInfo(ctx context.Context, orderID string, status int32, stationID, gridID, pickupCode string) (*biz.ExpressOrder, error) {
	objectID, err := primitive.ObjectIDFromHex(orderID)
	if err != nil {
		return nil, err
	}

	now := time.Now().Format(time.RFC3339)
	deliveryType := int32(1) // 默认驿站模式
	if gridID != "" {
		deliveryType = 2 // 快递柜模式
	}

	filter := bson.M{"_id": objectID}
	update := bson.M{
		"$set": bson.M{
			"status":       status,
			"stationId":    stationID,
			"gridId":       gridID,
			"deliveryType": deliveryType,
			"pickupCode":   pickupCode,
			"expireTime":   now, // TODO: 设置合理的过期时间
			"updateTime":   now,
		},
	}

	var doc expressDocument
	err = r.collection.FindOneAndUpdate(ctx, filter, update, options.FindOneAndUpdate().SetReturnDocument(options.After)).Decode(&doc)
	if err != nil {
		r.log.Errorf("Failed to update delivery info: %v", err)
		return nil, err
	}

	// 存入Redis
	if pickupCode != "" && r.redis != nil {
		ttl := 7 * 24 * time.Hour
		r.redis.Set(ctx, "pickup:"+pickupCode, orderID, ttl)
	}

	return r.documentToOrder(&doc), nil
}

// ============================================
// 内存存储实现 (备用方案)
// ============================================

type inMemoryExpressRepo struct {
	orders         map[string]*biz.ExpressOrder // ID -> Order
	ordersByCode   map[string]*biz.ExpressOrder // PickupCode -> Order
	ordersBySender map[string][]*biz.ExpressOrder // SenderPhone -> Orders
	ordersByReceiver map[string][]*biz.ExpressOrder // ReceiverPhone -> Orders
	nextID         primitive.ObjectID
	redis          *redis.Client
	log            *log.Helper
}

func newInMemoryExpressRepo(log *log.Helper, redisClient *redis.Client) *inMemoryExpressRepo {
	return &inMemoryExpressRepo{
		orders:          make(map[string]*biz.ExpressOrder),
		ordersByCode:    make(map[string]*biz.ExpressOrder),
		ordersBySender:  make(map[string][]*biz.ExpressOrder),
		ordersByReceiver: make(map[string][]*biz.ExpressOrder),
		nextID:          primitive.NewObjectID(),
		redis:           redisClient,
		log:             log,
	}
}

func (r *inMemoryExpressRepo) Create(ctx context.Context, order *biz.ExpressOrder) (*biz.ExpressOrder, error) {
	oid := r.nextID
	r.nextID = primitive.NewObjectID()
	order.ID = oid.Hex()
	r.orders[order.ID] = order

	if order.SenderPhone != "" {
		r.ordersBySender[order.SenderPhone] = append(r.ordersBySender[order.SenderPhone], order)
	}
	if order.ReceiverPhone != "" {
		r.ordersByReceiver[order.ReceiverPhone] = append(r.ordersByReceiver[order.ReceiverPhone], order)
	}
	if order.PickupCode != "" {
		r.ordersByCode[order.PickupCode] = order
	}

	r.log.Infof("Created express order in memory: id=%s, userId=%s", order.ID, order.UserID)
	return order, nil
}

func (r *inMemoryExpressRepo) FindByID(ctx context.Context, id string) (*biz.ExpressOrder, error) {
	return r.orders[id], nil
}

func (r *inMemoryExpressRepo) FindByPickupCode(ctx context.Context, pickupCode string) (*biz.ExpressOrder, error) {
	return r.ordersByCode[pickupCode], nil
}

func (r *inMemoryExpressRepo) FindBySenderPhone(ctx context.Context, senderPhone string, page, pageSize int32) ([]*biz.ExpressOrder, int64, error) {
	orders := r.ordersBySender[senderPhone]
	total := int64(len(orders))
	start := (page - 1) * pageSize
	end := start + pageSize
	if start >= int32(len(orders)) {
		return nil, total, nil
	}
	if end > int32(len(orders)) {
		end = int32(len(orders))
	}
	return orders[start:end], total, nil
}

func (r *inMemoryExpressRepo) FindByReceiverPhone(ctx context.Context, receiverPhone string, statuses []int32, page, pageSize int32) ([]*biz.ExpressOrder, int64, error) {
	allOrders := r.ordersByReceiver[receiverPhone]
	var filtered []*biz.ExpressOrder
	for _, o := range allOrders {
		for _, s := range statuses {
			if o.Status == s {
				filtered = append(filtered, o)
				break
			}
		}
	}
	total := int64(len(filtered))
	start := (page - 1) * pageSize
	end := start + pageSize
	if start >= int32(len(filtered)) {
		return nil, total, nil
	}
	if end > int32(len(filtered)) {
		end = int32(len(filtered))
	}
	return filtered[start:end], total, nil
}

func (r *inMemoryExpressRepo) Update(ctx context.Context, order *biz.ExpressOrder) (*biz.ExpressOrder, error) {
	r.orders[order.ID] = order
	if order.PickupCode != "" {
		r.ordersByCode[order.PickupCode] = order
	}
	return order, nil
}

// FindByStatus 查询指定状态的订单列表
func (r *inMemoryExpressRepo) FindByStatus(ctx context.Context, status int32, page, pageSize int32) ([]*biz.ExpressOrder, int64, error) {
	var filtered []*biz.ExpressOrder
	for _, o := range r.orders {
		if o.Status == status {
			filtered = append(filtered, o)
		}
	}
	total := int64(len(filtered))
	start := (page - 1) * pageSize
	end := start + pageSize
	if start >= int32(len(filtered)) {
		return nil, total, nil
	}
	if end > int32(len(filtered)) {
		end = int32(len(filtered))
	}
	return filtered[start:end], total, nil
}

// FindByCourierID 根据快递员ID查询待揽收订单
func (r *inMemoryExpressRepo) FindByCourierID(ctx context.Context, courierID string, page, pageSize int32) ([]*biz.ExpressOrder, int64, error) {
	var filtered []*biz.ExpressOrder
	for _, o := range r.orders {
		if o.CourierID == courierID && o.Status == biz.StatusPendingPickup {
			filtered = append(filtered, o)
		}
	}
	total := int64(len(filtered))
	start := (page - 1) * pageSize
	end := start + pageSize
	if start >= int32(len(filtered)) {
		return nil, total, nil
	}
	if end > int32(len(filtered)) {
		end = int32(len(filtered))
	}
	return filtered[start:end], total, nil
}

// FindTimeoutOrders 查询超时未取件的订单
func (r *inMemoryExpressRepo) FindTimeoutOrders(ctx context.Context, now string) ([]*biz.ExpressOrder, error) {
	var orders []*biz.ExpressOrder
	for _, o := range r.orders {
		if o.Status == biz.StatusPendingPickup && !o.TimeoutNotified && o.PickupTimeEnd < now {
			orders = append(orders, o)
		}
	}
	return orders, nil
}

// UpdateStatus 原子更新状态
func (r *inMemoryExpressRepo) UpdateStatus(ctx context.Context, orderID string, status int32) (*biz.ExpressOrder, error) {
	order, ok := r.orders[orderID]
	if !ok {
		return nil, nil
	}
	order.Status = status
	order.UpdateTime = time.Now().Format(time.RFC3339)
	return order, nil
}

// AppendTrace 原子追加轨迹
func (r *inMemoryExpressRepo) AppendTrace(ctx context.Context, orderID string, traceItem *biz.TraceItem) (*biz.ExpressOrder, error) {
	order, ok := r.orders[orderID]
	if !ok {
		return nil, nil
	}
	order.Trace = append(order.Trace, *traceItem)
	order.UpdateTime = traceItem.Time
	return order, nil
}

// UpdateDeliveryInfo 更新投递信息
func (r *inMemoryExpressRepo) UpdateDeliveryInfo(ctx context.Context, orderID string, status int32, stationID, gridID, pickupCode string) (*biz.ExpressOrder, error) {
	order, ok := r.orders[orderID]
	if !ok {
		return nil, nil
	}
	order.Status = status
	order.StationID = stationID
	order.GridID = gridID
	order.PickupCode = pickupCode
	if gridID != "" {
		order.DeliveryType = 2 // 快递柜模式
	} else {
		order.DeliveryType = 1 // 驿站模式
	}
	order.UpdateTime = time.Now().Format(time.RFC3339)

	if pickupCode != "" {
		r.ordersByCode[pickupCode] = order
	}

	return order, nil
}