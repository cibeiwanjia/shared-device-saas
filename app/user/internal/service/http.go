package service

import (
	"encoding/json"
	"io"
	"net/http"

	pb "shared-device-saas/api/user/v1"
	"shared-device-saas/app/user/internal/biz"
	"shared-device-saas/pkg/auth"

	khttp "github.com/go-kratos/kratos/v2/transport/http"
)

// decodeRequest 通用 JSON 解码
func decodeRequest(r *http.Request, req interface{}) error {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return err
	}
	defer r.Body.Close()
	if len(body) == 0 {
		return nil
	}
	return json.Unmarshal(body, req)
}

// LoginHTTP 登录
func (s *UserService) LoginHTTP(ctx khttp.Context) error {
	var req pb.LoginByPwdRequest
	if err := decodeRequest(ctx.Request(), &req); err != nil {
		return ctx.JSON(400, map[string]string{"error": err.Error()})
	}
	reply, err := s.Login(ctx, &req)
	if err != nil {
		return ctx.JSON(500, map[string]string{"error": err.Error()})
	}
	return ctx.JSON(200, reply)
}

// RefreshTokenHTTP 刷新 Token
func (s *UserService) RefreshTokenHTTP(ctx khttp.Context) error {
	var req pb.RefreshTokenRequest
	if err := decodeRequest(ctx.Request(), &req); err != nil {
		return ctx.JSON(400, map[string]string{"error": err.Error()})
	}
	reply, err := s.RefreshToken(ctx, &req)
	if err != nil {
		return ctx.JSON(500, map[string]string{"error": err.Error()})
	}
	return ctx.JSON(200, reply)
}

// LogoutHTTP 登出
func (s *UserService) LogoutHTTP(ctx khttp.Context) error {
	var req pb.LogoutRequest
	if err := decodeRequest(ctx.Request(), &req); err != nil {
		return ctx.JSON(400, map[string]string{"error": err.Error()})
	}
	reply, err := s.Logout(ctx, &req)
	if err != nil {
		return ctx.JSON(500, map[string]string{"error": err.Error()})
	}
	return ctx.JSON(200, reply)
}

// GetUserHTTP 获取用户信息
func (s *UserService) GetUserHTTP(ctx khttp.Context) error {
	reply, err := s.GetMe(ctx, &pb.GetMeRequest{})
	if err != nil {
		return ctx.JSON(500, map[string]string{"error": err.Error()})
	}
	return ctx.JSON(200, reply)
}

// UpdateUserHTTP 更新用户信息
func (s *UserService) UpdateUserHTTP(ctx khttp.Context) error {
	var req pb.UpdateUserRequest
	if err := decodeRequest(ctx.Request(), &req); err != nil {
		return ctx.JSON(400, map[string]string{"error": err.Error()})
	}
	reply, err := s.UpdateUser(ctx, &req)
	if err != nil {
		return ctx.JSON(500, map[string]string{"error": err.Error()})
	}
	return ctx.JSON(200, reply)
}

// UploadImageHTTP 图片上传
func (s *UserService) UploadImageHTTP(ctx khttp.Context) error {
	tenantID := auth.GetTenantID(ctx)
	userID := auth.GetUserIDInt64(ctx)

	var req struct {
		FileName    string `json:"file_name"`
		ContentType string `json:"content_type"`
		Data        []byte `json:"data"`
		Scene       string `json:"scene"`
	}
	if err := decodeRequest(ctx.Request(), &req); err != nil {
		return ctx.JSON(400, map[string]string{"error": err.Error()})
	}

	url, key, err := s.uploadUC.UploadImage(ctx, tenantID, userID, req.FileName, req.ContentType, req.Data, req.Scene)
	if err != nil {
		return ctx.JSON(500, map[string]string{"error": err.Error()})
	}
	return ctx.JSON(200, map[string]string{"url": url, "key": key})
}

// BatchUploadImagesHTTP 批量上传
func (s *UserService) BatchUploadImagesHTTP(ctx khttp.Context) error {
	return ctx.JSON(200, map[string]string{"message": "not implemented"})
}

