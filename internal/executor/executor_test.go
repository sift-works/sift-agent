package executor

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/sift-works/agent/internal/api"
	"github.com/sift-works/agent/internal/db"
)

type mockDriver struct {
	tables []db.Table
	detail db.Table
	rels   db.Relations
	err    error
}

func (m *mockDriver) GetTables(ctx context.Context) ([]db.Table, error) {
	return m.tables, m.err
}

func (m *mockDriver) GetTableDetails(ctx context.Context, tableName string) (db.Table, error) {
	return m.detail, m.err
}

func (m *mockDriver) GetRelations(ctx context.Context, tableName string) (db.Relations, error) {
	return m.rels, m.err
}

func TestExecute_UnknownCommand(t *testing.T) {
	exec := New(nil, &mockDriver{})
	result := exec.Execute(context.Background(), &api.Command{
		ID:      "test",
		Type:    "unknown_type",
		Payload: json.RawMessage(`{}`),
	})
	if result.Success {
		t.Error("expected failure for unknown command type")
	}
	if result.ErrorMessage == "" {
		t.Error("expected error message")
	}
}

func TestExecute_SchemaTables(t *testing.T) {
	driver := &mockDriver{
		tables: []db.Table{
			{Name: "users", Columns: []db.Column{{Name: "id", Type: "int", IsPrimary: true}}},
		},
	}
	exec := New(nil, driver)
	result := exec.Execute(context.Background(), &api.Command{
		ID:      "test",
		Type:    api.CmdSchemaTables,
		Payload: json.RawMessage(`{}`),
	})
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.ErrorMessage)
	}

	resultMap, ok := result.Result.(map[string]any)
	if !ok {
		t.Fatal("result is not a map")
	}
	tables, ok := resultMap["tables"].([]db.Table)
	if !ok {
		t.Fatal("tables is not []db.Table")
	}
	if len(tables) != 1 || tables[0].Name != "users" {
		t.Errorf("unexpected tables: %+v", tables)
	}
}

func TestExecute_SchemaTableDetails(t *testing.T) {
	driver := &mockDriver{
		detail: db.Table{
			Name: "users",
			Columns: []db.Column{
				{Name: "id", Type: "int", IsPrimary: true},
				{Name: "name", Type: "varchar(255)"},
			},
		},
	}
	exec := New(nil, driver)
	result := exec.Execute(context.Background(), &api.Command{
		ID:      "test",
		Type:    api.CmdSchemaTableDetails,
		Payload: json.RawMessage(`{"table_name":"users"}`),
	})
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.ErrorMessage)
	}

	details, ok := result.Result.(db.Table)
	if !ok {
		t.Fatal("result is not Table")
	}
	if details.Name != "users" || len(details.Columns) != 2 {
		t.Errorf("unexpected details: %+v", details)
	}
}

func TestExecute_SchemaRelations(t *testing.T) {
	driver := &mockDriver{
		rels: db.Relations{
			Outgoing: []db.Relation{{SourceTable: "posts", SourceColumn: "user_id", TargetTable: "users", TargetColumn: "id"}},
			Incoming: []db.Relation{},
		},
	}
	exec := New(nil, driver)
	result := exec.Execute(context.Background(), &api.Command{
		ID:      "test",
		Type:    api.CmdSchemaRelations,
		Payload: json.RawMessage(`{"table_name":"posts"}`),
	})
	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.ErrorMessage)
	}

	rels, ok := result.Result.(db.Relations)
	if !ok {
		t.Fatal("result is not Relations")
	}
	if len(rels.Outgoing) != 1 {
		t.Errorf("expected 1 outgoing relation, got %d", len(rels.Outgoing))
	}
}
