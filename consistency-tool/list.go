package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
)

func CreateList() {
	defer func() {
		r := recover()
		if r != nil {
			log.Println(r, string(debug.Stack()))
		}
		if db != nil {
			db.Close()
		}
	}()
	listCMD.StringVar(&sqlFile, "sqldb", "data.db", "sets the sql lite db file")
	listCMD.StringVar(&minioEndpoint, "minio", "127.0.0.1:9000", "sets the minio endpoint")
	listCMD.StringVar(&minioKey, "key", "minioadmin", "minio private key")
	listCMD.StringVar(&minioSecret, "secret", "minioadmin", "minio user secret")
	listCMD.BoolVar(&skipInsecure, "insecure", false, "skip insecure TLS validation")
	listCMD.BoolVar(&ignoreMultipart, "ignoreMultipart", false, "Ignore multipart objects. This is required if the minio release does not includes the GetObjectAttributes api")
	listCMD.IntVar(&concurrency, "concurrency", 1000, "controls how many objects we want to list per call to minio")
	listCMD.IntVar(&logN, "logN", 10000, "tell this program to print a report every Nth object listed")
	listCMD.BoolVar(&reset, "reset", false, "re-create the list from scratch")
	listCMD.StringVar(&bucket, "bucket", "", "defines the bucket used for listing")

	err := listCMD.Parse(os.Args[2:])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	if reset {
		_ = os.Remove(sqlFile)
		_ = os.Remove(sqlFile + "-shm")
		_ = os.Remove(sqlFile + "-wal")
	}

	dbFile, err = os.Open(sqlFile)
	if err != nil {
		dbFile, err = os.Create(sqlFile)
		if err != nil {
			panic(err)
		}
	}
	dbFile.Close()

	db, err = sql.Open("sqlite3", sqlFile)
	if err != nil {
		panic(err)
	}
	db.SetMaxOpenConns(1000)
	_, err = db.Exec("PRAGMA journal_mode = WAL")
	if err != nil {
		panic(err)
	}

	_, err = db.Exec(objectTable)
	if err != nil {
		if !strings.Contains(err.Error(), "objects already exists") {
			panic(err)
		}
	}

	_, err = db.Exec(resultsTable)
	if err != nil {
		if !strings.Contains(err.Error(), "results already exists") {
			panic(err)
		}
	}

	_, err = db.Exec(cursorTable)
	if err != nil {
		if !strings.Contains(err.Error(), "cursor already exists") {
			panic(err)
		}
	}
	InitCursorStatement, err = db.Prepare(InitCursor)
	if err != nil {
		panic(err)
	}

	_, err = InitCursorStatement.Exec(0)
	if err != nil {
		panic(err)
	}

	IObjectStatement, err = db.Prepare(IObjects)
	if err != nil {
		panic(err)
	}

	err = makeClient()
	if err != nil {
		panic(err)
	}

	opts := new(minio.ListObjectsOptions)
	opts.Recursive = true
	opts.MaxKeys = concurrency

	if !reset {

		var id int
		var path string
		var size int
		err := db.QueryRow(CObjects).Scan(&id, &path, &size)
		if err != nil {
			if !strings.Contains(err.Error(), "no rows") {
				panic(err)
			}
		}
		if path != "" {
			opts.StartAfter = strings.TrimPrefix(path, bucket+"/")
		}

	}

	feed := MClient.ListObjects(CancelContext, bucket, *opts)
	count := 0
	start := time.Now()
	var feedError error
	var partNumber int
	var attr *minio.ObjectAttributes

	for o := range feed {
		if o.Err != nil {
			feedError = o.Err
			SaveError(o.ETag, bucket, o.Key, 1, int(o.Size), 0, feedError.Error())
			break
		}

		count++
		if count%logN == 0 {
			fmt.Printf(
				"STATUS || count(%d) seconds(%.0f) minutes(%.0f) ",
				count,
				time.Since(start).Seconds(),
				time.Since(start).Minutes(),
			)
			PrintMemUsage()
		}

		se := strings.Split(o.ETag, "-")

		if len(se) < 2 {
			SaveObject(o.ETag, bucket, o.Key, 1, int(o.Size), 0)
			continue
		}

		partNumber, feedError = strconv.Atoi(se[1])
		if feedError != nil {
			SaveError(o.ETag, bucket, o.Key, 1, int(o.Size), 0, feedError.Error())
			break
		}

		if ignoreMultipart && len(se) > 1 {
			// if we are ignoring multipart but we have a multipart file
			// we devide size with part number and then read the entire
			// file later part by part.
			// NOTE: this read method (might) miss 1 byte at the end.
			SaveObject(o.ETag, bucket, o.Key, partNumber, int(o.Size)/partNumber, 0)
			continue
		}

		attr, feedError = MClient.GetObjectAttributes(CancelContext, bucket, o.Key, minio.ObjectAttributesOptions{
			MaxParts:         2,
			PartNumberMarker: partNumber - 2,
		})
		if feedError != nil {
			SaveError(o.ETag, bucket, o.Key, 0, 0, 0, "unable to get object attributes: "+feedError.Error())
			break
		}

		if len(attr.ObjectParts.Parts) < 2 {
			SaveError(o.ETag, bucket, o.Key, 0, 0, 0, "got less then 2 parts when getting object attributes")
			break
		}

		SaveObject(
			o.ETag,
			bucket,
			o.Key,
			partNumber,
			attr.ObjectParts.Parts[0].Size,
			attr.ObjectParts.Parts[1].Size,
		)

	}

	fmt.Printf(
		"DONE || count(%d) seconds(%.0f) minutes(%.0f) err:%v\n",
		count,
		time.Since(start).Seconds(),
		time.Since(start).Minutes(),
		feedError,
	)

	// fmt.Println("POST LISTING")
	// fmt.Println("POST LISTING")
	// fmt.Println("POST LISTING")
	// row, err := db.Query("SELECT * FROM objects ORDER BY id")
	// if err != nil {
	// 	panic(err)
	// }
	// defer row.Close()
	// for row.Next() {
	// 	var id int
	// 	var path string
	// 	var size int
	// 	var etag string
	// 	var key string
	// 	var bucket string
	// 	var errr string
	// 	var parts int
	// 	var lastpart int
	// 	err = row.Scan(&id, &etag, &bucket, &key, &parts, &size, &lastpart, &errr)
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// 	fmt.Println("O:", id, path, size, etag, key, bucket, parts, lastpart, err)
	// }
}
