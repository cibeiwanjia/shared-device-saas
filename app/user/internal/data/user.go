package data

import (
	"context"

	"shared-device-saas/app/user/internal/biz"
	"shared-device-saas/app/user/internal/conf"

	"github.com/go-kratos/kratos/v2/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// userRepo 用户仓储实现 (MongoDB 存储)
type userRepo struct {
	data       *Data
	collection *mongo.Collection
	log        *log.Helper
}

// userDocument MongoDB 用户文档结构（小驼峰命名）
type userDocument struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"`
	MysqlID    string             `bson:"mysql_id,omitempty"`
	Username   string             `bson:"username"`
	Password   string             `bson:"password"`
	Email      string             `bson:"email"`
	Phone      string             `bson:"phone"`
	Nickname   string             `bson:"nickname"`
	Avatar     string             `bson:"avatar"`
	InviteCode string             `bson:"inviteCode"`
	Status     int32              `bson:"status"`
	Role       string             `bson:"role"`
	CreateTime int64              `bson:"createTime"`
	UpdateTime int64              `bson:"updateTime"`
}

// NewUserRepo 创建双写用户仓储（MySQL 主库 + MongoDB 副本）
func NewUserRepo(data *Data, c *conf.Data, logger log.Logger) biz.UserRepo {
	helper := log.NewHelper(logger)

	mongoRepo := newMongoUserRepo(data, c, logger)
	mysqlRepo := newMysqlUserRepo(data.GetSqlDB(), logger)

	if mysqlRepo == nil || mysqlRepo.db == nil {
		helper.Warn("MySQL not available, falling back to MongoDB-only")
		return mongoRepo
	}

	helper.Info("Using dual-write user storage: MySQL (primary) + MongoDB (replica)")
	return &userDualWriteRepo{
		mysql:  mysqlRepo,
		mongo:  mongoRepo,
		log:    helper,
	}
}

// userDualWriteRepo MySQL主库 + MongoDB副本 双写仓储
type userDualWriteRepo struct {
	mysql *userMysqlRepo
	mongo biz.UserRepo
	log   *log.Helper
}

func (r *userDualWriteRepo) Create(ctx context.Context, user *biz.User) (*biz.User, error) {
	// 1. 写入 MySQL（主库，生成自增 ID）
	u, err := r.mysql.Create(ctx, user)
	if err != nil {
		return nil, err
	}

	// 2. 同步写入 MongoDB（best-effort 副本）
	if _, mongoErr := r.mongo.Create(ctx, u); mongoErr != nil {
		r.log.Warnf("MongoDB sync create failed (non-fatal): %v", mongoErr)
	}

	return u, nil
}

func (r *userDualWriteRepo) FindByPhone(ctx context.Context, phone string) (*biz.User, error) {
	return r.mysql.FindByPhone(ctx, phone)
}

func (r *userDualWriteRepo) FindByUsername(ctx context.Context, username string) (*biz.User, error) {
	return r.mysql.FindByUsername(ctx, username)
}

func (r *userDualWriteRepo) FindByID(ctx context.Context, id string) (*biz.User, error) {
	return r.mysql.FindByID(ctx, id)
}

func (r *userDualWriteRepo) Update(ctx context.Context, user *biz.User) (*biz.User, error) {
	u, err := r.mysql.Update(ctx, user)
	if err != nil {
		return nil, err
	}
	if _, mongoErr := r.mongo.Update(ctx, u); mongoErr != nil {
		r.log.Warnf("MongoDB sync update failed (non-fatal): %v", mongoErr)
	}
	return u, nil
}

// ============================================
// MongoDB UserRepo 实现
// ============================================

// newMongoUserRepo 创建 MongoDB 用户仓储
func newMongoUserRepo(data *Data, c *conf.Data, logger log.Logger) biz.UserRepo {
	helper := log.NewHelper(logger)

	mongoCfg := c.GetMongodb()
	if mongoCfg == nil || data.mongoDatabase == nil {
		helper.Warn("MongoDB not configured, using in-memory storage")
		return newInMemoryUserRepo(helper)
	}

	collection := data.GetCollection(mongoCfg.Collection)
	if collection == nil {
		helper.Warn("MongoDB collection not available, using in-memory storage")
		return newInMemoryUserRepo(helper)
	}

	helper.Infof("Using MongoDB storage: collection=%s", mongoCfg.Collection)
	return &userRepo{
		data:       data,
		collection: collection,
		log:        helper,
	}
}

// Create 创建用户
func (r *userRepo) Create(ctx context.Context, user *biz.User) (*biz.User, error) {
	doc := &userDocument{
		Username:   user.Username,
		Password:   user.Password,
		Email:      user.Email,
		Phone:      user.Phone,
		Nickname:   user.Nickname,
		Avatar:     user.Avatar,
		InviteCode: user.InviteCode,
		Status:     user.Status,
		Role:       user.Role,
		CreateTime: user.CreateTime,
		UpdateTime: user.UpdateTime,
	}

	// 如果已有 ID（来自 MySQL 自增），存为 mysql_id 字段
	if user.ID != "" {
		doc.MysqlID = user.ID
	}

	result, err := r.collection.InsertOne(ctx, doc)
	if err != nil {
		r.log.Errorf("Failed to create user: %v", err)
		return nil, err
	}

	// 如果没有预置 ID，使用 MongoDB ObjectID
	if user.ID == "" {
		if oid, ok := result.InsertedID.(primitive.ObjectID); ok {
			user.ID = oid.Hex()
		}
	}

	r.log.Infof("Created user in MongoDB: phone=%s, id=%s", user.Phone, user.ID)
	return user, nil
}

