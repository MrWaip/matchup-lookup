package store

import (
	"bytes"
	"compress/gzip"
	"io"
	"strconv"
	"strings"
)

// CompressRaw gzips a full Riot API response for the matches.raw_gz column.
// Match JSON shrinks to roughly a tenth of its size.
func CompressRaw(raw []byte) ([]byte, error) {
	var out bytes.Buffer
	w, err := gzip.NewWriterLevel(&out, gzip.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := w.Write(raw); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// DecompressRaw restores the Riot API response stored by CompressRaw.
func DecompressRaw(compressed []byte) ([]byte, error) {
	r, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return io.ReadAll(r)
}

// ParsePatch extracts the major and minor numbers of a game version such as
// "16.19.712.1234". Unparseable versions yield 0.0, which sorts as oldest.
func ParsePatch(version string) (major, minor int64) {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return 0, 0
	}
	major, errMajor := strconv.ParseInt(parts[0], 10, 64)
	minor, errMinor := strconv.ParseInt(parts[1], 10, 64)
	if errMajor != nil || errMinor != nil {
		return 0, 0
	}
	return major, minor
}
