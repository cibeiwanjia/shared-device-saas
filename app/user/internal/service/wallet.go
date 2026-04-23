package service

import (
	"context"

	pb "shared-device-saas/api/user/v1"
	"shared-device-saas/app/user/internal/biz"

	"google.golang.org/protobuf/types/known/timestamppb"
)

// GetWallet 查询钱包余额
func (s *UserService) GetWallet(ctx context.Context, _ *timestamppb.Timestamp) (*pb.GetWalletReply, error) {
	wallet, err := s.walletUC.GetWallet(ctx, getTenantID(ctx), getUserID(ctx))
	if err != nil {
		return nil, err
	}
	return &pb.GetWalletReply{
		WalletId:         wallet.ID,
		UserId:           wallet.UserID,
		AvailableBalance: wallet.Balance,
		FrozenAmount:     wallet.FrozenAmount,
	}, nil
}

// ListTransactions 查询流水
func (s *UserService) ListTransactions(ctx context.Context, req *pb.ListTransactionsRequest) (*pb.ListTransactionsReply, error) {
	filter := &biz.TransactionFilter{
		Type:          int32(req.Type),
		CreatedAfter:  req.CreatedAfter,
		CreatedBefore: req.CreatedBefore,
	}

	result, err := s.walletUC.ListTransactions(ctx, getTenantID(ctx), getUserID(ctx), filter, int(req.Limit), req.Cursor)
	if err != nil {
		return nil, err
	}

	items := make([]*pb.TransactionItem, 0, len(result.Transactions))
	for _, t := range result.Transactions {
		items = append(items, &pb.TransactionItem{
			Id:           t.ID,
			Type:         pb.WalletTransactionType(t.Type),
			Amount:       int32(t.Amount),
			BalanceAfter: t.BalanceAfter,
			OrderNo:      t.OrderNo,
			Description:  t.Description,
			CreatedAt:    t.CreatedAt,
		})
	}

	return &pb.ListTransactionsReply{
		Transactions: items,
		NextCursor:   result.NextCursor,
		HasMore:      result.HasMore,
	}, nil
}
