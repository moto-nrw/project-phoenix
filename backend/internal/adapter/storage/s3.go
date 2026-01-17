// Package storage provides file storage implementations.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/moto-nrw/project-phoenix/internal/core/port"
	"github.com/sirupsen/logrus"
)

// S3Config holds configuration for S3-compatible storage.
type S3Config struct {
	Bucket          string
	Region          string
	Endpoint        string
	PublicURLPrefix string
	KeyPrefix       string

	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string

	ForcePathStyle bool
}

// S3Storage implements port.FileStorage using S3-compatible object storage.
type S3Storage struct {
	client          *s3.Client
	bucket          string
	publicURLPrefix string
	keyPrefix       string
	logger          *logrus.Logger
}

// NewS3Storage creates a new S3Storage adapter.
func NewS3Storage(ctx context.Context, cfg S3Config, logger *logrus.Logger) (*S3Storage, error) {
	if cfg.Bucket == "" {
		return nil, errors.New("storage: bucket is required")
	}
	if cfg.Region == "" {
		return nil, errors.New("storage: region is required")
	}
	if cfg.PublicURLPrefix == "" {
		return nil, errors.New("storage: public URL prefix is required")
	}
	if (cfg.AccessKeyID == "") != (cfg.SecretAccessKey == "") {
		return nil, errors.New("storage: access key and secret key must be provided together")
	}

	loadOpts := []func(*config.LoadOptions) error{
		config.WithRegion(cfg.Region),
	}

	if cfg.Endpoint != "" {
		endpoint := cfg.Endpoint
		// Custom endpoint needed for MinIO compatibility; uses deprecated but still-supported AWS SDK interface
		//nolint:staticcheck
		loadOpts = append(loadOpts, config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...any) (aws.Endpoint, error) {
				if service == s3.ServiceID {
					//nolint:staticcheck
					return aws.Endpoint{
						URL:               endpoint,
						SigningRegion:     cfg.Region,
						HostnameImmutable: true,
					}, nil
				}
				//nolint:staticcheck
				return aws.Endpoint{}, &aws.EndpointNotFoundError{}
			},
		)))
	}

	if cfg.AccessKeyID != "" {
		loadOpts = append(loadOpts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken),
		))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("storage: failed to load AWS config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(options *s3.Options) {
		options.UsePathStyle = cfg.ForcePathStyle
	})

	return &S3Storage{
		client:          client,
		bucket:          cfg.Bucket,
		publicURLPrefix: strings.TrimRight(cfg.PublicURLPrefix, "/"),
		keyPrefix:       strings.Trim(cfg.KeyPrefix, "/"),
		logger:          logger,
	}, nil
}

// Save stores a file and returns its public URL path.
func (s *S3Storage) Save(ctx context.Context, key string, content io.Reader, contentType string) (string, error) {
	if err := validateObjectKey(key); err != nil {
		return "", err
	}

	objectKey := s.objectKey(key)
	input := &s3.PutObjectInput{
		Bucket: &s.bucket,
		Key:    &objectKey,
		Body:   content,
	}
	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}

	if _, err := s.client.PutObject(ctx, input); err != nil {
		return "", fmt.Errorf("storage: failed to upload object: %w", err)
	}

	return s.publicURL(key), nil
}

// Delete removes a file by its key.
func (s *S3Storage) Delete(ctx context.Context, key string) error {
	if err := validateObjectKey(key); err != nil {
		return err
	}

	objectKey := s.objectKey(key)
	if _, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &s.bucket,
		Key:    &objectKey,
	}); err != nil {
		return fmt.Errorf("storage: failed to delete object: %w", err)
	}

	return nil
}

// Exists checks if a file exists.
func (s *S3Storage) Exists(ctx context.Context, key string) (bool, error) {
	if err := validateObjectKey(key); err != nil {
		return false, err
	}

	objectKey := s.objectKey(key)
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &s.bucket,
		Key:    &objectKey,
	})
	if err == nil {
		return true, nil
	}
	if isNotFound(err) {
		return false, nil
	}
	return false, fmt.Errorf("storage: failed to check object: %w", err)
}

// Open retrieves a file by key for streaming.
func (s *S3Storage) Open(ctx context.Context, key string) (port.StoredFile, error) {
	if err := validateObjectKey(key); err != nil {
		return port.StoredFile{}, err
	}

	objectKey := s.objectKey(key)
	resp, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    &objectKey,
	})
	if err != nil {
		if isNotFound(err) {
			return port.StoredFile{}, port.ErrFileNotFound
		}
		return port.StoredFile{}, fmt.Errorf("storage: failed to fetch object: %w", err)
	}

	modTime := time.Time{}
	if resp.LastModified != nil {
		modTime = *resp.LastModified
	}

	size := int64(0)
	if resp.ContentLength != nil {
		size = *resp.ContentLength
	}

	return port.StoredFile{
		Reader:      resp.Body,
		Size:        size,
		ModTime:     modTime,
		ContentType: aws.ToString(resp.ContentType),
	}, nil
}

// GetPath returns an error for S3 storage (no local filesystem path).
func (s *S3Storage) GetPath(ctx context.Context, key string) (string, error) {
	return "", errors.New("storage: filesystem path not available for S3")
}

func (s *S3Storage) objectKey(key string) string {
	if s.keyPrefix == "" {
		return key
	}
	return path.Join(s.keyPrefix, key)
}

func (s *S3Storage) publicURL(key string) string {
	return s.publicURLPrefix + "/" + key
}

func validateObjectKey(key string) error {
	if key == "" {
		return errors.New("storage: key cannot be empty")
	}
	if strings.Contains(key, "..") {
		return errors.New("storage: invalid key (path traversal)")
	}
	if strings.HasPrefix(key, "/") {
		return errors.New("storage: key must be relative")
	}
	return nil
}

func isNotFound(err error) bool {
	var noSuchKey *types.NoSuchKey
	if errors.As(err, &noSuchKey) {
		return true
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey", "NoSuchBucket":
			return true
		}
	}

	var respErr *awshttp.ResponseError
	if errors.As(err, &respErr) && respErr.HTTPStatusCode() == 404 {
		return true
	}

	return false
}

// Ensure S3Storage implements port.FileStorage.
var _ port.FileStorage = (*S3Storage)(nil)
