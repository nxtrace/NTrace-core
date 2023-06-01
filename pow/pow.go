package pow

import (
	"fmt"
	"github.com/xgadget-lab/nexttrace/config"
	"os"
	"runtime"
)

const (
	baseURL = "https://api.leo.moe/v3/challenge"
)

// TODO: 在这里要实现优选IP

func GetToken() (string, error) {
	getTokenParams := NewGetTokenParams()
	getTokenParams.BaseUrl = baseURL
	getTokenParams.UserAgent = fmt.Sprintf("NextTrace %s/%s/%s", config.Version, runtime.GOOS, runtime.GOARCH)
	// 尝试三次RetToken，如果都失败了，异常退出
	for i := 0; i < 3; i++ {
		token, err := RetToken(getTokenParams)
		if err != nil {
			fmt.Println(err)
			continue
		}
		return token, nil
	}
	fmt.Println("RetToken failed 3 times, exit")
	os.Exit(1)
	return "", nil
}
