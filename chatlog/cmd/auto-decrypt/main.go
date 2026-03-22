package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/sjzar/chatlog/internal/chatlog"
	"github.com/sjzar/chatlog/internal/chatlog/ctx"
	"github.com/sjzar/chatlog/internal/chatlog/wechat"
)

func main() {
	m := chatlog.New(chatlog.ManagerTypeGRPC)

	configPath := "/Users/dingwenjiang/.chatlog"
	chatlogCtx, err := ctx.New(configPath)
	if err != nil {
		panic(err)
	}

	wechatService := wechat.NewService(chatlogCtx)

	m.SetContext(chatlogCtx, nil, nil, wechatService)

	err = m.StartAutoDecrypt()
	if err != nil {
		return
	}

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-sigChan:
			if err := m.StopAutoDecrypt(); err != nil {
				panic(err)
			}
			return
		default:
		}
	}
}
