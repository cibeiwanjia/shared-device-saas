package service

import (
	"encoding/json"
	"io"
	"net/http"

	pb "shared-device-saas/api/user/v1"
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

// encodeResponse 通用 JSON 编码
func encodeResponse(w http.ResponseWriter, resp interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(resp)
}

// LoginHTTP 登录
func (s *UserService) LoginHTTP(ctx khttp.Context) error {
	var req pb.LoginRequest
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
	_, err := s.Logout(ctx, &pb.LogoutRequest{})
	if err != nil {
		return ctx.JSON(500, map[string]string{"error": err.Error()})
	}
	return ctx.JSON(200, map[string]string{"message": "ok"})
}

// GetUserHTTP 获取用户信息
func (s *UserService) GetUserHTTP(ctx khttp.Context) error {
	reply, err := s.GetUser(ctx, &pb.GetUserRequest{})
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

// ListOrdersHTTP 多维订单查询
func (s *UserService) ListOrdersHTTP(ctx khttp.Context) error {
	var req pb.ListOrdersRequest
	if err := decodeRequest(ctx.Request(), &req); err != nil {
		return ctx.JSON(400, map[string]string{"error": err.Error()})
	}
	reply, err := s.ListOrders(ctx, &req)
	if err != nil {
		return ctx.JSON(500, map[string]string{"error": err.Error()})
	}
	return ctx.JSON(200, reply)
}

// UploadImageHTTP 图片上传
func (s *UserService) UploadImageHTTP(ctx khttp.Context) error {
	var req pb.UploadImageRequest
	if err := decodeRequest(ctx.Request(), &req); err != nil {
		return ctx.JSON(400, map[string]string{"error": err.Error()})
	}
	reply, err := s.UploadImage(ctx, &req)
	if err != nil {
		return ctx.JSON(500, map[string]string{"error": err.Error()})
	}
	return ctx.JSON(200, reply)
}

// BatchUploadImagesHTTP 批量上传
func (s *UserService) BatchUploadImagesHTTP(ctx khttp.Context) error {
	var req pb.BatchUploadImagesRequest
	if err := decodeRequest(ctx.Request(), &req); err != nil {
		return ctx.JSON(400, map[string]string{"error": err.Error()})
	}
	reply, err := s.BatchUploadImages(ctx, &req)
	if err != nil {
		return ctx.JSON(500, map[string]string{"error": err.Error()})
	}
	return ctx.JSON(200, reply)
}

// GetSignedURLHTTP 获取签名 URL
func (s *UserService) GetSignedURLHTTP(ctx khttp.Context) error {
	var req pb.GetSignedURLRequest
	if err := decodeRequest(ctx.Request(), &req); err != nil {
		return ctx.JSON(400, map[string]string{"error": err.Error()})
	}
	reply, err := s.GetSignedURL(ctx, &req)
	if err != nil {
		return ctx.JSON(500, map[string]string{"error": err.Error()})
	}
	return ctx.JSON(200, reply)
}

// GetWalletHTTP 查询钱包
func (s *UserService) GetWalletHTTP(ctx khttp.Context) error {
	reply, err := s.GetWallet(ctx, nil)
	if err != nil {
		return ctx.JSON(500, map[string]string{"error": err.Error()})
	}
	return ctx.JSON(200, reply)
}

// ListTransactionsHTTP 查询流水
func (s *UserService) ListTransactionsHTTP(ctx khttp.Context) error {
	var req pb.ListTransactionsRequest
	if err := decodeRequest(ctx.Request(), &req); err != nil {
		return ctx.JSON(400, map[string]string{"error": err.Error()})
	}
	reply, err := s.ListTransactions(ctx, &req)
	if err != nil {
		return ctx.JSON(500, map[string]string{"error": err.Error()})
	}
	return ctx.JSON(200, reply)
}

// CreateRechargeHTTP 创建充值
func (s *UserService) CreateRechargeHTTP(ctx khttp.Context) error {
	var req pb.CreateRechargeRequest
	if err := decodeRequest(ctx.Request(), &req); err != nil {
		return ctx.JSON(400, map[string]string{"error": err.Error()})
	}
	reply, err := s.CreateRecharge(ctx, &req)
	if err != nil {
		return ctx.JSON(500, map[string]string{"error": err.Error()})
	}
	return ctx.JSON(200, reply)
}

// RechargeCallbackHTTP 支付回调
func (s *UserService) RechargeCallbackHTTP(ctx khttp.Context) error {
	var req pb.RechargeCallbackRequest
	if err := decodeRequest(ctx.Request(), &req); err != nil {
		return ctx.JSON(400, map[string]string{"error": err.Error()})
	}
	reply, err := s.RechargeCallback(ctx, &req)
	if err != nil {
		return ctx.JSON(500, map[string]string{"error": err.Error()})
	}
	return ctx.JSON(200, reply)
}

// ListRechargesHTTP 查询充值记录
func (s *UserService) ListRechargesHTTP(ctx khttp.Context) error {
	var req pb.ListRechargesRequest
	if err := decodeRequest(ctx.Request(), &req); err != nil {
		return ctx.JSON(400, map[string]string{"error": err.Error()})
	}
	reply, err := s.ListRecharges(ctx, &req)
	if err != nil {
		return ctx.JSON(500, map[string]string{"error": err.Error()})
	}
	return ctx.JSON(200, reply)
}

// GetUserContext 从 Kratos HTTP Context 提取用户信息
func GetUserContext(ctx khttp.Context) (tenantID, userID int64) {
	c := ctx
	tenantID = auth.GetTenantID(c)
	userID = auth.GetUserID(c)
	return
}
