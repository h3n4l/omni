package parser

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestVerifyParse is the Parse Verifier. It reads SQL from corpus files,
// parses each with our parser, and optionally cross-validates against
// a real Oracle DB. It reports:
//   - PARSER GAP: Oracle DB accepts but our parser rejects
//   - PARSER LENIENT: Oracle DB rejects but our parser accepts (informational)
//   - PARSER CRASH: Our parser panics (always a bug)
//
// Corpus files live in oracle/quality/corpus/*.sql. Each file contains
// SQL statements separated by lines starting with "-- @" (metadata) or
// blank lines between statements terminated by ";".
//
// Run without Oracle DB (fast, parser-only):
//
//	go test ./oracle/parser/ -run TestVerifyParse -count=1
//
// Run with Oracle DB cross-validation:
//
//	go test ./oracle/parser/ -run TestVerifyParse -count=1 -timeout 300s
//
// The test automatically uses Oracle DB if Docker is available and
// -short is not set. Otherwise it runs parser-only checks.
func TestVerifyParse(t *testing.T) {
	corpusDir := filepath.Join("..", "quality", "corpus")
	entries, err := os.ReadDir(corpusDir)
	if err != nil {
		// Try from repo root (when run via go test ./oracle/parser/)
		corpusDir = filepath.Join("oracle", "quality", "corpus")
		entries, err = os.ReadDir(corpusDir)
		if err != nil {
			t.Fatalf("Cannot read corpus directory: %v", err)
		}
	}

	// Collect all .sql files
	var corpusFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			corpusFiles = append(corpusFiles, filepath.Join(corpusDir, e.Name()))
		}
	}
	if len(corpusFiles) == 0 {
		t.Skip("No corpus files found in oracle/quality/corpus/")
	}

	// Try to start Oracle DB (skip if unavailable)
	var db *oracleDB
	if !testing.Short() {
		db = startOracleDB(t)
		// db may be nil if Docker is unavailable (startOracleDB calls t.Skip)
	}

	var stats verifyStats
	for _, file := range corpusFiles {
		statements := loadCorpusFile(t, file)
		baseName := filepath.Base(file)

		for _, stmt := range statements {
			name := stmt.name
			if name == "" {
				name = truncateSQL(stmt.sql, 60)
			}
			testName := baseName + "/" + name

			t.Run(testName, func(t *testing.T) {
				stats.total++

				// --- Parser check (always) ---
				parserAccepts := verifyParserAccepts(t, stmt.sql)

				// --- Oracle DB check (if available) ---
				if db != nil {
					oracleErr := db.canExecute(stmt.sql)
					oracleAccepts := oracleErr == nil

					switch {
					case oracleAccepts && !parserAccepts:
						stats.parserGaps++
						t.Errorf("PARSER GAP: Oracle accepts but parser rejects")
					case !oracleAccepts && parserAccepts:
						stats.parserLenient++
						t.Logf("PARSER LENIENT: Oracle rejects (may need schema objects) but parser accepts")
					case oracleAccepts && parserAccepts:
						stats.bothAccept++
					case !oracleAccepts && !parserAccepts:
						stats.bothReject++
					}
				} else {
					// No Oracle DB — just verify parser doesn't crash
					if parserAccepts {
						stats.parserOnly++
					}
				}
			})
		}
	}

	// Print summary
	t.Logf("\n=== Parse Verifier Summary ===")
	t.Logf("Total statements:    %d", stats.total)
	if db != nil {
		t.Logf("Both accept:         %d", stats.bothAccept)
		t.Logf("Both reject:         %d", stats.bothReject)
		t.Logf("PARSER GAPS:         %d (Oracle accepts, parser rejects)", stats.parserGaps)
		t.Logf("Parser lenient:      %d (Oracle rejects, parser accepts)", stats.parserLenient)
	} else {
		t.Logf("Parser-only checks:  %d (no Oracle DB available)", stats.parserOnly)
	}
}

type verifyStats struct {
	total         int
	bothAccept    int
	bothReject    int
	parserGaps    int
	parserLenient int
	parserOnly    int
}

// corpusStatement is a single SQL statement from a corpus file.
type corpusStatement struct {
	sql  string
	name string // optional name from "-- @name: ..." annotation
	tags string // optional tags from "-- @tags: ..." annotation
}

// loadCorpusFile reads a .sql corpus file and extracts individual statements.
//
// Format:
//
//	-- @name: descriptive name
//	-- @tags: select,join,subquery
//	SELECT * FROM employees
//	WHERE dept_id = 10;
//
//	-- @name: next statement
//	INSERT INTO t VALUES (1);
//
// Statements are separated by blank lines or new "-- @" annotations.
// Lines starting with "--" (but not "-- @") are SQL comments and included in the statement.
func loadCorpusFile(t *testing.T, path string) []corpusStatement {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("Cannot open corpus file %s: %v", path, err)
	}
	defer f.Close()

	var statements []corpusStatement
	var current corpusStatement
	var lines []string

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		// Metadata annotation
		if strings.HasPrefix(line, "-- @name:") {
			// Flush previous statement if any
			if sql := buildSQL(lines); sql != "" {
				current.sql = sql
				statements = append(statements, current)
			}
			current = corpusStatement{name: strings.TrimSpace(strings.TrimPrefix(line, "-- @name:"))}
			lines = nil
			continue
		}
		if strings.HasPrefix(line, "-- @tags:") {
			current.tags = strings.TrimSpace(strings.TrimPrefix(line, "-- @tags:"))
			continue
		}

		// Blank line = statement separator (if we have accumulated lines)
		if strings.TrimSpace(line) == "" {
			if sql := buildSQL(lines); sql != "" {
				current.sql = sql
				statements = append(statements, current)
				current = corpusStatement{}
				lines = nil
			}
			continue
		}

		lines = append(lines, line)
	}

	// Flush last statement
	if sql := buildSQL(lines); sql != "" {
		current.sql = sql
		statements = append(statements, current)
	}

	return statements
}

// buildSQL joins lines into a single SQL string, trimming trailing semicolons
// for Oracle DB compatibility (Oracle doesn't want semicolons in executed SQL).
func buildSQL(lines []string) string {
	sql := strings.TrimSpace(strings.Join(lines, "\n"))
	// Remove trailing semicolons (Oracle DB exec doesn't want them)
	sql = strings.TrimRight(sql, "; \t\n")
	return sql
}

// verifyParserAccepts tries to parse the SQL and returns true if successful.
// It catches panics and reports them as test failures.
func verifyParserAccepts(t *testing.T, sql string) (accepts bool) {
	t.Helper()

	defer func() {
		if r := recover(); r != nil {
			t.Errorf("PARSER CRASH: panic on input: %s\npanic: %v", truncateSQL(sql, 200), r)
			accepts = false
		}
	}()

	_, err := Parse(sql)
	return err == nil
}

// truncateSQL returns a shortened version of sql for display.
func truncateSQL(sql string, maxLen int) string {
	// Collapse whitespace for display
	s := strings.Join(strings.Fields(sql), " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
