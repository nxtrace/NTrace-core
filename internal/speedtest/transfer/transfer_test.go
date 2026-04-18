package transfer

import (
	"context"
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
