package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func CatchSignal() {
	defer func() {
		r := recover()
		if r != nil {
			log.Println(r, string(debug.Stack()))
		}
	}()

	<-quit
	fmt.Println("Quit signal caught, cleaning up and exiting")
	CancelFunc()
	close(objectChan)
	close(concurrencyChan)
	fmt.Println("waiting for object parser to exit...")
	<-finalDone

	time.Sleep(2 * time.Second)
	os.Exit(1)
}

func isDone() bool {
	select {
	case <-CancelContext.Done():
		return true
	default:
	}
	return false
}

var (
	endpoint       string
	secret         string
	key            string
	inputFile      = "input.json"
	doneFile       = "done.json"
	outFile        = "out.json"
	secure         bool
	outFilePointer *os.File
	client         *minio.Client
	BucketInfo     []minio.BucketInfo
	GlobalContext  = context.Background()
	CancelContext  context.Context
	CancelFunc     context.CancelFunc
	concurrency    = 10
	sleepMS        = 0

	objectMap       = make(map[string]*Object)
	quit            = make(chan os.Signal, 10)
	objectChan      = make(chan *Object, 100)
	concurrencyChan chan int
	finalDone       = make(chan struct{}, 10)

	pipeDONE bool
	start    time.Time
)

type Object struct {
	Status         string    `json:"status"`
	Type           string    `json:"type"`
	LastModified   time.Time `json:"lastModified"`
	Size           int       `json:"size"`
	Key            string    `json:"key"`
	Etag           string    `json:"etag"`
	URL            string    `json:"url"`
	VersionID      string    `json:"versionId"`
	VersionOrdinal int       `json:"versionOrdinal"`
	StorageClass   string    `json:"storageClass"`

	// Custom
	Parsed   bool   `json:"Parsed"`
	Error    string `json:"Error"`
	ReadTime int64  `json:"ReadTime"`
}

func main() {
	CancelContext, CancelFunc = context.WithCancel(GlobalContext)

	endpoint = os.Args[1]
	key = os.Args[2]
	secret = os.Args[3]

	conInt, err := strconv.Atoi(os.Args[4])
	if err != nil {
		panic(err)
	}
	concurrency = conInt

	sleepInt, err := strconv.Atoi(os.Args[5])
	if err != nil {
		panic(err)
	}
	sleepMS = sleepInt

	quit = make(chan os.Signal, concurrency+100)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	go CatchSignal()

	concurrencyChan = make(chan int, concurrency)
	for i := 1; i <= concurrency; i++ {
		concurrencyChan <- i
	}

	fmt.Println("_____ STARTING CONSISTENCY CHECKER _____")
	fmt.Println("endpoint:", endpoint)
	fmt.Println("secret:", secret)
	fmt.Println("key:", key)
	fmt.Println("inputFile:", inputFile)
	fmt.Println("doneFile:", doneFile)
	fmt.Println("concurrency:", concurrency)

	fileTimePreFix := time.Now().Format("2006-01-02-15-04-05")
	outFilePointer, err = os.OpenFile(
		fileTimePreFix+"."+outFile,
		os.O_CREATE|os.O_RDWR,
		0o777,
	)
	if err != nil {
		fmt.Println("error opening or creating out file:", err)
		os.Exit(1)
	}

	fmt.Println("outFile:", fileTimePreFix+"."+outFile)
	fmt.Println("_____ STARTING CONSISTENCY CHECKER _____")

	if strings.Contains(endpoint, "https") {
		secure = true
	}

	err = parseFullList(objectMap, inputFile)
	if err != nil {
		fmt.Println("error parsing file:", err)
		os.Exit(1)
	}

	_, err = os.Stat(doneFile)
	if err == nil {
		err = parseFullList(objectMap, doneFile)
		if err != nil {
			fmt.Println("error parsing file:", err)
			os.Exit(1)
		}
	}

	err = makeClient()
	if err != nil {
		fmt.Println("error creating minio client:", err)
		os.Exit(1)
	}

	fmt.Println("_____ FILE STATES ______")
	doneCount := 0
	remainingCount := 0
	for i := range objectMap {
		if !objectMap[i].Parsed {
			remainingCount++
		} else {
			doneCount++
		}
	}
	fmt.Println("Finished Files:", doneCount)
	fmt.Println("Remaining Files:", remainingCount)
	fmt.Println("Total Files:", len(objectMap))
	fmt.Println("_____ FILE STATES ______")

	start = time.Now()
	go pipeObjects()
	readObjectsToCheckConsistency()
}

func parseFullList(fileMap map[string]*Object, path string) (err error) {
	filePointer, err := os.Open(path)
	if err != nil {
		return
	}
	defer filePointer.Close()

	lineCount := 0
	scanner := bufio.NewScanner(filePointer)
	for scanner.Scan() {
		lineCount++

		if isDone() {
			fmt.Println("Stopping file list parser, was parsing: ", path, " ... stopped on line:", lineCount)
			return errors.New("ctx done/cancelled")
		}

		b := scanner.Bytes()
		b = bytes.Replace(b, []byte{10}, []byte{}, -1)
		if len(b) == 0 {
			continue
		}
		object := new(Object)
		err := json.Unmarshal(b, object)
		if err != nil {
			fmt.Println("could not unmarshal line:", path, " // err:", err)
			fmt.Println("LINE: ", string(b))
			os.Exit(1)
		}
		if object.Type == "file" {
			fileMap[object.Key+object.VersionID] = object
		}
	}

	err = scanner.Err()
	if err != nil {
		fmt.Println("error reading file:", err)
		return
	}
	return
}

