package ipgeo

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ──────── cidrRangeContains ────────

func TestCidrRangeContains_Match(t *testing.T) {
	assert.True(t, cidrRangeContains("10.0.0.0/8", "10.1.2.3"))
}

func TestCidrRangeContains_NoMatch(t *testing.T) {
	assert.False(t, cidrRangeContains("10.0.0.0/8", "11.0.0.1"))
}

func TestCidrRangeContains_InvalidCIDR(t *testing.T) {
	assert.False(t, cidrRangeContains("invalid", "10.0.0.1"))
}

func TestCidrRangeContains_InvalidIP(t *testing.T) {
	// net.ParseIP("notanip") returns nil; ipNet.Contains(nil) returns false
	assert.False(t, cidrRangeContains("10.0.0.0/8", "notanip"))
}

// ──────── Filter: IPv4 RFC ranges ────────

func TestFilter_RFC1918_Private(t *testing.T) {
	for _, ip := range []string{"10.0.0.1", "172.16.0.1", "192.168.1.1"} {
		geo, ok := Filter(ip)
		require.True(t, ok, "expected %s to be filtered", ip)
		assert.Equal(t, "RFC1918", geo.Whois, "ip=%s", ip)
	}
}

func TestFilter_Loopback(t *testing.T) {
	geo, ok := Filter("127.0.0.1")
	require.True(t, ok)
	assert.Equal(t, "RFC1122", geo.Whois)
}

func TestFilter_LinkLocal(t *testing.T) {
	geo, ok := Filter("169.254.1.1")
	require.True(t, ok)
	assert.Equal(t, "RFC3927", geo.Whois)
}

func TestFilter_CGNAT(t *testing.T) {
	geo, ok := Filter("100.64.0.1")
	require.True(t, ok)
	assert.Equal(t, "RFC6598", geo.Whois)
}

func TestFilter_Documentation_192_0_2(t *testing.T) {
	geo, ok := Filter("192.0.2.1")
	require.True(t, ok)
	assert.Equal(t, "RFC5737", geo.Whois)
}

func TestFilter_Documentation_198_51_100(t *testing.T) {
	geo, ok := Filter("198.51.100.1")
	require.True(t, ok)
	assert.Equal(t, "RFC5737", geo.Whois)
}

func TestFilter_Documentation_203_0_113(t *testing.T) {
	geo, ok := Filter("203.0.113.1")
	require.True(t, ok)
	assert.Equal(t, "RFC5737", geo.Whois)
}

func TestFilter_Benchmark(t *testing.T) {
	geo, ok := Filter("198.18.0.1")
	require.True(t, ok)
	assert.Equal(t, "RFC2544", geo.Whois)
}

func TestFilter_Multicast(t *testing.T) {
	geo, ok := Filter("224.0.0.1")
	require.True(t, ok)
	assert.Equal(t, "RFC5771", geo.Whois)
}

func TestFilter_DOD(t *testing.T) {
	for _, ip := range []string{"6.0.0.1", "7.0.0.1", "11.0.0.1", "21.0.0.1", "22.0.0.1",
		"26.0.0.1", "28.0.0.1", "29.0.0.1", "30.0.0.1", "33.0.0.1", "55.0.0.1",
		"214.0.0.1", "215.0.0.1"} {
		geo, ok := Filter(ip)
		require.True(t, ok, "expected %s to be filtered as DOD", ip)
		assert.Equal(t, "DOD", geo.Whois, "ip=%s", ip)
	}
}

func TestFilter_PublicIPv4_NotFiltered(t *testing.T) {
	_, ok := Filter("8.8.8.8")
	assert.False(t, ok)
}

func TestFilter_PublicIPv4_1_1_1_1_NotFiltered(t *testing.T) {
	_, ok := Filter("1.1.1.1")
	assert.False(t, ok)
}

// ──────── Filter: IPv6 ranges ────────

func TestFilter_IPv6_LinkLocal(t *testing.T) {
	geo, ok := Filter("fe80::1")
	require.True(t, ok)
	assert.Equal(t, "RFC4291", geo.Whois)
}

func TestFilter_IPv6_ULA(t *testing.T) {
	geo, ok := Filter("fd00::1")
	require.True(t, ok)
	assert.Equal(t, "RFC4193", geo.Whois)
}

func TestFilter_IPv6_Documentation(t *testing.T) {
	geo, ok := Filter("2001:db8::1")
	require.True(t, ok)
	assert.Equal(t, "RFC3849", geo.Whois)
}

func TestFilter_IPv6_Multicast(t *testing.T) {
	geo, ok := Filter("ff02::1")
	require.True(t, ok)
	assert.Equal(t, "RFC4291", geo.Whois)
}

func TestFilter_IPv6_GlobalUnicast_NotFiltered(t *testing.T) {
	_, ok := Filter("2606:4700::1")
	assert.False(t, ok)
}
