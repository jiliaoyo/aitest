package config

import "testing"

func TestParseTrustedProxyCIDRs(t *testing.T) {
	prefixes, err := parseTrustedProxyCIDRs("192.0.2.10/24, 2001:db8::/32")
	if err != nil || len(prefixes) != 2 || prefixes[0].String() != "192.0.2.0/24" {
		t.Fatalf("unexpected trusted proxy prefixes: %v, %v", prefixes, err)
	}
	if _, err := parseTrustedProxyCIDRs("not-a-cidr"); err == nil {
		t.Fatal("invalid trusted proxy CIDR should be rejected")
	}
}
