package util

import (
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ──────── GetIPHeaderLength ────────

func TestGetIPHeaderLength_IPv4_MinIHL(t *testing.T) {
	// IHL=5 → 20 bytes
	data := []byte{0x45} // version=4, IHL=5
	got, err := GetIPHeaderLength(data)
	require.NoError(t, err)
	assert.Equal(t, 20, got)
}

func TestGetIPHeaderLength_IPv4_WithOptions(t *testing.T) {
	// IHL=15 → 60 bytes (maximum)
	data := []byte{0x4F}
	got, err := GetIPHeaderLength(data)
	require.NoError(t, err)
	assert.Equal(t, 60, got)
}

func TestGetIPHeaderLength_IPv4_InvalidIHL(t *testing.T) {
	// IHL=3 < 5 → error
	data := []byte{0x43}
	_, err := GetIPHeaderLength(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid IPv4 header length")
}

func TestGetIPHeaderLength_IPv6(t *testing.T) {
	data := []byte{0x60} // version=6
	got, err := GetIPHeaderLength(data)
	require.NoError(t, err)
	assert.Equal(t, 40, got)
}

func TestGetIPHeaderLength_UnknownVersion(t *testing.T) {
	data := []byte{0x30} // version=3
	_, err := GetIPHeaderLength(data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown IP version")
}

func TestGetIPHeaderLength_Empty(t *testing.T) {
	_, err := GetIPHeaderLength(nil)
	assert.Error(t, err)
}

// ──────── GetICMPID / GetICMPSeq ────────

func TestGetICMPID_Valid(t *testing.T) {
	// ICMP header: type(1) code(1) checksum(2) ID(2) seq(2)
	data := make([]byte, 8)
	binary.BigEndian.PutUint16(data[4:6], 0x1234) // ID
	got, err := GetICMPID(data)
	require.NoError(t, err)
	assert.Equal(t, 0x1234, got)
}

func TestGetICMPID_TooShort(t *testing.T) {
	data := make([]byte, 5)
	_, err := GetICMPID(data)
	assert.Error(t, err)
}

func TestGetICMPSeq_Valid(t *testing.T) {
	data := make([]byte, 8)
	binary.BigEndian.PutUint16(data[6:8], 42)
	got, err := GetICMPSeq(data)
	require.NoError(t, err)
	assert.Equal(t, 42, got)
}

func TestGetICMPSeq_TooShort(t *testing.T) {
	data := make([]byte, 7)
	_, err := GetICMPSeq(data)
	assert.Error(t, err)
}

// ──────── GetTCPPorts / GetTCPSeq ────────

func TestGetTCPPorts_Valid(t *testing.T) {
	data := make([]byte, 8)
	binary.BigEndian.PutUint16(data[0:2], 12345) // src
	binary.BigEndian.PutUint16(data[2:4], 80)    // dst
	src, dst, err := GetTCPPorts(data)
	require.NoError(t, err)
	assert.Equal(t, 12345, src)
	assert.Equal(t, 80, dst)
}

func TestGetTCPPorts_TooShort(t *testing.T) {
	data := make([]byte, 3)
	_, _, err := GetTCPPorts(data)
	assert.Error(t, err)
}

func TestGetTCPSeq_Valid(t *testing.T) {
	data := make([]byte, 8)
	binary.BigEndian.PutUint32(data[4:8], 0xDEADBEEF)
	got, err := GetTCPSeq(data)
	require.NoError(t, err)
	assert.Equal(t, int(uint32(0xDEADBEEF)), got)
}

func TestGetTCPSeq_TooShort(t *testing.T) {
	data := make([]byte, 7)
	_, err := GetTCPSeq(data)
	assert.Error(t, err)
}

// ──────── GetUDPPorts / GetUDPSeq / GetUDPSeqv6 ────────

func TestGetUDPPorts_Valid(t *testing.T) {
	data := make([]byte, 8)
	binary.BigEndian.PutUint16(data[0:2], 33494)
	binary.BigEndian.PutUint16(data[2:4], 443)
	src, dst, err := GetUDPPorts(data)
	require.NoError(t, err)
	assert.Equal(t, 33494, src)
	assert.Equal(t, 443, dst)
}

func TestGetUDPPorts_TooShort(t *testing.T) {
	data := make([]byte, 3)
	_, _, err := GetUDPPorts(data)
	assert.Error(t, err)
}

func TestGetUDPSeq_Valid(t *testing.T) {
	// Build a mini IPv4 packet with IHL=5 (20 bytes) + enough for IP ID field
	ipHdr := make([]byte, 20)
	ipHdr[0] = 0x45 // v4, IHL=5
	binary.BigEndian.PutUint16(ipHdr[4:6], 9999)
	got, err := GetUDPSeq(ipHdr)
	require.NoError(t, err)
	assert.Equal(t, 9999, got)
}

func TestGetUDPSeq_TooShort(t *testing.T) {
	_, err := GetUDPSeq(nil)
	assert.Error(t, err)
}

func TestGetUDPSeqv6_Valid(t *testing.T) {
	data := make([]byte, 8)
	binary.BigEndian.PutUint16(data[6:8], 7777)
	got, err := GetUDPSeqv6(data)
	require.NoError(t, err)
	assert.Equal(t, 7777, got)
}

func TestGetUDPSeqv6_TooShort(t *testing.T) {
	data := make([]byte, 7)
	_, err := GetUDPSeqv6(data)
	assert.Error(t, err)
}

// ──────── GetICMPResponsePayload ────────

func TestGetICMPResponsePayload_IPv4_Simple(t *testing.T) {
	// IPv4 header (IHL=5, 20 bytes) + 4 bytes payload
	pkt := make([]byte, 24)
	pkt[0] = 0x45
	pkt[20] = 0xAA
	pkt[21] = 0xBB
	pkt[22] = 0xCC
	pkt[23] = 0xDD
	payload, err := GetICMPResponsePayload(pkt)
	require.NoError(t, err)
	assert.Equal(t, []byte{0xAA, 0xBB, 0xCC, 0xDD}, payload)
}

func TestGetICMPResponsePayload_IPv6_NoExtHeaders(t *testing.T) {
	// IPv6 fixed header (40 bytes) with NextHeader=58 (ICMPv6) + 4 bytes payload
	pkt := make([]byte, 44)
	pkt[0] = 0x60 // version 6
	pkt[6] = 58   // Next Header: ICMPv6 (upper-layer, not extension)
	pkt[40] = 0x11
	pkt[41] = 0x22
	pkt[42] = 0x33
	pkt[43] = 0x44
	payload, err := GetICMPResponsePayload(pkt)
	require.NoError(t, err)
	assert.Equal(t, []byte{0x11, 0x22, 0x33, 0x44}, payload)
}

func TestGetICMPResponsePayload_Empty(t *testing.T) {
	_, err := GetICMPResponsePayload(nil)
	assert.Error(t, err)
}
