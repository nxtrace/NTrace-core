package transfer

import (
	"context"
	"errors"
	"io"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nxtrace/NTrace-core/internal/speedtest/provider"
)

type Direction string

const (
	Download Direction = "download"
	Upload   Direction = "upload"
)

type Result struct {
	Direction  Direction
	Threads    int
	TotalBytes int64
	Duration   time.Duration
	Mbps       float64
	FaultCount int
	HadFault   bool
}

type ProgressFunc func(dir Direction, totalBytes int64, elapsed time.Duration, mbps float64)

func Run(
	ctx context.Context,
	client *http.Client,
	spec provider.RequestSpec,
	dir Direction,
	threads int,
	timeout time.Duration,
	progress ProgressFunc,
) Result {
	var totalBytes atomic.Int64
	var faultCount atomic.Int32
	var wg sync.WaitGroup

	ctx2, cancel := context.WithTimeout(ctx, timeout+2*time.Second)
	defer cancel()
	start := time.Now()

	progressDone := make(chan struct{})
	go func() {
		defer close(progressDone)
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				cur := totalBytes.Load()
				elapsed := time.Since(start)
				if elapsed > 0 && progress != nil {
					mbps := float64(cur) * 8 / (elapsed.Seconds() * 1_000_000)
					progress(dir, cur, elapsed, mbps)
				}
			case <-ctx2.Done():
				return
			}
		}
	}()

	for i := 0; i < threads; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var fault bool
			if dir == Download {
				_, fault = doDownload(ctx2, client, spec, timeout, &totalBytes)
			} else {
				_, fault = doUpload(ctx2, client, spec, timeout, &totalBytes)
			}
			if fault {
				faultCount.Add(1)
			}
		}()
	}

	wg.Wait()
	cancel()
	<-progressDone

	dur := time.Since(start)
	total := totalBytes.Load()
	secs := dur.Seconds()
	if secs <= 0 {
		secs = 1
	}
	mbps := float64(total) * 8 / (secs * 1_000_000)
	fc := int(faultCount.Load())
	return Result{
		Direction:  dir,
		Threads:    threads,
		TotalBytes: total,
		Duration:   dur,
		Mbps:       mbps,
		FaultCount: fc,
		HadFault:   fc > 0,
	}
}

func doDownload(ctx context.Context, client *http.Client, spec provider.RequestSpec, timeout time.Duration, shared *atomic.Int64) (int64, bool) {
	ctx2, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := newRequest(ctx2, spec)
	if err != nil {
		return 0, true
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, true
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode >= http.StatusBadRequest {
		return 0, true
	}
	buf := make([]byte, 256*1024)
	var total int64
	for {
		readBuf := buf
		if remaining := spec.ResponseLimit - total; spec.ResponseLimit > 0 && remaining <= 0 {
			break
		} else if spec.ResponseLimit > 0 && remaining < int64(len(buf)) {
			readBuf = buf[:int(remaining)]
		}
		n, readErr := resp.Body.Read(readBuf)
		if n > 0 {
			total += int64(n)
			shared.Add(int64(n))
		}
		if readErr != nil {
			if isExpectedTransferStop(readErr) {
				return total, false
			}
			return total, true
		}
	}
	return total, false
}

type countingReader struct {
	r      io.Reader
	count  atomic.Int64
	shared *atomic.Int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	if n > 0 {
		c.count.Add(int64(n))
		if c.shared != nil {
			c.shared.Add(int64(n))
		}
	}
	return n, err
}

func doUpload(ctx context.Context, client *http.Client, spec provider.RequestSpec, timeout time.Duration, shared *atomic.Int64) (int64, bool) {
	ctx2, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if spec.BodyFactory == nil {
		return 0, true
	}
	cr := &countingReader{r: spec.BodyFactory(), shared: shared}
	req, err := http.NewRequestWithContext(ctx2, spec.Method, spec.URL, cr)
	if err != nil {
		return 0, true
	}
	req.ContentLength = spec.ContentLength
	for k, v := range spec.Headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		if isExpectedTransferStop(err) {
			return cr.count.Load(), false
		}
		return cr.count.Load(), true
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= http.StatusBadRequest {
		return cr.count.Load(), true
	}
	return cr.count.Load(), false
}

func newRequest(ctx context.Context, spec provider.RequestSpec) (*http.Request, error) {
	var body io.Reader
	if spec.BodyFactory != nil {
		body = spec.BodyFactory()
	}
	req, err := http.NewRequestWithContext(ctx, spec.Method, spec.URL, body)
	if err != nil {
		return nil, err
	}
	if spec.ContentLength != 0 {
		req.ContentLength = spec.ContentLength
	}
	for k, v := range spec.Headers {
		req.Header.Set(k, v)
	}
	return req, nil
}

func isExpectedTransferStop(err error) bool {
	return err == nil ||
		errors.Is(err, io.EOF) ||
		errors.Is(err, context.Canceled) ||
		errors.Is(err, context.DeadlineExceeded)
}
