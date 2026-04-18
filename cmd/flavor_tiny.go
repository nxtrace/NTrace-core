//go:build flavor_tiny

package cmd

const (
	appBinName       = "nexttrace-tiny"
	enableWebUI      = false
	enableGlobalping = false
	enableMTR        = false
	enableMTU        = true
	enableSpeed      = false
	defaultMTR       = false
)
