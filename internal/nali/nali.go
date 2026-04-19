package nali

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/nxtrace/NTrace-core/ipgeo"
)

type Family int

const (
	FamilyAll Family = iota
	Family4
	Family6
)

const maxCacheEntries = 4096

type Config struct {
	Source  ipgeo.Source
	Timeout time.Duration
	Lang    string
	Family  Family
}

type Span struct {
	Start     int
	End       int
	InsertEnd int
	ScanEnd   int
	Text      string
	LookupIP  string
	Family    Family
}

type Annotator struct {
	cfg       Config
	cacheMu   sync.RWMutex
	cache     map[string]string
	cacheRing []string
	cacheNext int
}

func New(cfg Config) *Annotator {
	if cfg.Timeout <= 0 {
		cfg.Timeout = time.Second
	}
	return &Annotator{
		cfg:   cfg,
		cache: make(map[string]string),
	}
}

func Run(ctx context.Context, cfg Config, input io.Reader, output io.Writer, target string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if input == nil {
		input = strings.NewReader("")
	}
	if output == nil {
		output = io.Discard
	}
	annotator := New(cfg)
	if target != "" {
		_, err := fmt.Fprintln(output, annotator.AnnotateLine(ctx, target))
		return err
	}

	reader := bufio.NewReader(input)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		line, err := reader.ReadString('\n')
		if line != "" {
			if isExitLine(line) {
				return nil
			}
			if _, writeErr := io.WriteString(output, annotator.AnnotateLine(ctx, line)); writeErr != nil {
				return writeErr
			}
		}
		if err == nil {
			continue
		}
		if err == io.EOF {
			return nil
		}
		return err
	}
}

func isExitLine(line string) bool {
	switch strings.TrimSpace(line) {
	case "quit", "exit":
		return true
	default:
		return false
	}
}

func (a *Annotator) AnnotateLine(ctx context.Context, line string) string {
	spans := FindIPSpans(line)
	if len(spans) == 0 {
		return line
	}

	var out strings.Builder
	out.Grow(len(line) + len(spans)*24)
	cursor := 0
	for _, span := range spans {
		if !a.familyAllowed(span.Family) {
			continue
		}
		label := a.lookupLabel(ctx, span.LookupIP)
		if label == "" {
			continue
		}
		out.WriteString(line[cursor:span.InsertEnd])
		out.WriteString(" [")
		out.WriteString(label)
		out.WriteString("]")
		cursor = span.InsertEnd
	}
	if cursor == 0 {
		return line
	}
	out.WriteString(line[cursor:])
	return out.String()
}

func (a *Annotator) familyAllowed(f Family) bool {
	return a.cfg.Family == FamilyAll || a.cfg.Family == f
}

func (a *Annotator) lookupLabel(ctx context.Context, ip string) string {
	if ip == "" {
		return ""
	}
	if label, ok := a.cachedLabel(ip); ok {
		return label
	}

	label := ""
	if geo, ok := ipgeo.Filter(ip); ok {
		label = FormatGeo(geo, a.cfg.Lang)
	} else if a.cfg.Source != nil && ctx.Err() == nil {
		if geo, err := a.cfg.Source(ip, a.cfg.Timeout, a.cfg.Lang, false); err == nil {
			label = FormatGeo(geo, a.cfg.Lang)
		}
	}
	a.storeLabel(ip, label)
	return label
}

func (a *Annotator) cachedLabel(ip string) (string, bool) {
	a.cacheMu.RLock()
	defer a.cacheMu.RUnlock()
	label, ok := a.cache[ip]
	return label, ok
}

func (a *Annotator) storeLabel(ip, label string) {
	a.cacheMu.Lock()
	defer a.cacheMu.Unlock()

	if _, ok := a.cache[ip]; ok {
		a.cache[ip] = label
		return
	}
	if len(a.cacheRing) < maxCacheEntries {
		a.cacheRing = append(a.cacheRing, ip)
	} else {
		oldest := a.cacheRing[a.cacheNext]
		delete(a.cache, oldest)
		a.cacheRing[a.cacheNext] = ip
		a.cacheNext = (a.cacheNext + 1) % maxCacheEntries
	}
	a.cache[ip] = label
}

func FindIPSpans(line string) []Span {
	spans := make([]Span, 0, 2)
	for i := 0; i < len(line); {
		if !isIPStart(line[i]) || !isIPLeftBoundary(line, i) {
			i++
			continue
		}
		end := scanCandidateEnd(line, i)
		if end <= i {
			i++
			continue
		}
		span, ok := parseCandidate(line[i:end], i, end)
		if !ok {
			i = end
			continue
		}
		span.InsertEnd = insertionEnd(line, span)
		spans = append(spans, span)
		if span.InsertEnd > end {
			i = span.InsertEnd
		} else {
			i = end
		}
	}
	return spans
}

