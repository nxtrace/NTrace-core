package dn42

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReadGeoFeedAndFindRow(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	geofeedPath := filepath.Join(dir, "geofeed.csv")
	content := "192.0.2.0/24,cn,CN,Beijing,AS65000,ExampleWhois\n" +
		"2001:db8::/32,us,US,New York,AS65001,ExampleWhoisV6\n" +
		"invalid,us,US,Nowhere\n"
	require.NoError(t, os.WriteFile(geofeedPath, []byte(content), 0o644))

	viper.Set("geoFeedPath", geofeedPath)
	t.Cleanup(viper.Reset)

	rows, err := ReadGeoFeed()
	require.NoError(t, err)
	require.Len(t, rows, 2)

	t.Run("ipv4 hit", func(t *testing.T) {
		row, found := FindGeoFeedRow("192.0.2.1", rows)
		require.True(t, found)
		assert.Equal(t, "192.0.2.0/24", row.CIDR)
		assert.Equal(t, "cn", row.LtdCode)
		assert.Equal(t, "Beijing", row.City)
		assert.Equal(t, "AS65000", row.ASN)
		assert.Equal(t, "ExampleWhois", row.IPWhois)
	})

	t.Run("ipv6 hit", func(t *testing.T) {
		row, found := FindGeoFeedRow("2001:db8::1", rows)
		require.True(t, found)
		assert.Equal(t, "2001:db8::/32", row.CIDR)
		assert.Equal(t, "us", row.LtdCode)
		assert.Equal(t, "New York", row.City)
	})

	t.Run("miss", func(t *testing.T) {
		_, found := FindGeoFeedRow("203.0.113.5", rows)
		assert.False(t, found)
	})
}

func TestFindGeoFeedRowInvalidIP(t *testing.T) {
	row, found := FindGeoFeedRow("not-an-ip", nil)
	assert.False(t, found)
	assert.Equal(t, GeoFeedRow{}, row)
}
