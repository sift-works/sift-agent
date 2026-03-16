package db

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Table represents a database table with its columns.
type Table struct {
	Name    string   `json:"name"`
	Columns []Column `json:"columns"`
}

// Column represents a column in a database table.
type Column struct {
	Name       string      `json:"name"`
	Type       string      `json:"type"`
	IsPrimary  bool        `json:"isPrimary"`
	IsForeign  bool        `json:"isForeign"`
	ForeignKey *ForeignKey `json:"foreignKey"`
}

// ForeignKey represents a foreign key relationship.
type ForeignKey struct {
	Table  string `json:"table"`
	Column string `json:"column"`
}

// Relation represents a relationship between tables.
type Relation struct {
	SourceTable  string `json:"source_table"`
	SourceColumn string `json:"source_column"`
	TargetTable  string `json:"target_table"`
	TargetColumn string `json:"target_column"`
}

// Relations contains all relationships for a table.
type Relations struct {
	Outgoing []Relation `json:"outgoing"`
	Incoming []Relation `json:"incoming"`
}

// Driver defines the interface for database schema introspection.
type Driver interface {
	GetTables(ctx context.Context) ([]Table, error)
	GetTableDetails(ctx context.Context, tableName string) (Table, error)
	GetRelations(ctx context.Context, tableName string) (Relations, error)
}

// ParseDSN converts a mysql:// URL to go-sql-driver/mysql format.
func ParseDSN(dsn string) (driver string, connString string, err error) {
	if strings.HasPrefix(dsn, "mysql://") {
		driver = "mysql"
		u, err := url.Parse(dsn)
		if err != nil {
			return "", "", fmt.Errorf("invalid DSN: %w", err)
		}

		user := u.User.Username()
		pass, _ := u.User.Password()
		host := u.Hostname()
		port := u.Port()
		if port == "" {
			port = "3306"
		}
		dbName := strings.TrimPrefix(u.Path, "/")

		// go-sql-driver format: user:pass@tcp(host:port)/db?params
		connString = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true&multiStatements=true",
			user, pass, host, port, dbName)

		return driver, connString, nil
	}

	return "", "", fmt.Errorf("unsupported DSN scheme (only mysql:// is supported in v1)")
}

// Open creates a database connection pool from a DSN.
func Open(dsn string) (*sql.DB, Driver, error) {
	driverName, connString, err := ParseDSN(dsn)
	if err != nil {
		return nil, nil, err
	}

	db, err := sql.Open(driverName, connString)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open database: %w", err)
	}

	db.SetMaxOpenConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	var drv Driver
	switch driverName {
	case "mysql":
		d, err := NewMySQLDriver(db)
		if err != nil {
			db.Close()
			return nil, nil, fmt.Errorf("initializing mysql driver: %w", err)
		}
		drv = d
	default:
		db.Close()
		return nil, nil, fmt.Errorf("unsupported driver: %s", driverName)
	}

	return db, drv, nil
}
