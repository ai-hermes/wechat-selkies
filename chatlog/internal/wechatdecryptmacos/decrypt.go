package wechatdecryptmacos

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/crypto/pbkdf2"
)

const (
	pageSize   = 4096
	saltSize   = 16
	ivSize     = 16
	hmacSize   = 64
	reserveLen = 80
)

var sqliteHeader = []byte("SQLite format 3\x00")

func DecryptAll(ctx context.Context, dbDir, keysFile, outDir string) error {
	if dbDir == "" {
		return errors.New("db_dir is empty")
	}
	if keysFile == "" {
		return errors.New("keys file is empty")
	}
	keys, err := LoadKeys(keysFile)
	if err != nil {
		return err
	}
	dbFiles, err := collectDBFiles(dbDir)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	fmt.Println("============================================================")
	fmt.Println("  WeChat 4.0 数据库解密器 (Go)")
	fmt.Println("============================================================")
	fmt.Printf("\n加载 %d 个数据库密钥\n", len(keys))
	fmt.Printf("输出目录: %s\n", outDir)
	fmt.Printf("找到 %d 个数据库文件\n\n", len(dbFiles))

	var success, failed int
	var totalBytes int64
	for _, db := range dbFiles {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		keyInfo, ok := FindKey(keys, db.RelPath)
		if !ok {
			fmt.Printf("SKIP: %s (无密钥)\n", db.RelPath)
			failed++
			continue
		}
		encKey, err := hex.DecodeString(keyInfo.EncKey)
		if err != nil || len(encKey) != 32 {
			fmt.Printf("SKIP: %s (密钥格式错误)\n", db.RelPath)
			failed++
			continue
		}
		outPath := filepath.Join(outDir, db.RelPath)
		if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
			return err
		}
		fmt.Printf("解密: %s (%.1fMB) ... ", db.RelPath, float64(db.Size)/1024.0/1024.0)
		if err := decryptDatabase(db.Path, outPath, encKey); err != nil {
			fmt.Printf("[ERROR] %v\n", err)
			failed++
			continue
		}
		fmt.Println("OK")
		success++
		totalBytes += db.Size
	}

	fmt.Printf("\n============================================================\n")
	fmt.Printf("结果: %d 成功, %d 失败, 共 %d 个\n", success, failed, len(dbFiles))
	fmt.Printf("解密数据量: %.1fGB\n", float64(totalBytes)/1024.0/1024.0/1024.0)
	fmt.Printf("解密文件在: %s\n", outDir)
	return nil
}

type dbFile struct {
	RelPath string
	Path    string
	Size    int64
}

func collectDBFiles(dbDir string) ([]dbFile, error) {
	var files []dbFile
	err := filepath.WalkDir(dbDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		name := strings.ToLower(d.Name())
		if !strings.HasSuffix(name, ".db") || strings.HasSuffix(name, "-wal") || strings.HasSuffix(name, "-shm") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(dbDir, path)
		if err != nil {
			return nil
		}
		files = append(files, dbFile{RelPath: filepath.ToSlash(rel), Path: path, Size: info.Size()})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].Size < files[j].Size
	})
	return files, nil
}

func decryptDatabase(dbPath, outPath string, encKey []byte) error {
	fileInfo, err := os.Stat(dbPath)
	if err != nil {
		return err
	}
	totalPages := int(fileInfo.Size() / pageSize)
	if fileInfo.Size()%pageSize != 0 {
		totalPages++
	}
	if totalPages == 0 {
		return errors.New("empty file")
	}
	page1, err := readPage(dbPath, 1)
	if err != nil {
		return err
	}
	if len(page1) < pageSize {
		return errors.New("file too small")
	}
	if err := verifyPage1(page1, encKey); err != nil {
		return err
	}

	in, err := os.Open(dbPath)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer out.Close()

	for pageNo := 1; pageNo <= totalPages; pageNo++ {
		page := make([]byte, pageSize)
		n, err := io.ReadFull(in, page)
		if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
			return err
		}
		if n == 0 {
			break
		}
		if n < pageSize {
			for i := n; i < pageSize; i++ {
				page[i] = 0
			}
		}
		decrypted, err := decryptPage(page, encKey, pageNo)
		if err != nil {
			return err
		}
		if _, err := out.Write(decrypted); err != nil {
			return err
		}
	}
	return nil
}

func readPage(path string, pageNo int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	buf := make([]byte, pageSize)
	_, err = f.ReadAt(buf, int64((pageNo-1)*pageSize))
	return buf, err
}

func verifyPage1(page []byte, encKey []byte) error {
	salt := page[:saltSize]
	macSalt := xorBytes(salt, 0x3a)
	macKey := pbkdf2.Key(encKey, macSalt, 2, 32, sha512.New)

	data := page[saltSize : pageSize-reserveLen+ivSize]
	stored := page[pageSize-hmacSize:]
	mac := hmac.New(sha512.New, macKey)
	mac.Write(data)
	pageNo := make([]byte, 4)
	binary.LittleEndian.PutUint32(pageNo, 1)
	mac.Write(pageNo)
	if !hmac.Equal(mac.Sum(nil), stored) {
		return errors.New("page 1 HMAC 验证失败")
	}
	return nil
}

func decryptPage(page []byte, encKey []byte, pageNo int) ([]byte, error) {
	iv := page[pageSize-reserveLen : pageSize-reserveLen+ivSize]
	block, err := aes.NewCipher(encKey)
	if err != nil {
		return nil, err
	}
	if pageNo == 1 {
		encrypted := page[saltSize : pageSize-reserveLen]
		decrypted := make([]byte, len(encrypted))
		cipher.NewCBCDecrypter(block, iv).CryptBlocks(decrypted, encrypted)
		out := make([]byte, 0, pageSize)
		out = append(out, sqliteHeader...)
		out = append(out, decrypted...)
		out = append(out, make([]byte, reserveLen)...)
		return out, nil
	}
	encrypted := page[:pageSize-reserveLen]
	decrypted := make([]byte, len(encrypted))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(decrypted, encrypted)
	out := make([]byte, 0, pageSize)
	out = append(out, decrypted...)
	out = append(out, make([]byte, reserveLen)...)
	return out, nil
}

func xorBytes(src []byte, v byte) []byte {
	out := make([]byte, len(src))
	for i, b := range src {
		out[i] = b ^ v
	}
	return out
}
