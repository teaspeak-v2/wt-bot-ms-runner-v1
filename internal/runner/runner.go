package runner

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
)

// Config is the runtime configuration for the runner.
type Config struct {
	BotImage             string
	BotServiceURL        string
	BotServiceTimeout    time.Duration
	TeamSpeakServiceURL  string
	TeamSpeakServiceTimeout time.Duration
	ServiceAPIKey        string
	QueryTimeout         time.Duration
	QueryKeepAlive       time.Duration
	ReconnectInterval    time.Duration
	ShutdownTimeout      time.Duration
	DockerHost           string
	DockerNetwork        string
	DockerPullPolicy     string
	DockerAutoRemove     bool
}

// Result is the status of a bot container.
type Result struct {
	ContainerID string
	Status      string
}

// Runner spawns and manages Docker containers for bot instances.
type Runner struct {
	cfg Config
	cli *client.Client
}

// New creates a new Runner.
func New(cfg Config) (*Runner, error) {
	opts := []client.Opt{client.FromEnv}
	if cfg.DockerHost != "" {
		opts = append(opts, client.WithHost(cfg.DockerHost))
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("docker client: %w", err)
	}
	cli.NegotiateAPIVersion(context.Background())

	return &Runner{cfg: cfg, cli: cli}, nil
}

// Close closes the underlying Docker client.
func (r *Runner) Close() error {
	return r.cli.Close()
}

// Ping checks the Docker daemon connection.
func (r *Runner) Ping(ctx context.Context) error {
	_, err := r.cli.Ping(ctx)
	return err
}

func (r *Runner) containerName(botID string) string {
	return "wt-bot-" + botID
}

// Spawn creates and starts a Docker container for the bot.
func (r *Runner) Spawn(ctx context.Context, botID uuid.UUID) (*Result, error) {
	if err := r.ensureImage(ctx); err != nil {
		return nil, fmt.Errorf("image: %w", err)
	}

	existing, err := r.findContainer(ctx, botID)
	if err != nil {
		return nil, err
	}

	if existing != "" {
		info, err := r.cli.ContainerInspect(ctx, existing)
		if err != nil {
			return nil, err
		}
		if info.State.Running {
			return &Result{ContainerID: info.ID, Status: "running"}, nil
		}
		if err := r.cli.ContainerStart(ctx, existing, container.StartOptions{}); err != nil {
			return nil, fmt.Errorf("start existing: %w", err)
		}
		return &Result{ContainerID: existing, Status: "started"}, nil
	}

	name := r.containerName(botID.String())
	env := []string{
		fmt.Sprintf("BOT_ID=%s", botID.String()),
		fmt.Sprintf("BOT_SERVICE_URL=%s", r.cfg.BotServiceURL),
		fmt.Sprintf("BOT_SERVICE_TIMEOUT=%s", r.cfg.BotServiceTimeout),
		fmt.Sprintf("TEAMSPEAK_SERVICE_URL=%s", r.cfg.TeamSpeakServiceURL),
		fmt.Sprintf("TEAMSPEAK_SERVICE_TIMEOUT=%s", r.cfg.TeamSpeakServiceTimeout),
		fmt.Sprintf("SERVICE_API_KEY=%s", r.cfg.ServiceAPIKey),
		fmt.Sprintf("QUERY_TIMEOUT=%s", r.cfg.QueryTimeout),
		fmt.Sprintf("QUERY_KEEPALIVE=%s", r.cfg.QueryKeepAlive),
		fmt.Sprintf("RECONNECT_INTERVAL=%s", r.cfg.ReconnectInterval),
		fmt.Sprintf("SHUTDOWN_TIMEOUT=%s", r.cfg.ShutdownTimeout),
	}

	cfg := &container.Config{
		Image: r.cfg.BotImage,
		Env:   env,
		Labels: map[string]string{
			"app":    "wt-bot-runner",
			"bot_id": botID.String(),
		},
	}

	hostCfg := &container.HostConfig{
		AutoRemove:    r.cfg.DockerAutoRemove,
		RestartPolicy: container.RestartPolicy{Name: container.RestartPolicyDisabled},
	}
	if r.cfg.DockerNetwork != "" {
		hostCfg.NetworkMode = container.NetworkMode(r.cfg.DockerNetwork)
	}

	netCfg := &network.NetworkingConfig{}
	if r.cfg.DockerNetwork != "" {
		netCfg.EndpointsConfig = map[string]*network.EndpointSettings{
			r.cfg.DockerNetwork: {},
		}
	}

	resp, err := r.cli.ContainerCreate(ctx, cfg, hostCfg, netCfg, nil, name)
	if err != nil {
		return nil, fmt.Errorf("create container: %w", err)
	}

	if err := r.cli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("start container: %w", err)
	}

	return &Result{ContainerID: resp.ID, Status: "running"}, nil
}

// Stop stops and removes a bot container.
func (r *Runner) Stop(ctx context.Context, botID uuid.UUID) error {
	existing, err := r.findContainer(ctx, botID)
	if err != nil {
		return err
	}
	if existing == "" {
		return nil
	}

	timeout := 10
	if err := r.cli.ContainerStop(ctx, existing, container.StopOptions{Timeout: &timeout}); err != nil {
		return fmt.Errorf("stop container: %w", err)
	}

	if !r.cfg.DockerAutoRemove {
		if err := r.cli.ContainerRemove(ctx, existing, container.RemoveOptions{Force: false}); err != nil {
			return fmt.Errorf("remove container: %w", err)
		}
	}
	return nil
}

// Status returns the current status of a bot container.
func (r *Runner) Status(ctx context.Context, botID uuid.UUID) (*Result, error) {
	existing, err := r.findContainer(ctx, botID)
	if err != nil {
		return nil, err
	}
	if existing == "" {
		return &Result{ContainerID: "", Status: "missing"}, nil
	}

	info, err := r.cli.ContainerInspect(ctx, existing)
	if err != nil {
		return nil, err
	}

	status := "exited"
	if info.State.Running {
		status = "running"
	}
	return &Result{ContainerID: info.ID, Status: status}, nil
}

func (r *Runner) findContainer(ctx context.Context, botID uuid.UUID) (string, error) {
	f := filters.NewArgs()
	f.Add("label", "app=wt-bot-runner")
	f.Add("label", "bot_id="+botID.String())
	containers, err := r.cli.ContainerList(ctx, container.ListOptions{All: true, Filters: f})
	if err != nil {
		return "", err
	}
	if len(containers) > 0 {
		return containers[0].ID, nil
	}

	info, err := r.cli.ContainerInspect(ctx, r.containerName(botID.String()))
	if err != nil {
		if client.IsErrNotFound(err) {
			return "", nil
		}
		return "", err
	}
	return info.ID, nil
}

func (r *Runner) ensureImage(ctx context.Context) error {
	if r.cfg.DockerPullPolicy == "never" {
		return nil
	}

	_, err := r.cli.ImageInspect(ctx, r.cfg.BotImage)
	if err == nil {
		if r.cfg.DockerPullPolicy != "always" {
			return nil
		}
	} else if !client.IsErrNotFound(err) {
		return err
	}

	rc, err := r.cli.ImagePull(ctx, r.cfg.BotImage, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pull image: %w", err)
	}
	defer rc.Close()
	_, _ = io.Copy(io.Discard, rc)
	return nil
}