// FindByPhone 根据手机号查找
func (r *userRepo) FindByPhone(ctx context.Context, phone string) (*biz.User, error) {
	filter := bson.M{"phone": phone}

	var doc userDocument
	err := r.collection.FindOne(ctx, filter).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}

	return r.documentToUser(&doc), nil
}

// FindByUsername 根据用户名查找
func (r *userRepo) FindByUsername(ctx context.Context, username string) (*biz.User, error) {
	filter := bson.M{"username": username}

	var doc userDocument
	err := r.collection.FindOne(ctx, filter).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}

	return r.documentToUser(&doc), nil
}

// FindByID 根据 ID 查找（ID 是 ObjectID 的 Hex 字符串）
func (r *userRepo) FindByID(ctx context.Context, id string) (*biz.User, error) {
	// 从 Hex 字符串解析 ObjectID
	objectID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		r.log.Errorf("Invalid user ID format: %s", id)
		return nil, nil
	}

	filter := bson.M{"_id": objectID}

	var doc userDocument
	err = r.collection.FindOne(ctx, filter).Decode(&doc)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}

	return r.documentToUser(&doc), nil
}

// Update 更新用户
func (r *userRepo) Update(ctx context.Context, user *biz.User) (*biz.User, error) {
	// 从 ID 解析 ObjectID
	objectID, err := primitive.ObjectIDFromHex(user.ID)
	if err != nil {
		return nil, err
	}

	filter := bson.M{"_id": objectID}
	update := bson.M{
		"$set": bson.M{
			"username":   user.Username,
			"email":      user.Email,
			"nickname":   user.Nickname,
			"avatar":     user.Avatar,
			"updateTime": user.UpdateTime,
		},
	}

	_, err = r.collection.UpdateOne(ctx, filter, update)
	if err != nil {
		r.log.Errorf("Failed to update user: %v", err)
		return nil, err
	}

	return user, nil
}

// documentToUser 将 MongoDB 文档转换为业务实体
func (r *userRepo) documentToUser(doc *userDocument) *biz.User {
	return &biz.User{
		ID:         doc.ID.Hex(), // ObjectID 转 Hex 字符串
		Username:   doc.Username,
		Password:   doc.Password,
		Email:      doc.Email,
		Phone:      doc.Phone,
		Nickname:   doc.Nickname,
		Avatar:     doc.Avatar,
		InviteCode: doc.InviteCode,
		Status:     doc.Status,
		Role:       doc.Role,
		CreateTime: doc.CreateTime,
		UpdateTime: doc.UpdateTime,
	}
}

// ============================================
// 内存存储实现 (备用方案)
// ============================================

type inMemoryUserRepo struct {
	users        map[string]*biz.User      // ID -> User
	usersByPhone map[string]*biz.User      // Phone -> User
	usersByName  map[string]*biz.User      // Username -> User
	nextID       primitive.ObjectID
	log          *log.Helper
}

func newInMemoryUserRepo(log *log.Helper) *inMemoryUserRepo {
	return &inMemoryUserRepo{
		users:        make(map[string]*biz.User),
		usersByPhone: make(map[string]*biz.User),
		usersByName:  make(map[string]*biz.User),
		nextID:       primitive.NewObjectID(),
		log:          log,
	}
}

func (r *inMemoryUserRepo) Create(ctx context.Context, user *biz.User) (*biz.User, error) {
	oid := r.nextID
	r.nextID = primitive.NewObjectID()
	user.ID = oid.Hex()
	r.users[user.ID] = user
	r.usersByPhone[user.Phone] = user
	if user.Username != "" {
		r.usersByName[user.Username] = user
	}
	r.log.Infof("Created user in memory: id=%s, phone=%s", user.ID, user.Phone)
	return user, nil
}

func (r *inMemoryUserRepo) FindByPhone(ctx context.Context, phone string) (*biz.User, error) {
	user, ok := r.usersByPhone[phone]
	if !ok {
		return nil, nil
	}
	return user, nil
}

func (r *inMemoryUserRepo) FindByUsername(ctx context.Context, username string) (*biz.User, error) {
	user, ok := r.usersByName[username]
	if !ok {
		return nil, nil
	}
	return user, nil
}

func (r *inMemoryUserRepo) FindByID(ctx context.Context, id string) (*biz.User, error) {
	user, ok := r.users[id]
	if !ok {
		return nil, nil
	}
	return user, nil
}

func (r *inMemoryUserRepo) Update(ctx context.Context, user *biz.User) (*biz.User, error) {
	r.users[user.ID] = user
	r.usersByPhone[user.Phone] = user
	if user.Username != "" {
		r.usersByName[user.Username] = user
	}
	return user, nil
}