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

const DefaultPlayersSource = "https://api.github.com/repos/MrWaip/matchup-lookup/contents/players.json"

var seedHTTPClient = &http.Client{Timeout: 20 * time.Second}

func LoadSeeds(source string) ([]Seed, error) {
	data, ext, err := readSeedSource(source)
	if err != nil {
		return nil, err
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
			seeds = append(seeds, Seed{GameName: get(row, "gamename"), TagLine: get(row, "tagline"), Region: get(row, "region"), Source: get(row, "source")})
		}
	} else if err := json.Unmarshal(data, &seeds); err != nil {
		return nil, err
	}
	for i := range seeds {
		seeds[i].GameName = strings.TrimSpace(seeds[i].GameName)
		seeds[i].TagLine = strings.TrimSpace(seeds[i].TagLine)
		seeds[i].Region = strings.ToLower(strings.TrimSpace(seeds[i].Region))
		if seeds[i].GameName == "" || seeds[i].TagLine == "" {
			return nil, fmt.Errorf("seed %d needs gameName and tagLine", i+1)
		}
		if seeds[i].Region != "euw1" {
			return nil, fmt.Errorf("seed %d: MVP supports only euw1", i+1)
		}
	}
	return seeds, nil
}

func readSeedSource(source string) ([]byte, string, error) {
	u, err := url.Parse(source)
	if err != nil {
		return nil, "", err
	}
	if u.Scheme == "" {
		data, err := os.ReadFile(source)
		return data, filepath.Ext(source), err
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
