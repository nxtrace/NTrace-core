package ipgeo

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"testing"
)

// import (
// 	"testing"

// 	"github.com/stretchr/testify/assert"
// )

func TestXxx(t *testing.T) {
	const IdFixedHeader = "10"
	var processID = fmt.Sprintf("%07b", os.Getpid()&0x7f) //取进程ID的前7位
	var ttl = fmt.Sprintf("%06b", 95)                     //取TTL的后6位
	fmt.Println(os.Getpid()&0x7f, 95)

	var parity int
	id := IdFixedHeader + processID + ttl
	for _, c := range id {
		if c == '1' {
			parity++
		}
	}
	if parity%2 == 0 {
		id += "1"
	} else {
		id += "0"
	}
	processId, ttlR, _ := reverseID(id)
	log.Println(processId, ttlR)
}

func TestFilter(t *testing.T) {
	res, err := Filter("fd11::1")
	//打印whois信息
	fmt.Println(res.Whois)
	print(err)
}

func reverseID(id string) (int64, int64, error) {
	ttl, _ := strconv.ParseInt(id[9:15], 2, 32)
	//process ID
	processID, _ := strconv.ParseInt(id[2:9], 2, 32)

	parity := 0
	for i := 0; i < len(id)-1; i++ {
		if id[i] == '1' {
			parity++
		}
	}

	if parity%2 == 1 {
		if id[len(id)-1] == '0' {
			fmt.Println("Parity check passed.")
		} else {
			fmt.Println("Parity check failed.")
			return 0, 0, errors.New("err")
		}
	} else {
		if id[len(id)-1] == '1' {
			fmt.Println("Parity check passed.")
		} else {
			fmt.Println("Parity check failed.")
			return 0, 0, errors.New("err")
		}
	}
	return processID, ttl, nil
}

// func TestLeoIP(t *testing.T) {
// 	// res, err := LeoIP("1.1.1.1")
// 	// assert.Nil(t, err)
// 	// assert.NotNil(t, res)
// 	// assert.NotEmpty(t, res.Asnumber)
// 	// assert.NotEmpty(t, res.Isp)
// }

// func TestIPSB(t *testing.T) {
// 	// Not available
// 	//res, err := IPSB("1.1.1.1")
// 	//assert.Nil(t, err)
// 	//assert.NotNil(t, res)
// 	//assert.NotEmpty(t, res.Asnumber)
// 	//assert.NotEmpty(t, res.Isp)
// }

// func TestIPInfo(t *testing.T) {
// 	res, err := IPInfo("1.1.1.1")
// 	assert.Nil(t, err)
// 	assert.NotNil(t, res)
// 	// assert.NotEmpty(t, res.Country)
// 	assert.NotEmpty(t, res.City)
// 	assert.NotEmpty(t, res.Prov)
// }

// func TestIPInSight(t *testing.T) {
// 	// res, err := IPInSight("1.1.1.1")
// 	// assert.Nil(t, err)
// 	// assert.NotNil(t, res)
// 	// assert.NotEmpty(t, res.Country)
// 	// assert.NotEmpty(t, res.Prov)
// 	// 这个库有时候不提供城市信息，返回值为""
// 	//assert.NotEmpty(t, res.City)
// }

// func TestIPApiCom(t *testing.T) {
// 	res, err := IPApiCom("1.1.1.1")
// 	assert.Nil(t, err)
// 	assert.NotNil(t, res)
// 	assert.NotEmpty(t, res.Country)
// 	assert.NotEmpty(t, res.City)
// 	assert.NotEmpty(t, res.Prov)
// }
