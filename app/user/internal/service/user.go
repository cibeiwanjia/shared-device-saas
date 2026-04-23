package service

import (
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
