package catalog

import (
	"testing"
)

// ---------------------------------------------------------------------------
// 2.4 Function/Procedure Changes
// ---------------------------------------------------------------------------

func TestOracleFunc(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping oracle test: requires Docker")
	}
	oracle := startPGOracle(t)

	// Base function used by most tests:
	// CREATE FUNCTION calc(a integer, b integer) RETURNS integer
	//     LANGUAGE sql VOLATILE AS 'SELECT a + b';

	t.Run("change_function_body", func(t *testing.T) {
		before := `CREATE FUNCTION calc(a integer, b integer) RETURNS integer
    LANGUAGE sql VOLATILE AS 'SELECT a + b';`
		after := `CREATE FUNCTION calc(a integer, b integer) RETURNS integer
    LANGUAGE sql VOLATILE AS 'SELECT a * b';`
		assertOracleRoundtrip(t, oracle, before, after)
	})

	t.Run("change_function_volatility", func(t *testing.T) {
		before := `CREATE FUNCTION calc(a integer, b integer) RETURNS integer
    LANGUAGE sql VOLATILE AS 'SELECT a + b';`
		after := `CREATE FUNCTION calc(a integer, b integer) RETURNS integer
    LANGUAGE sql STABLE AS 'SELECT a + b';`
		assertOracleRoundtrip(t, oracle, before, after)
	})

	t.Run("change_function_strict", func(t *testing.T) {
		before := `CREATE FUNCTION calc(a integer, b integer) RETURNS integer
    LANGUAGE sql VOLATILE AS 'SELECT a + b';`
		after := `CREATE FUNCTION calc(a integer, b integer) RETURNS integer
    LANGUAGE sql VOLATILE STRICT AS 'SELECT a + b';`
		assertOracleRoundtrip(t, oracle, before, after)
	})

	t.Run("change_function_return_type", func(t *testing.T) {
		// Changing return type requires DROP+CREATE
		before := `CREATE FUNCTION calc(a integer, b integer) RETURNS integer
    LANGUAGE sql VOLATILE AS 'SELECT a + b';`
		after := `CREATE FUNCTION calc(a integer, b integer) RETURNS bigint
    LANGUAGE sql VOLATILE AS 'SELECT (a + b)::bigint';`
		assertOracleRoundtrip(t, oracle, before, after)
	})

	t.Run("add_function_parameter", func(t *testing.T) {
		// Adding a parameter requires DROP+CREATE
		before := `CREATE FUNCTION calc(a integer, b integer) RETURNS integer
    LANGUAGE sql VOLATILE AS 'SELECT a + b';`
		after := `CREATE FUNCTION calc(a integer, b integer, c integer) RETURNS integer
    LANGUAGE sql VOLATILE AS 'SELECT a + b + c';`
		assertOracleRoundtrip(t, oracle, before, after)
	})

	t.Run("change_procedure_body", func(t *testing.T) {
		before := `CREATE TABLE log_tbl (msg text);
CREATE PROCEDURE do_log()
    LANGUAGE sql AS 'INSERT INTO log_tbl VALUES (''hello'')';`
		after := `CREATE TABLE log_tbl (msg text);
CREATE PROCEDURE do_log()
    LANGUAGE sql AS 'INSERT INTO log_tbl VALUES (''goodbye'')';`
		assertOracleRoundtrip(t, oracle, before, after)
	})

	t.Run("add_function_comment", func(t *testing.T) {
		before := `CREATE FUNCTION calc(a integer, b integer) RETURNS integer
    LANGUAGE sql VOLATILE AS 'SELECT a + b';`
		after := `CREATE FUNCTION calc(a integer, b integer) RETURNS integer
    LANGUAGE sql VOLATILE AS 'SELECT a + b';
COMMENT ON FUNCTION calc(integer, integer) IS 'Add two numbers';`
		assertOracleRoundtrip(t, oracle, before, after)
	})

	t.Run("add_procedure_comment", func(t *testing.T) {
		before := `CREATE TABLE log_tbl (msg text);
CREATE PROCEDURE do_log()
    LANGUAGE sql AS 'INSERT INTO log_tbl VALUES (''hello'')';`
		after := `CREATE TABLE log_tbl (msg text);
CREATE PROCEDURE do_log()
    LANGUAGE sql AS 'INSERT INTO log_tbl VALUES (''hello'')';
COMMENT ON PROCEDURE do_log() IS 'Logs a message';`
		assertOracleRoundtrip(t, oracle, before, after)
	})
}

// ---------------------------------------------------------------------------
// 2.4b Function Parameter Gaps
// ---------------------------------------------------------------------------

