package core

// Server is a platform offered by the server filter.
type Server struct {
	Platform string // e.g. "euw1"
	Label    string // e.g. "EUW"
	Players  int    // tracked players on it
}

// Servers returns every supported platform with its tracked player count.
func (s *Store) Servers() ([]Server, error) {
	players, err := s.ListPlayers()
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, p := range players {
		counts[p.Region]++
	}
	servers := make([]Server, len(Platforms))
	for i, platform := range Platforms {
		servers[i] = Server{Platform: platform, Label: PlatformLabel(platform), Players: counts[platform]}
	}
	return servers, nil
}
