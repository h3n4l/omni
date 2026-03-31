package parser

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/bytebase/omni/mysql/ast"
)

// TestVerifyCorpus is the multi-dimensional corpus verifier. It reads SQL from
// corpus files and runs each statement through multiple verification layers:
//
//  1. Crash   — the parser must never panic on any input
//  2. Parse   — does the parser accept/reject as expected? (@valid annotation)
//  3. Loc     — all AST nodes must have valid, set Loc (Start > 0, End > Start)
//  4. LocText — sql[Start:End] must produce a non-empty, non-whitespace substring
//  5. Stability — parsing the same SQL twice must yield identical AST (NodeToString)
//  6. Error   — if @valid: false, error must have non-empty message AND valid position
//
// Corpus files live in mysql/quality/corpus/*.sql with annotations:
//
//	-- @name: descriptive name
//	-- @valid: true|false       (expected parse result)
//	-- @source: where this SQL comes from
//
// Run:
//
//	go test ./mysql/parser/ -run TestVerifyCorpus -count=1 -v
func TestVerifyCorpus(t *testing.T) {
	corpusDir := filepath.Join("..", "quality", "corpus")
	entries, err := os.ReadDir(corpusDir)
	if err != nil {
		corpusDir = filepath.Join("mysql", "quality", "corpus")
		entries, err = os.ReadDir(corpusDir)
		if err != nil {
			t.Fatalf("Cannot read corpus directory: %v", err)
		}
	}

	var corpusFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			corpusFiles = append(corpusFiles, filepath.Join(corpusDir, e.Name()))
		}
	}
	if len(corpusFiles) == 0 {
		t.Skip("No corpus files found in mysql/quality/corpus/")
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
				result := verifyStatement(t, stmt)
				accumulateStats(&stats, result)
			})
		}
	}

	// Print summary.
	t.Logf("\n=== Corpus Verifier Summary ===")
	t.Logf("Total statements:       %d", stats.total)
	t.Logf("Parse OK:               %d", stats.parseOK)
	t.Logf("Parse expected fail:    %d", stats.parseExpectedFail)
	t.Logf("PARSE VIOLATIONS:       %d (should accept but rejects)", stats.parseViolations)
	t.Logf("PARSE LENIENT:          %d (should reject but accepts)", stats.parseLenient)
	t.Logf("Loc clean:              %d (all nodes have valid Loc)", stats.locClean)
	t.Logf("LOC VIOLATIONS:         %d (nodes with bad/missing Loc)", stats.locViolations)
	t.Logf("LOC TEXT VIOLATIONS:    %d (Loc range produces bad substring)", stats.locTextViolations)
	t.Logf("STABILITY VIOLATIONS:   %d (non-deterministic parse)", stats.stabilityViolations)
	t.Logf("ERROR QUALITY ISSUES:   %d (bad error position or empty message)", stats.errorQualityIssues)
	t.Logf("CRASHES:                %d", stats.crashes)

	issues := stats.parseViolations + stats.crashes + stats.stabilityViolations
	if issues > 0 {
		t.Logf("\n!! %d critical issue(s) require attention", issues)
	}
}

type verifyStats struct {
	total               int
	parseOK             int
	parseExpectedFail   int
	parseViolations     int
	parseLenient        int
	locClean            int
	locViolations       int
	locTextViolations   int
	stabilityViolations int
	errorQualityIssues  int
	crashes             int
}

type verifyResult struct {
	crashed            bool
	parsed             bool
	parseErr           error
	locViolations      []LocViolation
	locTextViolations  []locTextViolation
	stabilityViolation bool
	errorQualityIssue  bool
	parseLenient       bool
	stmt               corpusStatement
}

// locTextViolation records a Loc range that does not produce a sensible substring.
type locTextViolation struct {
	Path    string
	NodeTag string
	Start   int
	End     int
	Text    string
	Reason  string
}

func (v locTextViolation) String() string {
	return fmt.Sprintf("%s [%s]: Start=%d End=%d text=%q — %s", v.Path, v.NodeTag, v.Start, v.End, v.Text, v.Reason)
}

