# Sift Connection Agent

The Sift Connection Agent is a lightweight process that runs in your infrastructure and proxies database queries between Sift and your database. Your database credentials never leave your network.

## How It Works

```
┌─────────────┐         HTTPS (outbound)        ┌─────────────┐
│  Your Infra │ ──────────────────────────────── │    Sift     │
│             │                                  │             │
│  ┌────────┐ │  1. Poll for commands            │  ┌───────┐  │
│  │ Agent  │──│──GET /api/agent/commands────────│──│ Queue │  │
│  │        │ │                                  │  └───────┘  │
│  │        │ │  2. Execute locally              │             │
│  │   ↕    │ │                                  │             │
│  │  ┌──┐  │ │  3. Return results               │             │
│  │  │DB│  │──│──POST /api/agent/commands/…────│──│  Results │ │
│  │  └──┘  │ │                                  │             │
│  └────────┘ │  4. Heartbeat                    │             │
│             │──POST /api/agent/heartbeat──────│──           │
└─────────────┘                                  └─────────────┘
```

1. The agent polls Sift for pending commands (queries, schema requests, connection tests)
2. When a command arrives, the agent executes it against your local database
3. Results are posted back to Sift over HTTPS
4. Periodic heartbeats keep Sift informed of agent status

All communication is **outbound** from your network. No inbound ports need to be opened.

## Setup

### 1. Create an Agent Connection in Sift

1. Go to **Connections** and click **New Connection**
2. Select the **Agent** tab
3. Enter a name and choose your database type (MySQL or PostgreSQL)
4. Click **Create Agent Connection**
5. Copy the token — it is shown only once

### 2. Install the Agent

```bash
curl -sSL https://sift.works/install-agent | sh
```

Or download the binary directly from the [releases page](https://github.com/sift-works/agent/releases).

### 3. Run the Agent

```bash
sift-agent \
  --token <YOUR_AGENT_TOKEN> \
  --dsn "mysql://user:password@localhost:3306/mydb"
```

## Configuration

### Command-Line Flags

| Flag | Required | Description | Default |
|------|----------|-------------|---------|
| `--token` | Yes | Sanctum API token from Sift | — |
| `--dsn` | Yes | Database connection string | — |
| `--api-url` | No | Sift API base URL | `https://app.sift.works` |
| `--poll-interval` | No | Seconds between polls | `2` |
| `--heartbeat-interval` | No | Seconds between heartbeats | `10` |
| `--log-level` | No | Log verbosity (`debug`, `info`, `warn`, `error`) | `info` |

### DSN Format

```
<driver>://[user[:password]@]host[:port]/database[?param=value]
```

**MySQL:**
```
mysql://root:secret@127.0.0.1:3306/production
```

**PostgreSQL:**
```
postgresql://postgres:secret@127.0.0.1:5432/production?sslmode=require
```

### Environment Variables

All flags can also be set via environment variables:

| Variable | Flag equivalent |
|----------|----------------|
| `SIFT_TOKEN` | `--token` |
| `SIFT_DSN` | `--dsn` |
| `SIFT_API_URL` | `--api-url` |
| `SIFT_POLL_INTERVAL` | `--poll-interval` |
| `SIFT_HEARTBEAT_INTERVAL` | `--heartbeat-interval` |
| `SIFT_LOG_LEVEL` | `--log-level` |

## Running as a Service

### systemd (Linux)

Create `/etc/systemd/system/sift-agent.service`:

```ini
[Unit]
Description=Sift Connection Agent
After=network.target

[Service]
Type=simple
User=sift-agent
Environment=SIFT_TOKEN=your-token-here
Environment=SIFT_DSN=mysql://user:pass@localhost:3306/mydb
ExecStart=/usr/local/bin/sift-agent
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl enable sift-agent
sudo systemctl start sift-agent
```

### Docker

```bash
docker run -d \
  --name sift-agent \
  --restart unless-stopped \
  -e SIFT_TOKEN=your-token-here \
  -e SIFT_DSN=mysql://user:pass@host.docker.internal:3306/mydb \
  siftworks/agent:latest
```

Or with Docker Compose:

```yaml
services:
  sift-agent:
    image: siftworks/agent:latest
    restart: unless-stopped
    environment:
      SIFT_TOKEN: ${SIFT_TOKEN}
      SIFT_DSN: mysql://user:pass@db:3306/mydb
    depends_on:
      - db
```

## API Protocol Reference

The agent communicates with Sift using three HTTP endpoints. This section is useful if you are building a custom agent implementation.

### Authentication

All requests must include a Bearer token in the `Authorization` header:

```
Authorization: Bearer <token>
```

### Poll for Commands

```
GET /api/agent/commands
```

**Response (command available):** `200 OK`
```json
{
  "id": "abc-123",
  "type": "query",
  "payload": {
    "sql": "SELECT * FROM users LIMIT 10"
  }
}
```

**Response (no commands):** `204 No Content`

Command types:
- `query` — Execute SQL. Payload: `{ "sql": "..." }`
- `test_connection` — Verify database reachability. Payload: `{}`
- `schema_tables` — List all tables with column metadata. Payload: `{}`
- `schema_table_details` — Get detailed info for one table. Payload: `{ "table_name": "..." }`
- `schema_relations` — Get foreign key relations for a table. Payload: `{ "table_name": "..." }`

### Submit Result

```
POST /api/agent/commands/{commandId}/result
Content-Type: application/json

{
  "success": true,
  "result": {
    "columns": ["id", "name", "email"],
    "rows": [
      [1, "Alice", "alice@example.com"],
      [2, "Bob", "bob@example.com"]
    ],
    "row_count": 2,
    "duration_ms": 12
  }
}
```

For failures:
```json
{
  "success": false,
  "error_message": "Table 'users' doesn't exist"
}
```

### Heartbeat

```
POST /api/agent/heartbeat
Content-Type: application/json

{
  "version": "1.0.0",
  "database_reachable": true
}
```

**Response:** `200 OK`
```json
{
  "name": "Production DB",
  "type": "mysql"
}
```

## Security

- **Zero knowledge:** Sift never sees or stores your database credentials
- **Outbound only:** The agent initiates all connections; no inbound ports required
- **Scoped tokens:** Each agent token is scoped to a single connection and cannot access other connections or user data
- **Token rotation:** Tokens can be regenerated from the Sift UI at any time, immediately invalidating the old token
- **TLS:** All communication uses HTTPS

## Troubleshooting

### Agent shows "offline" in Sift

- Verify the agent process is running: `systemctl status sift-agent`
- Check the agent can reach Sift: `curl -s https://app.sift.works/health`
- Verify the token is correct (tokens are invalidated on regeneration)

### Queries time out

- Default query timeout is 300 seconds (5 minutes)
- Check your database server's own timeout settings
- Monitor agent logs for slow queries: `--log-level debug`

### Connection test fails

- Verify the DSN is correct by testing locally: `mysql -u user -p -h host database`
- Check that the database user has the necessary permissions
- Ensure the agent host can reach the database host on the specified port
