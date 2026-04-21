package service

import (
	"context"

	v1 "shared-device-saas/api/user/v1"
	"shared-device-saas/app/user/internal/biz"
)

// UserService 用户服务
type UserService struct {
	v1.UnimplementedUserServiceServer

	uc *biz.UserUsecase
}

// NewUserService 创建用户服务
func NewUserService(uc *biz.UserUsecase) *UserService {
	return &UserService{uc: uc}
}

// Register 用户注册
func (s *UserService) Register(ctx context.Context, req *v1.RegisterRequest) (*v1.RegisterReply, error) {
	user, token, err := s.uc.Register(ctx, &biz.User{
		Username: req.Username,
		Password: req.Password,
		Email:    req.Email,
		Phone:    req.Phone,
	})
	if err != nil {
		return nil, err
	}
	return &v1.RegisterReply{
		Id:       user.ID,
		Username: user.Username,
		Token:    token,
	}, nil
}

// Login 用户登录
func (s *UserService) Login(ctx context.Context, req *v1.LoginRequest) (*v1.LoginReply, error) {
	user, token, err := s.uc.Login(ctx, req.Username, req.Password)
	if err != nil {
		return nil, err
	}
	return &v1.LoginReply{
		Id:       user.ID,
		Username: user.Username,
		Token:    token,
	}, nil
}

// GetUser 获取用户信息
func (s *UserService) GetUser(ctx context.Context, req *v1.GetUserRequest) (*v1.GetUserReply, error) {
	user, err := s.uc.GetUser(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &v1.GetUserReply{
		Id:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		Phone:     user.Phone,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
	}, nil
}

// UpdateUser 更新用户信息
func (s *UserService) UpdateUser(ctx context.Context, req *v1.UpdateUserRequest) (*v1.UpdateUserReply, error) {
	user, err := s.uc.UpdateUser(ctx, req.Id, req.Email, req.Phone)
	if err != nil {
		return nil, err
	}
	return &v1.UpdateUserReply{
		Id:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		Phone:    user.Phone,
	}, nil
}