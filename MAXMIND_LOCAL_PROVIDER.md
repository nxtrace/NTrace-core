# MaxMind Local Provider

## Overview

This contribution adds a new offline GeoIP data provider (`maxmind-local`) to NextTrace.
It queries the free **MaxMind GeoLite2** databases directly from disk, requiring no
external API calls or internet connectivity during traceroute execution.

## Motivation

Existing providers depend on external HTTP/WebSocket APIs. This provider enables fully
offline usage, which is useful in air-gapped environments or when minimizing external
dependencies is a priority.

## Implementation

### New file: `ipgeo/maxmindLocal.go`

- Uses `github.com/oschwald/geoip2-golang v1.13.0` (typed wrapper over the `.mmdb` format)
- Opens two GeoLite2 databases on each call:
  - `GeoLite2-ASN.mmdb` — for ASN number and organization name
  - `GeoLite2-City.mmdb` — for country, city, subdivision, and coordinates
- Respects the `lang` parameter: prefers `zh-CN` names when `--language cn` is set,
  falls back to English otherwise
- Returns a fully populated `*IPGeoData` struct

### Modified: `ipgeo/ipgeo.go`

- Registered `MaxMindLocal` in `GetSource()` under the key `"MAXMIND-LOCAL"`

### Modified: `cmd/cmd.go`

- Added `"maxmind-local"` to the `--data-provider` / `-d` selector

### Dependencies: `go.mod` / `go.sum`

- Added `github.com/oschwald/geoip2-golang v1.13.0`

## Fields Populated

| Field      | Source                                  |
|------------|-----------------------------------------|
| `Asnumber` | `"AS"` + `AutonomousSystemNumber`       |
| `Owner`    | `AutonomousSystemOrganization`          |
| `Country`  | ISO country code (e.g. `BR`, `US`)      |
| `City`     | City name (`zh-CN` if lang is `cn`)     |
| `Prov`     | First subdivision name (state/province) |
| `Lat`      | Geographic latitude (`float64`)         |
| `Lng`      | Geographic longitude (`float64`)        |
| `District` | Always empty (not available in GeoLite2)|

## Database Files

The free GeoLite2 databases must be downloaded from MaxMind (free account required):
https://www.maxmind.com/en/geolite2/signup

Expected paths on Linux:

```
/usr/local/share/nexttrace/GeoLite2-ASN.mmdb
/usr/local/share/nexttrace/GeoLite2-City.mmdb
```

## Usage

```bash
nexttrace --data-provider maxmind-local 8.8.8.8
```
