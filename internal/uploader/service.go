// Package uploader provides a service for uploading and deleting files in an S3 bucket.
package uploader

import (
	"context"
	"fmt"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/s3/transfermanager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/google/uuid"
)

const defaultRegion = "us-east-2"

type Service interface {
	Upload(ctx context.Context, input UploadInput) (*transfermanager.UploadObjectOutput, error)
	Delete(ctx context.Context, bucket, key string) error
}

type UploadInput struct {
	File        multipart.File
	Filename    string
	ContentType string
}

type uploaderService struct {
	transferClient *transfermanager.Client
	s3Client       *s3.Client
	bucket         string
	keyPrefix      string
}

func NewService(bucket, keyPrefix string) Service {
	s3Client, transferClient := mountS3()

	return &uploaderService{
		transferClient: transferClient,
		s3Client:       s3Client,
		bucket:         bucket,
		keyPrefix:      keyPrefix,
	}
}

func initS3() (*s3.Client, *transfermanager.Client, error) {
	region := getRegion()

	cfg, err := config.LoadDefaultConfig(
		context.TODO(),
		config.WithRegion(region),
	)
	if err != nil {
		return nil, nil, err
	}

	s3Client := s3.NewFromConfig(cfg)

	uploader := transfermanager.New(s3Client, func(o *transfermanager.Options) {
		o.PartSizeBytes = 5 * 1024 * 1024
		o.Concurrency = 2
	})

	return s3Client, uploader, nil
}

func mountS3() (*s3.Client, *transfermanager.Client) {
	s3Client, uploader, err := initS3()
	if err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
	return s3Client, uploader
}

func getRegion() string {
	region := os.Getenv("AWS_REGION")
	if region == "" {
		region = defaultRegion
	}
	return region
}

func (s *uploaderService) Upload(ctx context.Context, input UploadInput) (*transfermanager.UploadObjectOutput, error) {
	ext := strings.TrimPrefix(filepath.Ext(input.Filename), ".")
	prefix := strings.TrimSuffix(s.keyPrefix, "/")
	key := fmt.Sprintf("%s/%s.%s", prefix, uuid.NewString(), ext)

	out, err := s.transferClient.UploadObject(ctx, &transfermanager.UploadObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        input.File,
		ContentType: aws.String(input.ContentType),
	})

	return out, err
}

func (s *uploaderService) Delete(ctx context.Context, bucket, key string) error {
	_, err := s.s3Client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err
}
