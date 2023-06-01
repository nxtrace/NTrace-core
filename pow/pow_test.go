package pow

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetToken(t *testing.T) {
	token, err := GetToken("103.120.18.35", "api.leo.moe", "443")
	fmt.Println(token, err)
	assert.NoError(t, err, "GetToken() returned an error")
}
