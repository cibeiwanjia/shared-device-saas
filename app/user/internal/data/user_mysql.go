package data

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"

	"shared-device-saas/app/user/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

// userMysqlRepo 用户仓储 MySQL 实现（主库）
type userMysqlRepo struct {
	db  *sql.DB
	log *log.Helper
}

// newMysqlUserRepo 创建 MySQL 用户仓储
func newMysqlUserRepo(db *sql.DB, logger log.Logger) *userMysqlRepo {
	return &userMysqlRepo{db: db, log: log.NewHelper(logger)}
}

// Create 创建用户
func (r *userMysqlRepo) Create(ctx context.Context, user *biz.User) (*biz.User, error) {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO users (username, password, email, phone, nickname, avatar, invite_code, status, role, create_time, update_time)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		user.Username, user.Password, user.Email, user.Phone,
		user.Nickname, user.Avatar, user.InviteCode,
		user.Status, user.Role, user.CreateTime, user.UpdateTime,
	)
	if err != nil {
		r.log.Errorf("MySQL Create user error: %v", err)
		return nil, fmt.Errorf("mysql create user: %w", err)
	}

	id, _ := result.LastInsertId()
	user.ID = strconv.FormatInt(id, 10)
	r.log.Infof("Created user in MySQL: phone=%s, id=%s", user.Phone, user.ID)
	return user, nil
}

// FindByPhone 根据手机号查找
func (r *userMysqlRepo) FindByPhone(ctx context.Context, phone string) (*biz.User, error) {
	var u biz.User
	var id int64
	err := r.db.QueryRowContext(ctx,
		`SELECT id, username, password, email, phone, nickname, avatar, invite_code, status, role, create_time, update_time
		 FROM users WHERE phone = ?`, phone,
	).Scan(&id, &u.Username, &u.Password, &u.Email, &u.Phone,
		&u.Nickname, &u.Avatar, &u.InviteCode,
		&u.Status, &u.Role, &u.CreateTime, &u.UpdateTime)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("mysql find by phone: %w", err)
	}
	u.ID = strconv.FormatInt(id, 10)
	return &u, nil
}

// FindByUsername 根据用户名查找
func (r *userMysqlRepo) FindByUsername(ctx context.Context, username string) (*biz.User, error) {
	if username == "" {
		return nil, nil
	}
	var u biz.User
	var id int64
	err := r.db.QueryRowContext(ctx,
		`SELECT id, username, password, email, phone, nickname, avatar, invite_code, status, role, create_time, update_time
		 FROM users WHERE username = ?`, username,
	).Scan(&id, &u.Username, &u.Password, &u.Email, &u.Phone,
		&u.Nickname, &u.Avatar, &u.InviteCode,
		&u.Status, &u.Role, &u.CreateTime, &u.UpdateTime)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("mysql find by username: %w", err)
	}
	u.ID = strconv.FormatInt(id, 10)
	return &u, nil
}

// FindByID 根据 ID 查找
func (r *userMysqlRepo) FindByID(ctx context.Context, id string) (*biz.User, error) {
	nid, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, nil
	}
	var u biz.User
	err = r.db.QueryRowContext(ctx,
		`SELECT id, username, password, email, phone, nickname, avatar, invite_code, status, role, create_time, update_time
		 FROM users WHERE id = ?`, nid,
	).Scan(&nid, &u.Username, &u.Password, &u.Email, &u.Phone,
		&u.Nickname, &u.Avatar, &u.InviteCode,
		&u.Status, &u.Role, &u.CreateTime, &u.UpdateTime)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("mysql find by id: %w", err)
	}
	u.ID = strconv.FormatInt(nid, 10)
	return &u, nil
}

// Update 更新用户
func (r *userMysqlRepo) Update(ctx context.Context, user *biz.User) (*biz.User, error) {
	nid, err := strconv.ParseInt(user.ID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid user id: %w", err)
	}
	_, err = r.db.ExecContext(ctx,
		`UPDATE users SET username=?, email=?, nickname=?, avatar=?, update_time=? WHERE id=?`,
		user.Username, user.Email, user.Nickname, user.Avatar, user.UpdateTime, nid)
	if err != nil {
		return nil, fmt.Errorf("mysql update user: %w", err)
	}
	return user, nil
}
