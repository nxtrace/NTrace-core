package apple

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/nxtrace/NTrace-core/internal/speedtest/provider"
)

const DefaultUserAgent = "networkQuality/194.80.3 CFNetwork/3860.400.51 Darwin/25.3.0"

const (
	defaultDownloadURL = "https://mensura.cdn-apple.com/api/v1/gm/large"
	defaultUploadURL   = "https://mensura.cdn-apple.com/api/v1/gm/slurp"
	defaultLatencyURL  = "https://mensura.cdn-apple.com/api/v1/gm/small"
)

var appleURLs = struct {
	mu       sync.RWMutex
	download string
	upload   string
	latency  string
}{
	download: defaultDownloadURL,
	upload:   defaultUploadURL,
	latency:  defaultLatencyURL,
}

type Provider struct{}

func New() *Provider {
	return &Provider{}
}

func (p *Provider) Name() string {
	return "apple"
}

func (p *Provider) Host() string {
	downloadURL, uploadURL, latencyURL := currentURLs()
	hosts := []string{
		parseHost(downloadURL),
		parseHost(uploadURL),
		parseHost(latencyURL),
	}
	if hosts[0] == "" {
		return ""
	}
	for _, host := range hosts[1:] {
		if host != hosts[0] {
			return ""
		}
	}
	return hosts[0]
}

func (p *Provider) UserAgent() string {
	return DefaultUserAgent
}

func (p *Provider) IdleLatencyRequest(ctx context.Context) (provider.RequestSpec, error) {
	_, _, latencyURL := currentURLs()
	return provider.RequestSpec{
		Method:  http.MethodGet,
		URL:     latencyURL,
		Headers: defaultHeaders(),
	}, nil
}

func (p *Provider) LoadedLatencyRequest(ctx context.Context, phase string) (provider.RequestSpec, error) {
	return p.IdleLatencyRequest(ctx)
}

func (p *Provider) DownloadRequest(ctx context.Context, maxBytes int64) (provider.RequestSpec, error) {
	downloadURL, _, _ := currentURLs()
	return provider.RequestSpec{
		Method:        http.MethodGet,
		URL:           downloadURL,
		Headers:       defaultHeaders(),
		ResponseLimit: maxBytes,
	}, nil
}

func (p *Provider) UploadRequest(ctx context.Context, maxBytes int64) (provider.RequestSpec, error) {
	_, uploadURL, _ := currentURLs()
	hs := defaultHeaders()
	hs["Upload-Draft-Interop-Version"] = "6"
	hs["Upload-Complete"] = "?1"
	return provider.RequestSpec{
		Method:        http.MethodPut,
		URL:           uploadURL,
		Headers:       hs,
		ContentLength: -1,
		BodyFactory: func() io.Reader {
			return provider.ZeroBody(maxBytes)
		},
	}, nil
}

func (p *Provider) ParseMetadata(resp *http.Response, body []byte) map[string]any {
	if resp == nil {
		return nil
	}
	meta := map[string]any{}
	if v := resp.Header.Get("Via"); v != "" {
		meta["via"] = v
	}
	if v := resp.Header.Get("X-Cache"); v != "" {
		meta["x_cache"] = v
	}
	if v := resp.Header.Get("CDNUUID"); v != "" {
		meta["cdn_uuid"] = v
	}
	if len(meta) == 0 {
		return nil
	}
	return meta
}

func defaultHeaders() map[string]string {
	return map[string]string{
		"User-Agent":      DefaultUserAgent,
		"Accept":          "*/*",
		"Accept-Language": "zh-CN,zh-Hans;q=0.9",
		"Accept-Encoding": "identity",
	}
}

func SetBaseForTest(base string) func() {
	base = strings.TrimRight(strings.TrimSpace(base), "/")
	prevDown, prevUp, prevLatency := currentURLs()
	appleURLs.mu.Lock()
	appleURLs.download = base + "/api/v1/gm/large"
	appleURLs.upload = base + "/api/v1/gm/slurp"
	appleURLs.latency = base + "/api/v1/gm/small"
	appleURLs.mu.Unlock()
	return func() {
		appleURLs.mu.Lock()
		appleURLs.download = prevDown
		appleURLs.upload = prevUp
		appleURLs.latency = prevLatency
		appleURLs.mu.Unlock()
	}
}

func currentURLs() (download, upload, latency string) {
	appleURLs.mu.RLock()
	defer appleURLs.mu.RUnlock()
	return appleURLs.download, appleURLs.upload, appleURLs.latency
}

func parseHost(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Hostname()
}
