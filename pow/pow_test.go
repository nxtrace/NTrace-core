package pow

import (
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestGetToken(t *testing.T) {
	// 网络可达性前置检查：尝试 TCP 连接目标服务器
	conn, err := net.DialTimeout("tcp", "origin-fallback.nxtrace.org:443", 3*time.Second)
	if err != nil {
		t.Skipf("skipping: network unreachable (origin-fallback.nxtrace.org:443): %v", err)
	}
	conn.Close()

	// 计时开始
	start := time.Now()
	token, err := GetToken("origin-fallback.nxtrace.org", "origin-fallback.nxtrace.org", "443")
	// 计时结束
	end := time.Now()
	fmt.Println("耗时：", end.Sub(start))
	fmt.Println("token:", token)
	assert.NoError(t, err, "GetToken() returned an error")
}
