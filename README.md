# WT-Bot Bot Runner Microservice

`wt-bot-ms-runner-v1` is a small service that runs on the bot host and spawns `wt-bot-bot` Docker containers on request from `wt-bot-ms-bots-v1`.

## Flow

1. `wt-bot-ms-bots-v1` calls `POST /api/v1/bots/{id}/spawn` with the `X-Service-Key` header.
2. The runner ensures the `BOT_IMAGE` is available and creates/starts a Docker container for the bot.
3. The bot container starts with `BOT_ID` and URLs to the other microservices.
4. The bot fetches its runtime config from `wt-bot-ms-bots-v1` and connection details from `wt-bot-ms-teamspeaks-v1`.
5. The bot connects to the TeamSpeak ServerQuery and begins listening.

## API

- `GET /healthz` — liveness
- `GET /readyz` — readiness (also checks Docker connectivity)
- `POST /api/v1/bots/{id}/spawn` — spawn a bot container
- `POST /api/v1/bots/{id}/stop` — stop and remove a bot container
- `GET /api/v1/bots/{id}/status` — container status

All `/api/v1` routes require the `X-Service-Key` header.

## Configuration

Copy `.env.example` to `.env` and adjust values.

## Commands

- `make build` — compile the service
- `make test` — run tests
- `make vet` — run Go vet
- `make lint` — run golangci-lint
- `make run` — run locally
- `make docker` — build the Docker image

## Docker

The runner needs access to the Docker daemon. When running in Docker, mount the Docker socket:

```yaml
volumes:
  - /var/run/docker.sock:/var/run/docker.sock
```
