//go:build !flavor_tiny && !flavor_ntr

package cmd

const (
	appBinName      = "nexttrace"
	enableWebUI     = true
	enableGlobalping = true
	enableMTR       = true
	defaultMTR      = false
)