// GetSignedURLHTTP 获取签名 URL
func (s *UserService) GetSignedURLHTTP(ctx khttp.Context) error {
	var req struct {
		Key           string `json:"key"`
		ExpirySeconds int64  `json:"expiry_seconds"`
	}
	if err := decodeRequest(ctx.Request(), &req); err != nil {
		return ctx.JSON(400, map[string]string{"error": err.Error()})
	}

	url, err := s.uploadUC.GetSignedURL(ctx, req.Key, req.ExpirySeconds)
	if err != nil {
		return ctx.JSON(500, map[string]string{"error": err.Error()})
	}
	return ctx.JSON(200, map[string]string{"url": url})
}

// GetWalletHTTP 查询钱包
func (s *UserService) GetWalletHTTP(ctx khttp.Context) error {
	tenantID := auth.GetTenantID(ctx)
	userID := auth.GetUserIDInt64(ctx)

	wallet, err := s.walletUC.GetWallet(ctx, tenantID, userID)
	if err != nil {
		return ctx.JSON(500, map[string]string{"error": err.Error()})
	}
	return ctx.JSON(200, wallet)
}

// ListTransactionsHTTP 查询流水
func (s *UserService) ListTransactionsHTTP(ctx khttp.Context) error {
	tenantID := auth.GetTenantID(ctx)
	userID := auth.GetUserIDInt64(ctx)

	var req struct {
		Type          int32  `json:"type"`
		CreatedAfter  string `json:"created_after"`
		CreatedBefore string `json:"created_before"`
		Limit         int    `json:"limit"`
		Cursor        string `json:"cursor"`
	}
	_ = decodeRequest(ctx.Request(), &req)

	filter := &biz.TransactionFilter{
		Type:          req.Type,
		CreatedAfter:  req.CreatedAfter,
		CreatedBefore: req.CreatedBefore,
	}

	result, err := s.walletUC.ListTransactions(ctx, tenantID, userID, filter, req.Limit, req.Cursor)
	if err != nil {
		return ctx.JSON(500, map[string]string{"error": err.Error()})
	}
	return ctx.JSON(200, result)
}

// CreateRechargeHTTP 创建充值
func (s *UserService) CreateRechargeHTTP(ctx khttp.Context) error {
	tenantID := auth.GetTenantID(ctx)
	userID := auth.GetUserIDInt64(ctx)

	var req struct {
		Amount        int64 `json:"amount"`
		PaymentMethod int32 `json:"payment_method"`
	}
	if err := decodeRequest(ctx.Request(), &req); err != nil {
		return ctx.JSON(400, map[string]string{"error": err.Error()})
	}

	order, payParams, err := s.rechargeUC.CreateRecharge(ctx, tenantID, userID, req.Amount, req.PaymentMethod)
	if err != nil {
		return ctx.JSON(500, map[string]string{"error": err.Error()})
	}
	return ctx.JSON(200, map[string]interface{}{"order_no": order.OrderNo, "amount": order.Amount, "pay_params": payParams})
}

// RechargeCallbackHTTP 支付回调
func (s *UserService) RechargeCallbackHTTP(ctx khttp.Context) error {
	var req struct {
		PaymentMethod int32  `json:"payment_method"`
		Payload       string `json:"payload"`
		Signature     string `json:"signature"`
	}
	if err := decodeRequest(ctx.Request(), &req); err != nil {
		return ctx.JSON(400, map[string]string{"error": err.Error()})
	}

	err := s.rechargeUC.HandleCallback(ctx, req.PaymentMethod, req.Payload, req.Signature)
	if err != nil {
		return ctx.JSON(500, map[string]string{"error": err.Error()})
	}
	return ctx.JSON(200, map[string]bool{"success": true})
}

// ListRechargesHTTP 查询充值记录
func (s *UserService) ListRechargesHTTP(ctx khttp.Context) error {
	tenantID := auth.GetTenantID(ctx)
	userID := auth.GetUserIDInt64(ctx)

	var req struct {
		Status        int32  `json:"status"`
		CreatedAfter  string `json:"created_after"`
		CreatedBefore string `json:"created_before"`
		Limit         int    `json:"limit"`
		Cursor        string `json:"cursor"`
	}
	_ = decodeRequest(ctx.Request(), &req)

	orders, nextCursor, hasMore, err := s.rechargeUC.ListRecharges(ctx, tenantID, userID, req.Status, req.CreatedAfter, req.CreatedBefore, req.Limit, req.Cursor)
	if err != nil {
		return ctx.JSON(500, map[string]string{"error": err.Error()})
	}
	return ctx.JSON(200, map[string]interface{}{"recharges": orders, "next_cursor": nextCursor, "has_more": hasMore})
}
