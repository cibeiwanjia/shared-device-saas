package biz

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"shared-device-saas/pkg/storage"

	"github.com/go-kratos/kratos/v2/log"
)

// UploadUsecase 图片上传业务逻辑
type UploadUsecase struct {
	provider storage.StorageProvider
	bucket   string
	log      *log.Helper
}

// 支持的文件类型
var allowedTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

// 场景 → 路径前缀
var scenePaths = map[string]string{
	"avatar":     "avatars",
	"order":      "orders",
	"work_order": "work-orders",
}

// NewUploadUsecase 创建 UploadUsecase
func NewUploadUsecase(provider storage.StorageProvider, bucket string, logger log.Logger) *UploadUsecase {
	return &UploadUsecase{provider: provider, bucket: bucket, log: log.NewHelper(logger)}
}

// UploadImage 单张图片上传
func (uc *UploadUsecase) UploadImage(ctx context.Context, tenantID, userID int64, fileName, contentType string, data []byte, scene string) (url, key string, err error) {
	// 校验文件类型
	if !allowedTypes[contentType] {
		return "", "", fmt.Errorf("unsupported file type: %s, only jpg/png/webp allowed", contentType)
	}

	// 校验文件大小（5MB for avatar, 10MB for others）
	maxSize := int64(5 * 1024 * 1024)
	if scene != "avatar" {
		maxSize = 10 * 1024 * 1024
	}
	if int64(len(data)) > maxSize {
		return "", "", fmt.Errorf("file size exceeds %d bytes limit", maxSize)
	}

	// 生成存储路径: {tenant_id}/{scene}/{user_id}/{timestamp}.{ext}
	ext := path.Ext(fileName)
	if ext == "" {
		ext = ".jpg"
	}
	prefix := scenePaths[scene]
	if prefix == "" {
		prefix = "misc"
	}
	key = fmt.Sprintf("%d/%s/%d/%d%s", tenantID, prefix, userID, time.Now().UnixMilli(), ext)

	// 上传
	url, err = uc.provider.Upload(ctx, uc.bucket, key, strings.NewReader(string(data)), contentType)
	if err != nil {
		return "", "", fmt.Errorf("upload to storage: %w", err)
	}

	return url, key, nil
}

// GetSignedURL 获取签名 URL
func (uc *UploadUsecase) GetSignedURL(ctx context.Context, key string, expirySeconds int64) (string, error) {
	if expirySeconds <= 0 {
		expirySeconds = 1800 // 默认 30 分钟
	}
	return uc.provider.GetSignedURL(ctx, uc.bucket, key, time.Duration(expirySeconds)*time.Second)
}

// DeleteImage 删除图片
func (uc *UploadUsecase) DeleteImage(ctx context.Context, key string) error {
	return uc.provider.Delete(ctx, uc.bucket, key)
}
