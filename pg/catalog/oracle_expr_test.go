package catalog

import (
	"testing"
)

// ---------------------------------------------------------------------------
// Phase 1: Expression Types in Column DEFAULT
// ---------------------------------------------------------------------------

func TestOracleExprDefault_ValueExpressions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping oracle test: requires Docker")
	}
	oracle := startPGOracle(t)

	// 1.1 Value Expressions in DEFAULT

	t.Run("CURRENT_TIMESTAMP", func(t *testing.T) {
		assertOracleRoundtrip(t, oracle,
			`CREATE TABLE t (id int PRIMARY KEY);`,
			`CREATE TABLE t (id int PRIMARY KEY, ts timestamptz DEFAULT CURRENT_TIMESTAMP);`,
		)
	})

	t.Run("CURRENT_DATE", func(t *testing.T) {
		assertOracleRoundtrip(t, oracle,
			`CREATE TABLE t (id int PRIMARY KEY);`,
			`CREATE TABLE t (id int PRIMARY KEY, d date DEFAULT CURRENT_DATE);`,
		)
	})

	t.Run("CURRENT_USER", func(t *testing.T) {
		assertOracleRoundtrip(t, oracle,
			`CREATE TABLE t (id int PRIMARY KEY);`,
			`CREATE TABLE t (id int PRIMARY KEY, u text DEFAULT CURRENT_USER);`,
		)
	})

	t.Run("LOCALTIME", func(t *testing.T) {
		assertOracleRoundtrip(t, oracle,
			`CREATE TABLE t (id int PRIMARY KEY);`,
			`CREATE TABLE t (id int PRIMARY KEY, lt time DEFAULT LOCALTIME);`,
		)
	})

	t.Run("LOCALTIMESTAMP", func(t *testing.T) {
		assertOracleRoundtrip(t, oracle,
			`CREATE TABLE t (id int PRIMARY KEY);`,
			`CREATE TABLE t (id int PRIMARY KEY, lt timestamp DEFAULT LOCALTIMESTAMP);`,
		)
	})

	t.Run("SESSION_USER", func(t *testing.T) {
		assertOracleRoundtrip(t, oracle,
			`CREATE TABLE t (id int PRIMARY KEY);`,
			`CREATE TABLE t (id int PRIMARY KEY, u text DEFAULT SESSION_USER);`,
		)
	})
}

func TestOracleExprDefault_FunctionLikeExpressions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping oracle test: requires Docker")
	}
	oracle := startPGOracle(t)

	// 1.2 Function-Like Expressions in DEFAULT

	t.Run("COALESCE", func(t *testing.T) {
		assertOracleRoundtrip(t, oracle,
			`CREATE TABLE t (id int PRIMARY KEY);`,
			`CREATE TABLE t (id int PRIMARY KEY, val int DEFAULT COALESCE(NULL, 42));`,
		)
	})

	t.Run("NULLIF", func(t *testing.T) {
		assertOracleRoundtrip(t, oracle,
			`CREATE TABLE t (id int PRIMARY KEY);`,
			`CREATE TABLE t (id int PRIMARY KEY, val int DEFAULT NULLIF(0, 0));`,
		)
	})

	t.Run("GREATEST", func(t *testing.T) {
		assertOracleRoundtrip(t, oracle,
			`CREATE TABLE t (id int PRIMARY KEY);`,
			`CREATE TABLE t (id int PRIMARY KEY, val int DEFAULT GREATEST(1, 2));`,
		)
	})

	t.Run("LEAST", func(t *testing.T) {
		assertOracleRoundtrip(t, oracle,
			`CREATE TABLE t (id int PRIMARY KEY);`,
			`CREATE TABLE t (id int PRIMARY KEY, val int DEFAULT LEAST(1, 2));`,
		)
	})

	t.Run("CASE_WHEN", func(t *testing.T) {
		assertOracleRoundtrip(t, oracle,
			`CREATE TABLE t (id int PRIMARY KEY);`,
			`CREATE TABLE t (id int PRIMARY KEY, val text DEFAULT CASE WHEN true THEN 'yes' ELSE 'no' END);`,
		)
	})

	t.Run("now_function", func(t *testing.T) {
		assertOracleRoundtrip(t, oracle,
			`CREATE TABLE t (id int PRIMARY KEY);`,
			`CREATE TABLE t (id int PRIMARY KEY, ts timestamptz DEFAULT now());`,
		)
	})
}

func TestOracleExprDefault_TypeAndArrayExpressions(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping oracle test: requires Docker")
	}
	oracle := startPGOracle(t)

	// 1.3 Type and Array Expressions in DEFAULT

	t.Run("type_cast", func(t *testing.T) {
		assertOracleRoundtrip(t, oracle,
			`CREATE TABLE t (id int PRIMARY KEY);`,
			`CREATE TABLE t (id int PRIMARY KEY, val numeric DEFAULT 0::numeric);`,
		)
	})

	t.Run("array_constructor", func(t *testing.T) {
		assertOracleRoundtrip(t, oracle,
			`CREATE TABLE t (id int PRIMARY KEY);`,
			`CREATE TABLE t (id int PRIMARY KEY, vals int[] DEFAULT ARRAY[1,2,3]);`,
		)
	})

	t.Run("array_with_type_cast", func(t *testing.T) {
		assertOracleRoundtrip(t, oracle,
			`CREATE TABLE t (id int PRIMARY KEY);`,
			`CREATE TABLE t (id int PRIMARY KEY, vals text[] DEFAULT ARRAY['a','b']::text[]);`,
		)
	})

	t.Run("string_concatenation", func(t *testing.T) {
		assertOracleRoundtrip(t, oracle,
			`CREATE TABLE t (id int PRIMARY KEY);`,
			`CREATE TABLE t (id int PRIMARY KEY, val text DEFAULT 'prefix' || 'suffix');`,
		)
	})

	t.Run("arithmetic", func(t *testing.T) {
		assertOracleRoundtrip(t, oracle,
			`CREATE TABLE t (id int PRIMARY KEY);`,
			`CREATE TABLE t (id int PRIMARY KEY, val int DEFAULT 1 + 1);`,
		)
	})

	t.Run("complex_expression", func(t *testing.T) {
		assertOracleRoundtrip(t, oracle,
			`CREATE TABLE t (id int PRIMARY KEY);`,
			`CREATE TABLE t (id int PRIMARY KEY, val text DEFAULT COALESCE(CURRENT_USER, 'anon') || '-' || CAST(CURRENT_DATE AS text));`,
		)
	})
}
