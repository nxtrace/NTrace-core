package server

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

//go:embed web/*
var webContent embed.FS

const defaultListenAddr = ":1080"

var indexPage []byte
var assetsFS fs.FS

func init() {
	var err error
	indexPage, err = webContent.ReadFile("web/index.html")
	if err != nil {
		panic(fmt.Errorf("web assets missing index.html: %w", err))
	}

	assetsFS, err = fs.Sub(webContent, "web/assets")
	if err != nil {
		panic(fmt.Errorf("web assets missing asset directory: %w", err))
	}
}

// Run starts the Gin HTTP server that exposes the traceroute UI and APIs.
func Run(listenAddr string) error {
	if listenAddr == "" {
		listenAddr = defaultListenAddr
	}

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())

	router.GET("/", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexPage)
	})

	router.StaticFS("/assets", http.FS(assetsFS))

	router.GET("/api/options", optionsHandler)
	router.POST("/api/trace", traceHandler)

	srv := &http.Server{Addr: listenAddr, Handler: router}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	err := srv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		if strings.Contains(err.Error(), "address already in use") {
			return fmt.Errorf("listen %s: %w", listenAddr, err)
		}
		return err
	}

	return nil
}
