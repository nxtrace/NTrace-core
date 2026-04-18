package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/nxtrace/NTrace-core/internal/speedtest"
	"github.com/nxtrace/NTrace-core/internal/speedtest/result"
)

type ipInfo struct {
	Status     string `json:"status"`
	Query      string `json:"query"`
	AS         string `json:"as"`
	ISP        string `json:"isp"`
	Org        string `json:"org"`
	City       string `json:"city"`
	RegionName string `json:"regionName"`
	Country    string `json:"country"`
}

var (
	fetchIPDescFn   = fetchIPDescription
	fetchPeerInfoFn = fetchPeerInfo
)

func fetchIPDescription(ctx context.Context, ip, lang string) string {
	info, err := lookupIPInfo(ctx, ip)
	if err != nil {
		return speedtest.Text(lang, "lookup failed", "查询失败")
	}
	loc := formatLocation(info.City, info.RegionName, info.Country)
	if loc == "" {
		loc = speedtest.Text(lang, "unknown location", "未知位置")
	}
	asn := info.AS
	if asn == "" {
		asn = info.Org
	}
	if asn != "" {
		loc += " (" + asn + ")"
	}
	return loc
}

func fetchPeerInfo(ctx context.Context, target, lang string) result.PeerInfo {
	info, err := lookupIPInfo(ctx, target)
	if err != nil {
		return result.PeerInfo{Status: "unavailable"}
	}
	isp := info.ISP
	if isp == "" {
		isp = info.Org
	}
	peer := result.PeerInfo{
		Status:   "ok",
		IP:       info.Query,
		ISP:      isp,
		ASN:      info.AS,
		Location: formatLocation(info.City, info.RegionName, info.Country),
	}
	if peer.Location == "" {
		peer.Location = speedtest.Text(lang, "unknown", "未知")
	}
	return peer
}

func lookupIPInfo(ctx context.Context, target string) (ipInfo, error) {
	ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	fields := "status,query,as,isp,org,city,regionName,country"
	url := fmt.Sprintf("http://ip-api.com/json/%s?fields=%s", target, fields)
	if target == "" {
		url = fmt.Sprintf("http://ip-api.com/json/?fields=%s", fields)
	}
	req, err := http.NewRequestWithContext(ctx2, http.MethodGet, url, nil)
	if err != nil {
		return ipInfo{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ipInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return ipInfo{}, fmt.Errorf("metadata lookup returned HTTP %d", resp.StatusCode)
	}
	var info ipInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return ipInfo{}, err
	}
	if info.Status != "" && info.Status != "success" {
		return ipInfo{}, fmt.Errorf("metadata lookup status %q", info.Status)
	}
	return info, nil
}

func formatLocation(city, region, country string) string {
	out := ""
	if city != "" {
		out = city
	}
	if region != "" && region != city {
		if out != "" {
			out += ", "
		}
		out += region
	}
	if country != "" {
		if out != "" {
			out += ", "
		}
		out += country
	}
	return out
}
