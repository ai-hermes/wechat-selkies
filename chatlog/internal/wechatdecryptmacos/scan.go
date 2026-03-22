package wechatdecryptmacos

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/sjzar/chatlog/internal/wechat/key/darwin/glance"
)

type KeyEntry struct {
	KeyHex  string
	SaltHex string
}

type DBInfo struct {
	RelPath string
	SaltHex string
}

func ScanAndSave(ctx context.Context, pid int, dbBase string, outPath string) error {
	fmt.Println("============================================================")
	fmt.Println("  macOS WeChat Memory Key Scanner (Go)")
	fmt.Println("============================================================")
	fmt.Printf("WeChat PID: %d\n", pid)

	dbInfos, saltMap, err := CollectDBSalts(dbBase)
	if err != nil {
		return err
	}
	fmt.Printf("Found %d encrypted DBs\n", len(dbInfos))

	keys, stats, err := ScanProcessMemory(ctx, pid)
	if err != nil {
		return err
	}

	fmt.Printf("\nScan complete: %dMB scanned, %d regions, %d unique keys\n",
		stats.TotalBytes/1024/1024, stats.RegionCount, len(keys))

	matched := map[string]KeyInfo{}
	matchCount := 0
	for _, key := range keys {
		if rel, ok := saltMap[key.SaltHex]; ok {
			matched[filepath.ToSlash(rel)] = KeyInfo{EncKey: key.KeyHex}
			matchCount++
		}
	}

	fmt.Println()
	printKeyTable(keys, saltMap)
	fmt.Printf("\nMatched %d/%d keys to known DBs\n", matchCount, len(keys))

	if err := saveKeys(outPath, matched); err != nil {
		return err
	}
	fmt.Printf("Saved to %s\n", outPath)
	return nil
}

func CollectDBSalts(dbBase string) ([]DBInfo, map[string]string, error) {
	if dbBase == "" {
		return nil, nil, errors.New("db base directory is empty")
	}
	var dbInfos []DBInfo
	saltMap := map[string]string{}

	entries, err := os.ReadDir(dbBase)
	if err != nil {
		return nil, nil, err
	}

	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		storagePath := filepath.Join(dbBase, entry.Name(), "db_storage")
		if info, err := os.Stat(storagePath); err != nil || !info.IsDir() {
			continue
		}
		filepath.WalkDir(storagePath, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(strings.ToLower(d.Name()), ".db") {
				return nil
			}
			saltHex, err := readDBSalt(path)
			if err != nil {
				return nil
			}
			rel := relPathFromDBStorage(path)
			dbInfos = append(dbInfos, DBInfo{RelPath: rel, SaltHex: saltHex})
			if _, ok := saltMap[saltHex]; !ok {
				saltMap[saltHex] = rel
			}
			fmt.Printf("  %s: salt=%s\n", rel, saltHex)
			return nil
		})
	}
	sort.Slice(dbInfos, func(i, j int) bool {
		return dbInfos[i].RelPath < dbInfos[j].RelPath
	})
	return dbInfos, saltMap, nil
}

type ScanStats struct {
	TotalBytes  uint64
	RegionCount int
}

func ScanProcessMemory(ctx context.Context, pid int) ([]KeyEntry, ScanStats, error) {
	fmt.Println("\nScanning memory for keys...")
	g := glance.NewGlance(uint32(pid))
	memoryChannel := make(chan []byte, 64)
	errCh := make(chan error, 1)

	go func() {
		errCh <- g.Read2Chan(ctx, memoryChannel)
		close(memoryChannel)
	}()

	entries := map[string]KeyEntry{}
	var total uint64
	for chunk := range memoryChannel {
		total += uint64(len(chunk))
		scanChunk(chunk, entries)
	}
	if err := <-errCh; err != nil {
		return nil, ScanStats{}, err
	}

	keys := make([]KeyEntry, 0, len(entries))
	for _, entry := range entries {
		keys = append(keys, entry)
	}
	return keys, ScanStats{TotalBytes: total, RegionCount: len(g.MemRegions)}, nil
}

func scanChunk(buf []byte, entries map[string]KeyEntry) {
	for i := 0; i+99 <= len(buf); i++ {
		if buf[i] != 'x' || buf[i+1] != '\'' {
			continue
		}
		if i+2+96 >= len(buf) {
			break
		}
		ok := true
		for j := 0; j < 96; j++ {
			if !isHexChar(buf[i+2+j]) {
				ok = false
				break
			}
		}
		if !ok || buf[i+2+96] != '\'' {
			continue
		}
		keyHex := strings.ToLower(string(buf[i+2 : i+2+64]))
		saltHex := strings.ToLower(string(buf[i+2+64 : i+2+96]))
		id := keyHex + ":" + saltHex
		if _, exists := entries[id]; exists {
			continue
		}
		if _, err := hex.DecodeString(keyHex + saltHex); err != nil {
			continue
		}
		entries[id] = KeyEntry{KeyHex: keyHex, SaltHex: saltHex}
	}
}

func isHexChar(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func readDBSalt(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	header := make([]byte, 16)
	if _, err := f.Read(header); err != nil {
		return "", err
	}
	if string(header[:15]) == "SQLite format 3" {
		return "", errors.New("unencrypted")
	}
	return hex.EncodeToString(header), nil
}

func relPathFromDBStorage(path string) string {
	slash := filepath.ToSlash(path)
	idx := strings.Index(slash, "db_storage/")
	if idx >= 0 {
		return slash[idx+len("db_storage/"):]
	}
	return filepath.Base(path)
}

func saveKeys(path string, keys map[string]KeyInfo) error {
	data, err := json.MarshalIndent(keys, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0600)
}

func printKeyTable(keys []KeyEntry, saltMap map[string]string) {
	fmt.Printf("\n%-25s %-66s %s\n", "Database", "Key", "Salt")
	fmt.Printf("%-25s %-66s %s\n",
		"-------------------------",
		"------------------------------------------------------------------",
		"--------------------------------")
	for _, key := range keys {
		dbName := "(unknown)"
		if rel, ok := saltMap[key.SaltHex]; ok {
			dbName = rel
		}
		fmt.Printf("%-25s %-66s %s\n", dbName, key.KeyHex, key.SaltHex)
	}
}
