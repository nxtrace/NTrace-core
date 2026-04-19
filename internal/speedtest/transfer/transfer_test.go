package transfer

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/nxtrace/NTrace-core/internal/speedtest/provider"
)

func TestRunDownload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "hello world")
	}))
	defer srv.Close()

	res := Run(context.Background(), srv.Client(), provider.RequestSpec{
		Method: http.MethodGet,
		URL:    srv.URL,
	}, Download, 1, time.Second, nil)
	if res.TotalBytes == 0 {
		t.Fatal("TotalBytes = 0, want > 0")
	}
	if res.HadFault {
		t.Fatal("HadFault = true, want false")
	}
}

func TestRunAcceptsNilContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "hello")
	}))
	defer srv.Close()

	res := Run(nil, srv.Client(), provider.RequestSpec{
		Method: http.MethodGet,
		URL:    srv.URL,
	}, Download, 1, time.Second, nil)
	if res.TotalBytes == 0 {
		t.Fatal("TotalBytes = 0, want > 0")
	}
	if res.HadFault {
		t.Fatal("HadFault = true, want false")
	}
}

func TestRunUpload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	res := Run(context.Background(), srv.Client(), provider.RequestSpec{
		Method:        http.MethodPost,
		URL:           srv.URL,
		ContentLength: 32,
		BodyFactory: func() io.Reader {
			return provider.ZeroBody(32)
		},
	}, Upload, 1, time.Second, nil)
	if res.TotalBytes == 0 {
		t.Fatal("TotalBytes = 0, want > 0")
	}
	if res.HadFault {
		t.Fatal("HadFault = true, want false")
	}
}

func TestRunDownloadTimeoutDoesNotCountAsFault(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, _ := w.(http.Flusher)
		for i := 0; i < 4; i++ {
			_, _ = io.WriteString(w, "payload")
			if flusher != nil {
				flusher.Flush()
			}
			time.Sleep(40 * time.Millisecond)
		}
	}))
	defer srv.Close()

	res := Run(context.Background(), srv.Client(), provider.RequestSpec{
		Method: http.MethodGet,
		URL:    srv.URL,
	}, Download, 1, 50*time.Millisecond, nil)
	if res.TotalBytes == 0 {
		t.Fatal("TotalBytes = 0, want partial download bytes")
	}
	if res.HadFault {
		t.Fatal("HadFault = true, want false on timeout-driven stop")
	}
}

func TestRunUploadHTTPErrorKeepsCountMonotonic(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		http.Error(w, "nope", http.StatusForbidden)
	}))
	defer srv.Close()

	res := Run(context.Background(), srv.Client(), provider.RequestSpec{
		Method:        http.MethodPost,
		URL:           srv.URL,
		ContentLength: 32,
		BodyFactory: func() io.Reader {
			return provider.ZeroBody(32)
		},
	}, Upload, 1, time.Second, nil)
	if res.TotalBytes != 32 {
		t.Fatalf("TotalBytes = %d, want 32 bytes sent to remain counted", res.TotalBytes)
	}
	if !res.HadFault {
		t.Fatal("HadFault = false, want true on HTTP upload failure")
	}
}

func TestRunUploadResponseBodyReadErrorCountsFault(t *testing.T) {
	bodyErr := errors.New("response body failed")
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		_, _ = io.Copy(io.Discard, req.Body)
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       errReadCloser{err: bodyErr},
			Request:    req,
		}, nil
	})}

	res := Run(context.Background(), client, provider.RequestSpec{
		Method:        http.MethodPost,
		URL:           "http://example.test/upload",
		ContentLength: 32,
		BodyFactory: func() io.Reader {
			return provider.ZeroBody(32)
		},
	}, Upload, 1, time.Second, nil)
	if !res.HadFault {
		t.Fatal("HadFault = false, want true on response body read error")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return fn(req)
}

type errReadCloser struct {
	err error
}

func (r errReadCloser) Read([]byte) (int, error) {
	return 0, r.err
}

func (r errReadCloser) Close() error {
	return nil
}
