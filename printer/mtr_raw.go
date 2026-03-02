package printer

import (
	"fmt"
	"strings"

	"github.com/nxtrace/NTrace-core/trace"
)

// FormatMTRRawLine formats one MTR raw stream record with fixed 12 columns:
// ttl|ip|ptr|rtt|asn|country|prov|city|district|owner|lat|lng
func FormatMTRRawLine(rec trace.MTRRawRecord) string {
	if !rec.Success && rec.IP == "" && rec.Host == "" {
		// timeout row: keep a stable 12-column layout for machine parsers
		return fmt.Sprintf("%d|*||||||||||", rec.TTL)
	}

	rtt := ""
	if rec.RTTMs > 0 {
		rtt = fmt.Sprintf("%.2f", rec.RTTMs)
	}

	lat := ""
	lng := ""
	if rec.Lat != 0 || rec.Lng != 0 {
		lat = fmt.Sprintf("%.4f", rec.Lat)
		lng = fmt.Sprintf("%.4f", rec.Lng)
	}

	cols := []string{
		fmt.Sprintf("%d", rec.TTL),
		sanitizeRawField(rec.IP),
		sanitizeRawField(rec.Host),
		rtt,
		sanitizeRawField(rec.ASN),
		sanitizeRawField(rec.Country),
		sanitizeRawField(rec.Prov),
		sanitizeRawField(rec.City),
		sanitizeRawField(rec.District),
		sanitizeRawField(rec.Owner),
		lat,
		lng,
	}
	return strings.Join(cols, "|")
}

func sanitizeRawField(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	// Preserve one-record-per-line and stable split by '|'.
	s = strings.ReplaceAll(s, "|", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}
