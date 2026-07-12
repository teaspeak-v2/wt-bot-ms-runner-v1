package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/go-playground/validator/v10"
	"github.com/joho/godotenv"
)

type Config struct {
	App               AppConfig     `envPrefix:"APP_"`
	Server            ServerConfig  `envPrefix:"SERVER_"`
	ServiceAPIKey     string        `env:"SERVICE_API_KEY" validate:"required"`
	BotImage          string        `env:"BOT_IMAGE" envDefault:"wt-bot-bot:latest"`
	BotService        ServiceConfig `envPrefix:"BOT_SERVICE_"`
	TeamSpeakService  ServiceConfig `envPrefix:"TEAMSPEAK_SERVICE_"`
	Bot               BotConfig     `envPrefix:"BOT_"`
	Docker            DockerConfig  `envPrefix:"DOCKER_"`
}

type AppConfig struct {
	Env      string `env:"ENV" envDefault:"development" validate:"required,oneof=development staging production test"`
	Name     string `env:"NAME" envDefault:"wt-bot-ms-runner-v1" validate:"required"`
	Version  string `env:"VERSION" envDefault:"0.1.0" validate:"required"`
	LogLevel string `env:"LOG_LEVEL" envDefault:"info" validate:"required"`
}

type ServerConfig struct {
	Addr            string        `env:"ADDR" envDefault:":8080" validate:"required"`
	ReadTimeout     time.Duration `env:"READ_TIMEOUT" envDefault:"10s" validate:"required"`
	WriteTimeout    time.Duration `env:"WRITE_TIMEOUT" envDefault:"10s" validate:"required"`
	IdleTimeout     time.Duration `env:"IDLE_TIMEOUT" envDefault:"60s" validate:"required"`
	ShutdownTimeout time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"15s" validate:"required"`
}

type ServiceConfig struct {
	URL     string        `env:"URL" validate:"required,url"`
	Timeout time.Duration `env:"TIMEOUT" envDefault:"10s" validate:"required"`
}

type BotConfig struct {
	QueryTimeout      time.Duration `env:"QUERY_TIMEOUT" envDefault:"10s" validate:"required"`
	QueryKeepAlive    time.Duration `env:"QUERY_KEEPALIVE" envDefault:"200s" validate:"required"`
	ReconnectInterval time.Duration `env:"RECONNECT_INTERVAL" envDefault:"10s" validate:"required"`
	ShutdownTimeout   time.Duration `env:"SHUTDOWN_TIMEOUT" envDefault:"15s" validate:"required"`
}

type DockerConfig struct {
	Host        string `env:"HOST" envDefault:"unix:///var/run/docker.sock"`
	Network     string `env:"NETWORK" envDefault:""`
	PullPolicy  string `env:"PULL_POLICY" envDefault:"missing" validate:"required,oneof=missing always never"`
	AutoRemove  bool   `env:"AUTO_REMOVE" envDefault:"true"`
}

func Load() (Config, error) {
	return LoadFromPath(".env")
}

func LoadFromPath(envFile string) (Config, error) {
	_ = godotenv.Load(envFile)
	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		return Config{}, err
	}
	if err := validator.New().Struct(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func MustLoad() Config {
	cfg, err := Load()
	if err != nil {
		panic(fmt.Sprintf("load config: %v", err))
	}
	return cfg
}
