package cmd

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsDigitsOnly(t *testing.T) {
	assert.True(t, isDigitsOnly("12345"))
	assert.False(t, isDigitsOnly("12a45"))
	assert.False(t, isDigitsOnly(""))
}

func TestBuildListenInfoPortOnly(t *testing.T) {
	info := buildListenInfo("8080")
	assert.Equal(t, "http://0.0.0.0:8080", info.Binding)
	assert.NotEmpty(t, info.Access)
	assert.True(t, strings.HasSuffix(info.Access, ":8080"))
}

func TestBuildListenInfoHostPort(t *testing.T) {
	info := buildListenInfo("192.0.2.1:9000")
	assert.Equal(t, "http://192.0.2.1:9000", info.Binding)
	assert.Equal(t, "http://192.0.2.1:9000", info.Access)
}

func TestBuildListenInfoKeepsInvalidInput(t *testing.T) {
	info := buildListenInfo("not a valid endpoint")
	assert.Equal(t, "not a valid endpoint", info.Binding)
	assert.Empty(t, info.Access)
}
