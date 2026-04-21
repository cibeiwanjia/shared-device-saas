package data

import (
	"context"
	"sync"

	"shared-device-saas/app/user/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

// userRepo 用户仓储实现 (内存存储，实际项目应使用数据库)
type userRepo struct {
	data *Data
	log  *log.Helper
	// 内存存储 (简化实现)
	users    map[int64]*biz.User
	usersMap map[string]*biz.User // username -> user
	nextID   int64
	mu       sync.RWMutex
}

// NewUserRepo 创建用户仓储
func NewUserRepo(data *Data, logger log.Logger) biz.UserRepo {
	return &userRepo{
		data:     data,
		log:      log.NewHelper(logger),
		users:    make(map[int64]*biz.User),
		usersMap: make(map[string]*biz.User),
		nextID:   1,
	}
}

// Create 创建用户
func (r *userRepo) Create(ctx context.Context, user *biz.User) (*biz.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	user.ID = r.nextID
	r.nextID++
	r.users[user.ID] = user
	r.usersMap[user.Username] = user

	r.log.Infof("Created user: %d, %s", user.ID, user.Username)
	return user, nil
}

// FindByUsername 根据用户名查找
func (r *userRepo) FindByUsername(ctx context.Context, username string) (*biz.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, ok := r.usersMap[username]
	if !ok {
		return nil, nil
	}
	return user, nil
}

// FindByID 根据ID查找
func (r *userRepo) FindByID(ctx context.Context, id int64) (*biz.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, ok := r.users[id]
	if !ok {
		return nil, nil
	}
	return user, nil
}

// Update 更新用户
func (r *userRepo) Update(ctx context.Context, user *biz.User) (*biz.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.users[user.ID] = user
	r.usersMap[user.Username] = user

	return user, nil
}