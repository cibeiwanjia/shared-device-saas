package service

import (
	"context"
	"fmt"
	"time"

	pb "shared-device-saas/api/storage/v1"
	"shared-device-saas/app/storage/internal/biz"
	"shared-device-saas/pkg/auth"

	"github.com/go-kratos/kratos/v2/log"
)

type StorageService struct {
	pb.UnimplementedStorageServiceServer

	deliveryIn  *biz.DeliveryInUsecase
	deliveryOut *biz.DeliveryOutUsecase
	storageUc   *biz.StorageUsecase
	pricing     *biz.PricingEngine
	pickup      *biz.PickupCodeManager
	allocator   *biz.CellAllocator
	orderRepo   biz.OrderRepo
	cellRepo    biz.CellRepo
	cabinetRepo biz.CabinetRepo
	timeoutMgr  *biz.TimeoutManager
	log         *log.Helper
}

func NewStorageService(
	deliveryIn *biz.DeliveryInUsecase,
	deliveryOut *biz.DeliveryOutUsecase,
	storageUc *biz.StorageUsecase,
	pricing *biz.PricingEngine,
	pickup *biz.PickupCodeManager,
	allocator *biz.CellAllocator,
	orderRepo biz.OrderRepo,
	cellRepo biz.CellRepo,
	cabinetRepo biz.CabinetRepo,
	timeoutMgr *biz.TimeoutManager,
	logger log.Logger,
) *StorageService {
	return &StorageService{
		deliveryIn: deliveryIn, deliveryOut: deliveryOut, storageUc: storageUc,
		pricing: pricing, pickup: pickup, allocator: allocator,
		orderRepo: orderRepo, cellRepo: cellRepo, cabinetRepo: cabinetRepo,
		timeoutMgr: timeoutMgr, log: log.NewHelper(logger),
	}
}

func (s *StorageService) InitiateDelivery(ctx context.Context, req *pb.InitiateDeliveryRequest) (*pb.InitiateDeliveryReply, error) {
	tenantID := auth.GetTenantID(ctx)
	userID := auth.GetUserIDInt64(ctx)
	order, err := s.deliveryIn.InitiateDelivery(ctx, tenantID, userID, req.CabinetId, req.CellType, "")
	if err != nil {
		return nil, err
	}
	return &pb.InitiateDeliveryReply{OrderNo: order.OrderNo, CellId: order.CellID, SlotIndex: order.SlotIndex}, nil
}

func (s *StorageService) Pickup(ctx context.Context, req *pb.PickupRequest) (*pb.PickupReply, error) {
	tenantID := auth.GetTenantID(ctx)
	orderID, err := s.pickup.Verify(ctx, tenantID, req.PickupCode)
	if err != nil {
		return nil, err
	}
	order, err := s.orderRepo.GetByID(ctx, orderID)
	if err != nil || order == nil {
		return nil, biz.ErrOrderNotFound
	}

	if order.Status != biz.OrderStatusDeposited && order.Status != biz.OrderStatusTimeout {
		return nil, biz.ErrOrderNotPickable
	}

	fee := order.TotalAmount
	status := "FREE"
	if fee > 0 {
		status = "PAYMENT_REQUIRED"
	}

	cabinet, _ := s.cabinetRepo.FindByID(ctx, order.CabinetID)
	cabinetName := ""
	if cabinet != nil {
		cabinetName = cabinet.Name
	}

	return &pb.PickupReply{
		OrderNo:     order.OrderNo,
		Fee:         fee,
		Status:      status,
		CabinetName: cabinetName,
		SlotIndex:   order.SlotIndex,
	}, nil
}

