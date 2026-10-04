package core

import "testing"

func TestRegionalRoutes(t *testing.T) {
	for _, tc := range []struct{ platform, match, account string }{
		{"euw1", "europe", "europe"},
		{"na1", "americas", "americas"},
		{"kr", "asia", "asia"},
		{"oc1", "sea", "asia"},
	} {
		match, err := RegionalRoute(tc.platform)
		if err != nil || match != tc.match {
			t.Fatalf("match route %s: %s %v", tc.platform, match, err)
		}
		account, err := AccountRoute(tc.platform)
		if err != nil || account != tc.account {
			t.Fatalf("account route %s: %s %v", tc.platform, account, err)
		}
	}
	if _, err := RegionalRoute("unknown"); err == nil {
		t.Fatal("unknown platform accepted")
	}
}

func TestEveryPlatformIsRouted(t *testing.T) {
	for _, platform := range Platforms {
		if _, err := RegionalRoute(platform); err != nil {
			t.Error(err)
		}
	}
	if len(Platforms) != len(matchRoutes) {
		t.Fatalf("Platforms has %d entries, matchRoutes %d", len(Platforms), len(matchRoutes))
	}
}