func isIPStart(b byte) bool {
	return isHexByte(b) || b == ':'
}

func isIPLeftBoundary(s string, start int) bool {
	if start <= 0 {
		return true
	}
	prev := s[start-1]
	return !isWordByte(prev)
}

func scanCandidateEnd(s string, start int) int {
	for i := start; i < len(s); i++ {
		switch {
		case isHexByte(s[i]) || s[i] == '.' || s[i] == ':':
			continue
		case s[i] == '%':
			i++
			for i < len(s) && isZoneByte(s[i]) {
				i++
			}
			return i
		default:
			return i
		}
	}
	return len(s)
}

func isHexByte(b byte) bool {
	return (b >= '0' && b <= '9') ||
		(b >= 'a' && b <= 'f') ||
		(b >= 'A' && b <= 'F')
}

func isWordByte(b byte) bool {
	return (b >= '0' && b <= '9') ||
		(b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		b == '_' || b == '-'
}

func isZoneByte(b byte) bool {
	return (b >= '0' && b <= '9') ||
		(b >= 'a' && b <= 'z') ||
		(b >= 'A' && b <= 'Z') ||
		b == '_' || b == '-'
}

func parseCandidate(token string, base, scanEnd int) (Span, bool) {
	if token == "" {
		return Span{}, false
	}
	if trimmed := strings.TrimRight(token, "."); trimmed != token {
		return parseCandidate(trimmed, base, scanEnd)
	}
	if !strings.ContainsAny(token, ".:") {
		return Span{}, false
	}
	if strings.HasPrefix(token, ":") && !strings.HasPrefix(token, "::") {
		return parseCandidate(token[1:], base+1, scanEnd)
	}
	if host, ok := splitIPv4Port(token); ok {
		return parseAddr(host, base, base+len(host), scanEnd)
	}
	return parseAddr(token, base, base+len(token), scanEnd)
}

func splitIPv4Port(token string) (string, bool) {
	idx := strings.LastIndexByte(token, ':')
	if idx <= 0 || idx == len(token)-1 {
		return "", false
	}
	port := token[idx+1:]
	if _, err := strconv.Atoi(port); err != nil {
		return "", false
	}
	host := token[:idx]
	addr, err := netip.ParseAddr(host)
	if err != nil || !addr.Is4() {
		return "", false
	}
	return host, true
}

func parseAddr(raw string, start, end, scanEnd int) (Span, bool) {
	addr, err := netip.ParseAddr(raw)
	if err != nil || !addr.IsValid() {
		return Span{}, false
	}
	family := Family6
	if addr.Is4() {
		family = Family4
	}
	lookup := raw
	if addr.Zone() != "" {
		lookup = addr.WithZone("").String()
	}
	return Span{
		Start:     start,
		End:       end,
		InsertEnd: end,
		ScanEnd:   scanEnd,
		Text:      raw,
		LookupIP:  lookup,
		Family:    family,
	}, true
}

func insertionEnd(line string, span Span) int {
	if span.Start > 0 && line[span.Start-1] == '[' && span.End < len(line) && line[span.End] == ']' {
		return span.End + 1
	}
	return span.End
}

func FormatGeo(data *ipgeo.IPGeoData, lang string) string {
	if data == nil {
		return ""
	}

	asn := strings.TrimSpace(data.Asnumber)
	whois := strings.TrimSpace(data.Whois)
	country := localized(data.Country, data.CountryEn, lang)
	prov := localized(data.Prov, data.ProvEn, lang)
	city := localized(data.City, data.CityEn, lang)
	district := strings.TrimSpace(data.District)
	owner := strings.TrimSpace(data.Owner)
	if owner == "" {
		owner = strings.TrimSpace(data.Isp)
	}

	if asn == "" && country == "" && prov == "" && city == "" && district == "" && owner == "" {
		return whois
	}

	fields := make([]string, 0, 6)
	if asn != "" {
		fields = append(fields, normalizeASN(asn))
	}
	appendUnique := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		for _, existing := range fields {
			if existing == value {
				return
			}
		}
		fields = append(fields, value)
	}
	appendUnique(country)
	appendUnique(prov)
	appendUnique(city)
	appendUnique(district)
	appendUnique(owner)
	if len(fields) == 0 {
		return whois
	}
	return strings.Join(fields, ", ")
}

func localized(cn, en, lang string) string {
	cn = strings.TrimSpace(cn)
	en = strings.TrimSpace(en)
	if strings.EqualFold(lang, "en") {
		if en != "" {
			return en
		}
		return cn
	}
	if cn != "" {
		return cn
	}
	return en
}

func normalizeASN(asn string) string {
	asn = strings.TrimSpace(asn)
	if len(asn) >= 2 && strings.EqualFold(asn[:2], "AS") {
		asn = strings.TrimSpace(asn[2:])
	}
	return "AS" + asn
}