func (s *StorageService) ConfirmPickup(ctx context.Context, req *pb.ConfirmPickupRequest) (*pb.ConfirmPickupReply, error) {
	tenantID := auth.GetTenantID(ctx)
	orderID, err := s.pickup.Verify(ctx, tenantID, req.PickupCode)
	if err != nil {
		return nil, err
	}
	order, err := s.orderRepo.GetByID(ctx, orderID)
	if err != nil || order == nil {
		return nil, biz.ErrOrderNotFound
	}

	cell, err := s.cellRepo.FindByID(ctx, order.CellID)
	if err != nil || cell == nil {
		return nil, fmt.Errorf("cell not found")
	}

	if err := s.allocator.MarkOpening(ctx, cell.ID, biz.PendingActionPickup, order.ID); err != nil {
		return nil, err
	}

	_, err = s.deliveryIn.InitiateDelivery(ctx, tenantID, order.UserID, order.CabinetID, cell.CellType, order.OrderNo)
	_ = err

	return &pb.ConfirmPickupReply{Ok: true}, nil
}

func (s *StorageService) InitiateShipment(ctx context.Context, req *pb.InitiateShipmentRequest) (*pb.InitiateShipmentReply, error) {
	tenantID := auth.GetTenantID(ctx)
	userID := auth.GetUserIDInt64(ctx)
	order, err := s.deliveryOut.InitiateShipment(ctx, tenantID, userID, req.CabinetId, req.CellType)
	if err != nil {
		return nil, err
	}
	return &pb.InitiateShipmentReply{OrderNo: order.OrderNo, CellId: order.CellID, SlotIndex: order.SlotIndex}, nil
}

func (s *StorageService) InitiateStorage(ctx context.Context, req *pb.InitiateStorageRequest) (*pb.InitiateStorageReply, error) {
	tenantID := auth.GetTenantID(ctx)
	userID := auth.GetUserIDInt64(ctx)
	order, err := s.storageUc.InitiateStorage(ctx, tenantID, userID, req.CabinetId, req.CellType)
	if err != nil {
		return nil, err
	}
	return &pb.InitiateStorageReply{OrderNo: order.OrderNo, CellId: order.CellID, SlotIndex: order.SlotIndex}, nil
}

func (s *StorageService) RetrieveStorage(ctx context.Context, req *pb.RetrieveStorageRequest) (*pb.RetrieveStorageReply, error) {
	order, err := s.orderRepo.GetByOrderNo(ctx, req.OrderNo)
	if err != nil || order == nil {
		return nil, biz.ErrOrderNotFound
	}

	fee := order.TotalAmount
	status := "FREE"
	if fee > 0 {
		status = "PAYMENT_REQUIRED"
	}
	return &pb.RetrieveStorageReply{Fee: fee, Status: status}, nil
}

func (s *StorageService) ConfirmRetrieve(ctx context.Context, req *pb.ConfirmRetrieveRequest) (*pb.ConfirmRetrieveReply, error) {
	order, err := s.orderRepo.GetByOrderNo(ctx, req.OrderNo)
	if err != nil || order == nil {
		return nil, biz.ErrOrderNotFound
	}

	cell, err := s.cellRepo.FindByID(ctx, order.CellID)
	if err != nil || cell == nil {
		return nil, fmt.Errorf("cell not found")
	}

	if err := s.allocator.MarkOpening(ctx, cell.ID, biz.PendingActionPickup, order.ID); err != nil {
		return nil, err
	}

	return &pb.ConfirmRetrieveReply{Ok: true}, nil
}

func (s *StorageService) TempOpenCell(ctx context.Context, req *pb.TempOpenCellRequest) (*pb.TempOpenCellReply, error) {
	order, err := s.orderRepo.GetByOrderNo(ctx, req.OrderNo)
	if err != nil || order == nil {
		return nil, biz.ErrOrderNotFound
	}

	cell, err := s.cellRepo.FindByID(ctx, order.CellID)
	if err != nil || cell == nil {
		return nil, fmt.Errorf("cell not found")
	}

	if err := s.allocator.MarkOpening(ctx, cell.ID, biz.PendingActionTempOpen, order.ID); err != nil {
		return nil, err
	}

	return &pb.TempOpenCellReply{Ok: true}, nil
}

