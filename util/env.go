package util

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

var (
	DisableMPLS     = GetEnvBool("NEXTTRACE_DISABLEMPLS", false)
	EnableHidDstIP  = GetEnvBool("NEXTTRACE_ENABLEHIDDENDSTIP", false)
	EnvDevMode      = GetEnvBool("NEXTTRACE_DEVMODE", false)
	EnvRandomPort   = GetEnvBool("NEXTTRACE_RANDOMPORT", false)
	Uninterrupted   = GetEnvBool("NEXTTRACE_UNINTERRUPTED", false)
	EnvProxyURL     = GetEnvDefault("NEXTTRACE_PROXY", "")
	EnvToken        = GetEnvDefault("NEXTTRACE_TOKEN", "")
	EnvDataProvider = GetEnvDefault("NEXTTRACE_DATAPROVIDER", "")
	EnvHostPort     = GetEnvDefault("NEXTTRACE_HOSTPORT", "api.nxtrace.org")
	EnvPowProvider  = GetEnvDefault("NEXTTRACE_POWPROVIDER", "api.nxtrace.org")
	EnvDeployAddr   = GetEnvDefault("NEXTTRACE_DEPLOY_ADDR", "")
	EnvDeployToken  = GetEnvDefault("NEXTTRACE_DEPLOY_TOKEN", "")
	EnvMaxAttempts  = GetEnvInt("NEXTTRACE_MAXATTEMPTS", 0)
	EnvICMPMode     = GetEnvInt("NEXTTRACE_ICMPMODE", 0)
	GlobalpingToken = GetEnvDefault("GLOBALPING_TOKEN", "")
)

const EnvAllowCrossOriginKey = "NEXTTRACE_ALLOW_CROSS_ORIGIN"
const EnvNextTraceAPIV4TokenKey = "NEXTTRACE_API_V4_TOKEN"

func GetEnvTrimmed(key string) (string, bool) {
	v, ok := os.LookupEnv(key)
	if !ok {
		return "", false
	}
	val := strings.TrimSpace(v)
	if os.Getenv("NEXTTRACE_DEBUG") != "" {
		fmt.Println("ENV", key, "detected as", val)
	}
	return val, true
}

func GetEnvBool(key string, def bool) bool {
	if val, ok := GetEnvTrimmed(key); ok {
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

func GetEnvDefault(key string, def string) string {
	if val, ok := GetEnvTrimmed(key); ok {
		return val
	}
	return def
}

func GetSecretEnvDefault(key string, def string) string {
	v, ok := os.LookupEnv(key)
	if !ok {
		return def
	}
	val := strings.TrimSpace(v)
	if os.Getenv("NEXTTRACE_DEBUG") != "" {
		fmt.Println("ENV", key, "detected")
	}
	return val
}

func GetEnvInt(key string, def int) int {
	if val, ok := GetEnvTrimmed(key); ok {
		num, err := strconv.Atoi(val)
		if err != nil {
			return def
		}
		return num
	}
	return def
}

func AllowCrossOriginBrowserAccess() bool {
	return GetEnvBool(EnvAllowCrossOriginKey, false)
}

func GetNextTraceAPIV4Token() string {
	if token := GetSecretEnvDefault(EnvNextTraceAPIV4TokenKey, ""); token != "" {
		return token
	}
	token, err := ReadNextTraceAPIV4SessionToken()
	if err != nil {
		if os.Getenv("NEXTTRACE_DEBUG") != "" {
			fmt.Println("ENV", EnvNextTraceAPIV4TokenKey, "session token file read failed")
		}
		return ""
	}
	if token != "" {
		_ = os.Setenv(EnvNextTraceAPIV4TokenKey, token)
		if os.Getenv("NEXTTRACE_DEBUG") != "" {
			fmt.Println("ENV", EnvNextTraceAPIV4TokenKey, "loaded from session token file")
		}
	}
	return token
}
