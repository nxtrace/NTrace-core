package provider

import (
	"context"
	"io"
	"net/http"
)

const (
	LatencyIdle         = "idle"
	LatencyLoadDownload = "download"
	LatencyLoadUpload   = "upload"
)

type RequestSpec struct {
	Method        string
	URL           string
	Headers       map[string]string
	ContentLength int64
	ResponseLimit int64
	BodyFactory   func() io.Reader
}

type Provider interface {
	Name() string
	Host() string
	UserAgent() string
	IdleLatencyRequest(ctx context.Context) (RequestSpec, error)
	LoadedLatencyRequest(ctx context.Context, phase string) (RequestSpec, error)
	DownloadRequest(ctx context.Context, maxBytes int64) (RequestSpec, error)
	UploadRequest(ctx context.Context, maxBytes int64) (RequestSpec, error)
	ParseMetadata(resp *http.Response, body []byte) map[string]any
}
