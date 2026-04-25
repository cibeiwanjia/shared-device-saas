package service

import (
	"context"

	pb "shared-device-saas/api/user/v1"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// Login 登录（桩实现，待接入短信验证码服务）
func (s *UserService) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginReply, error) {
	return &pb.LoginReply{}, nil
}

// RefreshToken 刷新 Token（桩实现）
func (s *UserService) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenReply, error) {
	return &pb.RefreshTokenReply{}, nil
}

// Logout 登出（桩实现）
func (s *UserService) Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutReply, error) {
	return &pb.LogoutReply{}, nil
}

// GetUser 获取用户信息（桩实现）
func (s *UserService) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.UserInfo, error) {
	return &pb.UserInfo{}, nil
}

// UpdateUser 更新用户信息（桩实现）
func (s *UserService) UpdateUser(ctx context.Context, req *pb.UpdateUserRequest) (*pb.UpdateUserReply, error) {
	return &pb.UpdateUserReply{}, nil
}

// ListSessions 查询会话列表（桩实现）
func (s *UserService) ListSessions(ctx context.Context, req *timestamppb.Timestamp) (*pb.ListSessionsReply, error) {
	return &pb.ListSessionsReply{}, nil
}

// RevokeSession 撤销会话（桩实现）
func (s *UserService) RevokeSession(ctx context.Context, req *pb.RevokeSessionRequest) (*pb.RevokeSessionReply, error) {
	return &pb.RevokeSessionReply{}, nil
}
