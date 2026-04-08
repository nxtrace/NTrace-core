package server

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net"
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
	return RunWithReady(listenAddr, nil)
}

// RunWithReady starts the Gin HTTP server and invokes onReady after the listener
// has been successfully bound.
func RunWithReady(listenAddr string, onReady func(net.Addr)) error {
	if listenAddr == "" {
		listenAddr = defaultListenAddr
	}

	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	router.Use(browserAccessMiddleware())

	router.OPTIONS("/*path", func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})

	router.GET("/", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexPage)
	})

	router.StaticFS("/assets", http.FS(assetsFS))

	router.GET("/api/options", optionsHandler)
	router.POST("/api/trace", traceHandler)
	router.POST("/api/cache/clear", cacheClearHandler)
	router.GET("/ws/trace", traceWebsocketHandler)

	srv := &http.Server{Addr: listenAddr, Handler: router}
	listener, err := listenHTTP(listenAddr)
	if err != nil {
		return err
	}
	if onReady != nil {
		onReady(listener.Addr())
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()

	err = srv.Serve(listener)
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		if strings.Contains(err.Error(), "address already in use") {
			return fmt.Errorf("listen %s: %w", listenAddr, err)
		}
		return err
	}

	return nil
}

func listenHTTP(listenAddr string) (net.Listener, error) {
	if listenAddr == "" {
		listenAddr = defaultListenAddr
	}
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		if strings.Contains(err.Error(), "address already in use") {
			return nil, fmt.Errorf("listen %s: %w", listenAddr, err)
		}
		return nil, err
	}
	return listener, nil
}
