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
	"time"

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

	// PresignGetObject firma una URL de lectura de duración acotada sobre un único objeto. Es
	// la única forma en que un objeto privado sale del backend: nunca viaja su bucket ni su
	// clave.
	PresignGetObject(ctx context.Context, input PresignInput) (string, error)
}

type UploadInput struct {
	File        multipart.File
	Filename    string
	ContentType string
}

// PresignInput son los datos de la firma. Bucket vacío significa el bucket configurado del
// servicio: los documentos de título guardan solo su clave, porque se subieron a este mismo
// bucket y la fila de educación no persiste ninguna otra coordenada.
//
// Filename es el nombre con el que el navegador guarda el archivo, que no tiene por qué
// parecerse a la clave: la clave es un UUID.
type PresignInput struct {
	Bucket      string
	Key         string
	Filename    string
	ContentType string
	TTL         time.Duration
}

type uploaderService struct {
	transferClient *transfermanager.Client
	s3Client       *s3.Client
	presignClient  *s3.PresignClient
	bucket         string
	keyPrefix      string
}

func NewService(bucket, keyPrefix string) Service {
	s3Client, transferClient := mountS3()

	return &uploaderService{
		transferClient: transferClient,
		s3Client:       s3Client,
		// El cliente de presignado se arma una vez y no por pedido: es un envoltorio del
		// cliente de S3 y reconstruirlo en cada descarga solo repetiría la resolución de
		// credenciales.
		presignClient: s3.NewPresignClient(s3Client),
		bucket:        bucket,
		keyPrefix:     keyPrefix,
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

// PresignGetObject devuelve una URL firmada que sirve exactamente el objeto pedido durante el
// plazo pedido. El Content-Disposition se fija en la firma y no en el objeto: así el mismo
// objeto puede entregarse con el nombre que corresponda sin reescribirlo en S3.
func (s *uploaderService) PresignGetObject(ctx context.Context, input PresignInput) (string, error) {
	bucket := input.Bucket
	if bucket == "" {
		bucket = s.bucket
	}

	request, err := s.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket:                     aws.String(bucket),
		Key:                        aws.String(input.Key),
		ResponseContentDisposition: aws.String(contentDisposition(input.Filename)),
		ResponseContentType:        contentTypeOrNil(input.ContentType),
	}, s3.WithPresignExpires(input.TTL))
	if err != nil {
		return "", err
	}

	return request.URL, nil
}

// contentDisposition cita el nombre con %q, que además de encomillarlo escapa comillas,
// barras invertidas y saltos de línea. El nombre lo eligió quien subió el archivo, así que ese
// escape es lo que impide que un nombre con un salto de línea inyecte una cabecera.
func contentDisposition(filename string) string {
	if filename == "" {
		return "attachment"
	}

	return fmt.Sprintf("attachment; filename=%q", filename)
}

func contentTypeOrNil(contentType string) *string {
	if contentType == "" {
		return nil
	}

	return aws.String(contentType)
}
