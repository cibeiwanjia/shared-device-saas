package biz

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	v1 "shared-device-saas/api/user/v1"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/log"
)

var (
	// ErrUserNotFound 用户不存在
	ErrUserNotFound = errors.NotFound(v1.ErrorReason_USER_NOT_FOUND.String(), "user not found")
	// ErrUserAlreadyExists 用户已存在
	ErrUserAlreadyExists = errors.Conflict(v1.ErrorReason_USER_ALREADY_EXISTS.String(), "user already exists")
	// ErrInvalidPassword 密码错误
	ErrInvalidPassword = errors.Unauthorized(v1.ErrorReason_INVALID_PASSWORD.String(), "invalid password")
)

// User 用户实体
type User struct {
	ID        int64
	Username  string
	Password  string
	Email     string
	Phone     string
	CreatedAt int64
	UpdatedAt int64
}

// UserRepo 用户仓储接口
type UserRepo interface {
	Create(context.Context, *User) (*User, error)
	FindByUsername(context.Context, string) (*User, error)
	FindByID(context.Context, int64) (*User, error)
	Update(context.Context, *User) (*User, error)
}

// UserUsecase 用户用例
type UserUsecase struct {
	repo UserRepo
	log  *log.Helper
}

// NewUserUsecase 创建用户用例
func NewUserUsecase(repo UserRepo, logger log.Logger) *UserUsecase {
	return &UserUsecase{
		repo: repo,
		log:  log.NewHelper(logger),
	}
}

// Register 用户注册
func (uc *UserUsecase) Register(ctx context.Context, user *User) (*User, string, error) {
	// 检查用户是否已存在
	existing, err := uc.repo.FindByUsername(ctx, user.Username)
	if err == nil && existing != nil {
		return nil, "", ErrUserAlreadyExists
	}

	// 设置创建时间
	now := time.Now().Unix()
	user.CreatedAt = now
	user.UpdatedAt = now

	// 创建用户
	created, err := uc.repo.Create(ctx, user)
	if err != nil {
		return nil, "", err
	}

	// 生成简单 token (实际项目应使用 JWT)
	token := generateToken()

	uc.log.Infof("User registered: %s", created.Username)
	return created, token, nil
}

// Login 用户登录
func (uc *UserUsecase) Login(ctx context.Context, username, password string) (*User, string, error) {
	user, err := uc.repo.FindByUsername(ctx, username)
	if err != nil || user == nil {
		return nil, "", ErrUserNotFound
	}

	// 验证密码 (实际项目应使用 bcrypt)
	if user.Password != password {
		return nil, "", ErrInvalidPassword
	}

	// 生成 token
	token := generateToken()

	uc.log.Infof("User logged in: %s", user.Username)
	return user, token, nil
}

// GetUser 获取用户信息
func (uc *UserUsecase) GetUser(ctx context.Context, id int64) (*User, error) {
	user, err := uc.repo.FindByID(ctx, id)
	if err != nil || user == nil {
		return nil, ErrUserNotFound
	}
	return user, nil
}

// UpdateUser 更新用户信息
func (uc *UserUsecase) UpdateUser(ctx context.Context, id int64, email, phone string) (*User, error) {
	user, err := uc.repo.FindByID(ctx, id)
	if err != nil || user == nil {
		return nil, ErrUserNotFound
	}

	if email != "" {
		user.Email = email
	}
	if phone != "" {
		user.Phone = phone
	}
	user.UpdatedAt = time.Now().Unix()

	return uc.repo.Update(ctx, user)
}

// generateToken 生成随机 token
func generateToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}