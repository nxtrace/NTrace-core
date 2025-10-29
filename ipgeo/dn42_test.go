package ipgeo

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDN42GeoFeedAndPtrIntegration(t *testing.T) {
	dir := t.TempDir()
	geofeedPath := filepath.Join(dir, "geofeed.csv")
	ptrPath := filepath.Join(dir, "ptr.csv")

	geofeedContent := "192.0.2.0/24,hk,HK,Hong Kong,AS65000,Example Owner\n"
	ptrContent := "HKG,hk,Hong Kong,Hong Kong\n"

	require.NoError(t, os.WriteFile(geofeedPath, []byte(geofeedContent), 0o644))
	require.NoError(t, os.WriteFile(ptrPath, []byte(ptrContent), 0o644))

	viper.Set("geoFeedPath", geofeedPath)
	viper.Set("ptrPath", ptrPath)
	t.Cleanup(viper.Reset)

	res, err := DN42("192.0.2.8,core.hongkong-1.example", time.Second, "", false)
	require.NoError(t, err)

	assert.Equal(t, "China", res.Country)
	assert.Equal(t, "Hong Kong", res.Prov)
	assert.Equal(t, "Hong Kong", res.City)
	assert.Equal(t, "AS65000", res.Asnumber)
	assert.Equal(t, "Example Owner", res.Owner)
}

func TestDN42PtrFallback(t *testing.T) {
	dir := t.TempDir()
	geofeedPath := filepath.Join(dir, "geofeed.csv")
	ptrPath := filepath.Join(dir, "ptr.csv")

	// geofeed does not cover the IP, forcing the PTR fallback path
	require.NoError(t, os.WriteFile(geofeedPath, []byte("198.18.0.0/15,us,US,Test,AS65010,Owner\n"), 0o644))
	require.NoError(t, os.WriteFile(ptrPath, []byte("AMS,nl,Noord-Holland,Amsterdam\n"), 0o644))

	viper.Set("geoFeedPath", geofeedPath)
	viper.Set("ptrPath", ptrPath)
	t.Cleanup(viper.Reset)

	res, err := DN42("198.51.100.25,edge.ams01.provider", time.Second, "", false)
	require.NoError(t, err)

	assert.Equal(t, "Netherlands", res.Country)
	assert.Equal(t, "Noord-Holland", res.Prov)
	assert.Equal(t, "Amsterdam", res.City)
}

func TestDN42UnknownDefaults(t *testing.T) {
	dir := t.TempDir()
	geofeedPath := filepath.Join(dir, "geofeed.csv")
	ptrPath := filepath.Join(dir, "ptr.csv")

	require.NoError(t, os.WriteFile(geofeedPath, []byte(""), 0o644))
	require.NoError(t, os.WriteFile(ptrPath, []byte(""), 0o644))

	viper.Set("geoFeedPath", geofeedPath)
	viper.Set("ptrPath", ptrPath)
	t.Cleanup(viper.Reset)

	res, err := DN42("10.0.0.1", time.Second, "", false)
	require.NoError(t, err)

	assert.Equal(t, "Unknown", res.Country)
	assert.Empty(t, res.Prov)
	assert.Empty(t, res.City)
}
