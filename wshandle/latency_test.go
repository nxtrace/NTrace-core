package wshandle

import (
	"testing"
)

func TestGetFastIP(t *testing.T) {
	GetFastIP("api.leo.moe", "443")
}
