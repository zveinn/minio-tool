/*
 * MinIO Go Library for Amazon S3 Compatible Cloud Storage
 * Copyright 2015-2024 MinIO, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func main() {

	var (
		endpoint, accessKey, secretKey string

		startAfter string
		secure     bool
		parityLess int
	)

	flag.StringVar(&endpoint, "endpoint", "play.min.io", "MinIO S3 server address, e:g localhost:9000")
	flag.StringVar(&accessKey, "access-key", "Q3AM3UQ867SPQQA43P2F", "MinIO S3 server access key")
	flag.StringVar(&secretKey, "secret-key", "zuf+tfteSlswRu7BJ86wekitnifILbZam1KYY3TG", "MinIO S3 server secret key")
	flag.StringVar(&startAfter, "start-after", "", "Start after bucket/object")
	flag.IntVar(&parityLess, "parity-less-than", 4, "Show objects with parity less than")
	flag.BoolVar(&secure, "secure", true, "Enable HTTPS")

	flag.Parse()

	log.SetFlags(log.LstdFlags | log.Lshortfile)

	tr, err := minio.DefaultTransport(secure)
	if err != nil {
		log.Fatal(err)
	}
	if secure {
		tr.TLSClientConfig.InsecureSkipVerify = true
	}

	s3Client, err := minio.New(endpoint, &minio.Options{
		Creds:     credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure:    secure,
		Transport: tr,
	})
	if err != nil {
		log.Fatal(err)
	}

	var startBucket, startObject string
	if startAfter != "" {
		s := strings.SplitN(startAfter, "/", 2)
		if len(s) < 2 {
			log.Fatal("unexpected --start-after value, requires this format: bucket/object")
		}
		startBucket, startObject = s[0], s[1]
	}

	buckets, err := s3Client.ListBuckets(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	var count, low int

	for _, bucket := range buckets {
		if bucket.Name < startBucket {
			continue
		}

		opts := minio.ListObjectsOptions{
			Recursive:    true,
			WithMetadata: true,
			StartAfter:   startObject,
		}

		for obj := range s3Client.ListObjects(context.Background(), bucket.Name, opts) {
			if obj.Err != nil {
				log.Fatal(obj.Err)
			}
			count++
			if count%10000 == 0 {
				fmt.Fprintf(os.Stderr, "object-listed=%d, low-parity-found=%d\n", count, low)
			}
			if obj.Internal.M <= parityLess {
				low++
				fmt.Println(obj.Internal.K, obj.Internal.M, bucket.Name, obj.Key)
			}
		}

		if startBucket != "" {
			startBucket = ""
		}
		if startObject != "" {
			startObject = ""
		}
	}

	return
}
