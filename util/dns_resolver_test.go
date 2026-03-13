package util

import (
	"context"
	"net"
	"testing"
	"time"
)

// ── ResolverForDot 映射 ─────────────────────────────

func TestResolverMapping(t *testing.T) {
	known := []string{"dnssb", "aliyun", "dnspod", "google", "cloudflare"}
	for _, name := range known {
		r := ResolverForDot(name)
		if r == nil {
			t.Fatalf("ResolverForDot(%q) returned nil, want non-nil", name)
		}
		// 确认是自定义 dialer（PreferGo = true 且有 Dial）
		if !r.PreferGo {
			t.Errorf("ResolverForDot(%q).PreferGo = false, want true", name)
		}
	}
	// 空字符串 / 未知值返回 nil（表示系统默认）
	for _, name := range []string{"", "unknown", "xxx"} {
		if r := ResolverForDot(name); r != nil {
			t.Errorf("ResolverForDot(%q) = %v, want nil", name, r)
		}
	}
}

// ── IP 字面量短路 ────────────────────────────────────

func TestLookupHostForGeo_IPLiteral(t *testing.T) {
	// 无论 DoT 配置为何，IP 字面量应直接返回，不触发 DNS 查询。
	SetGeoDNSResolver("dnssb")
	defer SetGeoDNSResolver("")

	cases := []string{"1.1.1.1", "::1", "2001:db8::1", "192.168.0.1"}
	for _, addr := range cases {
		ips, err := LookupHostForGeo(context.Background(), addr)
		if err != nil {
			t.Errorf("LookupHostForGeo(%q) err = %v, want nil", addr, err)
			continue
		}
		if len(ips) != 1 || ips[0].String() != net.ParseIP(addr).String() {
			t.Errorf("LookupHostForGeo(%q) = %v, want [%s]", addr, ips, addr)
		}
	}
}

// ── DoT 成功时不走 fallback ──────────────────────────

func TestLookupHostForGeo_DoTSuccess(t *testing.T) {
	// 使用 Cloudflare DoT 解析一个可靠域名
	SetGeoDNSResolver("cloudflare")
	SetGeoDNSFallback(true)
	defer func() {
		SetGeoDNSResolver("")
		SetGeoDNSFallback(true)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ips, err := LookupHostForGeo(ctx, "one.one.one.one")
	if err != nil {
		t.Skipf("DoT lookup failed (network issue?): %v", err)
	}
	if len(ips) == 0 {
		t.Error("expected at least 1 IP, got 0")
	}
}

// ── 未配置 DoT 时直接走系统 DNS ──────────────────────

func TestLookupHostForGeo_NoDotFallsToSystem(t *testing.T) {
	// dotServer 为空 → ResolverForDot 返回 nil → 直接走系统 DNS。
	SetGeoDNSResolver("")
	SetGeoDNSFallback(true)
	defer func() {
		SetGeoDNSResolver("")
		SetGeoDNSFallback(true)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	ips, err := LookupHostForGeo(ctx, "one.one.one.one")
	if err != nil {
		t.Skipf("System DNS lookup failed (network issue?): %v", err)
	}
	if len(ips) == 0 {
		t.Error("expected at least 1 IP, got 0")
	}
}

// ── DoT 失败后回退系统 DNS ───────────────────────────

func TestLookupHostForGeo_DoTFailFallback(t *testing.T) {
	// 注入一个必定失败的 resolver（连接不可达地址），
	// 验证 fallback=true 时能回退到系统 DNS 成功解析。
	badResolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			// 拨向 RFC 5737 文档专用地址，必定失败
			return net.DialTimeout("tcp", "192.0.2.1:853", 200*time.Millisecond)
		},
	}
	SetGeoDNSResolver("cloudflare") // 需要非空，ResolverForDot 会被 override 覆盖
	SetGeoDNSFallback(true)
	setGeoResolverOverride(badResolver)
	defer func() {
		setGeoResolverOverride(nil)
		SetGeoDNSResolver("")
		SetGeoDNSFallback(true)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	ips, err := LookupHostForGeo(ctx, "one.one.one.one")
	if err != nil {
		t.Skipf("System DNS fallback also failed (network issue?): %v", err)
	}
	if len(ips) == 0 {
		t.Error("expected at least 1 IP from system DNS fallback, got 0")
	}
}

// ── DoT 失败且 fallback=false 时返回错误 ─────────────

func TestLookupHostForGeo_DoTFailNoFallback(t *testing.T) {
	badResolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			return net.DialTimeout("tcp", "192.0.2.1:853", 200*time.Millisecond)
		},
	}
	SetGeoDNSResolver("cloudflare")
	SetGeoDNSFallback(false)
	setGeoResolverOverride(badResolver)
	defer func() {
		setGeoResolverOverride(nil)
		SetGeoDNSResolver("")
		SetGeoDNSFallback(true)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, err := LookupHostForGeo(ctx, "one.one.one.one")
	if err == nil {
		t.Error("expected error when DoT fails and fallback=false, got nil")
	}
}