func (s *StorageService) GetOrder(ctx context.Context, req *pb.GetOrderRequest) (*pb.GetOrderReply, error) {
	order, err := s.orderRepo.GetByOrderNo(ctx, req.OrderNo)
	if err != nil || order == nil {
		return nil, biz.ErrOrderNotFound
	}
	reply := &pb.GetOrderReply{
		Id:          order.ID,
		OrderNo:     order.OrderNo,
		OrderType:   order.OrderType,
		Status:      order.Status,
		SlotIndex:   order.SlotIndex,
		TotalAmount: order.TotalAmount,
		CreatedAt:   formatTime(order.CreatedAt),
	}
	if order.PickupCode != nil {
		reply.PickupCode = *order.PickupCode
	}
	if order.DepositedAt != nil {
		reply.DepositedAt = formatTime(*order.DepositedAt)
	}
	return reply, nil
}

func (s *StorageService) ListMyOrders(ctx context.Context, req *pb.ListMyOrdersRequest) (*pb.ListMyOrdersReply, error) {
	tenantID := auth.GetTenantID(ctx)
	userID := auth.GetUserIDInt64(ctx)
	orders, total, err := s.orderRepo.ListByUser(ctx, tenantID, userID, req.OrderType, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	items := make([]*pb.OrderItem, 0, len(orders))
	for _, o := range orders {
		items = append(items, &pb.OrderItem{
			OrderNo:     o.OrderNo,
			OrderType:   o.OrderType,
			Status:      o.Status,
			SlotIndex:   o.SlotIndex,
			TotalAmount: o.TotalAmount,
			CreatedAt:   formatTime(o.CreatedAt),
		})
	}
	return &pb.ListMyOrdersReply{Items: items, Total: total}, nil
}

func (s *StorageService) ListCabinets(ctx context.Context, req *pb.ListCabinetsRequest) (*pb.ListCabinetsReply, error) {
	tenantID := auth.GetTenantID(ctx)
	cabinets, total, err := s.cabinetRepo.ListByTenant(ctx, tenantID, req.Status, req.Page, req.PageSize)
	if err != nil {
		return nil, err
	}
	items := make([]*pb.CabinetItem, 0, len(cabinets))
	for _, c := range cabinets {
		freeCount, _ := s.cabinetRepo.GetFreeCellCount(ctx, c.ID)
		items = append(items, &pb.CabinetItem{
			Id:           c.ID,
			Name:         c.Name,
			LocationName: c.LocationName,
			TotalCells:   c.TotalCells,
			FreeCells:    freeCount,
			Status:       c.Status,
		})
	}
	return &pb.ListCabinetsReply{Items: items, Total: total}, nil
}

func (s *StorageService) GetCabinetDetail(ctx context.Context, req *pb.GetCabinetDetailRequest) (*pb.GetCabinetDetailReply, error) {
	cabinet, err := s.cabinetRepo.FindByID(ctx, req.Id)
	if err != nil || cabinet == nil {
		return nil, fmt.Errorf("cabinet not found")
	}
	freeCount, _ := s.cabinetRepo.GetFreeCellCount(ctx, cabinet.ID)

	cells, _ := s.cellRepo.ListFreeByCabinet(ctx, cabinet.ID)
	cellDetails := make([]*pb.CellDetail, 0)
	if cells != nil {
		for _, c := range cells {
			cellDetails = append(cellDetails, &pb.CellDetail{
				Id:        c.ID,
				SlotIndex: c.SlotIndex,
				CellType:  c.CellType,
				Status:    c.Status,
			})
		}
	}

	return &pb.GetCabinetDetailReply{
		Id:           cabinet.ID,
		Name:         cabinet.Name,
		DeviceSn:     cabinet.DeviceSN,
		LocationName: cabinet.LocationName,
		TotalCells:   cabinet.TotalCells,
		FreeCells:    freeCount,
		Status:       cabinet.Status,
		Cells:        cellDetails,
	}, nil
}

func (s *StorageService) ForceOpenCell(ctx context.Context, req *pb.ForceOpenCellRequest) (*pb.ForceOpenCellReply, error) {
	return &pb.ForceOpenCellReply{Ok: true}, nil
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return fmt.Sprintf("%d", t.Unix())
}
