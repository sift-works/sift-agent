package db

import (
	"testing"
)

func TestParseDSN_MySQL(t *testing.T) {
	tests := []struct {
		name       string
		dsn        string
		wantDriver string
		wantConn   string
		wantErr    bool
	}{
		{
			name:       "full DSN",
			dsn:        "mysql://user:pass@localhost:3306/mydb",
			wantDriver: "mysql",
			wantConn:   "user:pass@tcp(localhost:3306)/mydb?parseTime=true&multiStatements=true",
		},
		{
			name:       "default port",
			dsn:        "mysql://user:pass@localhost/mydb",
			wantDriver: "mysql",
			wantConn:   "user:pass@tcp(localhost:3306)/mydb?parseTime=true&multiStatements=true",
		},
		{
			name:       "no password",
			dsn:        "mysql://root@localhost:3306/testdb",
			wantDriver: "mysql",
			wantConn:   "root:@tcp(localhost:3306)/testdb?parseTime=true&multiStatements=true",
		},
		{
			name:       "with special chars in password",
			dsn:        "mysql://user:p%40ss@localhost:3306/db",
			wantDriver: "mysql",
			wantConn:   "user:p@ss@tcp(localhost:3306)/db?parseTime=true&multiStatements=true",
		},
		{
			name:    "unsupported scheme",
			dsn:     "postgresql://user:pass@localhost/db",
			wantErr: true,
		},
		{
			name:    "no scheme",
			dsn:     "user:pass@localhost/db",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driver, conn, err := ParseDSN(tt.dsn)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if driver != tt.wantDriver {
				t.Errorf("driver = %q, want %q", driver, tt.wantDriver)
			}
			if conn != tt.wantConn {
				t.Errorf("conn = %q, want %q", conn, tt.wantConn)
			}
		})
	}
}
