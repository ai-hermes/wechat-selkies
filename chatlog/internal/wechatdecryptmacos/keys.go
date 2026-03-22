package wechatdecryptmacos

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"strings"
)

type KeyInfo struct {
	EncKey string `json:"enc_key"`
}

func LoadKeys(path string) (map[string]KeyInfo, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	raw := map[string]KeyInfo{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	keys := make(map[string]KeyInfo, len(raw))
	for k, v := range raw {
		if strings.HasPrefix(k, "_") {
			continue
		}
		if v.EncKey == "" {
			continue
		}
		keys[k] = v
	}
	return keys, nil
}

func FindKey(keys map[string]KeyInfo, relPath string) (KeyInfo, bool) {
	if !isSafeRelPath(relPath) {
		return KeyInfo{}, false
	}
	for _, candidate := range keyPathVariants(relPath) {
		if key, ok := keys[candidate]; ok {
			return key, true
		}
	}
	return KeyInfo{}, false
}

func keyPathVariants(relPath string) []string {
	normalized := filepath.ToSlash(relPath)
	variants := []string{
		relPath,
		normalized,
		strings.ReplaceAll(normalized, "/", string(os.PathSeparator)),
		strings.ReplaceAll(normalized, "/", "\\"),
	}
	seen := map[string]struct{}{}
	out := make([]string, 0, len(variants))
	for _, v := range variants {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func isSafeRelPath(relPath string) bool {
	normalized := path.Clean(strings.ReplaceAll(relPath, "\\", "/"))
	parts := strings.Split(normalized, "/")
	for _, part := range parts {
		if part == ".." {
			return false
		}
	}
	return true
}
