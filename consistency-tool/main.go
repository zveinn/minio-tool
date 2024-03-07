package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

var (
	//
	listCMD  *flag.FlagSet
	checkCMD *flag.FlagSet
	// doListing  bool
	// doChecking bool

	quit            = make(chan os.Signal, 10)
	GlobalContext   = context.Background()
	CancelContext   context.Context
	CancelFunc      context.CancelFunc
	reset           bool
	concurrencyChan chan int
	wg              sync.WaitGroup
	concurrency     = 1
	batchSleep      = 0

	// Minio
	MClient         *minio.Client
	minioEndpoint   string
	minioKey        string
	minioSecret     string
	skipInsecure    bool
	ignoreMultipart bool
	bucket          string
	shards          int
	logN            int

	// sql lite
	sqlFile             string
	dbFile              *os.File
	db                  *sql.DB
	IObjectStatement    *sql.Stmt
	GObjectsStatement   *sql.Stmt
	ICursorStatement    *sql.Stmt
	IResultsStatement   *sql.Stmt
	InitCursorStatement *sql.Stmt
	InitCursor          = `INSERT INTO cursor(id, cursor) VALUES (1, ?)`
	ICursor             = `UPDATE cursor SET cursor=? WHERE id = 1`
	GCursor             = "SELECT * FROM cursor ORDER BY (id) DESC"
	IResult             = `INSERT INTO results(bucket, key, error, msg) VALUES (?, ?, ?, ?)`
	IObjects            = `INSERT INTO objects(etag, bucket, key, parts, size, lastpart, error) VALUES (?, ?, ?, ?, ?, ?, ?)`
	CObjects            = "SELECT * FROM objects ORDER BY (id) DESC LIMIT 1"
	GObjects            = "SELECT * FROM objects WHERE error == '' ORDER BY (id) ASC LIMIT ? OFFSET ?"
	objectTable         = `CREATE TABLE objects (
		"id" integer NOT NULL PRIMARY KEY AUTOINCREMENT,		
		"etag" TEXT,
		"bucket" TEXT,
		"key" TEXT,
		"parts" INT,
		"size" INT,
		"lastpart" INT,
		"error" TEXT
	  );`
	resultsTable = `CREATE TABLE results (
		"id" integer NOT NULL PRIMARY KEY AUTOINCREMENT,	
		"bucket" TEXT,
		"key" TEXT,
		"error" TEXT,
		"msg" TEXT
	  );`
	cursorTable = `CREATE TABLE cursor (
		"id" INT NOT NULL UNIQUE,		
		"cursor" INT
	  );`

	bulk      int64  = 1000
	cursor    uint64 = 0
	processed int
)

func CatchSignal() {
	defer func() {
		r := recover()
		if r != nil {
			log.Println(r, string(debug.Stack()))
		}
	}()

	<-quit
	fmt.Println("exit signal caught...")
	CancelFunc()
}

func main() {
	CancelContext, CancelFunc = context.WithCancel(GlobalContext)
	quit = make(chan os.Signal, 10)
	go CatchSignal()

	signal.Notify(
		quit,
		os.Interrupt,
		syscall.SIGTERM,
	)
	listCMD = flag.NewFlagSet("list", flag.ExitOnError)
	checkCMD = flag.NewFlagSet("check", flag.ExitOnError)

	if len(os.Args) > 1 {
		op := os.Args[1]
		switch op {
		case "list":
			CreateList()
		case "check":
			CheckFiles()
		default:
		}
	} else {
		fmt.Println("__ commands __")
		fmt.Println("list - Creates list in an sql lite database")
		fmt.Println("check - Performance consistency checking on a list from an sql lite database")
		os.Exit(1)
	}
}