func makeClient() (err error) {
	trans, terr := createHTTPTransport()
	if terr != nil {
		fmt.Println(terr)
		err = terr
		return
	}
	finalEnd := strings.TrimPrefix(endpoint, "https://")
	finalEnd = strings.TrimPrefix(finalEnd, "http://")
	client, err = minio.New(finalEnd,
		&minio.Options{
			Creds:     credentials.NewStaticV4(key, secret, ""),
			Secure:    secure,
			Transport: trans,
		})
	if err != nil {
		return
	}
	return
}

func createHTTPTransport() (transport *http.Transport, err error) {
	transport, err = minio.DefaultTransport(secure)
	if err != nil {
		return
	}
	if secure {
		transport.TLSClientConfig.InsecureSkipVerify = true
	}
	return
}

func readObjectsToCheckConsistency() {
	defer func() {
		r := recover()
		if r != nil {
			log.Println(r, string(debug.Stack()))
		}
	}()

	var wg sync.WaitGroup
loop:
	for cid := range concurrencyChan {

		if isDone() {
			fmt.Println("context done or cancelled, exiting object parser loop")
			break
		}

		select {
		case o, ok := <-objectChan:
			if !ok {
				fmt.Println("concurrency channel closed: !ok read")
				break loop
			}

			wg.Add(1)
			go readObject(o, cid, &wg)
		default:
			if pipeDONE {
				fmt.Println("pipe complete, exiting reader loop")
				break loop
			}
			concurrencyChan <- cid
			time.Sleep(500 * time.Millisecond)
			continue
		}
	}

	fmt.Println("object parser exiting...")
	fmt.Println("currently processing:", cap(concurrencyChan)-len(concurrencyChan))
	fmt.Println("waiting for in-progress objects to finish...")
	wg.Wait()
	fmt.Println("objects in queue:", len(objectChan))
	fmt.Println("object parser done")
	fmt.Println("total runtime in minutes:", time.Since(start).Minutes())

	if outFilePointer != nil {
		_ = outFilePointer.Sync()
		_ = outFilePointer.Close()
	}

	finalDone <- struct{}{}
}

func pipeObjects() {
	defer func() {
		r := recover()
		if r != nil {
			log.Println("NOTE: this stacktrace is fine if we are exiting")
			log.Println(r, string(debug.Stack()))
		}
		pipeDONE = true
	}()

	for i := range objectMap {
		if objectMap[i].Parsed {
			err := saveFinishedObject(objectMap[i])
			if err != nil {
				return
			}
			continue
		}

		if isDone() {
			fmt.Println("ctx cancel: object file pipe closing")
			break
		}

		objectChan <- objectMap[i]
	}
}

func readObject(o *Object, cid int, wg *sync.WaitGroup) {
	var mo *minio.Object
	var err error
	defer func() {
		r := recover()
		if r != nil {
			log.Println(r, string(debug.Stack()))
		}
		wg.Done()

		if mo != nil {
			if err == nil {
				o.Parsed = true
			}
		} else {
			o.Error = "minio sdk returned nil object"
		}

		if err != nil {
			fmt.Println("ERR:", o.Key, " || err:", err)
			o.Error = err.Error()
		}
		_ = saveFinishedObject(o)

		if isDone() {
			return
		}

		concurrencyChan <- cid
	}()

	if sleepMS > 0 {
		time.Sleep(time.Duration(sleepMS) * time.Millisecond)
	}

	start := time.Now()
	keySplit := strings.Split(o.Key, "/")
	opts := minio.GetObjectOptions{}
	readSize := o.Size
	half := int64(o.Size / 2)

	if o.Size > 3000 {
		readSize = 2001
		_ = opts.SetRange(half-1000, half+1000)
		// fmt.Println("SR:", half-1000, half+1000, opts)
	} else if o.Size > 1000 && o.Size <= 3000 {
		quarter := int64(o.Size) / 4
		readSize = ((o.Size / 4) * 2) + 1
		_ = opts.SetRange(half-quarter, half+quarter)
		// fmt.Println("SR:", half-quarter, half+quarter, opts)
	}

	mo, err = client.GetObject(GlobalContext, keySplit[0], strings.Join(keySplit[1:], "/"), opts)
	if err != nil {
		return
	}

	defer mo.Close()
	rbytes, rerr := io.ReadAll(mo)
	o.ReadTime = time.Since(start).Milliseconds()
	if rerr != nil {
		err = rerr
	}
	if len(rbytes) != int(readSize) {
		err = fmt.Errorf("Short read. Expected %d bytes but read %d", readSize, len(rbytes))
	}
}

func saveFinishedObject(o *Object) (err error) {
	var jsonOut []byte
	defer func() {
		r := recover()
		if r != nil {
			log.Println(r, string(debug.Stack()))
		}
		if err != nil {
			fmt.Println("error saving finished object:", err)
			fmt.Println(string(jsonOut))
			fmt.Println(jsonOut)
			quit <- syscall.SIGTERM
		}
	}()

	jsonOut, err = json.Marshal(o)
	if err != nil {
		return err
	}
	var n int
	n, err = outFilePointer.Write(jsonOut)
	if err != nil {
		return err
	}
	if n != len(jsonOut) {
		return errors.New("error writing finished object to json, write inconsistency")
	}
	n, err = outFilePointer.Write([]byte{10})
	if err != nil {
		return err
	}
	if n != 1 {
		return errors.New("error writing newline, write inconsistency")
	}
	return
}
