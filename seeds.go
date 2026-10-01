package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const DefaultPlayersSource = "https://api.github.com/repos/MrWaip/matchup-lookup/contents/players/index.json"

var seedHTTPClient = &http.Client{Timeout: 20 * time.Second}

func LoadSeeds(source string) ([]Seed, error) {
	return loadSeeds(source, 0)
}

func loadSeeds(source string, depth int) ([]Seed, error) {
	if depth > 5 {
		return nil, fmt.Errorf("seed manifest nesting too deep")
	}
	if !strings.Contains(source, "://") {
		info, err := os.Stat(source)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			var all []Seed
			err := filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if entry.IsDir() || entry.Name() == "index.json" {
					return nil
				}
				ext := strings.ToLower(filepath.Ext(path))
				if ext != ".json" && ext != ".csv" {
					return nil
				}
				seeds, err := loadSeeds(path, depth+1)
				if err != nil {
					return err
				}
				all = append(all, seeds...)
				return nil
			})
			if err != nil {
				return nil, err
			}
			if len(all) == 0 {
				return nil, fmt.Errorf("no player JSON/CSV files in %s", source)
			}
			return all, nil
		}
	}
	data, ext, err := readSeedSource(source)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(ext, ".json") && len(bytes.TrimSpace(data)) > 0 && bytes.TrimSpace(data)[0] == '{' {
		var manifest struct {
			Files []string `json:"files"`
		}
		if err := json.Unmarshal(data, &manifest); err != nil {
			return nil, err
		}
		if len(manifest.Files) == 0 || len(manifest.Files) > 200 {
			return nil, fmt.Errorf("seed manifest needs 1-200 files")
		}
		var all []Seed
		for _, file := range manifest.Files {
			if file == "" || strings.Contains(file, "..") || strings.Contains(file, "://") || strings.HasPrefix(file, "/") {
				return nil, fmt.Errorf("invalid seed manifest path %q", file)
			}
			child := filepath.Join(filepath.Dir(source), file)
			if strings.Contains(source, "://") {
				base, err := url.Parse(source)
				if err != nil {
					return nil, err
				}
				rel, err := url.Parse(file)
				if err != nil {
					return nil, err
				}
				child = base.ResolveReference(rel).String()
			}
			seeds, err := loadSeeds(child, depth+1)
			if err != nil {
				return nil, err
			}
			all = append(all, seeds...)
		}
		return all, nil
	}
	var seeds []Seed
	if strings.EqualFold(ext, ".csv") {
		r := csv.NewReader(bytes.NewReader(data))
		r.FieldsPerRecord = -1
		rows, err := r.ReadAll()
		if err != nil {
			return nil, err
		}
		if len(rows) == 0 {
			return nil, fmt.Errorf("empty seed file")
		}
		head := make(map[string]int)
		for i, name := range rows[0] {
			head[strings.ToLower(strings.TrimSpace(name))] = i
		}
		get := func(row []string, key string) string {
			i, ok := head[key]
			if !ok || i >= len(row) {
				return ""
			}
			return strings.TrimSpace(row[i])
		}
		for _, row := range rows[1:] {
			seeds = append(seeds, Seed{GameName: get(row, "gamename"), TagLine: get(row, "tagline"), Region: get(row, "region"), Source: get(row, "source"), Champion: get(row, "champion")})
		}
	} else if err := json.Unmarshal(data, &seeds); err != nil {
		return nil, err
	}
	for i := range seeds {
		seeds[i].GameName = strings.TrimSpace(seeds[i].GameName)
		seeds[i].TagLine = strings.TrimSpace(seeds[i].TagLine)
		seeds[i].Region = strings.ToLower(strings.TrimSpace(seeds[i].Region))
		seeds[i].Champion = strings.TrimSpace(seeds[i].Champion)
		if seeds[i].GameName == "" || seeds[i].TagLine == "" {
			return nil, fmt.Errorf("seed %d needs gameName and tagLine", i+1)
		}
		if _, err := regionalRoute(seeds[i].Region); err != nil {
			return nil, fmt.Errorf("seed %d: %w", i+1, err)
		}
	}
	return seeds, nil
}

func readSeedSource(source string) ([]byte, string, error) {
	// A Windows path such as C:\players.json has a colon; it is still a file path.
	if !strings.Contains(source, "://") {
		data, err := os.ReadFile(source)
		return data, filepath.Ext(source), err
	}
	u, err := url.Parse(source)
	if err != nil {
		return nil, "", err
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return nil, "", fmt.Errorf("seed URL must be HTTP(S)")
	}
	req, err := http.NewRequest(http.MethodGet, source, nil)
	if err != nil {
		return nil, "", err
	}
	if strings.EqualFold(u.Hostname(), "api.github.com") {
		if token := githubToken(); token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		req.Header.Set("Accept", "application/vnd.github+json")
	}
	resp, err := seedHTTPClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, "", fmt.Errorf("seed URL returned HTTP %d (for private GitHub files, set GITHUB_TOKEN or run gh auth login)", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2<<20))
	if err != nil {
		return nil, "", err
	}
	if strings.EqualFold(u.Hostname(), "api.github.com") && strings.Contains(u.Path, "/contents/") {
		var file struct {
			Content  []byte `json:"content"`
			Encoding string `json:"encoding"`
			Name     string `json:"name"`
		}
		if err := json.Unmarshal(data, &file); err != nil {
			return nil, "", err
		}
		if file.Encoding != "base64" {
			return nil, "", fmt.Errorf("GitHub file has unsupported encoding %q", file.Encoding)
		}
		return file.Content, filepath.Ext(file.Name), nil
	}
	return data, filepath.Ext(u.Path), nil
}

func githubToken() string {
	if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" {
		return token
	}
	out, err := exec.Command("gh", "auth", "token").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
