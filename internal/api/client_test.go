package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"net/http/httptest"
	"testing"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, nil))
}

func TestPollCommand_OK(t *testing.T) {
	cmd := Command{
		ID:      "abc-123",
		Type:    "query",
		Payload: json.RawMessage(`{"sql":"SELECT 1"}`),
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/agent/commands" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("unexpected auth header: %s", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(cmd)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", testLogger())
	got, err := client.PollCommand(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got == nil {
		t.Fatal("expected command, got nil")
	}
	if got.ID != "abc-123" {
		t.Errorf("id = %q, want %q", got.ID, "abc-123")
	}
	if got.Type != "query" {
		t.Errorf("type = %q, want %q", got.Type, "query")
	}
}

func TestPollCommand_NoContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", testLogger())
	got, err := client.PollCommand(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Fatalf("expected nil, got %+v", got)
	}
}

func TestPollCommand_Unauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	client := NewClient(server.URL, "bad-token", testLogger())
	_, err := client.PollCommand(context.Background())
	if err != ErrUnauthorized {
		t.Errorf("expected ErrUnauthorized, got %v", err)
	}
}

func TestSubmitResult_OK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/agent/commands/abc-123/result" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != "POST" {
			t.Errorf("unexpected method: %s", r.Method)
		}

		var result CommandResult
		json.NewDecoder(r.Body).Decode(&result)
		if !result.Success {
			t.Error("expected success=true")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", testLogger())
	err := client.SubmitResult(context.Background(), "abc-123", CommandResult{
		Success: true,
		Result:  map[string]interface{}{"columns": []string{"id"}, "rows": [][]interface{}{{1}}, "row_count": 1, "duration": 0.01},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSubmitResult_Conflict(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusConflict)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", testLogger())
	err := client.SubmitResult(context.Background(), "abc-123", CommandResult{Success: true})
	if err != nil {
		t.Fatalf("conflict should not return error, got: %v", err)
	}
}

func TestHeartbeat_OK(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/agent/heartbeat" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}

		var req HeartbeatRequest
		json.NewDecoder(r.Body).Decode(&req)
		if !req.DatabaseReachable {
			t.Error("expected database_reachable=true")
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(HeartbeatResponse{Name: "test-db", Type: "mysql"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-token", testLogger())
	resp, err := client.Heartbeat(context.Background(), HeartbeatRequest{
		Version:           "dev",
		DatabaseReachable: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Name != "test-db" {
		t.Errorf("name = %q, want %q", resp.Name, "test-db")
	}
}
