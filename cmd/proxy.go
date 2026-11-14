package main

import (
	"os"
	"strings"
)

// proxySettings tells Fiber where the real client address is when the API sits behind a reverse
// proxy or load balancer. Without it every request looks like it comes from the proxy, so all
// clients would share one per-IP budget.
//
//	PROXY_HEADER     e.g. X-Forwarded-For (empty: use the socket address, the default)
//	TRUSTED_PROXIES  comma-separated IPs/CIDRs allowed to set that header; a header from anyone
//	                 else is ignored, so a client cannot fake its address
type proxySettings struct {
	Header      string
	EnableCheck bool
	Trusted     []string
}

func loadProxySettings() proxySettings {
	p := proxySettings{Header: strings.TrimSpace(os.Getenv("PROXY_HEADER"))}
	for _, v := range strings.Split(os.Getenv("TRUSTED_PROXIES"), ",") {
		if v = strings.TrimSpace(v); v != "" {
			p.Trusted = append(p.Trusted, v)
		}
	}
	p.EnableCheck = p.Header != "" && len(p.Trusted) > 0
	return p
}
