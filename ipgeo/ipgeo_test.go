package ipgeo

import (
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetSourceMappings(t *testing.T) {
	t.Helper()
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

func TestDisableGeoIP(t *testing.T) {
	res, err := disableGeoIP("1.1.1.1", time.Second, "en", false)
	require.NoError(t, err)
	assert.Equal(t, &IPGeoData{}, res)
}
