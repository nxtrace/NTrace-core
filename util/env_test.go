package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetEnvTrimmed(t *testing.T) {
	t.Setenv("TEST_TRIMMED_KEY", "  value  ")

	val, ok := GetEnvTrimmed("TEST_TRIMMED_KEY")
	assert.True(t, ok)
	assert.Equal(t, "value", val)

	_, ok = GetEnvTrimmed("TEST_TRIMMED_MISSING")
	assert.False(t, ok)
}

func TestGetEnvBool(t *testing.T) {
	t.Setenv("TEST_BOOL_TRUE", "1")
	assert.True(t, GetEnvBool("TEST_BOOL_TRUE", false))

	t.Setenv("TEST_BOOL_FALSE", "0")
	assert.False(t, GetEnvBool("TEST_BOOL_FALSE", true))

	t.Setenv("TEST_BOOL_INVALID", "maybe")
	assert.True(t, GetEnvBool("TEST_BOOL_INVALID", true))
}

func TestGetEnvDefault(t *testing.T) {
	t.Setenv("TEST_DEFAULT_KEY", " custom ")
	assert.Equal(t, "custom", GetEnvDefault("TEST_DEFAULT_KEY", "fallback"))

	assert.Equal(t, "fallback", GetEnvDefault("TEST_DEFAULT_MISSING", "fallback"))
}

func TestGetEnvInt(t *testing.T) {
	t.Setenv("TEST_INT_VALID", " 42 ")
	assert.Equal(t, 42, GetEnvInt("TEST_INT_VALID", 7))

	t.Setenv("TEST_INT_INVALID", "NaN")
	assert.Equal(t, 5, GetEnvInt("TEST_INT_INVALID", 5))

	assert.Equal(t, 9, GetEnvInt("TEST_INT_MISSING", 9))
}