// ── SetGeoDNSResolver / SetGeoDNSFallback 并发安全 ──

func TestGeoDNSConfig_ConcurrentAccess(t *testing.T) {
	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			SetGeoDNSResolver("google")
			SetGeoDNSFallback(false)
		}
		close(done)
	}()
	for i := 0; i < 1000; i++ {
		_, _ = getGeoDNSConfig()
	}
	<-done
	// 无 data race = 通过
}

func TestWithGeoDNSResolver_RestoresPreviousConfig(t *testing.T) {
	SetGeoDNSResolver("google")
	SetGeoDNSFallback(false)
	defer func() {
		SetGeoDNSResolver("")
		SetGeoDNSFallback(true)
	}()

	seenDot := ""
	seenFallback := true
	got, err := WithGeoDNSResolver("cloudflare", func() (string, error) {
		seenDot, seenFallback = getGeoDNSConfig()
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("WithGeoDNSResolver returned error: %v", err)
	}
	if got != "ok" {
		t.Fatalf("WithGeoDNSResolver result = %q, want ok", got)
	}
	if seenDot != "cloudflare" {
		t.Fatalf("callback saw dot resolver %q, want cloudflare", seenDot)
	}
	if seenFallback {
		t.Fatalf("callback saw fallback=true, want inherited false")
	}

	dot, fallback := getGeoDNSConfig()
	if dot != "google" || fallback {
		t.Fatalf("resolver restored to (%q, %t), want (%q, %t)", dot, fallback, "google", false)
	}
}

func TestWithGeoDNSResolver_AllowsNestedSameResolver(t *testing.T) {
	SetGeoDNSResolver("google")
	SetGeoDNSFallback(false)
	defer func() {
		SetGeoDNSResolver("")
		SetGeoDNSFallback(true)
	}()

	done := make(chan struct{})
	var (
		got            string
		err            error
		outerDot       string
		outerFallback  bool
		nestedDot      string
		nestedFallback bool
	)

	go func() {
		defer close(done)
		got, err = WithGeoDNSResolver("cloudflare", func() (string, error) {
			outerDot, outerFallback = getGeoDNSConfig()
			return WithGeoDNSResolver("cloudflare", func() (string, error) {
				nestedDot, nestedFallback = getGeoDNSConfig()
				return "nested", nil
			})
		})
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("nested WithGeoDNSResolver call deadlocked")
	}

	if err != nil {
		t.Fatalf("nested WithGeoDNSResolver returned error: %v", err)
	}
	if got != "nested" {
		t.Fatalf("nested WithGeoDNSResolver result = %q, want nested", got)
	}
	if outerDot != "cloudflare" || outerFallback {
		t.Fatalf("outer callback saw (%q, %t), want (%q, %t)", outerDot, outerFallback, "cloudflare", false)
	}
	if nestedDot != "cloudflare" || nestedFallback {
		t.Fatalf("nested callback saw (%q, %t), want (%q, %t)", nestedDot, nestedFallback, "cloudflare", false)
	}

	dot, fallback := getGeoDNSConfig()
	if dot != "google" || fallback {
		t.Fatalf("resolver restored to (%q, %t), want (%q, %t)", dot, fallback, "google", false)
	}
}