func CheckFiles() {
	defer func() {
		r := recover()
		if r != nil {
			log.Println(r, string(debug.Stack()))
		}
		if db != nil {
			db.Close()
		}
	}()
	checkCMD.StringVar(&sqlFile, "sqldb", "data.db", "sets the sql lite db file")
	checkCMD.StringVar(&minioEndpoint, "minio", "127.0.0.1:9000", "sets the minio endpoint")
	checkCMD.StringVar(&minioKey, "key", "minioadmin", "minio private key")
	checkCMD.StringVar(&minioSecret, "secret", "minioadmin", "minio user secret")
	checkCMD.BoolVar(&skipInsecure, "insecure", false, "skip insecure TLS validation")
	checkCMD.BoolVar(&ignoreMultipart, "ignoreMultipart", false, "Ignore multipart objects. This is required if the minio release does not includes the GetObjectAttributes api")
	checkCMD.IntVar(&concurrency, "concurrency", 10, "controls how many objects we check at the same time. We will always query [concurrency*3] objects from sqllite at a time.")
	checkCMD.IntVar(&logN, "logN", 10000, "tell this program to print a report every Nth object listed")
	checkCMD.IntVar(&shards, "shards", 2, "defines how many data shards each object part has")
	checkCMD.IntVar(&batchSleep, "sleep", 0, "sets a sleep timer in Milliseconds that is enforced after processing every object")
	checkCMD.BoolVar(&reset, "reset", false, "reset the cursor to 0")

	err := checkCMD.Parse(os.Args[2:])
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	concurrencyChan = make(chan int, concurrency)
	for i := 1; i <= concurrency; i++ {
		concurrencyChan <- i
	}

	db, err := sql.Open("sqlite3", sqlFile)
	if err != nil {
		panic(err)
	}
	db.SetMaxOpenConns(1000)
	_, err = db.Exec("PRAGMA journal_mode = WAL")
	if err != nil {
		panic(err)
	}

	ICursorStatement, err = db.Prepare(ICursor)
	if err != nil {
		panic(err)
	}

	IResultsStatement, err = db.Prepare(IResult)
	if err != nil {
		panic(err)
	}

	GObjectsStatement, err = db.Prepare(GObjects)
	if err != nil {
		panic(err)
	}

	if reset {
		cursor = 0
		SaveCursor()
	}

	err = makeClient()
	if err != nil {
		panic(err)
	}

	bulk = int64(concurrency) * 3

	fmt.Printf(
		"STARTING || db(%s) concurrency(%d) bulk(%d) cursor(%d)\n",
		sqlFile,
		concurrency,
		bulk,
		cursor,
	)

	start := time.Now()
	processObjects()
	wg.Wait()

	fmt.Printf(
		"DONE || count(%d) seconds(%.0f) minutes(%.0f)\n",
		processed,
		time.Since(start).Seconds(),
		time.Since(start).Minutes(),
	)
}

func SaveCursor() {
	res, qerr := ICursorStatement.Exec(cursor)
	if qerr != nil {
		fmt.Println("CURSOR ERROR || ", qerr)
		CancelFunc()
	}
	aff, err := res.RowsAffected()
	if err != nil || aff == 0 {
		fmt.Println("CURSOR ERROR || ", aff, err)
		CancelFunc()
	}
}

func processObjects() {
	defer func() {
		r := recover()
		if r != nil {
			log.Println(r, string(debug.Stack()))
		}
	}()

	var err error
	var rows *sql.Rows

	for {

		rows, err = GObjectsStatement.Query(bulk, cursor)
		if err != nil {
			panic(err)
		}

		rowCount := 0
		conCount := 0
		var id, size, parts, lastpart int
		var bucket, key, etag, error string
		for rows.Next() {
			err = rows.Scan(&id, &etag, &bucket, &key, &parts, &size, &lastpart, &error)
			if err != nil {
				panic(err)
			}
			rowCount++

			select {
			case cid := <-concurrencyChan:
				conCount++
				processed++
				if processed%logN == 0 {
					fmt.Printf(
						"STATUS || db(%s) processed(%d) cursor(%d) ",
						sqlFile,
						processed,
						cursor,
					)
					PrintMemUsage()
				}

				wg.Add(1)
				go ProcessObject(cid, id, etag, bucket, key, parts, size, lastpart, error)

				if conCount == cap(concurrencyChan) {
					conCount = 0
					time.Sleep(time.Duration(batchSleep) * time.Millisecond)
				}

			case <-CancelContext.Done():
				fmt.Println("PROCESSOR || CONTEXT DONE")
				goto EXIT
			}

		}

		if rowCount == 0 {
			cursor = 0
			SaveCursor()
			goto EXIT
		}

		cursor += uint64(rowCount)
		SaveCursor()
		if cursor == 0 {
			break
		}

	}

EXIT:
}

