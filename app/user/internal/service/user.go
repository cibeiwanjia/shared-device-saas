package service

import (
	"context"

	v1 "shared-device-saas/api/user/v1"
	"shared-device-saas/app/user/internal/biz"
	"shared-device-saas/pkg/auth"
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

// Register 用户注册（手机号 + 验证码 + 密码 + 确认密码）
func (s *UserService) Register(ctx context.Context, req *v1.RegisterRequest) (*v1.RegisterReply, error) {
	// 调用 biz 层注册方法（不再传 nickname）
	id, err := s.uc.Register(ctx, req.Phone, req.SmsCode, req.Password, req.ConfirmPassword, req.InviteCode)
	if err != nil {
		return nil, err
	}
	// 只返回账号（id）
	return &v1.RegisterReply{
		Id: id,
	}, nil
}

// Login 账号密码登录（手机号或邮箱 + 密码）
func (s *UserService) Login(ctx context.Context, req *v1.LoginByPwdRequest) (*v1.LoginReply, error) {
	// phone 和 email 二选一
	user, tokenPair, err := s.uc.LoginByPwd(ctx, req.Phone, req.Email, req.Password)
	if err != nil {
		return nil, err
	}
	return &v1.LoginReply{
		Id:           user.ID,        // 账号
		Phone:        maskPhone(user.Phone),
		Nickname:     user.Nickname,  // 昵称（登录时才能看到）
		Avatar:       user.Avatar,
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:    tokenPair.ExpiresIn,
	}, nil
}

// SendSmsCode 发送短信验证码
func (s *UserService) SendSmsCode(ctx context.Context, req *v1.SendSmsRequest) (*v1.SendSmsReply, error) {
	expire, err := s.uc.SendSms(ctx, req.Phone)
	if err != nil {
		return nil, err
	}
	return &v1.SendSmsReply{
		Success: true,
		Expire:  expire,
	}, nil
}

// LoginBySms 短信验证码登录（自动注册）
func (s *UserService) LoginBySms(ctx context.Context, req *v1.LoginBySmsRequest) (*v1.LoginReply, error) {
	user, tokenPair, err := s.uc.LoginBySms(ctx, req.Phone, req.Code)
	if err != nil {
		return nil, err
	}
	return &v1.LoginReply{
		Id:           user.ID,
		Phone:        maskPhone(user.Phone),
		Nickname:     user.Nickname,
		Avatar:       user.Avatar,
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:    tokenPair.ExpiresIn,
	}, nil
}

// RefreshToken 刷新 Token
func (s *UserService) RefreshToken(ctx context.Context, req *v1.RefreshTokenRequest) (*v1.LoginReply, error) {
	tokenPair, err := s.uc.RefreshToken(ctx, req.RefreshToken)
	if err != nil {
		return nil, err
	}
	return &v1.LoginReply{
		AccessToken:  tokenPair.AccessToken,
		RefreshToken: tokenPair.RefreshToken,
		ExpiresIn:    tokenPair.ExpiresIn,
	}, nil
}

// GetUserMe 获取当前登录用户信息（从 JWT Token 自动识别）
func (s *UserService) GetUserMe(ctx context.Context, req *v1.GetUserMeRequest) (*v1.GetUserMeReply, error) {
	userID := auth.GetUserID(ctx)
	if userID == "" {
		return nil, biz.ErrInvalidToken
	}

	user, err := s.uc.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &v1.GetUserMeReply{
		Id:         user.ID,
		Phone:      maskPhone(user.Phone),
		Nickname:   user.Nickname,
		Avatar:     user.Avatar,
		Email:      user.Email,
		Role:       user.Role,
		CreateTime: user.CreateTime, // RFC3339格式
	}, nil
}

// UpdateUserMe 部分更新当前登录用户信息（从 JWT Token 自动识别）
func (s *UserService) UpdateUserMe(ctx context.Context, req *v1.UpdateUserMeRequest) (*v1.UpdateUserMeReply, error) {
	userID := auth.GetUserID(ctx)
	if userID == "" {
		return nil, biz.ErrInvalidToken
	}

	user, err := s.uc.UpdateUser(ctx, userID, req.Nickname, req.Avatar)
	if err != nil {
		return nil, err
	}
	return &v1.UpdateUserMeReply{
		Id:       user.ID,
		Nickname: user.Nickname,
		Avatar:   user.Avatar,
	}, nil
}

// GetUserById 获取指定用户信息（管理员操作他人，需要权限检查）
func (s *UserService) GetUserById(ctx context.Context, req *v1.GetUserByIdRequest) (*v1.GetUserByIdReply, error) {
	user, err := s.uc.GetUser(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &v1.GetUserByIdReply{
		Id:         user.ID,
		Phone:      maskPhone(user.Phone),
		Nickname:   user.Nickname,
		Avatar:     user.Avatar,
		CreateTime: user.CreateTime, // RFC3339格式
	}, nil
}

// UpdateUserById 部分更新指定用户信息（管理员操作他人，需要权限检查）
func (s *UserService) UpdateUserById(ctx context.Context, req *v1.UpdateUserByIdRequest) (*v1.UpdateUserByIdReply, error) {
	user, err := s.uc.UpdateUser(ctx, req.Id, req.Nickname, req.Avatar)
	if err != nil {
		return nil, err
	}
	return &v1.UpdateUserByIdReply{
		Id:       user.ID,
		Nickname: user.Nickname,
		Avatar:   user.Avatar,
	}, nil
}

// UpdateProfile 部分更新个人信息（邮箱，从 JWT Token 自动识别）
func (s *UserService) UpdateProfile(ctx context.Context, req *v1.UpdateProfileRequest) (*v1.UpdateProfileReply, error) {
	userID := auth.GetUserID(ctx)
	if userID == "" {
		return nil, biz.ErrInvalidToken
	}

	user, err := s.uc.UpdateProfile(ctx, userID, req.Email)
	if err != nil {
		return nil, err
	}
	return &v1.UpdateProfileReply{
		Email: user.Email,
	}, nil
}

// Logout 退出登录
func (s *UserService) Logout(ctx context.Context, req *v1.LogoutRequest) (*v1.LogoutReply, error) {
	success, message, err := s.uc.Logout(ctx, req.RefreshToken)
	if err != nil {
		return nil, err
	}
	return &v1.LogoutReply{
		Success: success,
		Message: message,
	}, nil
}

// InternalGetUser 内部接口：获取用户真实信息（仅服务间调用，返回真实手机号）
func (s *UserService) InternalGetUser(ctx context.Context, req *v1.InternalGetUserRequest) (*v1.InternalGetUserReply, error) {
	user, err := s.uc.GetUser(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	// 返回真实手机号和邮箱（不脱敏）
	return &v1.InternalGetUserReply{
		Id:    user.ID,
		Phone: user.Phone, // 真实值
		Email: user.Email, // 真实值
	}, nil
}

// maskPhone 手机号脱敏
func maskPhone(phone string) string {
	if len(phone) < 7 {
		return phone
	}
	return phone[:3] + "****" + phone[len(phone)-4:]
}