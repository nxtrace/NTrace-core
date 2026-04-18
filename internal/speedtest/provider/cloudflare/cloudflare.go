package cloudflare

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/nxtrace/NTrace-core/internal/speedtest/provider"
)

const DefaultUserAgent = "NextTrace-Speed/1"

var DefaultBaseURL = "https://speed.cloudflare.com"

type Provider struct {
	measID string
}

func New(measID string) *Provider {
	return &Provider{measID: strings.TrimSpace(measID)}
}

func (p *Provider) Name() string {
	return "cloudflare"
}

func (p *Provider) Host() string {
	u, err := url.Parse(DefaultBaseURL)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

func (p *Provider) UserAgent() string {
	return DefaultUserAgent
}

func (p *Provider) IdleLatencyRequest(ctx context.Context) (provider.RequestSpec, error) {
	endpoint, err := p.buildDownURL(0, provider.LatencyIdle)
	if err != nil {
		return provider.RequestSpec{}, err
	}
	return provider.RequestSpec{
		Method:  http.MethodGet,
		URL:     endpoint,
		Headers: defaultHeaders(),
	}, nil
}

func (p *Provider) LoadedLatencyRequest(ctx context.Context, phase string) (provider.RequestSpec, error) {
	endpoint, err := p.buildDownURL(0, phase)
	if err != nil {
		return provider.RequestSpec{}, err
	}
	return provider.RequestSpec{
		Method:  http.MethodGet,
		URL:     endpoint,
		Headers: defaultHeaders(),
	}, nil
}

func (p *Provider) DownloadRequest(ctx context.Context, maxBytes int64) (provider.RequestSpec, error) {
	endpoint, err := p.buildDownURL(maxBytes, "")
	if err != nil {
		return provider.RequestSpec{}, err
	}
	return provider.RequestSpec{
		Method:  http.MethodGet,
		URL:     endpoint,
		Headers: defaultHeaders(),
	}, nil
}

func (p *Provider) UploadRequest(ctx context.Context, maxBytes int64) (provider.RequestSpec, error) {
	u, err := url.Parse(DefaultBaseURL)
	if err != nil {
		return provider.RequestSpec{}, err
	}
	up := u.JoinPath("/__up")
	q := up.Query()
	if p.measID != "" {
		q.Set("measId", p.measID)
	}
	up.RawQuery = q.Encode()
	return provider.RequestSpec{
		Method:        http.MethodPost,
		URL:           up.String(),
		Headers:       defaultHeaders(),
		ContentLength: maxBytes,
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

	setStringHeader := func(key, header string) {
		if v := resp.Header.Get(header); v != "" {
			meta[key] = v
		}
	}
	setStringHeader("client_ip", "cf-meta-ip")
	setStringHeader("colo", "cf-meta-colo")
	setStringHeader("city", "cf-meta-city")
	setStringHeader("country", "cf-meta-country")
	setStringHeader("postal_code", "cf-meta-postalCode")
	setStringHeader("timezone", "cf-meta-timezone")
	setStringHeader("cf_ray", "cf-ray")
	setStringHeader("request_time", "cf-meta-request-time")
	if v := resp.Header.Get("cf-meta-asn"); v != "" {
		meta["asn"] = v
	}
	if v := resp.Header.Get("cf-connecting-ip"); v != "" && meta["client_ip"] == nil {
		meta["client_ip"] = v
	}
	if meta["colo"] == nil {
		if ray, ok := meta["cf_ray"].(string); ok {
			parts := strings.Split(ray, "-")
			if len(parts) > 1 {
				meta["colo"] = parts[len(parts)-1]
			}
		}
	}
	if len(body) > 0 {
		for _, line := range strings.Split(string(body), "\n") {
			key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
			if !ok || value == "" {
				continue
			}
			switch key {
			case "ip":
				if meta["client_ip"] == nil {
					meta["client_ip"] = value
				}
			case "colo":
				if meta["colo"] == nil {
					meta["colo"] = value
				}
			case "loc":
				if meta["country"] == nil {
					meta["country"] = value
				}
			case "tls":
				meta["tls_version"] = value
			}
		}
	}
	if len(meta) == 0 {
		return nil
	}
	return meta
}

func (p *Provider) buildDownURL(bytes int64, phase string) (string, error) {
	u, err := url.Parse(DefaultBaseURL)
	if err != nil {
		return "", err
	}
	down := u.JoinPath("/__down")
	q := down.Query()
	q.Set("bytes", strconv.FormatInt(bytes, 10))
	switch phase {
	case provider.LatencyIdle:
		if p.measID != "" {
			q.Set("measId", p.measID)
		}
	case provider.LatencyLoadDownload, provider.LatencyLoadUpload:
		q.Set("during", phase)
	default:
		if p.measID != "" {
			q.Set("measId", p.measID)
		}
	}
	down.RawQuery = q.Encode()
	return down.String(), nil
}

func defaultHeaders() map[string]string {
	return map[string]string{
		"User-Agent": DefaultUserAgent,
		"Referer":    "https://speed.cloudflare.com/",
		"Accept":     "*/*",
	}
}

func NewMeasurementID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 10)
}
