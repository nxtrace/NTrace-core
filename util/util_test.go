package util

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ──────── IsIPv6 ────────

func TestIsIPv6_True(t *testing.T) {
	assert.True(t, IsIPv6(net.ParseIP("2001:db8::1")))
	assert.True(t, IsIPv6(net.ParseIP("::1")))
	assert.True(t, IsIPv6(net.ParseIP("fe80::1")))
}

func TestIsIPv6_False(t *testing.T) {
	assert.False(t, IsIPv6(net.ParseIP("1.2.3.4")))
	assert.False(t, IsIPv6(net.ParseIP("127.0.0.1")))
	assert.False(t, IsIPv6(nil))
}

// ──────── AddrIP ────────

func TestAddrIP_IPAddr(t *testing.T) {
	ip := net.ParseIP("8.8.8.8")
	got := AddrIP(&net.IPAddr{IP: ip})
	assert.Equal(t, ip, got)
}

func TestAddrIP_IPNet(t *testing.T) {
	ip := net.ParseIP("203.0.113.8")
	got := AddrIP(&net.IPNet{IP: ip, Mask: net.CIDRMask(24, 32)})
	assert.Equal(t, ip, got)
}

func TestAddrIP_TCPAddr(t *testing.T) {
	ip := net.ParseIP("1.1.1.1")
	got := AddrIP(&net.TCPAddr{IP: ip, Port: 80})
	assert.Equal(t, ip, got)
}

func TestAddrIP_UDPAddr(t *testing.T) {
	ip := net.ParseIP("2001:db8::1")
	got := AddrIP(&net.UDPAddr{IP: ip, Port: 53})
	assert.Equal(t, ip, got)
}

func TestAddrIP_UnixAddr(t *testing.T) {
	got := AddrIP(&net.UnixAddr{Name: "/tmp/sock"})
	assert.Nil(t, got)
}

func TestAddrIP_Nil(t *testing.T) {
	got := AddrIP(nil)
	assert.Nil(t, got)
}

// ──────── StringInSlice ────────

func TestStringInSlice_Found(t *testing.T) {
	assert.True(t, StringInSlice("b", []string{"a", "b", "c"}))
}

func TestStringInSlice_NotFound(t *testing.T) {
	assert.False(t, StringInSlice("z", []string{"a", "b"}))
}

func TestStringInSlice_EmptySlice(t *testing.T) {
	assert.False(t, StringInSlice("any", nil))
}

// ──────── HideIPPart ────────

func TestHideIPPart_IPv4(t *testing.T) {
	assert.Equal(t, "192.168.0.0/16", HideIPPart("192.168.1.1"))
}

func TestHideIPPart_IPv6(t *testing.T) {
	got := HideIPPart("2001:db8::1")
	assert.Equal(t, "2001:db8::/32", got)
}

func TestHideIPPart_Invalid(t *testing.T) {
	assert.Equal(t, "", HideIPPart("notanip"))
}

// ──────── UDPBaseSum ────────

func TestUDPBaseSum_IPv4_KnownValue(t *testing.T) {
	src := net.ParseIP("192.168.1.1").To4()
	dst := net.ParseIP("10.0.0.1").To4()
	payload := make([]byte, 4) // 4-byte payload, all zero
	udpLen := 8 + len(payload)
	got := UDPBaseSum(src, dst, 12345, 80, udpLen, payload)
	// Should produce a non-zero checksum partial
	assert.NotEqual(t, uint16(0), got)
}

func TestUDPBaseSum_IPv6_NonZero(t *testing.T) {
	src := net.ParseIP("2001:db8::1")
	dst := net.ParseIP("2001:db8::2")
	payload := []byte{0, 0, 0, 0}
	udpLen := 8 + len(payload)
	got := UDPBaseSum(src, dst, 1000, 2000, udpLen, payload)
	assert.NotEqual(t, uint16(0), got)
}

// ──────── FudgeWordForSeq ────────

