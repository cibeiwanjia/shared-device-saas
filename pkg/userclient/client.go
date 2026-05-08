package userclient

import (
	"context"

	v1 "shared-device-saas/api/user/v1"

	"github.com/go-kratos/kratos/v2/log"
	kratosgrpc "github.com/go-kratos/kratos/v2/transport/grpc"
	"google.golang.org/grpc"
)

// UserClient 用户服务gRPC客户端
type UserClient struct {
	client v1.UserServiceClient
	conn   *grpc.ClientConn
	log    *log.Helper
}

// NewUserClient 创建用户服务客户端
func NewUserClient(endpoint string, logger log.Logger) (*UserClient, error) {
	conn, err := kratosgrpc.DialInsecure(
		context.Background(),
		kratosgrpc.WithEndpoint(endpoint),
	)
	if err != nil {
		return nil, err
	}

	client := v1.NewUserServiceClient(conn)
	return &UserClient{
		client: client,
		conn:   conn,
		log:    log.NewHelper(logger),
	}, nil
}

// Close 关闭连接
func (c *UserClient) Close() error {
	return c.conn.Close()
}

// GetUserById 获取用户真实信息（内部接口，返回真实手机号）
func (c *UserClient) GetUserById(ctx context.Context, userID string) (*UserInfo, error) {
	reply, err := c.client.InternalGetUser(ctx, &v1.InternalGetUserRequest{
		Id: userID,
	})
	if err != nil {
		c.log.Errorf("InternalGetUser failed: %v", err)
		return nil, err
	}

	// 返回真实手机号（内部服务调用）
	return &UserInfo{
		ID:    reply.Id,
		Phone: reply.Phone, // 真实值
		Email: reply.Email, // 真实值
	}, nil
}

// GetUserMe 获取当前用户信息（需要传入Token）
// 注意：这个方法需要携带认证信息
func (c *UserClient) GetUserMe(ctx context.Context) (*UserInfo, error) {
	reply, err := c.client.GetUserMe(ctx, &v1.GetUserMeRequest{})
	if err != nil {
		c.log.Errorf("GetUserMe failed: %v", err)
		return nil, err
	}

	return &UserInfo{
		ID:       reply.Id,
		Phone:    reply.Phone, // 脱敏
		Nickname: reply.Nickname,
		Email:    reply.Email,
		Role:     reply.Role,
	}, nil
}

// UserInfo 用户信息
type UserInfo struct {
	ID       string
	Phone    string
	Nickname string
	Avatar   string
	Email    string
	Role     string
}