package apple

import (
	"context"
	"io"
	"net/http"
	"net/url"

	"github.com/nxtrace/NTrace-core/internal/speedtest/provider"
)

const DefaultUserAgent = "networkQuality/194.80.3 CFNetwork/3860.400.51 Darwin/25.3.0"

var (
	DefaultDownloadURL = "https://mensura.cdn-apple.com/api/v1/gm/large"
	DefaultUploadURL   = "https://mensura.cdn-apple.com/api/v1/gm/slurp"
	DefaultLatencyURL  = "https://mensura.cdn-apple.com/api/v1/gm/small"
)

type Provider struct{}

func New() *Provider {
	return &Provider{}
}

func (p *Provider) Name() string {
	return "apple"
}

func (p *Provider) Host() string {
	u, err := url.Parse(DefaultLatencyURL)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

func (p *Provider) UserAgent() string {
	return DefaultUserAgent
}

func (p *Provider) IdleLatencyRequest(ctx context.Context) (provider.RequestSpec, error) {
	return provider.RequestSpec{
		Method:  http.MethodGet,
		URL:     DefaultLatencyURL,
		Headers: defaultHeaders(),
	}, nil
}

func (p *Provider) LoadedLatencyRequest(ctx context.Context, phase string) (provider.RequestSpec, error) {
	return p.IdleLatencyRequest(ctx)
}

func (p *Provider) DownloadRequest(ctx context.Context, maxBytes int64) (provider.RequestSpec, error) {
	return provider.RequestSpec{
		Method:  http.MethodGet,
		URL:     DefaultDownloadURL,
		Headers: defaultHeaders(),
	}, nil
}

func (p *Provider) UploadRequest(ctx context.Context, maxBytes int64) (provider.RequestSpec, error) {
	hs := defaultHeaders()
	hs["Upload-Draft-Interop-Version"] = "6"
	hs["Upload-Complete"] = "?1"
	return provider.RequestSpec{
		Method:        http.MethodPut,
		URL:           DefaultUploadURL,
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
