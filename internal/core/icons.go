package core

import (
	"context"
	"net/url"
	"strconv"
	"strings"
)

const ddragon = "https://ddragon.leagueoflegends.com/cdn/"

// Icons resolves Data Dragon image URLs for champions, summoner spells and
// runes of the live patch. A missing entry, or a nil *Icons when Data Dragon
// was unreachable, yields "", so callers can fall back to text.
type Icons struct {
	version   string
	champions map[string]string // lower-case champion ID or name -> ID
	spells    map[int64]string  // spell key -> image file
	runes     map[int64]string  // rune ID -> image path
}

// LoadIcons fetches the spell and rune tables of the live patch; champions
// come from the cached catalog. It needs the network on each call.
func LoadIcons(ctx context.Context, s *Store) (*Icons, error) {
	if err := RefreshLivePatch(ctx, s); err != nil {
		return nil, err
	}
	version, _, err := s.setting("live_patch")
	if err != nil {
		return nil, err
	}
	champions, err := EnsureChampions(ctx, s)
	if err != nil {
		return nil, err
	}
	icons := &Icons{version: version, champions: map[string]string{}, spells: map[int64]string{}, runes: map[int64]string{}}
	for _, c := range champions {
		icons.champions[strings.ToLower(c.ID)] = c.ID
		icons.champions[strings.ToLower(c.Name)] = c.ID
	}

	var spells struct {
		Data map[string]struct {
			Key   string `json:"key"`
			Image struct {
				Full string `json:"full"`
			} `json:"image"`
		} `json:"data"`
	}
	base := ddragon + url.PathEscape(version)
	if err := catalogGET(ctx, base+"/data/en_US/summoner.json", &spells); err != nil {
		return nil, err
	}
	for _, spell := range spells.Data {
		if key, err := strconv.ParseInt(spell.Key, 10, 64); err == nil {
			icons.spells[key] = spell.Image.Full
		}
	}

	type rune struct {
		ID   int64  `json:"id"`
		Icon string `json:"icon"`
	}
	var trees []struct {
		rune
		Slots []struct {
			Runes []rune `json:"runes"`
		} `json:"slots"`
	}
	if err := catalogGET(ctx, base+"/data/en_US/runesReforged.json", &trees); err != nil {
		return nil, err
	}
	for _, tree := range trees {
		icons.runes[tree.ID] = tree.Icon
		for _, slot := range tree.Slots {
			for _, r := range slot.Runes {
				icons.runes[r.ID] = r.Icon
			}
		}
	}
	return icons, nil
}

// Champion accepts a catalog ID, a display name or a Match-V5 championName,
// whose case can differ from the image name (FiddleSticks vs Fiddlesticks).
func (i *Icons) Champion(name string) string {
	if i == nil {
		return ""
	}
	id, ok := i.champions[strings.ToLower(name)]
	if !ok {
		return ""
	}
	return ddragon + url.PathEscape(i.version) + "/img/champion/" + url.PathEscape(id) + ".png"
}

func (i *Icons) Spell(key int64) string {
	if i == nil {
		return ""
	}
	file, ok := i.spells[key]
	if !ok {
		return ""
	}
	return ddragon + url.PathEscape(i.version) + "/img/spell/" + url.PathEscape(file)
}

func (i *Icons) Rune(id int64) string {
	if i == nil {
		return ""
	}
	path, ok := i.runes[id]
	if !ok {
		return ""
	}
	return ddragon + "img/" + path
}
