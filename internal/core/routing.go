package core

import (
	"fmt"
	"strings"
)

// Match-V5 uses a continental route, while League-V4 uses the platform itself.
// Account-V1 is global and can be queried from any supported regional cluster.
var matchRoutes = map[string]string{
	"euw1": "europe", "eun1": "europe", "tr1": "europe", "ru": "europe",
	"na1": "americas", "br1": "americas", "la1": "americas", "la2": "americas",
	"kr": "asia", "jp1": "asia",
	"oc1": "sea", "sg2": "sea", "tw2": "sea", "vn2": "sea", "ph2": "sea", "th2": "sea",
}

func RegionalRoute(platform string) (string, error) {
	route, ok := matchRoutes[strings.ToLower(platform)]
	if !ok {
		return "", fmt.Errorf("unsupported Riot platform %q", platform)
	}
	return route, nil
}

func AccountRoute(platform string) (string, error) {
	route, err := RegionalRoute(platform)
	if err != nil {
		return "", err
	}
	if route == "sea" {
		return "asia", nil
	}
	return route, nil
}

func PlatformLabel(platform string) string {
	switch strings.ToLower(platform) {
	case "euw1":
		return "EUW"
	case "eun1":
		return "EUNE"
	case "na1":
		return "NA"
	case "br1":
		return "BR"
	case "la1":
		return "LAN"
	case "la2":
		return "LAS"
	case "kr":
		return "KR"
	case "jp1":
		return "JP"
	case "oc1":
		return "OCE"
	case "tr1":
		return "TR"
	case "ru":
		return "RU"
	default:
		return strings.ToUpper(platform)
	}
}
