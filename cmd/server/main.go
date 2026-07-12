package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/teaspeak-v2/wt-bot-ms-runner-v1/internal/app"
	"github.com/teaspeak-v2/wt-bot-ms-runner-v1/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		panic(err)
	}

	logger := app.NewLogger(app.LogLevel(cfg.App.LogLevel))

	application, err := app.New(cfg, logger)
	if err != nil {
		logger.Error("app init failed", "error", err)
		os.Exit(1)
	}
	defer func() { _ = application.Close() }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := application.Server.Run(ctx); err != nil {
		logger.Error("server stopped with error", "error", err)
		os.Exit(1)
	}
}
