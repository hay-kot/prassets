// Command setup creates the dev stack bucket. mise run dev:up runs it.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/hay-kot/prassets/internal/devstack"
)

func main() {
	endpoint := devstack.Endpoint
	if port := os.Getenv("PRASSETS_DEV_S3_PORT"); port != "" {
		endpoint = "127.0.0.1:" + port
	}
	if err := devstack.EnsureBucket(context.Background(), endpoint, devstack.AccessKey, devstack.SecretKey, devstack.Bucket); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("bucket %s ready at http://%s/%s\n", devstack.Bucket, endpoint, devstack.Bucket)
}
