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
	data       *Data             // 数据访问层
	collection *mongo.Collection // 用户集合
	log        *log.Helper       // 日志助手
}

// userDocument MongoDB 用户文档结构（小驼峰命名）
type userDocument struct {
	ID         primitive.ObjectID `bson:"_id,omitempty"` // 用户 ID（账号）
	Password   string             `bson:"password"`      // 密码（已哈希）
	Email      string             `bson:"email"`         // 邮箱
	Phone      string             `bson:"phone"`         // 手机号
	Nickname   string             `bson:"nickname"`      // 昵称
	Avatar     string             `bson:"avatar"`        // 头像 URL
	InviteCode string             `bson:"inviteCode"`    // 邀请码
	Status     int32              `bson:"status"`        // 状态（0:禁用, 1:启用）
	Role       string             `bson:"role"`          // 角色（user/admin）
	CreateTime string             `bson:"createTime"`    // RFC3339格式
	UpdateTime string             `bson:"updateTime"`    // RFC3339格式
}

// NewUserRepo 创建用户仓储
func NewUserRepo(data *Data, c *conf.Data, logger log.Logger) biz.UserRepo {
	helper := log.NewHelper(logger)
	// 获取 MongoDB 配置
	mongoCfg := c.GetMongodb()
	if mongoCfg == nil || data.mongoDatabase == nil {
		helper.Warn("MongoDB not configured, using in-memory storage")
		return newInMemoryUserRepo(helper)
	}

	// 获取 MongoDB 配置中的集合
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

	result, err := r.collection.InsertOne(ctx, doc)
	if err != nil {
		r.log.Errorf("Failed to create user: %v", err)
		return nil, err
	}

	// 使用 ObjectID 作为用户 ID（账号）
	if oid, ok := result.InsertedID.(primitive.ObjectID); ok {
		user.ID = oid.Hex()
	}

	r.log.Infof("Created user in MongoDB: phone=%s, id=%s, nickname=%s", user.Phone, user.ID, user.Nickname)
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

// FindByEmail 根据邮箱查找
func (r *userRepo) FindByEmail(ctx context.Context, email string) (*biz.User, error) {
	filter := bson.M{"email": email}

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

// FindByID 根据 ID 查找（ID 是 ObjectID 的 Hex 字符串，即账号）
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
		ID:         doc.ID.Hex(),
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
	users        map[string]*biz.User // ID -> User
	usersByPhone map[string]*biz.User // Phone -> User
	usersByEmail map[string]*biz.User // Email -> User
	nextID       primitive.ObjectID
	log          *log.Helper
}

func newInMemoryUserRepo(log *log.Helper) *inMemoryUserRepo {
	return &inMemoryUserRepo{
		users:        make(map[string]*biz.User),
		usersByPhone: make(map[string]*biz.User),
		usersByEmail: make(map[string]*biz.User),
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
	if user.Email != "" {
		r.usersByEmail[user.Email] = user
	}
	r.log.Infof("Created user in memory: id=%s, phone=%s, nickname=%s", user.ID, user.Phone, user.Nickname)
	return user, nil
}

func (r *inMemoryUserRepo) FindByPhone(ctx context.Context, phone string) (*biz.User, error) {
	user, ok := r.usersByPhone[phone]
	if !ok {
		return nil, nil
	}
	return user, nil
}

func (r *inMemoryUserRepo) FindByEmail(ctx context.Context, email string) (*biz.User, error) {
	user, ok := r.usersByEmail[email]
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
	if user.Email != "" {
		r.usersByEmail[user.Email] = user
	}
	return user, nil
}
