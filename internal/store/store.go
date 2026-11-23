// Package store wraps an S3-compatible bucket.
package store

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// Options configures a Store.
type Options struct {
	Endpoint     string
	Bucket       string
	Region       string
	Insecure     bool
	AccessKey    string
	SecretKey    string
	SessionToken string
}

// Store is one bucket.
type Store struct {
	client *minio.Client
	bucket string
}

// Object is a listed object.
type Object struct {
	Key  string
	Size int64
}

// New returns a Store. It does not contact the server.
func New(opts Options) (*Store, error) {
	client, err := minio.New(opts.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(opts.AccessKey, opts.SecretKey, opts.SessionToken),
		Secure: !opts.Insecure,
		Region: opts.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("create S3 client for %s: %w", opts.Endpoint, err)
	}
	return &Store{client: client, bucket: opts.Bucket}, nil
}

// Bucket returns the bucket name.
func (s *Store) Bucket() string { return s.bucket }

// Check confirms the bucket exists and the credentials can reach it.
func (s *Store) Check(ctx context.Context) error {
	ok, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("HEAD bucket %s: %w", s.bucket, err)
	}
	if !ok {
		return fmt.Errorf("bucket %s does not exist", s.bucket)
	}
	return nil
}

// Exists reports whether key exists.
func (s *Store) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.StatObject(ctx, s.bucket, key, minio.StatObjectOptions{})
	if err == nil {
		return true, nil
	}
	if isNotFound(err) {
		return false, nil
	}
	return false, fmt.Errorf("check %s: %w", key, err)
}

func isNotFound(err error) bool {
	resp := minio.ToErrorResponse(err)
	return resp.StatusCode == http.StatusNotFound || resp.Code == "NoSuchKey"
}

// PutOptions are the headers of an upload.
type PutOptions struct {
	ContentType  string
	CacheControl string
}

// Put uploads size bytes from r to key.
func (s *Store) Put(ctx context.Context, key string, r io.Reader, size int64, opts PutOptions) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, r, size, minio.PutObjectOptions{
		ContentType:  opts.ContentType,
		CacheControl: opts.CacheControl,
	})
	if err != nil {
		return fmt.Errorf("upload %s: %w", key, err)
	}
	return nil
}

// List returns every object under prefix, sorted by key.
func (s *Store) List(ctx context.Context, prefix string) ([]Object, error) {
	var out []Object
	for obj := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if obj.Err != nil {
			return nil, fmt.Errorf("list %s: %w", prefix, obj.Err)
		}
		out = append(out, Object{Key: obj.Key, Size: obj.Size})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

// ErrNotFound means the object does not exist.
var ErrNotFound = errors.New("object not found")

// Get writes the object at key to w.
func (s *Store) Get(ctx context.Context, key string, w io.Writer) error {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return fmt.Errorf("download %s: %w", key, err)
	}
	defer func() { _ = obj.Close() }()
	if _, err := io.Copy(w, obj); err != nil {
		if isNotFound(err) {
			return fmt.Errorf("%s: %w", key, ErrNotFound)
		}
		return fmt.Errorf("download %s: %w", key, err)
	}
	return nil
}

// Delete removes key.
func (s *Store) Delete(ctx context.Context, key string) error {
	if err := s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("delete %s: %w", key, err)
	}
	return nil
}
