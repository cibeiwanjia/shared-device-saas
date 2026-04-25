package storage

import (
	"context"
	"io"
	"time"
)

// StorageProvider 存储后端抽象接口（工厂模式）
type StorageProvider interface {
	// Upload 上传文件
	Upload(ctx context.Context, bucket, key string, reader io.Reader, contentType string) (url string, err error)
	// Delete 删除文件
	Delete(ctx context.Context, bucket, key string) error
	// GetSignedURL 生成临时签名访问 URL
	GetSignedURL(ctx context.Context, bucket, key string, expiry time.Duration) (string, error)
}
