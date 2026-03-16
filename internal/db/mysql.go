package db

import (
	"context"
	"database/sql"
	"fmt"
)

// MySQLDriver implements the Driver interface for MySQL databases.
type MySQLDriver struct {
	db     *sql.DB
	dbName string
}

// NewMySQLDriver creates a new MySQLDriver, querying the current database name once.
func NewMySQLDriver(db *sql.DB) (*MySQLDriver, error) {
	var dbName string
	if err := db.QueryRow("SELECT DATABASE()").Scan(&dbName); err != nil {
		return nil, fmt.Errorf("getting database name: %w", err)
	}
	return &MySQLDriver{db: db, dbName: dbName}, nil
}

// GetTables returns all tables with their columns, primary keys, and foreign keys.
func (d *MySQLDriver) GetTables(ctx context.Context) ([]Table, error) {
	rows, err := d.db.QueryContext(ctx,
		"SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? AND TABLE_TYPE = 'BASE TABLE' ORDER BY TABLE_NAME",
		d.dbName,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tableNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		tableNames = append(tableNames, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var tables []Table
	for _, name := range tableNames {
		t, err := d.GetTableDetails(ctx, name)
		if err != nil {
			return nil, err
		}
		tables = append(tables, t)
	}

	return tables, nil
}

// GetTableDetails returns detailed column info for a specific table.
func (d *MySQLDriver) GetTableDetails(ctx context.Context, tableName string) (Table, error) {
	rows, err := d.db.QueryContext(ctx, `
		SELECT
			c.COLUMN_NAME,
			c.COLUMN_TYPE,
			IF(c.COLUMN_KEY = 'PRI', 1, 0) AS is_primary
		FROM information_schema.COLUMNS c
		WHERE c.TABLE_SCHEMA = ? AND c.TABLE_NAME = ?
		ORDER BY c.ORDINAL_POSITION
	`, d.dbName, tableName)
	if err != nil {
		return Table{}, err
	}
	defer rows.Close()

	var columns []Column
	for rows.Next() {
		var col Column
		var isPrimary int
		if err := rows.Scan(&col.Name, &col.Type, &isPrimary); err != nil {
			return Table{}, err
		}
		col.IsPrimary = isPrimary == 1
		columns = append(columns, col)
	}
	if err := rows.Err(); err != nil {
		return Table{}, err
	}

	// Get foreign keys
	fkRows, err := d.db.QueryContext(ctx, `
		SELECT
			COLUMN_NAME,
			REFERENCED_TABLE_NAME,
			REFERENCED_COLUMN_NAME
		FROM information_schema.KEY_COLUMN_USAGE
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND REFERENCED_TABLE_NAME IS NOT NULL
	`, d.dbName, tableName)
	if err != nil {
		return Table{}, err
	}
	defer fkRows.Close()

	fkMap := make(map[string]*ForeignKey)
	for fkRows.Next() {
		var colName, refTable, refColumn string
		if err := fkRows.Scan(&colName, &refTable, &refColumn); err != nil {
			return Table{}, err
		}
		fkMap[colName] = &ForeignKey{
			Table:  refTable,
			Column: refColumn,
		}
	}
	if err := fkRows.Err(); err != nil {
		return Table{}, err
	}

	for i := range columns {
		if fk, ok := fkMap[columns[i].Name]; ok {
			columns[i].IsForeign = true
			columns[i].ForeignKey = fk
		}
	}

	return Table{
		Name:    tableName,
		Columns: columns,
	}, nil
}

// GetRelations returns outgoing and incoming foreign key relations for a table.
func (d *MySQLDriver) GetRelations(ctx context.Context, tableName string) (Relations, error) {
	outRows, err := d.db.QueryContext(ctx, `
		SELECT
			TABLE_NAME,
			COLUMN_NAME,
			REFERENCED_TABLE_NAME,
			REFERENCED_COLUMN_NAME
		FROM information_schema.KEY_COLUMN_USAGE
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND REFERENCED_TABLE_NAME IS NOT NULL
	`, d.dbName, tableName)
	if err != nil {
		return Relations{}, err
	}
	defer outRows.Close()

	var outgoing []Relation
	for outRows.Next() {
		var r Relation
		if err := outRows.Scan(&r.SourceTable, &r.SourceColumn, &r.TargetTable, &r.TargetColumn); err != nil {
			return Relations{}, err
		}
		outgoing = append(outgoing, r)
	}
	if err := outRows.Err(); err != nil {
		return Relations{}, err
	}

	inRows, err := d.db.QueryContext(ctx, `
		SELECT
			TABLE_NAME,
			COLUMN_NAME,
			REFERENCED_TABLE_NAME,
			REFERENCED_COLUMN_NAME
		FROM information_schema.KEY_COLUMN_USAGE
		WHERE TABLE_SCHEMA = ? AND REFERENCED_TABLE_NAME = ? AND REFERENCED_TABLE_NAME IS NOT NULL
	`, d.dbName, tableName)
	if err != nil {
		return Relations{}, err
	}
	defer inRows.Close()

	var incoming []Relation
	for inRows.Next() {
		var r Relation
		if err := inRows.Scan(&r.SourceTable, &r.SourceColumn, &r.TargetTable, &r.TargetColumn); err != nil {
			return Relations{}, err
		}
		incoming = append(incoming, r)
	}
	if err := inRows.Err(); err != nil {
		return Relations{}, err
	}

	if outgoing == nil {
		outgoing = []Relation{}
	}
	if incoming == nil {
		incoming = []Relation{}
	}

	return Relations{
		Outgoing: outgoing,
		Incoming: incoming,
	}, nil
}