func ProcessObject(
	cid int,
	_ int,
	_ string,
	bucket string,
	key string,
	parts int,
	size int,
	lastpart int,
	_ string,
) {
	defer func() {
		r := recover()
		if r != nil {
			log.Println(r, string(debug.Stack()))
		}
		wg.Done()
		concurrencyChan <- cid
	}()

	partReadInfo := make(map[int][]int64)

	shardSize := size / shards
	if parts > 1 { // MULTIPART OBJECT

		if ignoreMultipart {
			totalSize := int64(0)
			for p := 1; p <= parts; p++ {
				partReadInfo[1] = append(partReadInfo[1], totalSize)
				partReadInfo[1] = append(partReadInfo[1], totalSize+int64(size)-1)
				totalSize += int64(size)
			}
		} else {
			totalSize := int64(shardSize)
			for p := 1; p <= parts; p++ {
				for i := 1; i <= shards; i++ {
					partReadInfo[p] = append(partReadInfo[p], totalSize-500)
					partReadInfo[p] = append(partReadInfo[p], totalSize-1)
					if p == shards {
						totalSize += int64(lastpart)
					} else {
						totalSize += int64(shardSize)
					}
				}
			}
		}
	} else { // SINGLE PART OBJECTS
		totalSize := int64(shardSize)
		if shardSize < 16384 { // OBJECT FITS INSIDE XL.META (16 Kib)
			partReadInfo[1] = append(partReadInfo[1], 0)
			partReadInfo[1] = append(partReadInfo[1], int64(size)-1)
		} else {
			for i := 1; i <= shards; i++ {
				partReadInfo[1] = append(partReadInfo[1], totalSize-500)
				partReadInfo[1] = append(partReadInfo[1], totalSize-1)
				// fmt.Println("shard size:", accumilatedSize)
				totalSize += int64(shardSize)
			}
		}
	}

	opts := minio.GetObjectOptions{}

	shardCount := 0
	for i, v := range partReadInfo {
		for ii := 0; ii < len(v); ii++ {
			err := opts.SetRange(v[ii], v[ii+1])
			shardCount++

			if err != nil {
				fmt.Println("OBJECT || invalid range ", i, ii, v)
				panic(err)
			}

			getO, err := MClient.GetObject(CancelContext, bucket, key, opts)
			if err != nil {
				if getO != nil {
					getO.Close()
				}
				SaveResult(bucket, key, err.Error(), fmt.Sprintf("part %d, shard %d", i, shardCount))
				break
			}

			getB, err := io.Copy(io.Discard, getO)
			if err != nil {
				if getO != nil {
					getO.Close()
				}
				switch err.Error() {
				case "The object was stored using a form of Server Side Encryption. The correct parameters must be provided to retrieve the object.":
				case "The specified key does not exist.":
				default:
					SaveResult(bucket, key, err.Error(), fmt.Sprintf("part %d, shard %d", i, shardCount))
				}
				break
			}

			getO.Close()

			if int64(getB) != (v[ii+1]-v[ii])+1 {
				SaveResult(
					bucket,
					key,
					fmt.Sprintf("read %d, expected %d", int64(getB), (v[ii+1]-v[ii])),
					fmt.Sprintf("part %d, shard %d", i, shardCount),
				)
				break
			}

			ii++
		}
	}
}

func makeClient() (err error) {
	trans, terr := createHTTPTransport()
	if terr != nil {
		fmt.Println(terr)
		err = terr
		return
	}
	finalEnd := strings.TrimPrefix(minioEndpoint, "https://")
	finalEnd = strings.TrimPrefix(finalEnd, "http://")
	MClient, err = minio.New(finalEnd,
		&minio.Options{
			Creds:     credentials.NewStaticV4(minioKey, minioSecret, ""),
			Secure:    skipInsecure,
			Transport: trans,
		})
	if err != nil {
		return
	}
	return
}

func createHTTPTransport() (transport *http.Transport, err error) {
	transport, err = minio.DefaultTransport(skipInsecure)
	if err != nil {
		return
	}

	if skipInsecure {
		transport.TLSClientConfig.InsecureSkipVerify = true
	}
	return
}

func SaveError(etag, bucket, key string, parts, size, finalpart int, err string) {
	_, qerr := IObjectStatement.Exec(
		etag,
		bucket,
		key,
		parts,
		size,
		finalpart,
		err,
	)
	if qerr != nil {
		panic(qerr)
	}
}

func SaveObject(etag, bucket, key string, parts, size, finalpart int) {
	_, qerr := IObjectStatement.Exec(
		etag,
		bucket,
		key,
		parts,
		size,
		finalpart,
		"",
	)
	if qerr != nil {
		panic(qerr)
	}
}

func SaveResult(bucket, key, err, msg string) {
	_, qerr := IResultsStatement.Exec(
		bucket,
		key,
		err,
		msg,
	)
	if qerr != nil {
		panic(qerr)
	}
}

func PrintMemUsage() {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	fmt.Printf(
		"|| MEMORY (MB) || Stack(%v) Heap(%v) Historical(%v) System(%v) collections(%v)\n",
		m.StackInuse/1024/1024,
		m.Alloc/1024/1024,
		m.TotalAlloc/1024/1024,
		m.Sys/1024/1024,
		m.NumGC,
	)
}
