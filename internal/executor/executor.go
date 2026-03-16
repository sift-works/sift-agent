package executor

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/sift-works/agent/internal/api"
	"github.com/sift-works/agent/internal/db"
)

// Executor routes commands to their handlers and executes them.
type Executor struct {
	db     *sql.DB
	driver db.Driver
}

// New creates a new Executor.
func New(database *sql.DB, driver db.Driver) *Executor {
	return &Executor{
		db:     database,
		driver: driver,
	}
}

// Execute processes a command and returns a result.
func (e *Executor) Execute(ctx context.Context, cmd *api.Command) api.CommandResult {
	// 5-minute query timeout
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	switch cmd.Type {
	case api.CmdQuery:
		return e.executeQuery(ctx, cmd.Payload)
	case api.CmdTestConnection:
		return e.testConnection(ctx)
	case api.CmdSchemaTables:
		return e.schemaTables(ctx)
	case api.CmdSchemaTableDetails:
		return e.schemaTableDetails(ctx, cmd.Payload)
	case api.CmdSchemaRelations:
		return e.schemaRelations(ctx, cmd.Payload)
	default:
		return api.CommandResult{
			Success:      false,
			ErrorMessage: fmt.Sprintf("unknown command type: %s", cmd.Type),
		}
	}
}

func (e *Executor) executeQuery(ctx context.Context, payload json.RawMessage) api.CommandResult {
	var p struct {
		SQL string `json:"sql"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return api.CommandResult{Success: false, ErrorMessage: fmt.Sprintf("invalid payload: %v", err)}
	}

	start := time.Now()

	rows, err := e.db.QueryContext(ctx, p.SQL)
	if err != nil {
		return api.CommandResult{Success: false, ErrorMessage: err.Error()}
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return api.CommandResult{Success: false, ErrorMessage: fmt.Sprintf("getting columns: %v", err)}
	}

	var resultRows [][]any
	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return api.CommandResult{Success: false, ErrorMessage: fmt.Sprintf("scanning row: %v", err)}
		}

		// Convert []byte values to strings for JSON serialization
		row := make([]any, len(columns))
		for i, v := range values {
			if b, ok := v.([]byte); ok {
				row[i] = string(b)
			} else {
				row[i] = v
			}
		}
		resultRows = append(resultRows, row)
	}
	if err := rows.Err(); err != nil {
		return api.CommandResult{Success: false, ErrorMessage: fmt.Sprintf("iterating rows: %v", err)}
	}

	duration := time.Since(start).Seconds()

	if resultRows == nil {
		resultRows = [][]any{}
	}

	return api.CommandResult{
		Success: true,
		Result: map[string]any{
			"columns":   columns,
			"rows":      resultRows,
			"row_count": len(resultRows),
			"duration":  duration,
		},
	}
}

func (e *Executor) testConnection(ctx context.Context) api.CommandResult {
	if err := e.db.PingContext(ctx); err != nil {
		return api.CommandResult{Success: false, ErrorMessage: err.Error()}
	}
	return api.CommandResult{
		Success: true,
		Result:  map[string]any{"success": true},
	}
}

func (e *Executor) schemaTables(ctx context.Context) api.CommandResult {
	tables, err := e.driver.GetTables(ctx)
	if err != nil {
		return api.CommandResult{Success: false, ErrorMessage: err.Error()}
	}
	if tables == nil {
		tables = []db.Table{}
	}
	return api.CommandResult{
		Success: true,
		Result:  map[string]any{"tables": tables},
	}
}

func (e *Executor) schemaTableDetails(ctx context.Context, payload json.RawMessage) api.CommandResult {
	var p struct {
		TableName string `json:"table_name"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return api.CommandResult{Success: false, ErrorMessage: fmt.Sprintf("invalid payload: %v", err)}
	}

	details, err := e.driver.GetTableDetails(ctx, p.TableName)
	if err != nil {
		return api.CommandResult{Success: false, ErrorMessage: err.Error()}
	}

	return api.CommandResult{
		Success: true,
		Result:  details,
	}
}

func (e *Executor) schemaRelations(ctx context.Context, payload json.RawMessage) api.CommandResult {
	var p struct {
		TableName string `json:"table_name"`
	}
	if err := json.Unmarshal(payload, &p); err != nil {
		return api.CommandResult{Success: false, ErrorMessage: fmt.Sprintf("invalid payload: %v", err)}
	}

	relations, err := e.driver.GetRelations(ctx, p.TableName)
	if err != nil {
		return api.CommandResult{Success: false, ErrorMessage: err.Error()}
	}

	return api.CommandResult{
		Success: true,
		Result:  relations,
	}
}
