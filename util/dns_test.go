package util

import (
	"context"
	"fmt"
	"testing"
)

func TestDNS(t *testing.T) {
	resolver := DNSSB()
	ips, _ := resolver.LookupHost(context.Background(), "www.google.com")
	fmt.Println(ips)
}
