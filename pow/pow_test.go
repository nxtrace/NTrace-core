package pow

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestGetToken(t *testing.T) {
	// 计时开始
	start := time.Now()
	token, err := GetToken("origin-fallback.nxtrace.org", "origin-fallback.nxtrace.org", "443")
	// 计时结束
	end := time.Now()
	fmt.Println("耗时：", end.Sub(start))
	fmt.Println("token:", token)
	assert.NoError(t, err, "GetToken() returned an error")
}
