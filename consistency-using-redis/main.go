package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	redis "github.com/redis/go-redis/v9"
)

var (
	reset bool
	//
	GlobalContext = context.Background()
	CancelContext context.Context
	CancelFunc    context.CancelFunc
	//
	quit = make(chan os.Signal, 10)
	//
	concurrencyChan chan int
	wg              sync.WaitGroup
	concurrency     = 1
	batchSleep      = 0

	// Resid
	redisClient   *redis.Client
	bulk          int64  = 1000
	cursor        uint64 = 0
	redisEndpoint string

	//
	key = "bucketevents"
	// Stats
	processed uint64
)

func isDone() bool {
	select {
	case <-CancelContext.Done():
		return true
	default:
	}
	return false
}

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
	flag.IntVar(&concurrency, "concurrency", 1, "controls how many objects we check at the same time")
	flag.IntVar(&batchSleep, "sleep", 0, "sets a sleep timer in Milliseconds that is enforced after processing every object")
	flag.BoolVar(&reset, "reset", false, "reset the cursor to 0")
	flag.StringVar(&redisEndpoint, "redis", "0.0.0.0:6379", "sets the redis endpoint")
	flag.Parse()

	CancelContext, CancelFunc = context.WithCancel(GlobalContext)
	quit = make(chan os.Signal, concurrency+100)
	go CatchSignal()

	signal.Notify(
		quit,
		os.Interrupt,
		syscall.SIGTERM,
	)

	concurrencyChan = make(chan int, concurrency)
	for i := 1; i <= concurrency; i++ {
		concurrencyChan <- i
	}

	redisClient = redis.NewClient(&redis.Options{
		Addr:     redisEndpoint,
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	if reset {
		cursor = 0
		SaveCursor()
	}

	cmd := redisClient.Get(context.Background(), "listing-cursor")
	c := cmd.Val()
	cI, err := strconv.ParseUint(c, 10, 64)
	if err != nil {
		fmt.Println(err)
	}
	cursor = cI

	bulk = int64(concurrency) * 3
	if bulk < 500 {
		bulk = 500
	}

	fmt.Println("_____ STARTING CONSISTENCY CHECKER _____")
	fmt.Println("Redis", redisEndpoint)
	fmt.Println("Concurrency:", concurrency)
	fmt.Println("Object per redis call:", bulk)
	fmt.Println("Current redis cursor", cursor)

	processRedisEntities()

	wg.Wait()

	fmt.Println("total processed:", processed)
	// fmt.Println("TOTAL LISTING", len(counter))
}

func SaveCursor() {
	cmd := redisClient.Set(context.Background(), "listing-cursor", cursor, time.Duration(time.Hour*999999))
	err := cmd.Err()
	if err != nil {
		panic(err)
	}
}

// var counter = make(map[string]int)

func processRedisEntities() {
	// cl := len(concurrencyChan)
	var list []string
	var err error
	for {
		list = make([]string, 0)
		err = nil

		cmd := redisClient.HScan(context.Background(), key, cursor, "*", bulk)
		list, cursor, err = cmd.Result()
		if err != nil {
			panic(err)
		}

		for i := 0; i < len(list); i++ {
			select {
			case id := <-concurrencyChan:
				// fmt.Print("\033[G\033[K")
				// fmt.Print("\033[u\033[K")
				// for i := 0; i < cl; i++ {
				// 	if i == cl-1 {
				// 		fmt.Print("|||")
				// 	} else {
				// 		fmt.Print("=")
				// 	}
				// }
				// fmt.Print(len(concurrencyChan), cap(concurrencyChan))
				processed++
				// counter[list[i]]++
				// if counter[list[i]] > 1 {
				// 	panic("TOO MANY!")
				// }
				wg.Add(1)
				go ProcessObject(list[i], list[i+1], id)
			case <-CancelContext.Done():
				fmt.Println("context canceled inside processing loop")
				goto EXIT
			}

			i++
		}

		SaveCursor()
		if cursor == 0 {
			break
		}

	}
EXIT:
}

func ProcessObject(key, value string, id int) {
	defer func() {
		r := recover()
		if r != nil {
			log.Println(r, string(debug.Stack()))
		}
		wg.Done()
		concurrencyChan <- id
	}()

	e := new(Event)
	err := json.Unmarshal([]byte(value), e)
	if err != nil {
		panic(err)
	}

	// fmt.Println(id, key)
	for _, v := range e.Records {
		if strings.Contains(v.EventName, "s3:ObjectCreated") {
			fmt.Println("P:", v.S3.Bucket.Name+"/"+v.S3.Object.Key)
		} else if strings.Contains(v.EventName, "s3:ObjectRemoved") {
			fmt.Println("DEL:", v.S3.Bucket.Name+"/"+v.S3.Object.Key)
		}
	}
}

type Event struct {
	Records []struct {
		EventVersion string    `json:"eventVersion"`
		EventSource  string    `json:"eventSource"`
		EventTime    time.Time `json:"eventTime"`
		EventName    string    `json:"eventName"`
		S3           struct {
			S3SchemaVersion string `json:"s3SchemaVersion"`
			ConfigurationID string `json:"configurationId"`
			Bucket          struct {
				Name          string `json:"name"`
				OwnerIdentity struct {
					PrincipalID string `json:"principalId"`
				} `json:"ownerIdentity"`
				Arn string `json:"arn"`
			} `json:"bucket"`
			Object struct {
				Key          string `json:"key"`
				Size         int    `json:"size"`
				ETag         string `json:"eTag"`
				ContentType  string `json:"contentType"`
				UserMetadata struct {
					ContentType string `json:"content-type"`
				} `json:"userMetadata"`
				Sequencer string `json:"sequencer"`
			} `json:"object"`
		} `json:"s3"`
	} `json:"Records"`
}
