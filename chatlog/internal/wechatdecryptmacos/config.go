package wechatdecryptmacos

import (
	"encoding/json"
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var ErrDBDirNotFound = errors.New("db_dir not found")

type Config struct {
	DBDir         string `json:"db_dir"`
	KeysFile      string `json:"keys_file"`
	DecryptedDir  string `json:"decrypted_dir"`
	WeChatProcess string `json:"wechat_process"`
}

func DefaultConfig(baseDir string) Config {
	return Config{
		DBDir:         filepath.Join(userHomeDir(), "Documents", "xwechat_files", "your_wxid", "db_storage"),
		KeysFile:      filepath.Join(baseDir, "all_keys.json"),
		DecryptedDir:  filepath.Join(baseDir, "decrypted"),
		WeChatProcess: "WeChat",
	}
}

func DefaultDBBaseDir() string {
	return filepath.Join(userHomeDir(), "Library", "Containers", "com.tencent.xinWeChat", "Data", "Documents", "xwechat_files")
}

func LoadConfig(baseDir string) (Config, error) {
	cfg := DefaultConfig(baseDir)
	configPath := filepath.Join(baseDir, "config.json")
	if data, err := os.ReadFile(configPath); err == nil {
		var loaded Config
		if err := json.Unmarshal(data, &loaded); err != nil {
			return cfg, err
		}
		if loaded.DBDir != "" {
			cfg.DBDir = loaded.DBDir
		}
		if loaded.KeysFile != "" {
			cfg.KeysFile = resolvePath(baseDir, loaded.KeysFile)
		}
		if loaded.DecryptedDir != "" {
			cfg.DecryptedDir = resolvePath(baseDir, loaded.DecryptedDir)
		}
		if loaded.WeChatProcess != "" {
			cfg.WeChatProcess = loaded.WeChatProcess
		}
	}

	if cfg.DBDir == "" || strings.Contains(cfg.DBDir, "your_wxid") {
		detected := autoDetectDBDir()
		if detected == "" {
			return cfg, ErrDBDirNotFound
		}
		cfg.DBDir = detected
	}

	return cfg, nil
}

func resolvePath(baseDir, path string) string {
	if path == "" {
		return path
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(baseDir, path)
}

func userHomeDir() string {
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		if u, err := user.Lookup(sudoUser); err == nil && u.HomeDir != "" {
			return u.HomeDir
		}
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return home
	}
	return "/var/root"
}

func autoDetectDBDir() string {
	base := DefaultDBBaseDir()
	entries, err := os.ReadDir(base)
	if err != nil {
		return ""
	}
	type candidate struct {
		path  string
		score time.Time
	}
	var candidates []candidate
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		dbStorage := filepath.Join(base, entry.Name(), "db_storage")
		info, err := os.Stat(dbStorage)
		if err != nil || !info.IsDir() {
			continue
		}
		score := dirMTime(filepath.Join(dbStorage, "message"))
		if score.IsZero() {
			score = dirMTime(dbStorage)
		}
		candidates = append(candidates, candidate{path: dbStorage, score: score})
	}
	if len(candidates) == 0 {
		return ""
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score.After(candidates[j].score)
	})
	return candidates[0].path
}

func dirMTime(path string) time.Time {
	if info, err := os.Stat(path); err == nil {
		return info.ModTime()
	}
	return time.Time{}
}
