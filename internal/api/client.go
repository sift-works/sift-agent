package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

var ErrUnauthorized = errors.New("unauthorized: invalid or expired token")

// CommandType identifies the type of command.
type CommandType string

const (
	CmdQuery              CommandType = "query"
	CmdTestConnection     CommandType = "test_connection"
	CmdSchemaTables       CommandType = "schema_tables"
	CmdSchemaTableDetails CommandType = "schema_table_details"
	CmdSchemaRelations    CommandType = "schema_relations"
)

// Command represents a command received from the Sift API.
type Command struct {
	ID      string          `json:"uuid"`
	Type    CommandType     `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

// CommandResult is the result submitted back to the API.
type CommandResult struct {
	Success      bool   `json:"success"`
	Result       any    `json:"result,omitempty"`
	ErrorMessage string `json:"error_message,omitempty"`
}

// HeartbeatRequest is sent to the heartbeat endpoint.
type HeartbeatRequest struct {
	Version           string `json:"version"`
	DatabaseReachable bool   `json:"database_reachable"`
}

// HeartbeatResponse is returned from the heartbeat endpoint.
type HeartbeatResponse struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// Client communicates with the Sift API.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
	logger     *slog.Logger
}

// NewClient creates a new API client.
func NewClient(baseURL, token string, logger *slog.Logger) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// PollCommand fetches the next available command. Returns nil if no command is available.
func (c *Client) PollCommand(ctx context.Context) (*Command, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/agent/commands", nil)
	if err != nil {
		return nil, fmt.Errorf("creating poll request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("polling commands: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var cmd Command
		if err := json.NewDecoder(resp.Body).Decode(&cmd); err != nil {
			return nil, fmt.Errorf("decoding command: %w", err)
		}
		return &cmd, nil
	case http.StatusNoContent:
		return nil, nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, ErrUnauthorized
	default:
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}
}

// SubmitResult sends a command result back to the API with retry logic.
func (c *Client) SubmitResult(ctx context.Context, commandID string, result CommandResult) error {
	body, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("marshaling result: %w", err)
	}

	delays := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second}

	var lastErr error
	for attempt := 0; attempt <= len(delays); attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(delays[attempt-1]):
			}
		}

		req, err := http.NewRequestWithContext(ctx, "POST",
			c.baseURL+"/api/agent/commands/"+commandID+"/result",
			bytes.NewReader(body),
		)
		if err != nil {
			return fmt.Errorf("creating submit request: %w", err)
		}
		c.setHeaders(req)

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("submitting result: %w", err)
			c.logger.Warn("submit result failed, retrying", "attempt", attempt+1, "error", err)
			continue
		}
		resp.Body.Close()

		switch resp.StatusCode {
		case http.StatusOK, http.StatusCreated, http.StatusNoContent:
			return nil
		case http.StatusConflict:
			// Command no longer active — don't retry
			c.logger.Warn("command no longer active", "command_id", commandID)
			return nil
		case http.StatusUnauthorized, http.StatusForbidden:
			return ErrUnauthorized
		default:
			lastErr = fmt.Errorf("unexpected status %d submitting result", resp.StatusCode)
			c.logger.Warn("submit result unexpected status, retrying", "attempt", attempt+1, "status", resp.StatusCode)
			continue
		}
	}

	return lastErr
}

// Heartbeat sends a heartbeat to the API.
func (c *Client) Heartbeat(ctx context.Context, hb HeartbeatRequest) (*HeartbeatResponse, error) {
	body, err := json.Marshal(hb)
	if err != nil {
		return nil, fmt.Errorf("marshaling heartbeat: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/agent/heartbeat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("creating heartbeat request: %w", err)
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sending heartbeat: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var hbResp HeartbeatResponse
		if err := json.NewDecoder(resp.Body).Decode(&hbResp); err != nil {
			return nil, fmt.Errorf("decoding heartbeat response: %w", err)
		}
		return &hbResp, nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, ErrUnauthorized
	default:
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(respBody))
	}
}

func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
}
