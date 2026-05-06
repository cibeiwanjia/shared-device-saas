package biz

import (
	"context"

	"github.com/go-kratos/kratos/v2/log"
)

type TimeoutHandler struct {
	deliveryIn  *DeliveryInUsecase
	deliveryOut *DeliveryOutUsecase
	storageUc   *StorageUsecase
	orderRepo   OrderRepo
	log         *log.Helper
}

func NewTimeoutHandler(
	deliveryIn *DeliveryInUsecase,
	deliveryOut *DeliveryOutUsecase,
	storageUc *StorageUsecase,
	orderRepo OrderRepo,
	logger log.Logger,
) *TimeoutHandler {
	return &TimeoutHandler{
		deliveryIn:  deliveryIn,
		deliveryOut: deliveryOut,
		storageUc:   storageUc,
		orderRepo:   orderRepo,
		log:         log.NewHelper(logger),
	}
}

func (h *TimeoutHandler) HandleOrderTimeout(ctx context.Context, orderID int64) error {
	order, err := h.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return err
	}
	if order == nil {
		return nil
	}

	switch order.OrderType {
	case OrderTypeDeliveryIn:
		return h.deliveryIn.OnOrderTimeout(ctx, order)
	case OrderTypeDeliveryOut:
		return h.deliveryOut.OnOrderTimeout(ctx, order)
	case OrderTypeStorage:
		return h.storageUc.OnOrderTimeout(ctx, order)
	default:
		h.log.Warnf("unknown order type for timeout: %s", order.OrderType)
		return nil
	}
}
