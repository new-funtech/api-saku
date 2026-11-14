package main

import "testing"

func TestProxySettings(t *testing.T) {
	t.Setenv("PROXY_HEADER", "")
	t.Setenv("TRUSTED_PROXIES", "")
	if p := loadProxySettings(); p.Header != "" || p.EnableCheck {
		t.Errorf("default must use the socket address, got %+v", p)
	}
	t.Setenv("PROXY_HEADER", "X-Forwarded-For")
	t.Setenv("TRUSTED_PROXIES", " 10.0.0.0/8 , 172.16.0.1,, ")
	p := loadProxySettings()
	if p.Header != "X-Forwarded-For" || !p.EnableCheck || len(p.Trusted) != 2 || p.Trusted[0] != "10.0.0.0/8" {
		t.Errorf("settings = %+v", p)
	}
	t.Setenv("TRUSTED_PROXIES", "")
	if p := loadProxySettings(); p.EnableCheck {
		t.Error("a header without trusted proxies must not be honoured (a client could fake its address)")
	}
}
