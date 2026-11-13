package storage

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
)

var ErrNotConfigured = errors.New("object storage is not configured")

type S3 struct {
	bucket string
	client *s3.S3
}

// NewFromEnv returns nil when no recording bucket is configured. The caller
// treats that as "recording unavailable", never as "record somewhere else".
func NewFromEnv() (*S3, error) {
	bucket := strings.TrimSpace(os.Getenv("RECORDING_S3_BUCKET"))
	if bucket == "" {
		return nil, nil
	}
	region := strings.TrimSpace(os.Getenv("RECORDING_S3_REGION"))
	if region == "" {
		region = "ap-south-1"
	}
	cfg := aws.NewConfig().WithRegion(region).WithS3ForcePathStyle(
		strings.EqualFold(os.Getenv("RECORDING_S3_FORCE_PATH_STYLE"), "true"),
	)
	if endpoint := strings.TrimSpace(os.Getenv("RECORDING_S3_ENDPOINT")); endpoint != "" {
		cfg = cfg.WithEndpoint(endpoint)
	}
	access := strings.TrimSpace(os.Getenv("RECORDING_S3_ACCESS_KEY"))
	secret := strings.TrimSpace(os.Getenv("RECORDING_S3_SECRET"))
	if access != "" && secret != "" {
		cfg = cfg.WithCredentials(credentials.NewStaticCredentials(access, secret, ""))
	}
	sess, err := session.NewSession(cfg)
	if err != nil {
		return nil, err
	}
	return &S3{bucket: bucket, client: s3.New(sess)}, nil
}

func (s *S3) Configured() bool { return s != nil && s.client != nil }

func (s *S3) PresignGet(_ context.Context, key string, ttl time.Duration) (string, error) {
	if !s.Configured() {
		return "", ErrNotConfigured
	}
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	req, _ := s.client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return req.Presign(ttl)
}

func (s *S3) Delete(_ context.Context, key string) error {
	if !s.Configured() {
		return ErrNotConfigured
	}
	_, err := s.client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}
