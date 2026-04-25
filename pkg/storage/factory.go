package storage

import "fmt"

// StorageConfig 存储配置
type StorageConfig struct {
	Provider string // oss / minio / rustfs

	// 阿里云 OSS 配置
	OSSEndpoint        string
	OSSAccessKeyID     string
	OSSAccessKeySecret string
	OSSBucket          string

	// MinIO 配置
	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string
	MinioBucket    string
	MinioUseSSL    bool

	// RustFS 配置（兼容 S3 协议）
	RustfsEndpoint  string
	RustfsAccessKey string
	RustfsSecretKey string
	RustfsBucket    string
	RustfsUseSSL    bool
}

// NewStorageProvider 工厂方法：根据配置创建对应的存储 Provider
func NewStorageProvider(cfg *StorageConfig) (StorageProvider, error) {
	switch cfg.Provider {
	case "oss":
		return NewOSSProvider(cfg)
	case "minio":
		return NewMinioProvider(cfg)
	case "rustfs":
		return NewRustfsProvider(cfg)
	default:
		return nil, fmt.Errorf("unsupported storage provider: %s", cfg.Provider)
	}
}