func TestOracleFuncParams(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping oracle test: requires Docker")
	}
	oracle := startPGOracle(t)

	// --- Function with OUT parameter ---
	t.Run("function_with_out_param", func(t *testing.T) {
		before := ``
		after := `CREATE FUNCTION f_out(IN x integer, OUT y integer) RETURNS integer LANGUAGE sql AS 'SELECT x * 2';`
		assertOracleRoundtrip(t, oracle, before, after)
	})

	// --- Function with INOUT parameter ---
	t.Run("function_with_inout_param", func(t *testing.T) {
		before := ``
		after := `CREATE FUNCTION f_inout(INOUT x integer) LANGUAGE plpgsql AS 'BEGIN x := x * 2; END';`
		assertOracleRoundtrip(t, oracle, before, after)
	})

	// --- Function with VARIADIC parameter ---
	t.Run("function_with_variadic_param", func(t *testing.T) {
		before := ``
		after := `CREATE FUNCTION f_variadic(VARIADIC args integer[]) RETURNS integer LANGUAGE sql AS 'SELECT array_length(args, 1)';`
		assertOracleRoundtrip(t, oracle, before, after)
	})

	// --- DROP function with OUT params (modify function that has OUT) ---
	t.Run("drop_function_with_out_params", func(t *testing.T) {
		before := `CREATE FUNCTION f_out(IN x integer, OUT y integer) RETURNS integer LANGUAGE sql AS 'SELECT x * 2';`
		after := `CREATE FUNCTION f_out(IN x integer, OUT y integer) RETURNS integer LANGUAGE sql AS 'SELECT x * 3';`
		assertOracleRoundtrip(t, oracle, before, after)
	})
}

// ---------------------------------------------------------------------------
// 2.5 View/MatView Changes
// ---------------------------------------------------------------------------

func TestOracleView(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping oracle test: requires Docker")
	}
	oracle := startPGOracle(t)

	// Base:
	// CREATE TABLE t1 (id int, name text, active boolean);
	// CREATE VIEW v1 AS SELECT id, name FROM t1 WHERE active;

	t.Run("change_view_definition", func(t *testing.T) {
		before := `CREATE TABLE t1 (id int, name text, active boolean);
CREATE VIEW v1 AS SELECT id, name FROM t1 WHERE active;`
		after := `CREATE TABLE t1 (id int, name text, active boolean);
CREATE VIEW v1 AS SELECT id, name FROM t1 WHERE active = true;`
		assertOracleRoundtrip(t, oracle, before, after)
	})

	t.Run("add_view_comment", func(t *testing.T) {
		before := `CREATE TABLE t1 (id int, name text, active boolean);
CREATE VIEW v1 AS SELECT id, name FROM t1 WHERE active;`
		after := `CREATE TABLE t1 (id int, name text, active boolean);
CREATE VIEW v1 AS SELECT id, name FROM t1 WHERE active;
COMMENT ON VIEW v1 IS 'Active items only';`
		assertOracleRoundtrip(t, oracle, before, after)
	})

	t.Run("change_matview_definition", func(t *testing.T) {
		// Changing matview definition requires DROP+CREATE
		before := `CREATE TABLE t1 (id int, name text, active boolean);
CREATE MATERIALIZED VIEW mv1 AS SELECT id, name FROM t1 WHERE active WITH DATA;`
		after := `CREATE TABLE t1 (id int, name text, active boolean);
CREATE MATERIALIZED VIEW mv1 AS SELECT id, name, active FROM t1 WITH DATA;`
		assertOracleRoundtrip(t, oracle, before, after)
	})

	t.Run("add_matview_comment", func(t *testing.T) {
		before := `CREATE TABLE t1 (id int, name text, active boolean);
CREATE MATERIALIZED VIEW mv1 AS SELECT id, name FROM t1 WITH DATA;`
		after := `CREATE TABLE t1 (id int, name text, active boolean);
CREATE MATERIALIZED VIEW mv1 AS SELECT id, name FROM t1 WITH DATA;
COMMENT ON MATERIALIZED VIEW mv1 IS 'Cached names';`
		assertOracleRoundtrip(t, oracle, before, after)
	})

	t.Run("add_matview_index", func(t *testing.T) {
		before := `CREATE TABLE t1 (id int, name text, active boolean);
CREATE MATERIALIZED VIEW mv1 AS SELECT id, name FROM t1 WITH DATA;`
		after := `CREATE TABLE t1 (id int, name text, active boolean);
CREATE MATERIALIZED VIEW mv1 AS SELECT id, name FROM t1 WITH DATA;
CREATE INDEX idx_mv1_id ON mv1(id);`
		assertOracleRoundtrip(t, oracle, before, after)
	})

	t.Run("change_view_with_check_option", func(t *testing.T) {
		before := `CREATE TABLE t1 (id int, name text, active boolean);
CREATE VIEW v1 AS SELECT id, name FROM t1 WHERE active;`
		after := `CREATE TABLE t1 (id int, name text, active boolean);
CREATE VIEW v1 AS SELECT id, name FROM t1 WHERE active
    WITH CASCADED CHECK OPTION;`
		assertOracleRoundtrip(t, oracle, before, after)
	})
}
