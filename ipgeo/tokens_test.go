package ipgeo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBaseOrDefault_Empty(t *testing.T) {
	td := &tokenData{baseUrl: ""}
	assert.Equal(t, "https://default.example.com", td.BaseOrDefault("https://default.example.com"))
}

func TestBaseOrDefault_Custom(t *testing.T) {
	td := &tokenData{baseUrl: "https://custom.example.com"}
	assert.Equal(t, "https://custom.example.com", td.BaseOrDefault("https://default.example.com"))
}
