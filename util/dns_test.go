package util

import (
	"context"
	"fmt"
	"testing"
)

func TestDNS(t *testing.T) {
	resolver := DNSSB()
	ips, _ := resolver.LookupHost(context.Background(), "www.bing.com")
	fmt.Println(ips)
}

func TestDomainLookUp(t *testing.T) {
	ips := DomainLookUp("pek-4134.nexttrace-io-fasttrace-endpoint.win.", "all", "", false)
	fmt.Println(ips)
	ips = DomainLookUp("pek-4134.nexttrace-io-fasttrace-endpoint.win.", "4", "", false)
	fmt.Println(ips)
}
