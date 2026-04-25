package service

import (
<<<<<<< HEAD
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

// Register 用户注册（手机号 + 验证码 + 密码）
func (s *UserService) Register(ctx context.Context, req *v1.RegisterRequest) (*v1.RegisterReply, error) {
	id, err := s.uc.Register(ctx, req.Phone, req.SmsCode, req.Password, req.Nickname, req.InviteCode)
	if err != nil {
		return nil, err
	}
	return &v1.RegisterReply{
		Id: id,
	}, nil
}

// Login 账号密码登录 (LoginByPwd)
func (s *UserService) Login(ctx context.Context, req *v1.LoginByPwdRequest) (*v1.LoginReply, error) {
	user, tokenPair, err := s.uc.LoginByPwd(ctx, req.Username, req.Password)
	if err != nil {
		return nil, err
	}
	return &v1.LoginReply{
		Id:           user.ID,
		Username:     user.Username,
		Phone:        maskPhone(user.Phone),
		Nickname:     user.Nickname,
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
		Username:     user.Username,
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

// GetMe 获取当前登录用户信息（从 JWT Token 自动识别）
func (s *UserService) GetMe(ctx context.Context, req *v1.GetMeRequest) (*v1.GetMeReply, error) {
	// 从 Context 获取当前用户 ID（JWT 中间件已注入）
	userID := auth.GetUserID(ctx)
	if userID == "" {
		// 返回未授权错误
		return nil, biz.ErrInvalidToken
	}

	// 查询用户信息
	user, err := s.uc.GetUser(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &v1.GetMeReply{
		Id:         user.ID,
		Username:   user.Username,
		Phone:      maskPhone(user.Phone),
		Nickname:   user.Nickname,
		Avatar:     user.Avatar,
		Role:       user.Role,
		CreateTime: user.CreateTime,
	}, nil
}

// GetUser 获取用户信息（需要权限检查）
func (s *UserService) GetUser(ctx context.Context, req *v1.GetUserRequest) (*v1.GetUserReply, error) {
	user, err := s.uc.GetUser(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &v1.GetUserReply{
		Id:         user.ID,
		Username:   user.Username,
		Phone:      maskPhone(user.Phone),
		Nickname:   user.Nickname,
		Avatar:     user.Avatar,
		CreateTime: user.CreateTime,
	}, nil
}

// UpdateUser 更新用户信息（需要权限检查）
func (s *UserService) UpdateUser(ctx context.Context, req *v1.UpdateUserRequest) (*v1.UpdateUserReply, error) {
	user, err := s.uc.UpdateUser(ctx, req.Id, req.Nickname, req.Avatar)
	if err != nil {
		return nil, err
	}
	return &v1.UpdateUserReply{
		Id:       user.ID,
		Nickname: user.Nickname,
		Avatar:   user.Avatar,
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

// maskPhone 手机号脱敏
func maskPhone(phone string) string {
	if len(phone) < 7 {
		return phone
	}
	return phone[:3] + "****" + phone[len(phone)-4:]
}
=======
	pb "shared-device-saas/api/user/v1"
	"shared-device-saas/app/user/internal/biz"

	"github.com/go-kratos/kratos/v2/log"
)

// UserService 统一用户服务（实现 proto 定义的 UserServiceServer）
type UserService struct {
	pb.UnimplementedUserServiceServer

	orderUC    *biz.OrderUsecase
	uploadUC   *biz.UploadUsecase
	walletUC   *biz.WalletUsecase
	rechargeUC *biz.RechargeUsecase
	log        *log.Helper
}

// NewUserService 创建统一用户服务
func NewUserService(
	orderUC *biz.OrderUsecase,
	uploadUC *biz.UploadUsecase,
	walletUC *biz.WalletUsecase,
	rechargeUC *biz.RechargeUsecase,
	logger log.Logger,
) *UserService {
	return &UserService{
		orderUC:    orderUC,
		uploadUC:   uploadUC,
		walletUC:   walletUC,
		rechargeUC: rechargeUC,
		log:        log.NewHelper(logger),
	}
}
>>>>>>> dev/wangqinghua