func TestFudgeWordForSeq_RoundTrip(t *testing.T) {
	// Given a base sum S0 and a target checksum, the fudge word should allow
	// reconstructing the target. We verify by recomputing.
	S0 := uint16(0x1234)
	target := uint16(42)
	fudge := FudgeWordForSeq(S0, target)
	// Reconstruct: fold16(S0 + fudge) should equal ~target
	sum := uint32(S0) + uint32(fudge)
	for (sum >> 16) != 0 {
		sum = (sum & 0xFFFF) + (sum >> 16)
	}
	reconstructed := ^uint16(sum)
	assert.Equal(t, target, reconstructed)
}

// ──────── MakePayloadWithTargetChecksum ────────

func TestMakePayloadWithTargetChecksum_RoundTrip(t *testing.T) {
	src := net.ParseIP("10.0.0.1").To4()
	dst := net.ParseIP("10.0.0.2").To4()
	payload := make([]byte, 8)
	targetCS := uint16(9999)

	err := MakePayloadWithTargetChecksum(payload, src, dst, 33494, 33434, targetCS)
	require.NoError(t, err)

	// Verify: compute the full checksum using the modified payload
	udpLen := 8 + len(payload)
	finalSum := UDPBaseSum(src, dst, 33494, 33434, udpLen, payload)
	finalChecksum := ^finalSum
	assert.Equal(t, targetCS, finalChecksum)
}

func TestMakePayloadWithTargetChecksum_TooShort(t *testing.T) {
	src := net.ParseIP("10.0.0.1").To4()
	dst := net.ParseIP("10.0.0.2").To4()
	payload := make([]byte, 1) // too short
	err := MakePayloadWithTargetChecksum(payload, src, dst, 100, 200, 42)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "too short")
}

func TestMakePayloadWithTargetChecksum_VersionMismatch(t *testing.T) {
	src := net.ParseIP("10.0.0.1").To4()
	dst := net.ParseIP("2001:db8::1") // v6
	payload := make([]byte, 4)
	err := MakePayloadWithTargetChecksum(payload, src, dst, 100, 200, 42)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mismatch")
}

type fakeHostLookupResolver struct {
	hosts []string
	err   error
}

func (f fakeHostLookupResolver) LookupHost(context.Context, string) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.hosts, nil
}

type fakeHostLookupResolverWithContext struct {
	lookup func(ctx context.Context, host string) ([]string, error)
}

func (f fakeHostLookupResolverWithContext) LookupHost(ctx context.Context, host string) ([]string, error) {
	return f.lookup(ctx, host)
}

type fakeAddrLookupResolver struct {
	lookup func(ctx context.Context, addr string) ([]string, error)
}

func (f fakeAddrLookupResolver) LookupAddr(ctx context.Context, addr string) ([]string, error) {
	return f.lookup(ctx, addr)
}

func TestLookupIPs_SkipsInvalidValues(t *testing.T) {
	ips, err := lookupIPs(context.Background(), fakeHostLookupResolver{
		hosts: []string{"1.1.1.1", "not-an-ip", "2606:4700::1"},
	}, "example.com")
	require.NoError(t, err)
	require.Len(t, ips, 2)
	assert.Equal(t, "1.1.1.1", ips[0].String())
	assert.Equal(t, "2606:4700::1", ips[1].String())
}

func TestLookupIPs_ReturnsWrappedError(t *testing.T) {
	_, err := lookupIPs(context.Background(), fakeHostLookupResolver{err: errors.New("boom")}, "example.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "DNS lookup failed")
}

func TestDomainLookUpWithContextReturnsContextCanceled(t *testing.T) {
	oldFactory := domainResolverFactory
	domainResolverFactory = func(string) hostLookupResolver {
		return fakeHostLookupResolverWithContext{
			lookup: func(ctx context.Context, host string) ([]string, error) {
				<-ctx.Done()
				return nil, ctx.Err()
			},
		}
	}
	defer func() { domainResolverFactory = oldFactory }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	_, err := DomainLookUpWithContext(ctx, "example.com", "all", "", true)
	require.Error(t, err)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("DomainLookUpWithContext error = %v, want context.Canceled", err)
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("DomainLookUpWithContext returned too slowly after cancel: %v", elapsed)
	}
}

