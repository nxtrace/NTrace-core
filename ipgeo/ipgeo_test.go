package ipgeo

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestLeoIP(t *testing.T) {
	res, err := LeoIP("1.1.1.1")
	assert.Nil(t, err)
	assert.NotNil(t, res)
	assert.NotEmpty(t, res.Asnumber)
	assert.NotEmpty(t, res.Isp)
}

func TestIPInSight(t *testing.T) {
	res, err := IPInSight("1.1.1.1")
	assert.Nil(t, err)
	assert.NotNil(t, res)
	assert.NotEmpty(t, res.Country)
	assert.NotEmpty(t, res.City)
	assert.NotEmpty(t, res.Prov)
}
