//go:build !flavor_tiny && !flavor_ntr

package cmd

const (
	appBinName       = "nexttrace"
	enableWebUI      = true
	enableGlobalping = true
	enableMTR        = true
	enableMTU        = true
	enableSpeed      = true
	enableNali       = true
	defaultMTR       = false
)
