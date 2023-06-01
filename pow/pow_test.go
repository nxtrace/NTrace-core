package pow

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetToken(t *testing.T) {
	token, err := GetToken()
	fmt.Println(token, err)
	assert.NoError(t, err, "GetToken() returned an error")
}
