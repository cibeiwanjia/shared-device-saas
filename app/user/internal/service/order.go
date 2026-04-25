package service

import (
	"context"

	pb "shared-device-saas/api/user/v1"
	"shared-device-saas/app/user/internal/biz"
)

// ListOrders 多维订单查询
func (s *UserService) ListOrders(ctx context.Context, req *pb.ListOrdersRequest) (*pb.ListOrdersReply, error) {
	tenantID := getTenantID(ctx)
	userID := getUserID(ctx)

	var filter *biz.OrderFilter
	if req.Filter != nil {
		filter = &biz.OrderFilter{
			CreatedAfter:  req.Filter.CreatedAfter,
			CreatedBefore: req.Filter.CreatedBefore,
			Source:        int32(req.Filter.Source),
			Status:        int32(req.Filter.Status),
			MinAmount:     int32(req.Filter.MinAmount),
			MaxAmount:     int32(req.Filter.MaxAmount),
			PaymentMethod: int32(req.Filter.PaymentMethod),
		}
	}

	var sort *biz.OrderSort
	if req.Sort != nil {
		sort = &biz.OrderSort{
			Field:     req.Sort.Field,
			Direction: req.Sort.Direction,
		}
	}

	result, err := s.orderUC.ListOrders(ctx, tenantID, userID, filter, sort, int(req.Limit), req.Cursor)
	if err != nil {
		return nil, err
	}

	orders := make([]*pb.OrderItem, 0, len(result.Orders))
	for _, o := range result.Orders {
		orders = append(orders, &pb.OrderItem{
			Id:            o.ID,
			OrderNo:       o.OrderNo,
			Source:        pb.OrderSource(o.Source),
			Status:        pb.OrderStatus(o.Status),
			TotalAmount:   o.TotalAmount,
			Currency:      o.Currency,
			PaymentMethod: pb.PaymentMethod(o.PaymentMethod),
			Title:         o.Title,
			Description:   o.Description,
			CreatedAt:     o.CreatedAt,
			UpdatedAt:     o.UpdatedAt,
		})
	}

	return &pb.ListOrdersReply{
		Orders:     orders,
		NextCursor: result.NextCursor,
		HasMore:    result.HasMore,
		TotalCount: result.TotalCount,
	}, nil
}
