package runner

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/containerd/errdefs"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/google/uuid"
)

// Config is the runtime configuration for the runner.
type Config struct {
	BotImage                string
	BotServiceURL           string
	BotServiceTimeout       time.Duration
	TeamSpeakServiceURL     string
	TeamSpeakServiceTimeout time.Duration
	ServiceAPIKey           string
	QueryTimeout            time.Duration
	QueryKeepAlive          time.Duration
	ReconnectInterval       time.Duration
	ShutdownTimeout         time.Duration
	DockerHost              string
	DockerNetwork           string
	DockerPullPolicy        string
	DockerAutoRemove        bool
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
		if errdefs.IsNotFound(err) {
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
	} else if !errdefs.IsNotFound(err) {
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

// ContainerInfo is a summary of a Docker container for the infra report.
type ContainerInfo struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Image   string `json:"image"`
	Status  string `json:"status"`
	State   string `json:"state"`
	Health  string `json:"health"`
	Uptime  int64  `json:"uptime_seconds"`
	Created int64  `json:"created"`
}

// ListContainers returns all containers (running and stopped) on the host.
func (r *Runner) ListContainers(ctx context.Context) ([]ContainerInfo, error) {
	containers, err := r.cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return nil, fmt.Errorf("list containers: %w", err)
	}

	result := make([]ContainerInfo, 0, len(containers))
	for _, c := range containers {
		name := ""
		if len(c.Names) > 0 {
			name = c.Names[0]
			if len(name) > 0 && name[0] == '/' {
				name = name[1:]
			}
		}
		health := ""
		if c.State == "running" && c.Status != "" {
			// Status string like "Up 5 minutes (healthy)" or "Up 5 minutes"
			if idx := indexOf(c.Status, "("); idx >= 0 {
				end := indexOf(c.Status, ")")
				if end > idx {
					health = c.Status[idx+1 : end]
				}
			}
		}
		uptime := int64(0)
		if c.State == "running" {
			uptime = int64(time.Since(time.Unix(c.Created, 0)).Seconds())
		}
		result = append(result, ContainerInfo{
			ID:      c.ID[:12],
			Name:    name,
			Image:   c.Image,
			Status:  c.Status,
			State:   c.State,
			Health:  health,
			Uptime:  uptime,
			Created: c.Created,
		})
	}
	return result, nil
}

// ContainerLogs returns the last N lines of logs from a container by name.
func (r *Runner) ContainerLogs(ctx context.Context, containerName string, tail int) (string, error) {
	if tail <= 0 || tail > 2000 {
		tail = 200
	}
	opts := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       fmt.Sprintf("%d", tail),
		Timestamps: true,
	}
	reader, err := r.cli.ContainerLogs(ctx, containerName, opts)
	if err != nil {
		return "", fmt.Errorf("container logs: %w", err)
	}
	defer reader.Close()
	data, err := io.ReadAll(reader)
	if err != nil {
		return "", fmt.Errorf("read logs: %w", err)
	}
	return string(data), nil
}

// InfraReport is a summary of the Docker infrastructure.
type InfraReport struct {
	TotalContainers     int             `json:"total_containers"`
	RunningContainers   int             `json:"running_containers"`
	StoppedContainers   int             `json:"stopped_containers"`
	HealthyContainers   int             `json:"healthy_containers"`
	UnhealthyContainers int             `json:"unhealthy_containers"`
	Images              int             `json:"images"`
	DockerVersion       string          `json:"docker_version"`
	Containers          []ContainerInfo `json:"containers"`
}

// InfraReport returns a summary of the Docker infrastructure.
func (r *Runner) InfraReport(ctx context.Context) (*InfraReport, error) {
	containers, err := r.ListContainers(ctx)
	if err != nil {
		return nil, err
	}

	imgs, err := r.cli.ImageList(ctx, image.ListOptions{})
	if err != nil {
		// Non-fatal — just report 0
		imgs = nil
	}

	info, err := r.cli.Info(ctx)
	dockerVersion := ""
	if err == nil {
		dockerVersion = info.ServerVersion
	}

	report := &InfraReport{
		TotalContainers: len(containers),
		Containers:      containers,
		Images:          len(imgs),
		DockerVersion:   dockerVersion,
	}
	for _, c := range containers {
		if c.State == "running" {
			report.RunningContainers++
			if c.Health == "healthy" {
				report.HealthyContainers++
			} else if c.Health == "unhealthy" {
				report.UnhealthyContainers++
			}
		} else {
			report.StoppedContainers++
		}
	}
	return report, nil
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
