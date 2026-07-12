package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/teaspeak-v2/wt-bot-ms-runner-v1/internal/config"
	"github.com/teaspeak-v2/wt-bot-ms-runner-v1/internal/handlers"
	"github.com/teaspeak-v2/wt-bot-ms-runner-v1/internal/httpserver"
	"github.com/teaspeak-v2/wt-bot-ms-runner-v1/internal/runner"
)

// App is the runner application container.
type App struct {
	Config config.Config
	Logger *slog.Logger
	Runner *runner.Runner
	Server *httpserver.Server
}

// New creates a new App.
func New(cfg config.Config, logger *slog.Logger) (*App, error) {
	r, err := runner.New(runner.Config{
		BotImage:                cfg.BotImage,
		BotServiceURL:           cfg.BotService.URL,
		BotServiceTimeout:       cfg.BotService.Timeout,
		TeamSpeakServiceURL:     cfg.TeamSpeakService.URL,
		TeamSpeakServiceTimeout: cfg.TeamSpeakService.Timeout,
		ServiceAPIKey:           cfg.ServiceAPIKey,
		QueryTimeout:            cfg.Bot.QueryTimeout,
		QueryKeepAlive:          cfg.Bot.QueryKeepAlive,
		ReconnectInterval:       cfg.Bot.ReconnectInterval,
		ShutdownTimeout:         cfg.Bot.ShutdownTimeout,
		DockerHost:              cfg.Docker.Host,
		DockerNetwork:           cfg.Docker.Network,
		DockerPullPolicy:        cfg.Docker.PullPolicy,
		DockerAutoRemove:        cfg.Docker.AutoRemove,
	})
	if err != nil {
		return nil, fmt.Errorf("runner: %w", err)
	}

	runnerHandler := handlers.NewRunnerHandler(r, logger)
	healthHandler := handlers.NewHealthHandler(func() error { return r.Ping(context.Background()) })

	router := httpserver.NewRouter(httpserver.RouterDeps{
		RunnerHandler: runnerHandler,
		HealthHandler: healthHandler,
		ServiceAPIKey: cfg.ServiceAPIKey,
	})

	server := httpserver.New(
		cfg.Server.Addr,
		router,
		cfg.Server.ReadTimeout,
		cfg.Server.WriteTimeout,
		cfg.Server.IdleTimeout,
		cfg.Server.ShutdownTimeout,
	)

	return &App{Config: cfg, Logger: logger, Runner: r, Server: server}, nil
}

// Close closes the app.
func (a *App) Close() error {
	if a.Runner != nil {
		return a.Runner.Close()
	}
	return nil
}

// LogLevel parses the configured log level.
func LogLevel(level string) slog.Level {
	switch level {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// NewLogger creates a JSON logger.
func NewLogger(level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}
