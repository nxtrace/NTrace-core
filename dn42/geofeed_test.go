package dn42

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindGeoFeedRowInvalidIP(t *testing.T) {
	row, found := FindGeoFeedRow("not-an-ip", nil)
	assert.False(t, found)
	assert.Equal(t, GeoFeedRow{}, row)
}
