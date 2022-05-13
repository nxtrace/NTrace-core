package ipgeo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLeoIP(t *testing.T) {
	res, err := LeoIP("1.1.1.1")
	assert.Nil(t, err)
	assert.NotNil(t, res)
	assert.NotEmpty(t, res.Asnumber)
	assert.NotEmpty(t, res.Isp)
}

func TestIPSB(t *testing.T) {
	res, err := GetIPGeoByIPSB("1.1.1.1")
	assert.Nil(t, err)
	assert.NotNil(t, res)
	assert.NotEmpty(t, res.Asnumber)
	assert.NotEmpty(t, res.Isp)
}

func TestIPInfo(t *testing.T) {
	res, err := GetIPGeoByIPInfo("1.1.1.1")
	assert.Nil(t, err)
	assert.NotNil(t, res)
	assert.NotEmpty(t, res.Country)
	assert.NotEmpty(t, res.City)
	assert.NotEmpty(t, res.Prov)
}

func TestIPInSight(t *testing.T) {
	res, err := IPInSight("1.1.1.1")
	assert.Nil(t, err)
	assert.NotNil(t, res)
	assert.NotEmpty(t, res.Country)
	assert.NotEmpty(t, res.Prov)
	// 这个库有时候不提供城市信息，返回值为""
	//assert.NotEmpty(t, res.City)
}
