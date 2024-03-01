package main

import (
	"fmt"
	"log"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
	m "github.com/labstack/echo/v4/middleware"
)

func main() {
	START_API()
}

func START_API() {
	defer func() {
		r := recover()
		if r != nil {
			log.Println(r, string(debug.Stack()))
		}
	}()

	E := echo.New()

	corsConfig := m.CORSConfig{
		Skipper:      m.DefaultSkipper,
		AllowOrigins: []string{"*"},
		AllowMethods: []string{"POST", "OPTIONS"},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderCookie, echo.HeaderSetCookie, echo.HeaderXRequestedWith},
	}

	E.Use(m.CORSWithConfig(corsConfig))

	E.POST("/", event)

	S := http.Server{
		Handler: E,
	}

	S.Addr = "0.0.0.0:8888"
	if err := S.ListenAndServe(); err != http.ErrServerClosed {
		fmt.Println("api", "api error: ", err)
	}
}

func event(c echo.Context) error {
	e := new(Event)
	err := c.Bind(e)
	if err != nil {
		return c.JSON(400, err)
	}

	fmt.Println("E:", e)
	for _, v := range e.Records {
		if strings.Contains(v.EventName, "s3:ObjectCreated") {
			fmt.Println("P:", v.S3.Bucket.Name+"/"+v.S3.Object.Key)
		} else if strings.Contains(v.EventName, "s3:ObjectRemoved") {
			fmt.Println("DEL:", v.S3.Bucket.Name+"/"+v.S3.Object.Key)
		}
	}

	return c.JSON(200, nil)
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