// verifyStatement runs all verification layers on a single corpus statement.
func verifyStatement(t *testing.T, stmt corpusStatement) verifyResult {
	t.Helper()
	var result verifyResult
	result.stmt = stmt

	// --- Layer 1: Crash check (always) ---
	var parseResult *ast.List
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("CRASH: parser panicked on: %s\npanic: %v",
					truncateSQL(stmt.sql, 200), r)
				result.crashed = true
			}
		}()
		var err error
		parseResult, err = Parse(stmt.sql)
		result.parseErr = err
		result.parsed = err == nil
	}()

	if result.crashed {
		return result
	}

	// --- Layer 2: Parse check (if @valid is annotated) ---
	switch stmt.valid {
	case "true":
		if !result.parsed {
			t.Errorf("PARSE VIOLATION: @valid: true but parser rejects: %v", result.parseErr)
		}
	case "false":
		if result.parsed {
			t.Errorf("PARSE LENIENT: @valid: false but parser accepts (stricter parsers should reject)")
			result.parseLenient = true
		}
		if !result.parsed {
			// --- Layer 6: Error quality check ---
			result.errorQualityIssue = checkErrorQuality(t, result.parseErr)
		}
	}

	if !result.parsed || parseResult == nil {
		return result
	}

	// --- Layer 3: Loc check (all nodes must have valid Loc) ---
	locViolations := checkLocOnResult(parseResult, stmt.sql)
	result.locViolations = locViolations
	if len(locViolations) > 0 {
		t.Errorf("LOC VIOLATIONS: %d nodes with invalid Loc:", len(locViolations))
		for i, v := range locViolations {
			if i >= 5 {
				t.Errorf("  ... and %d more", len(locViolations)-5)
				break
			}
			t.Errorf("  %s", v)
		}
	}

	// --- Layer 4: Loc text check (sql[Start:End] must be sensible) ---
	textViolations := checkLocText(parseResult, stmt.sql)
	result.locTextViolations = textViolations
	if len(textViolations) > 0 {
		t.Errorf("LOC TEXT VIOLATIONS: %d nodes where Loc range produces bad text:", len(textViolations))
		for i, v := range textViolations {
			if i >= 5 {
				t.Errorf("  ... and %d more", len(textViolations)-5)
				break
			}
			t.Errorf("  %s", v)
		}
	}

	// --- Layer 5: Stability check (parse twice, compare NodeToString) ---
	result.stabilityViolation = checkStability(t, stmt.sql, parseResult)

	return result
}

// checkErrorQuality verifies error has a non-empty message and valid position.
// Returns true if there is a quality issue.
func checkErrorQuality(t *testing.T, err error) bool {
	t.Helper()
	if err == nil {
		return false
	}

	hasIssue := false
	msg := err.Error()
	if msg == "" {
		t.Errorf("ERROR QUALITY: error message is empty")
		hasIssue = true
	}

	if pe, ok := err.(*ParseError); ok {
		if pe.Position < 0 {
			t.Errorf("ERROR QUALITY: error position = %d (want >= 0)", pe.Position)
			hasIssue = true
		}
		if pe.Line <= 0 {
			t.Errorf("ERROR QUALITY: error line = %d (want > 0)", pe.Line)
			hasIssue = true
		}
		if pe.Column <= 0 {
			t.Errorf("ERROR QUALITY: error column = %d (want > 0)", pe.Column)
			hasIssue = true
		}
	}

	return hasIssue
}

// checkLocOnResult runs the Loc walker on parse results and returns violations.
func checkLocOnResult(result *ast.List, sql string) []LocViolation {
	var violations []LocViolation
	if result == nil {
		return violations
	}

	for i, item := range result.Items {
		path := fmt.Sprintf("Items[%d]", i)
		walkNodeLocs(reflect.ValueOf(item), path, &violations)
	}

	return violations
}

// checkLocText extracts sql[Start:End] for every Loc-bearing node and validates
// the substring is non-empty and within bounds.
func checkLocText(result *ast.List, sql string) []locTextViolation {
	var violations []locTextViolation
	if result == nil {
		return violations
	}

	for i, item := range result.Items {
		path := fmt.Sprintf("Items[%d]", i)
		walkLocText(reflect.ValueOf(item), path, sql, &violations)
	}

	return violations
}

