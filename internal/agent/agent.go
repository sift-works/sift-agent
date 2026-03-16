package agent

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/sift-works/agent/internal/api"
	"github.com/sift-works/agent/internal/config"
	"github.com/sift-works/agent/internal/executor"
)

// DBPinger is satisfied by *sql.DB and allows testing.
type DBPinger interface {
	PingContext(ctx context.Context) error
}

// Agent is the main agent that polls for commands and sends heartbeats.
type Agent struct {
	cfg           config.Config
	client        *api.Client
	executor      *executor.Executor
	db            DBPinger
	logger        *slog.Logger
	firstBeatDone bool
}

// New creates a new Agent.
func New(cfg config.Config, client *api.Client, exec *executor.Executor, db DBPinger, logger *slog.Logger) *Agent {
	return &Agent{
		cfg:      cfg,
		client:   client,
		executor: exec,
		db:       db,
		logger:   logger,
	}
}

// Run starts the poll and heartbeat loops and blocks until the context is cancelled.
func (a *Agent) Run(ctx context.Context) error {
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		a.pollLoop(ctx)
	}()
	go func() {
		defer wg.Done()
		a.heartbeatLoop(ctx)
	}()

	wg.Wait()
	return nil
}

func (a *Agent) pollLoop(ctx context.Context) {
	var commandWg sync.WaitGroup
	defer func() {
		// Wait for in-flight commands with a hard timeout
		done := make(chan struct{})
		go func() {
			commandWg.Wait()
			close(done)
		}()
		select {
		case <-done:
			a.logger.Info("all in-flight commands completed")
		case <-time.After(30 * time.Second):
			a.logger.Warn("timed out waiting for in-flight commands")
		}
	}()

	backoff := newBackoff(1*time.Second, 60*time.Second)

	for {
		select {
		case <-ctx.Done():
			a.logger.Info("poll loop stopping")
			return
		default:
		}

		cmd, err := a.client.PollCommand(ctx)
		if err != nil {
			if errors.Is(err, api.ErrUnauthorized) {
				a.logger.Error("authentication failed — check your token", "error", err)
				return
			}
			if ctx.Err() != nil {
				return
			}
			delay := backoff.next()
			a.logger.Error("poll failed, backing off", "error", err, "retry_in", delay)
			select {
			case <-ctx.Done():
				return
			case <-time.After(delay):
			}
			continue
		}

		backoff.reset()

		if cmd == nil {
			// No command available — wait before next poll
			select {
			case <-ctx.Done():
				return
			case <-time.After(a.cfg.PollInterval):
			}
			continue
		}

		a.logger.Info("command received", "type", cmd.Type, "id", cmd.ID)

		commandWg.Add(1)
		go func(c *api.Command) {
			defer commandWg.Done()
			a.handleCommand(ctx, c)
		}(cmd)
	}
}

func (a *Agent) handleCommand(ctx context.Context, cmd *api.Command) {
	start := time.Now()
	result := a.executor.Execute(ctx, cmd)
	duration := time.Since(start)

	a.logger.Info("command executed",
		"type", cmd.Type,
		"id", cmd.ID,
		"success", result.Success,
		"duration", duration,
	)

	if err := a.client.SubmitResult(ctx, cmd.ID, result); err != nil {
		if errors.Is(err, api.ErrUnauthorized) {
			a.logger.Error("authentication failed submitting result", "error", err)
			return
		}
		a.logger.Error("failed to submit result", "command_id", cmd.ID, "error", err)
	}
}

func (a *Agent) heartbeatLoop(ctx context.Context) {
	ticker := time.NewTicker(a.cfg.HeartbeatInterval)
	defer ticker.Stop()

	// Send first heartbeat immediately
	a.sendHeartbeat(ctx)

	for {
		select {
		case <-ctx.Done():
			a.logger.Info("heartbeat loop stopping")
			return
		case <-ticker.C:
			a.sendHeartbeat(ctx)
		}
	}
}

func (a *Agent) sendHeartbeat(ctx context.Context) {
	dbReachable := a.db.PingContext(ctx) == nil

	resp, err := a.client.Heartbeat(ctx, api.HeartbeatRequest{
		Version:           config.Version,
		DatabaseReachable: dbReachable,
	})
	if err != nil {
		if errors.Is(err, api.ErrUnauthorized) {
			a.logger.Error("authentication failed during heartbeat", "error", err)
			return
		}
		if ctx.Err() != nil {
			return
		}
		a.logger.Error("heartbeat failed", "error", err)
		return
	}

	if !a.firstBeatDone {
		a.logger.Info("connected to Sift", "connection_name", resp.Name, "type", resp.Type)
		a.firstBeatDone = true
	}

	a.logger.Debug("heartbeat sent", "database_reachable", dbReachable)
}

// backoff implements exponential backoff with jitter.
type backoff struct {
	base    time.Duration
	max     time.Duration
	attempt int
}

func newBackoff(base, max time.Duration) *backoff {
	return &backoff{base: base, max: max}
}

func (b *backoff) next() time.Duration {
	delay := time.Duration(float64(b.base) * math.Pow(2, float64(b.attempt)))
	if delay > b.max {
		delay = b.max
	}
	b.attempt++

	// Add 25% jitter
	jitter := time.Duration(float64(delay) * 0.25 * rand.Float64())
	return delay + jitter
}

func (b *backoff) reset() {
	b.attempt = 0
}
