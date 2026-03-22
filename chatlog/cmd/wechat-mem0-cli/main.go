package main

import (
	"flag"
	"os"

	"github.com/rs/zerolog/log"
	"github.com/sjzar/chatlog/internal/wechatdb"
	"github.com/sjzar/chatlog/pkg/logger"
)

func main() {
	// Parse command line flags
	logLevel := flag.String("log-level", "info", "Log level (debug, info, warn, error)")
	workDir := flag.String("work-dir", "", "dir of wechat db")
	platform := flag.String("platform", "", "os platform")
	version := flag.Int("version", 4, "version of wechat")
	flag.Parse()

	// Override with environment variable if set
	if envLevel := os.Getenv("LOG_LEVEL"); envLevel != "" {
		*logLevel = envLevel
	}

	// Initialize logger
	logger.Init(*logLevel)

	// Create your Manager implementation here
	// TODO: Replace StubManager with your actual implementation
	// mgr := manager.NewStubManager()
	db, err := wechatdb.New(*workDir, *platform, *version)
	if err != nil {
		logger.Error().Msgf("failed to create wechat db, err:%+v", err)
		return
	}
	defer db.Close()

	contacts, err := db.GetContacts("", 0, 0)
	if err != nil {
		log.Error().Err(err).Msg("failed to get contacts")
		return
	}
	for _, item := range contacts.Items {
		log.Info().Msgf("userName:%s-remark:%s", item.UserName, item.Remark)
	}

}
