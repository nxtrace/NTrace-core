//go:build windows && amd64

package internal

import (
	"errors"
	"testing"

	"github.com/google/gopacket/layers"
	wd "github.com/xjasonlyu/windivert-go"
	"golang.org/x/sys/windows"
)

func TestShouldUseICMPv6RawSend(t *testing.T) {
	if shouldUseICMPv6RawSend(nil) {
		t.Fatal("nil header should not use raw send")
	}
	if shouldUseICMPv6RawSend(&layers.IPv6{}) {
		t.Fatal("zero traffic class should keep socket send")
	}
	if !shouldUseICMPv6RawSend(&layers.IPv6{TrafficClass: 46}) {
		t.Fatal("non-zero traffic class should use raw send")
	}
}

func TestShouldUseICMPv4RawSend(t *testing.T) {
	if shouldUseICMPv4RawSend(nil) {
		t.Fatal("nil header should not use raw send")
	}
	if shouldUseICMPv4RawSend(&layers.IPv4{}) {
		t.Fatal("zero tos should keep socket send")
	}
	if shouldUseICMPv4RawSend(&layers.IPv4{TOS: 46}) {
		t.Fatal("non-zero tos should keep socket send on Windows ICMPv4")
	}
}

func TestEnsureICMPSendHandlePreservesWrappedWinDivertError(t *testing.T) {
	oldCheck := checkWinDivertDLL
	oldOpen := openWinDivertCall
	checkWinDivertDLL = func() error {
		return &winDivertError{
			Kind:  winDivertErrorDriverMissing,
			Cause: wd.Error(windows.ERROR_FILE_NOT_FOUND),
		}
	}
	openWinDivertCall = func(string, uint64) (wd.Handle, error) {
		t.Fatal("openWinDivertCall should not run when DLL check fails")
		return 0, nil
	}
	defer func() {
		checkWinDivertDLL = oldCheck
		openWinDivertCall = oldOpen
	}()

	err := (&ICMPSpec{}).ensureICMPSendHandle(true)
	if err == nil {
		t.Fatal("ensureICMPSendHandle() error = nil, want non-nil")
	}
	var wrapped *winDivertError
	if !errors.As(err, &wrapped) {
		t.Fatalf("ensureICMPSendHandle() error = %T, want wrapped *winDivertError", err)
	}
	if wrapped.Kind != winDivertErrorDriverMissing {
		t.Fatalf("wrapped.Kind = %v, want %v", wrapped.Kind, winDivertErrorDriverMissing)
	}
}
