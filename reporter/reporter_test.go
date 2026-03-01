package reporter

import (
	"bytes"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/nxtrace/NTrace-core/ipgeo"
	"github.com/nxtrace/NTrace-core/trace"
)

var testResult = &trace.Result{
	Hops: [][]trace.Hop{
		{
			{
				Success:  true,
				Address:  &net.IPAddr{IP: net.ParseIP("192.168.3.1")},
				Hostname: "test",
				TTL:      0,
				RTT:      10 * time.Millisecond,
				Error:    nil,
				Geo: &ipgeo.IPGeoData{
					Asnumber: "4808",
					Country:  "中国",
					Prov:     "北京市",
					City:     "北京市",
					District: "北京市",
					Owner:    "",
					Isp:      "中国联通",
				},
			},
		},
		{
			{
				Success:  true,
				Address:  &net.IPAddr{IP: net.ParseIP("114.249.16.1")},
				Hostname: "test",
				TTL:      0,
				RTT:      10 * time.Millisecond,
				Error:    nil,
				Geo: &ipgeo.IPGeoData{
					Asnumber: "4808",
					Country:  "中国",
					Prov:     "北京市",
					City:     "北京市",
					District: "北京市",
					Owner:    "",
					Isp:      "中国联通",
				},
			},
		},
		{
			{
				Success:  true,
				Address:  &net.IPAddr{IP: net.ParseIP("219.158.5.150")},
				Hostname: "test",
				TTL:      0,
				RTT:      10 * time.Millisecond,
				Error:    nil,
				Geo: &ipgeo.IPGeoData{
					Asnumber: "4837",
					Country:  "中国",
					Prov:     "",
					City:     "",
					District: "",
					Owner:    "",
					Isp:      "中国联通",
				},
			},
		},
		{
			{
				Success:  true,
				Address:  &net.IPAddr{IP: net.ParseIP("62.115.125.160")},
				Hostname: "test",
				TTL:      0,
				RTT:      10 * time.Millisecond,
				Error:    nil,
				Geo: &ipgeo.IPGeoData{
					Asnumber: "1299",
					Country:  "Sweden",
					Prov:     "Stockholm County",
					City:     "Stockholm",
					District: "",
					Owner:    "",
					Isp:      "Telia Company AB",
				},
			},
		},
		{
			{
				Success:  true,
				Address:  &net.IPAddr{IP: net.ParseIP("213.226.68.73")},
				Hostname: "test",
				TTL:      0,
				RTT:      10 * time.Millisecond,
				Error:    nil,
				Geo: &ipgeo.IPGeoData{
					Asnumber: "56630",
					Country:  "Germany",
					Prov:     "Hesse, Frankfurt",
					City:     "",
					District: "",
					Owner:    "",
					Isp:      "Melbikomas UAB",
				},
			},
		},
	},
}

// captureStdout redirects os.Stdout to a pipe and returns the captured output.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	r.Close()
	return buf.String()
}

func TestPrint(t *testing.T) {
	output := captureStdout(t, func() {
		r := New(testResult, "213.226.68.73")
		r.Print()
	})

	// 验证实验标签
	if !strings.Contains(output, "Route-Path 功能实验室") {
		t.Error("expected output to contain experiment tag 'Route-Path 功能实验室'")
	}

	// 验证包含各 ASN
	for _, asn := range []string{"AS4808", "AS4837", "AS1299", "AS56630"} {
		if !strings.Contains(output, asn) {
			t.Errorf("expected output to contain %s", asn)
		}
	}

	// 验证包含 ISP 名称
	for _, isp := range []string{"中国联通", "Telia Company AB", "Melbikomas UAB"} {
		if !strings.Contains(output, isp) {
			t.Errorf("expected output to contain ISP %q", isp)
		}
	}

	// 验证包含跨 ASN 分隔符
	if !strings.Contains(output, "╭╯") || !strings.Contains(output, "╰") {
		t.Error("expected output to contain ASN transition markers (╭╯/╰)")
	}

	// 验证包含地理信息括号
	if !strings.Contains(output, "「") || !strings.Contains(output, "」") {
		t.Error("expected output to contain geographic brackets (「」)")
	}

	// 验证包含城市分隔
	if !strings.Contains(output, "『") || !strings.Contains(output, "』") {
		t.Error("expected output to contain city brackets (『』)")
	}
}
