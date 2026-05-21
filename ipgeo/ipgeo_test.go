package ipgeo

import (
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/nxtrace/NTrace-core/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSourceMappings(t *testing.T) {
	t.Helper()
	t.Setenv(util.EnvAPIV4TokenKey, "")
	tests := []struct {
		name  string
		input string
		want  Source
	}{
		{name: "dn42", input: "DN42", want: DN42},
		{name: "leo default", input: "LEOMOEAPI", want: LeoIP},
		{name: "ipsb", input: "ip.sb", want: IPSB},
		{name: "ipinsight", input: "ipinsight", want: IPInSight},
		{name: "ipapi alias", input: "ip-api.com", want: IPApiCom},
		{name: "ipapi uppercase", input: "IPAPI.COM", want: IPApiCom},
		{name: "ipinfo", input: "IPINFO", want: IPInfo},
		{name: "ipinfo local", input: "ipinfolocal", want: IPInfoLocal},
		{name: "chunzhen", input: "ChunZhen", want: Chunzhen},
		{name: "disable geoip", input: "disable-geoip", want: disableGeoIP},
		{name: "ipdb", input: "IPDB.One", want: IPDBOne},
		{name: "fallback", input: "unknown", want: LeoIP},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := GetSource(tc.input)
			require.NotNil(t, got)
			assert.Equal(t, reflect.ValueOf(tc.want).Pointer(), reflect.ValueOf(got).Pointer())
		})
	}
}

func TestGetSourceUsesAPIV4ForLeoMoeOnlyWhenTokenConfigured(t *testing.T) {
	t.Setenv(util.EnvAPIV4TokenKey, "v4-token")

	got := GetSource("LeoMoeAPI")
	require.NotNil(t, got)
	assert.Equal(t, reflect.ValueOf(LeoIPV4HTTP).Pointer(), reflect.ValueOf(got).Pointer())

	fallback := GetSource("unknown")
	require.NotNil(t, fallback)
	assert.Equal(t, reflect.ValueOf(LeoIPV4HTTP).Pointer(), reflect.ValueOf(fallback).Pointer())

	nonLeo := GetSource("IPInfo")
	require.NotNil(t, nonLeo)
	assert.Equal(t, reflect.ValueOf(IPInfo).Pointer(), reflect.ValueOf(nonLeo).Pointer())
}

func TestAPIV4TokenConfiguredReadsCurrentProcessEnv(t *testing.T) {
	t.Setenv(util.EnvAPIV4TokenKey, "")
	assert.False(t, APIV4TokenConfigured())

	require.NoError(t, os.Setenv(util.EnvAPIV4TokenKey, " runtime-token "))
	t.Cleanup(func() { _ = os.Unsetenv(util.EnvAPIV4TokenKey) })
	assert.True(t, APIV4TokenConfigured())
}

func TestDisableGeoIP(t *testing.T) {
	res, err := disableGeoIP("1.1.1.1", time.Second, "en", false)
	require.NoError(t, err)
	assert.Equal(t, &IPGeoData{}, res)
}
