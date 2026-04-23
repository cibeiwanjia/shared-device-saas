package service

import (
	"context"
	"fmt"

	pb "shared-device-saas/api/user/v1"
)

// UploadImage 单张图片上传
func (s *UserService) UploadImage(ctx context.Context, req *pb.UploadImageRequest) (*pb.UploadImageReply, error) {
	tenantID := getTenantID(ctx)
	userID := getUserID(ctx)

	url, key, err := s.uploadUC.UploadImage(ctx, tenantID, userID, req.FileName, req.ContentType, req.Data, req.Scene)
	if err != nil {
		return nil, fmt.Errorf("upload image: %w", err)
	}

	return &pb.UploadImageReply{Url: url, Key: key}, nil
}

// BatchUploadImages 批量图片上传
func (s *UserService) BatchUploadImages(ctx context.Context, req *pb.BatchUploadImagesRequest) (*pb.BatchUploadImagesReply, error) {
	if len(req.Files) > 9 {
		return nil, fmt.Errorf("batch upload limit is 9 images, got %d", len(req.Files))
	}

	tenantID := getTenantID(ctx)
	userID := getUserID(ctx)

	results := make([]*pb.UploadImageReply, 0, len(req.Files))
	for _, f := range req.Files {
		url, key, err := s.uploadUC.UploadImage(ctx, tenantID, userID, f.FileName, f.ContentType, f.Data, f.Scene)
		if err != nil {
			s.log.Warnf("batch upload file %s failed: %v", f.FileName, err)
			continue
		}
		results = append(results, &pb.UploadImageReply{Url: url, Key: key})
	}

	return &pb.BatchUploadImagesReply{Results: results}, nil
}

// GetSignedURL 获取签名 URL
func (s *UserService) GetSignedURL(ctx context.Context, req *pb.GetSignedURLRequest) (*pb.GetSignedURLReply, error) {
	url, err := s.uploadUC.GetSignedURL(ctx, req.Key, req.ExpirySeconds)
	if err != nil {
		return nil, fmt.Errorf("get signed url: %w", err)
	}
	return &pb.GetSignedURLReply{Url: url}, nil
}
