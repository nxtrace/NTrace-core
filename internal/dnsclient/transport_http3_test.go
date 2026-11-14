//go:build !flavor_tiny && !flavor_ntr

package dnsclient

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/miekg/dns"
	"github.com/natesales/q/cli"
	"github.com/quic-go/quic-go/http3"
	"github.com/stretchr/testify/require"
)

const doH3SmokeHelperEnv = "NEXTTRACE_DOH3_SMOKE_HELPER"

func TestQPublicTransportAgainstLocalDoH3(t *testing.T) {
	if os.Getenv(doH3SmokeHelperEnv) != "1" {
		// q v0.19.12 closes idle HTTP connections but not the HTTP/3
		// transport's UDP socket. Bound its lifetime to a helper process,
		// as well as bounding exchanges that have no context parameter.
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		command := exec.CommandContext(ctx, os.Args[0],
			"-test.run=^TestQPublicTransportAgainstLocalDoH3$", "-test.timeout=10s")
		command.Env = append(os.Environ(), doH3SmokeHelperEnv+"=1")
		output, err := command.CombinedOutput()
		if ctx.Err() != nil {
			t.Fatalf("HTTP/3 helper timed out: %v, output=%q", ctx.Err(), output)
		}
		if err != nil {
			t.Fatalf("HTTP/3 helper failed: %v, output=%q", err, output)
		}
		return
	}

	for _, method := range []string{http.MethodGet, http.MethodPost} {
		t.Run(method, func(t *testing.T) {
			listener, err := net.ListenPacket("udp", "127.0.0.1:0")
			require.NoError(t, err)
			t.Cleanup(func() { _ = listener.Close() })
			server := &http3.Server{
				TLSConfig: &tls.Config{
					Certificates: []tls.Certificate{testTLSCertificate(t)},
					MinVersion:   tls.VersionTLS13,
				},
				Handler: http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
					if request.ProtoMajor != 3 || request.Method != method || request.URL.Path != "/dns-query" {
						http.Error(writer, "unexpected DNS request", http.StatusBadRequest)
						return
					}
					if request.Header.Get("Accept") != "application/dns-message" {
						http.Error(writer, "missing DNS Accept header", http.StatusBadRequest)
						return
					}
					var wire []byte
					var readErr error
					if method == http.MethodGet {
						wire, readErr = base64.RawURLEncoding.DecodeString(request.URL.Query().Get("dns"))
					} else {
						if request.Header.Get("Content-Type") != "application/dns-message" {
							http.Error(writer, "missing DNS Content-Type", http.StatusBadRequest)
							return
						}
						wire, readErr = io.ReadAll(request.Body)
					}
					query := new(dns.Msg)
					if readErr != nil || query.Unpack(wire) != nil {
						http.Error(writer, "invalid DNS query", http.StatusBadRequest)
						return
					}
					response, packErr := testDNSReply(query).Pack()
					if packErr != nil {
						http.Error(writer, packErr.Error(), http.StatusInternalServerError)
						return
					}
					writer.Header().Set("Content-Type", "application/dns-message")
					_, _ = writer.Write(response)
				}),
			}
			done := make(chan error, 1)
			go func() { done <- server.Serve(listener) }()
			t.Cleanup(func() {
				_ = server.Close()
				select {
				case serveErr := <-done:
					require.ErrorIs(t, serveErr, http.ErrServerClosed)
				case <-time.After(5 * time.Second):
					t.Error("HTTP/3 server did not stop")
				}
			})

			parsed := mustParseServer(t, "https://"+listener.LocalAddr().String()+"/dns-query")
			txp, err := NewTransport(parsed, cli.Flags{
				HTTP3: true, HTTPMethod: method, ReuseConn: true,
			}, insecureTestTLSConfig())
			require.NoError(t, err)
			t.Cleanup(func() { _ = txp.Close() })
			// A second exchange also exercises the reused HTTP/3 transport.
			for range 2 {
				reply, exchangeErr := txp.Exchange(testDNSQuery())
				assertTestDNSReply(t, reply, exchangeErr)
			}
		})
	}
}
