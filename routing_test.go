package main

import "testing"

func TestRegionalRoutes(t *testing.T) {
	for _, tc := range []struct{ platform, match, account string }{
		{"euw1", "europe", "europe"},
		{"na1", "americas", "americas"},
		{"kr", "asia", "asia"},
		{"oc1", "sea", "asia"},
	} {
		match, err := regionalRoute(tc.platform)
		if err != nil || match != tc.match {
			t.Fatalf("match route %s: %s %v", tc.platform, match, err)
		}
		account, err := accountRoute(tc.platform)
		if err != nil || account != tc.account {
			t.Fatalf("account route %s: %s %v", tc.platform, account, err)
		}
	}
	if _, err := regionalRoute("unknown"); err == nil {
		t.Fatal("unknown platform accepted")
	}
}
