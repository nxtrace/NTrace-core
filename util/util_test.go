package util

import (
	"net"
	"testing"

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
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too short")
}

func TestMakePayloadWithTargetChecksum_VersionMismatch(t *testing.T) {
	src := net.ParseIP("10.0.0.1").To4()
	dst := net.ParseIP("2001:db8::1") // v6
	payload := make([]byte, 4)
	err := MakePayloadWithTargetChecksum(payload, src, dst, 100, 200, 42)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "mismatch")
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
