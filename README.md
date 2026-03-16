# Sift Agent

A lightweight agent that runs inside your infrastructure to securely connect [Sift](https://sift.works) to your database. The agent polls the Sift API for commands, executes them against your local database, and returns results — your database credentials never leave your network.

## Quick Start

### Install

```sh
curl -fsSL https://sift.works/install-agent | sh
```

Or download a binary directly from [GitHub Releases](https://github.com/sift-works/agent/releases).

### Run

```sh
sift-agent --token <your-sift-token> --dsn mysql://user:pass@localhost:3306/mydb
```

That's it. The agent connects to Sift, starts polling for commands, and sends heartbeats to confirm it's alive.

## Configuration

Every option can be set via CLI flag or environment variable. Flags take priority over env vars.

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--token` | `SIFT_TOKEN` | — | **(Required)** Your Sift API token |
| `--dsn` | `SIFT_DSN` | — | **(Required)** Database connection string |
| `--api-url` | `SIFT_API_URL` | `https://app.sift.works` | Sift API base URL |
| `--poll-interval` | `SIFT_POLL_INTERVAL` | `2` | Seconds between polls |
| `--heartbeat-interval` | `SIFT_HEARTBEAT_INTERVAL` | `10` | Seconds between heartbeats |
| `--log-level` | `SIFT_LOG_LEVEL` | `info` | Log level (`debug`, `info`, `warn`, `error`) |
| `--version` | — | — | Print version and exit |

### DSN Format

```
mysql://user:password@host:port/database
```

Only MySQL is supported in v1. PostgreSQL support is planned.

## Deployment

### Docker

```sh
docker run -d \
  -e SIFT_TOKEN=your-token \
  -e SIFT_DSN=mysql://user:pass@host:3306/mydb \
  siftworks/agent:latest
```

### Docker Compose

```yaml
services:
  sift-agent:
    image: siftworks/agent:latest
    environment:
      SIFT_TOKEN: ${SIFT_TOKEN}
      SIFT_DSN: mysql://user:pass@db:3306/mydb
    restart: unless-stopped
    depends_on:
      - db
```

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: sift-agent
spec:
  replicas: 1
  selector:
    matchLabels:
      app: sift-agent
  template:
    metadata:
      labels:
        app: sift-agent
    spec:
      containers:
        - name: sift-agent
          image: siftworks/agent:latest
          env:
            - name: SIFT_TOKEN
              valueFrom:
                secretKeyRef:
                  name: sift-agent
                  key: token
            - name: SIFT_DSN
              valueFrom:
                secretKeyRef:
                  name: sift-agent
                  key: dsn
          resources:
            requests:
              memory: 32Mi
              cpu: 50m
            limits:
              memory: 128Mi
```

### systemd

```ini
[Unit]
Description=Sift Agent
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/sift-agent
Environment=SIFT_TOKEN=your-token
Environment=SIFT_DSN=mysql://user:pass@localhost:3306/mydb
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Save to `/etc/systemd/system/sift-agent.service`, then:

```sh
sudo systemctl daemon-reload
sudo systemctl enable --now sift-agent
```

## How It Works

1. The agent connects to your database and verifies it's reachable.
2. It starts two background loops:
   - **Poll loop** — checks the Sift API for pending commands (queries, schema introspection, connection tests) and executes them against your database.
   - **Heartbeat loop** — periodically reports the agent version and database reachability to Sift.
3. On `SIGTERM` or `SIGINT`, the agent waits for in-flight commands to finish (up to 30s) before shutting down.

All communication is outbound (agent → Sift API). No inbound ports need to be opened.

## Building from Source

```sh
git clone https://github.com/sift-works/agent.git
cd agent
make build        # binary at ./bin/sift-agent
make test         # run tests with race detector
make lint         # go vet
```

Requires Go 1.23+.
