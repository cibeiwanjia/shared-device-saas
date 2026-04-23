package service

import (
	"context"

	pb "shared-device-saas/api/user/v1"
)

// CreateRecharge 创建充值订单
func (s *UserService) CreateRecharge(ctx context.Context, req *pb.CreateRechargeRequest) (*pb.CreateRechargeReply, error) {
	order, payParams, err := s.rechargeUC.CreateRecharge(ctx, getTenantID(ctx), getUserID(ctx), int64(req.Amount), int32(req.PaymentMethod))
	if err != nil {
		return nil, err
	}
	return &pb.CreateRechargeReply{
		OrderId:   order.ID,
		OrderNo:   order.OrderNo,
		PayParams: payParams,
	}, nil
}

// RechargeCallback 支付回调
func (s *UserService) RechargeCallback(ctx context.Context, req *pb.RechargeCallbackRequest) (*pb.RechargeCallbackReply, error) {
	var method int32
	switch req.Channel {
	case "wechat":
		method = 1
	case "alipay":
		method = 2
	default:
		return &pb.RechargeCallbackReply{Success: false}, nil
	}

	err := s.rechargeUC.HandleCallback(ctx, method, req.Payload, req.Signature)
	return &pb.RechargeCallbackReply{Success: err == nil}, err
}

// ListRecharges 查询充值记录
func (s *UserService) ListRecharges(ctx context.Context, req *pb.ListRechargesRequest) (*pb.ListRechargesReply, error) {
	orders, nextCursor, hasMore, err := s.rechargeUC.ListRecharges(ctx, getTenantID(ctx), getUserID(ctx), int32(req.Status), req.CreatedAfter, req.CreatedBefore, int(req.Limit), req.Cursor)
	if err != nil {
		return nil, err
	}

	items := make([]*pb.RechargeItem, 0, len(orders))
	for _, o := range orders {
		items = append(items, &pb.RechargeItem{
			Id:            o.ID,
			OrderNo:       o.OrderNo,
			Amount:        int32(o.Amount),
			PaymentMethod: pb.PaymentMethod(o.PaymentMethod),
			Status:        pb.RechargeStatus(o.Status),
			CreatedAt:     o.CreatedAt,
			PaidAt:        o.PaidAt,
		})
	}

	return &pb.ListRechargesReply{
		Recharges:  items,
		NextCursor: nextCursor,
		HasMore:    hasMore,
	}, nil
}
