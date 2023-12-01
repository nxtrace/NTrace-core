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
	ips, _ := DomainLookUp("pek-4134.endpoint.nxtrace.org.", "all", "", false)
	fmt.Println(ips)
	ips, _ = DomainLookUp("pek-4134.endpoint.nxtrace.org.", "4", "", false)
	fmt.Println(ips)
}