// walkLocText recursively walks a reflected AST value, checking every Loc field
// produces a sensible sql substring.
func walkLocText(v reflect.Value, path, sql string, violations *[]locTextViolation) {
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Struct:
		typeName := v.Type().Name()

		locField := v.FieldByName("Loc")
		if locField.IsValid() && locField.Type() == reflect.TypeOf(ast.Loc{}) {
			loc := locField.Interface().(ast.Loc)
			if loc.Start > 0 && loc.End > loc.Start {
				// Both are set and valid — check the text.
				if loc.End > len(sql) {
					*violations = append(*violations, locTextViolation{
						Path:    path,
						NodeTag: typeName,
						Start:   loc.Start,
						End:     loc.End,
						Reason:  fmt.Sprintf("End (%d) exceeds sql length (%d)", loc.End, len(sql)),
					})
				} else {
					text := sql[loc.Start:loc.End]
					if strings.TrimSpace(text) == "" {
						*violations = append(*violations, locTextViolation{
							Path:    path,
							NodeTag: typeName,
							Start:   loc.Start,
							End:     loc.End,
							Text:    text,
							Reason:  "Loc range produces empty/whitespace-only text",
						})
					}
				}
			}
		}

		for i := 0; i < v.NumField(); i++ {
			field := v.Type().Field(i)
			if !field.IsExported() || field.Name == "Loc" {
				continue
			}
			walkLocText(v.Field(i), path+"."+field.Name, sql, violations)
		}

	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			elem := v.Index(i)
			elemPath := fmt.Sprintf("%s[%d]", path, i)
			actual := elem
			for actual.Kind() == reflect.Ptr || actual.Kind() == reflect.Interface {
				if actual.IsNil() {
					break
				}
				actual = actual.Elem()
			}
			if actual.IsValid() && actual.Kind() == reflect.Struct {
				elemPath = fmt.Sprintf("%s[%d](%s)", path, i, actual.Type().Name())
			}
			walkLocText(elem, elemPath, sql, violations)
		}
	}
}

// checkStability parses the same SQL a second time and compares NodeToString
// output. Returns true if there is a stability violation.
func checkStability(t *testing.T, sql string, first *ast.List) bool {
	t.Helper()

	var second *ast.List
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("STABILITY CRASH: second parse panicked: %v", r)
			}
		}()
		var err error
		second, err = Parse(sql)
		if err != nil {
			t.Errorf("STABILITY: first parse succeeded but second parse fails: %v", err)
		}
	}()

	if second == nil {
		return true
	}

	s1 := ast.NodeToString(first)
	s2 := ast.NodeToString(second)
	if s1 != s2 {
		t.Errorf("STABILITY VIOLATION: parsing %q produces different AST on second parse:\n  first:  %s\n  second: %s",
			truncateSQL(sql, 100), truncateSQL(s1, 200), truncateSQL(s2, 200))
		return true
	}
	return false
}

func accumulateStats(stats *verifyStats, r verifyResult) {
	if r.crashed {
		stats.crashes++
		return
	}

	switch r.stmt.valid {
	case "true":
		if r.parsed {
			stats.parseOK++
		} else {
			stats.parseViolations++
		}
	case "false":
		stats.parseExpectedFail++
		if r.parseLenient {
			stats.parseLenient++
		}
	default:
		if r.parsed {
			stats.parseOK++
		}
	}

	if r.parsed {
		if len(r.locViolations) == 0 {
			stats.locClean++
		} else {
			stats.locViolations++
		}
		if len(r.locTextViolations) > 0 {
			stats.locTextViolations++
		}
		if r.stabilityViolation {
			stats.stabilityViolations++
		}
	}

	if r.errorQualityIssue {
		stats.errorQualityIssues++
	}
}

// --- Corpus file parsing ---

// corpusStatement is a single SQL statement from a corpus file.
type corpusStatement struct {
	sql    string
	name   string // from "-- @name: ..."
	valid  string // "true", "false", or "" (unknown)
	source string // from "-- @source: ..."
}

// loadCorpusFile reads a .sql corpus file and extracts individual statements.
//
// Format:
//
//	-- @name: descriptive name
//	-- @valid: true
//	-- @source: MySQL 8.0 Reference Manual
//	SELECT * FROM employees
//	WHERE dept_id = 10
//
//	-- @name: next statement
//	INSERT INTO t VALUES (1)
//
// Statements are separated by blank lines or new "-- @name:" annotations.
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

	flush := func() {
		if sql := buildSQL(lines); sql != "" {
			current.sql = sql
			statements = append(statements, current)
		}
		current = corpusStatement{}
		lines = nil
	}

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()

		// Metadata annotations.
		if strings.HasPrefix(line, "-- @name:") {
			flush()
			current.name = strings.TrimSpace(strings.TrimPrefix(line, "-- @name:"))
			continue
		}
		if strings.HasPrefix(line, "-- @valid:") {
			current.valid = strings.TrimSpace(strings.TrimPrefix(line, "-- @valid:"))
			continue
		}
		if strings.HasPrefix(line, "-- @source:") {
			current.source = strings.TrimSpace(strings.TrimPrefix(line, "-- @source:"))
			continue
		}

		// Blank line = statement separator.
		if strings.TrimSpace(line) == "" {
			if len(lines) > 0 {
				flush()
			}
			continue
		}

		lines = append(lines, line)
	}

	flush()
	return statements
}

// buildSQL joins lines into a single SQL string.
func buildSQL(lines []string) string {
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// truncateSQL returns a shortened version of sql for display.
func truncateSQL(sql string, maxLen int) string {
	s := strings.Join(strings.Fields(sql), " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
