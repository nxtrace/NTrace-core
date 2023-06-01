package pow

import (
	"fmt"
	"github.com/tsosunchia/powclient"
	"github.com/xgadget-lab/nexttrace/util"
	"net/url"
	"os"
)

const (
	baseURL = "/v3/challenge"
)

func GetToken(fastIp string, host string, port string) (string, error) {
	getTokenParams := powclient.NewGetTokenParams()
	u := url.URL{Scheme: "https", Host: fastIp + ":" + port, Path: baseURL}
	getTokenParams.BaseUrl = u.String()
	getTokenParams.SNI = host
	getTokenParams.Host = host
	getTokenParams.UserAgent = util.UserAgent
	var err error
	// 尝试三次RetToken，如果都失败了，异常退出
	for i := 0; i < 3; i++ {
		token, err := powclient.RetToken(getTokenParams)
		if err != nil {
			continue
		}
		//fmt.Println("GetToken success", token, getTokenParams.UserAgent)
		return token, nil
	}
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("RetToken failed 3 times, exit")
	os.Exit(1)
	return "", nil
}
