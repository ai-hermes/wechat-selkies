package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sjzar/chatlog/internal/wechatdecryptmacos"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "scan":
		if err := runScan(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "scan failed:", err)
			os.Exit(1)
		}
	case "decrypt":
		if err := runDecrypt(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "decrypt failed:", err)
			os.Exit(1)
		}
	case "all":
		if err := runAll(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, "all failed:", err)
			os.Exit(1)
		}
	default:
		printUsage()
		os.Exit(1)
	}
}

func runScan(args []string) error {
	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	pid := fs.Int("pid", 0, "")
	outPath := fs.String("out", "", "")
	dbBase := fs.String("db-base", "", "")
	fs.Parse(args)

	baseDir, err := os.Getwd()
	if err != nil {
		return err
	}
	cfg, cfgErr := wechatdecryptmacos.LoadConfig(baseDir)
	if cfgErr != nil && !errors.Is(cfgErr, wechatdecryptmacos.ErrDBDirNotFound) {
		return cfgErr
	}

	if *outPath == "" {
		*outPath = cfg.KeysFile
	}
	if *dbBase == "" {
		*dbBase = wechatdecryptmacos.DefaultDBBaseDir()
	}

	targetPID := *pid
	if targetPID == 0 {
		targetPID, err = wechatdecryptmacos.FindWeChatPID()
		if err != nil {
			return err
		}
	}

	absOut, err := filepath.Abs(*outPath)
	if err != nil {
		return err
	}

	return wechatdecryptmacos.ScanAndSave(context.Background(), targetPID, *dbBase, absOut)
}

func runDecrypt(args []string) error {
	fs := flag.NewFlagSet("decrypt", flag.ExitOnError)
	dbDir := fs.String("db-dir", "", "")
	keysFile := fs.String("keys", "", "")
	outDir := fs.String("out", "", "")
	fs.Parse(args)

	baseDir, err := os.Getwd()
	if err != nil {
		return err
	}
	cfg, cfgErr := wechatdecryptmacos.LoadConfig(baseDir)
	if cfgErr != nil && !errors.Is(cfgErr, wechatdecryptmacos.ErrDBDirNotFound) {
		return cfgErr
	}

	if *dbDir == "" {
		*dbDir = cfg.DBDir
	}
	if *keysFile == "" {
		*keysFile = cfg.KeysFile
	}
	if *outDir == "" {
		*outDir = cfg.DecryptedDir
	}

	return wechatdecryptmacos.DecryptAll(context.Background(), *dbDir, *keysFile, *outDir)
}

func runAll(args []string) error {
	fs := flag.NewFlagSet("all", flag.ExitOnError)
	pid := fs.Int("pid", 0, "")
	outPath := fs.String("out", "", "")
	dbBase := fs.String("db-base", "", "")
	dbDir := fs.String("db-dir", "", "")
	keysFile := fs.String("keys", "", "")
	outDir := fs.String("out-dir", "", "")
	fs.Parse(args)

	baseDir, err := os.Getwd()
	if err != nil {
		return err
	}
	cfg, cfgErr := wechatdecryptmacos.LoadConfig(baseDir)
	if cfgErr != nil && !errors.Is(cfgErr, wechatdecryptmacos.ErrDBDirNotFound) {
		return cfgErr
	}

	if *outPath == "" {
		*outPath = cfg.KeysFile
	}
	if *dbBase == "" {
		*dbBase = wechatdecryptmacos.DefaultDBBaseDir()
	}
	if *dbDir == "" {
		*dbDir = cfg.DBDir
	}
	if *keysFile == "" {
		*keysFile = cfg.KeysFile
	}
	if *outDir == "" {
		*outDir = cfg.DecryptedDir
	}

	targetPID := *pid
	if targetPID == 0 {
		targetPID, err = wechatdecryptmacos.FindWeChatPID()
		if err != nil {
			return err
		}
	}

	if err := wechatdecryptmacos.ScanAndSave(context.Background(), targetPID, *dbBase, *outPath); err != nil {
		return err
	}
	return wechatdecryptmacos.DecryptAll(context.Background(), *dbDir, *keysFile, *outDir)
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  wechat-decrypt-macos scan    [-pid PID] [-out all_keys.json] [-db-base <xwechat_files>]")
	fmt.Println("  wechat-decrypt-macos decrypt [-db-dir <db_storage>] [-keys all_keys.json] [-out decrypted]")
	fmt.Println("  wechat-decrypt-macos all     [-pid PID] [-out all_keys.json] [-db-base <xwechat_files>] [-db-dir <db_storage>] [-keys all_keys.json] [-out-dir decrypted]")
}
