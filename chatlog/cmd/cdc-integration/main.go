package main

import (
	"os"
	"strconv"

	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog/log"
	"github.com/sjzar/chatlog/internal/wechatdb"
	"github.com/sjzar/chatlog/pkg/backup"

	_ "github.com/joho/godotenv/autoload"
)

func main() {

	// debug 场景没法突破内存限制，用单独的命令行可以解析出来
	workDir := os.Getenv("WORK_DIR")
	platform := os.Getenv("PLATFORM")
	version, _ := strconv.Atoi(os.Getenv("VERSION"))

	// Initialize WeChat Source DB
	db, err := wechatdb.New(workDir, platform, version)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create wechat db")
		return
	}

	// Backup Configuration (Example: SQLite)
	// In a real scenario, these could come from CLI flags or ENV vars
	backupConfig := backup.Config{
		Driver: backup.DriverSQLite,
		DSN:    "/Users/dingwenjiang/Library/Application Support/rethink-ai/wechat-mem0-chats.db", // Target DB
	}

	// Initialize Backup Service
	svc, err := backup.NewService(backupConfig, db)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to create backup service")
		return
	}

	// Run Backup
	if err := svc.MessageCDC(); err != nil {
		log.Fatal().Err(err).Msg("backup process failed")
		return
	}

	log.Info().Msg("Backup completed successfully via GORM service")

}
