//go:build aix || android || darwin || dragonfly || freebsd || linux || netbsd || openbsd || solaris

package util

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeNextTraceAPIV4TokenDirInfo struct{}

func (fakeNextTraceAPIV4TokenDirInfo) Name() string       { return "nexttrace-token-dir" }
func (fakeNextTraceAPIV4TokenDirInfo) Size() int64        { return 0 }
func (fakeNextTraceAPIV4TokenDirInfo) Mode() os.FileMode  { return os.ModeDir | 0o700 }
func (fakeNextTraceAPIV4TokenDirInfo) ModTime() time.Time { return time.Time{} }
func (fakeNextTraceAPIV4TokenDirInfo) IsDir() bool        { return true }
func (fakeNextTraceAPIV4TokenDirInfo) Sys() any           { return nil }

func TestCheckNextTraceAPIV4TokenDirOwnerRejectsUnknownSys(t *testing.T) {
	err := checkNextTraceAPIV4TokenDirOwner(fakeNextTraceAPIV4TokenDirInfo{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot verify owner")
}
