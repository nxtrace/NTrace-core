package util

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

var (
	DisableMPLS        = GetEnvBool("NEXTTRACE_DISABLEMPLS", false)
	EnableHidDstIP     = GetEnvBool("NEXTTRACE_ENABLEHIDDENDSTIP", false)
	EnvHostPort        = GetEnvDefault("NEXTTRACE_HOSTPORT", "api.nxtrace.org")
	EnvIPInfoLocalPath = GetEnvDefault("NEXTTRACE_IPINFOLOCALPATH", "")
	EnvMaxAttempts     = GetEnvInt("NEXTTRACE_MAXATTEMPTS", 0)
	EnvPowProvider     = GetEnvDefault("NEXTTRACE_POWPROVIDER", "api.nxtrace.org")
	EnvProxyURL        = GetEnvDefault("NEXTTRACE_PROXY", "")
	EnvRandomPort      = GetEnvBool("NEXTTRACE_RANDOMPORT", false)
	EnvToken           = GetEnvDefault("NEXTTRACE_TOKEN", "")
	Uninterrupted      = GetEnvBool("NEXTTRACE_UNINTERRUPTED", false)
)

func GetEnvDefault(key, defVal string) string {
	if v, ok := os.LookupEnv(key); ok {
		val := strings.TrimSpace(v)
		if os.Getenv("NEXTTRACE_DEBUG") != "" {
			fmt.Println("ENV", key, "detected as", val)
		}
		return val
	}
	return defVal
}

func GetEnvBool(key string, def bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		val := strings.TrimSpace(v)
		if os.Getenv("NEXTTRACE_DEBUG") != "" {
			fmt.Println("ENV", key, "detected as", val)
		}
		switch val {
		case "1":
			return true
		case "0":
			return false
		default:
			return def
		}
	}
	return def
}

func GetEnvInt(key string, def int) int {
	if v, ok := os.LookupEnv(key); ok {
		val := strings.TrimSpace(v)
		if os.Getenv("NEXTTRACE_DEBUG") != "" {
			fmt.Println("ENV", key, "detected as", val)
		}
		n, err := strconv.Atoi(val)
		if err != nil {
			return def
		}
		return n
	}
	return def
}