func TestLookupAddrWithContextUsesCache(t *testing.T) {
	oldResolver := rdnsResolver
	rdnsResolver = fakeAddrLookupResolver{
		lookup: func(context.Context, string) ([]string, error) {
			t.Fatal("resolver should not be called when cache is warm")
			return nil, nil
		},
	}
	defer func() { rdnsResolver = oldResolver }()

	rDNSCache = sync.Map{}
	rDNSCache.Store("1.1.1.1", "cached.example.")

	names, err := LookupAddrWithContext(context.Background(), "1.1.1.1")
	require.NoError(t, err)
	require.Equal(t, []string{"cached.example."}, names)
}

func TestLookupAddrWithContextStoresResultInCache(t *testing.T) {
	oldResolver := rdnsResolver
	rdnsResolver = fakeAddrLookupResolver{
		lookup: func(context.Context, string) ([]string, error) {
			return []string{"resolver.example."}, nil
		},
	}
	defer func() { rdnsResolver = oldResolver }()

	rDNSCache = sync.Map{}

	names, err := LookupAddrWithContext(context.Background(), "1.1.1.1")
	require.NoError(t, err)
	require.Equal(t, []string{"resolver.example."}, names)

	cached, ok := rDNSCache.Load("1.1.1.1")
	require.True(t, ok)
	require.Equal(t, "resolver.example.", cached)
}

func TestLookupAddrWithContextReturnsContextCanceled(t *testing.T) {
	oldResolver := rdnsResolver
	rdnsResolver = fakeAddrLookupResolver{
		lookup: func(ctx context.Context, addr string) ([]string, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	}
	defer func() { rdnsResolver = oldResolver }()

	rDNSCache = sync.Map{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	_, err := LookupAddrWithContext(ctx, "1.1.1.1")
	require.Error(t, err)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("LookupAddrWithContext error = %v, want context.Canceled", err)
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("LookupAddrWithContext returned too slowly after cancel: %v", elapsed)
	}
}

func TestFilterByFamily_PicksFirstMatchingAddress(t *testing.T) {
	ips := []net.IP{
		net.ParseIP("2606:4700::1"),
		net.ParseIP("1.1.1.1"),
		net.ParseIP("8.8.8.8"),
	}
	filtered := filterByFamily(ips, "4")
	require.Len(t, filtered, 1)
	assert.Equal(t, "1.1.1.1", filtered[0].String())
}

func TestSelectResolvedIP_PromptErrorFallsBackToFirst(t *testing.T) {
	ips := []net.IP{net.ParseIP("1.1.1.1"), net.ParseIP("8.8.8.8")}
	selected, err := selectResolvedIP(ips, false, func([]net.IP) (int, error) {
		return 0, errors.New("stdin closed")
	})
	require.NoError(t, err)
	assert.Equal(t, "1.1.1.1", selected.String())
}

func TestSelectResolvedIP_InvalidIndex(t *testing.T) {
	ips := []net.IP{net.ParseIP("1.1.1.1"), net.ParseIP("8.8.8.8")}
	_, err := selectResolvedIP(ips, false, func([]net.IP) (int, error) {
		return 10, nil
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid selection")
}

func TestResolveFamilyLabel(t *testing.T) {
	assert.Equal(t, "IPv4", resolveFamilyLabel("4"))
	assert.Equal(t, "IPv6", resolveFamilyLabel("6"))
	assert.Equal(t, "IPv4/IPv6", resolveFamilyLabel("all"))
}

// ──────── GetPowProvider ────────

func TestGetPowProvider_Default(t *testing.T) {
	old := PowProviderParam
	oldEnv := EnvPowProvider
	defer func() { PowProviderParam = old; EnvPowProvider = oldEnv }()

	PowProviderParam = ""
	EnvPowProvider = ""
	assert.Equal(t, "", GetPowProvider())
}

func TestGetPowProvider_Sakura(t *testing.T) {
	old := PowProviderParam
	defer func() { PowProviderParam = old }()

	PowProviderParam = "sakura"
	assert.Equal(t, "pow.nexttrace.owo.13a.com", GetPowProvider())
}
