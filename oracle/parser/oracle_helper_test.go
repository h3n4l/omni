package parser

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/sijms/go-ora/v2"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// oracleDB wraps a real Oracle Database container connection for cross-validation testing.
type oracleDB struct {
	db  *sql.DB
	ctx context.Context
}

var (
	oracleOnce    sync.Once
	oracleInst    *oracleDB
	oracleInitErr error
)

// startOracleDB starts a shared Oracle Database 23ai Free container. The container
// is reused across all tests via sync.Once. Returns nil and skips the test if
// Docker is unavailable or the container fails to start.
func startOracleDB(t *testing.T) *oracleDB {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping Oracle DB test in short mode")
	}

	oracleOnce.Do(func() {
		ctx := context.Background()

		req := testcontainers.ContainerRequest{
			Image:        "gvenzl/oracle-free:23-slim-faststart",
			ExposedPorts: []string{"1521/tcp"},
			Env: map[string]string{
				"ORACLE_PASSWORD": "testpass",
				"APP_USER":        "testuser",
				"APP_USER_PASSWORD": "testpass",
			},
			WaitingFor: wait.ForLog("DATABASE IS READY TO USE!").
				WithStartupTimeout(5 * time.Minute),
		}

		container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		})
		if err != nil {
			oracleInitErr = fmt.Errorf("failed to start Oracle container: %w", err)
			return
		}

		host, err := container.Host(ctx)
		if err != nil {
			_ = testcontainers.TerminateContainer(container)
			oracleInitErr = fmt.Errorf("failed to get container host: %w", err)
			return
		}

		mappedPort, err := container.MappedPort(ctx, "1521")
		if err != nil {
			_ = testcontainers.TerminateContainer(container)
			oracleInitErr = fmt.Errorf("failed to get mapped port: %w", err)
			return
		}

		// go-ora connection string format: oracle://user:pass@host:port/service_name
		connStr := fmt.Sprintf("oracle://testuser:testpass@%s:%s/FREEPDB1", host, mappedPort.Port())

		db, err := sql.Open("oracle", connStr)
		if err != nil {
			_ = testcontainers.TerminateContainer(container)
			oracleInitErr = fmt.Errorf("failed to open database: %w", err)
			return
		}

		if err := db.PingContext(ctx); err != nil {
			db.Close()
			_ = testcontainers.TerminateContainer(container)
			oracleInitErr = fmt.Errorf("failed to ping Oracle database: %w", err)
			return
		}

		oracleInst = &oracleDB{db: db, ctx: ctx}
		// Container cleanup happens when the process exits.
	})

	if oracleInitErr != nil {
		t.Skipf("Oracle container not available: %v", oracleInitErr)
	}
	return oracleInst
}

// canExecute checks whether Oracle DB accepts the given SQL statement.
// It uses EXPLAIN PLAN to validate the syntax without executing the statement.
// For DDL statements that cannot be explained, it uses DBMS_SQL.PARSE()
// to validate syntax without execution, avoiding side effects.
//
// NOTE: For DDL that must actually be executed (e.g., Stage 6 catalog tests),
// use execSQL() with explicit cleanup instead.
func (o *oracleDB) canExecute(sqlStr string) error {
	// Try EXPLAIN PLAN first — works for DML (SELECT, INSERT, UPDATE, DELETE, MERGE).
	explainSQL := fmt.Sprintf("EXPLAIN PLAN FOR %s", sqlStr)
	_, err := o.db.ExecContext(o.ctx, explainSQL)
	if err == nil {
		return nil
	}

	// For DDL and other statements that EXPLAIN PLAN doesn't support,
	// use DBMS_SQL.PARSE to validate syntax without executing.
	// DBMS_SQL.PARSE parses the statement and raises an exception on syntax errors,
	// but does not execute it — no side effects on the database state.
	parseCheck := fmt.Sprintf(`DECLARE
  c INTEGER := DBMS_SQL.OPEN_CURSOR;
BEGIN
  DBMS_SQL.PARSE(c, '%s', DBMS_SQL.NATIVE);
  DBMS_SQL.CLOSE_CURSOR(c);
EXCEPTION
  WHEN OTHERS THEN
    DBMS_SQL.CLOSE_CURSOR(c);
    RAISE;
END;`, strings.ReplaceAll(sqlStr, "'", "''"))
	_, parseErr := o.db.ExecContext(o.ctx, parseCheck)
	return parseErr
}

// execSQL executes a SQL statement on the Oracle container, failing the test on error.
func (o *oracleDB) execSQL(t *testing.T, sqlStr string) {
	t.Helper()
	_, err := o.db.ExecContext(o.ctx, sqlStr)
	if err != nil {
		t.Fatalf("Oracle exec failed: %v\nSQL: %s", err, truncateOracleSQL(sqlStr, 500))
	}
}

// truncateOracleSQL truncates a SQL string for error messages.
func truncateOracleSQL(sqlStr string, maxLen int) string {
	if len(sqlStr) <= maxLen {
		return sqlStr
	}
	return sqlStr[:maxLen] + "..."
}
