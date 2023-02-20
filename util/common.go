package util

import "net"

func Dnspod() *net.Resolver {
	return newDoTResolver("dot.pub", "dot.pub:853")
}

func Aliyun() *net.Resolver {
	return newDoTResolver("dns.alidns.com", "dns.alidns.com:853")
}

func DNSSB() *net.Resolver {
	return newDoTResolver("45.11.45.11", "dot.sb:853")
}

func Cloudflare() *net.Resolver {
	return newDoTResolver("one.one.one.one", "one.one.one.one:853")
}

func Google() *net.Resolver {
	return newDoTResolver("dns.google", "dns.google:853")
}
