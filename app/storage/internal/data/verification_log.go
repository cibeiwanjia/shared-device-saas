package data

import (
	"context"
	"time"

	"shared-device-saas/app/storage/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// verificationLogRepo 核验审计日志仓储实现
type verificationLogRepo struct {
	data       *Data
	collection *mongo.Collection
	log        *log.Helper
}

// verificationLogDocument MongoDB 核验日志文档结构
type verificationLogDocument struct {
	ID             primitive.ObjectID `bson:"_id,omitempty"`
	OrderID        string             `bson:"orderId"`
	CourierID      string             `bson:"courierId"`
	InputShortCode string             `bson:"inputShortCode"`
	Result         string             `bson:"result"` // success/failed
	FailReason     string             `bson:"failReason"`
	Timestamp      string             `bson:"timestamp"`
}

// NewVerificationLogRepo 创建核验日志仓储
func NewVerificationLogRepo(data *Data, logger log.Logger) biz.VerificationLogRepo {
	helper := log.NewHelper(logger)
	collection := data.mongoDatabase.Collection("verification_logs")
	if collection == nil {
		helper.Warn("verification_logs collection not available")
	}

	return &verificationLogRepo{
		data:       data,
		collection: collection,
		log:        helper,
	}
}

// Create 创建核验日志
func (r *verificationLogRepo) Create(ctx context.Context, logEntry *biz.VerificationLog) (*biz.VerificationLog, error) {
	now := time.Now().Format(time.RFC3339)
	if logEntry.Timestamp == "" {
		logEntry.Timestamp = now
	}

	doc := &verificationLogDocument{
		OrderID:        logEntry.OrderID,
		CourierID:      logEntry.CourierID,
		InputShortCode: logEntry.InputShortCode,
		Result:         logEntry.Result,
		FailReason:     logEntry.FailReason,
		Timestamp:      logEntry.Timestamp,
	}

	result, err := r.collection.InsertOne(ctx, doc)
	if err != nil {
		r.log.Errorf("Failed to create verification log: %v", err)
		return nil, err
	}

	if oid, ok := result.InsertedID.(primitive.ObjectID); ok {
		logEntry.ID = oid.Hex()
	}

	r.log.Infof("Verification log created: orderId=%s, result=%s", logEntry.OrderID, logEntry.Result)
	return logEntry, nil
}

// FindByOrderID 根据订单ID查询核验历史
func (r *verificationLogRepo) FindByOrderID(ctx context.Context, orderID string) ([]*biz.VerificationLog, error) {
	filter := bson.M{"orderId": orderID}
	opts := options.Find().SetSort(bson.D{{"timestamp", -1}})

	cursor, err := r.collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var logs []*biz.VerificationLog
	for cursor.Next(ctx) {
		var doc verificationLogDocument
		if err := cursor.Decode(&doc); err != nil {
			r.log.Errorf("Failed to decode document: %v", err)
			continue
		}
		logs = append(logs, &biz.VerificationLog{
			ID:             doc.ID.Hex(),
			OrderID:        doc.OrderID,
			CourierID:      doc.CourierID,
			InputShortCode: doc.InputShortCode,
			Result:         doc.Result,
			FailReason:     doc.FailReason,
			Timestamp:      doc.Timestamp,
		})
	}

	return logs, nil
}