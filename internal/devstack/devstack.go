// Package devstack prepares a local S3 backend (the RustFS dev stack) for
// manual testing and the integration tests.
package devstack

import (
	"context"
	"fmt"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Defaults match dev/compose.yaml.
const (
	Endpoint  = "127.0.0.1:9000"
	AccessKey = "prassets-access"
	SecretKey = "prassets-dev-secret" //nolint:gosec // fixed credentials of the local dev stack
	Bucket    = "prassets-dev"
)

// publicRead lets anyone GET objects, as a public assets bucket does, so the
// URLs pra prints work against the dev stack.
const publicRead = `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::%s/*"]}]}`

// EnsureBucket waits for the server, then creates bucket with a public-read
// policy if it does not exist.
func EnsureBucket(ctx context.Context, endpoint, accessKey, secretKey, bucket string) error {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: false,
	})
	if err != nil {
		return err
	}

	var exists bool
	deadline := time.Now().Add(30 * time.Second)
	for {
		exists, err = client.BucketExists(ctx, bucket)
		if err == nil || time.Now().After(deadline) {
			break
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
	if err != nil {
		return fmt.Errorf("S3 server at %s is not ready: %w", endpoint, err)
	}
	if !exists {
		if err := client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("create bucket %s: %w", bucket, err)
		}
	}
	if err := client.SetBucketPolicy(ctx, bucket, fmt.Sprintf(publicRead, bucket)); err != nil {
		return fmt.Errorf("set public-read policy on %s: %w", bucket, err)
	}
	return nil
}
