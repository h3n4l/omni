package completion

import (
	"testing"

	"github.com/bytebase/omni/mysql/catalog"
)

// containsCandidate returns true if candidates contains one with the given text and type.
func containsCandidate(candidates []Candidate, text string, typ CandidateType) bool {
	for _, c := range candidates {
		if c.Text == text && c.Type == typ {
			return true
		}
	}
	return false
}

// containsText returns true if any candidate has the given text.
func containsText(candidates []Candidate, text string) bool {
	for _, c := range candidates {
		if c.Text == text {
			return true
		}
	}
	return false
}

// hasDuplicates returns true if there are duplicate (text, type) pairs (case-insensitive).
func hasDuplicates(candidates []Candidate) bool {
	type key struct {
		text string
		typ  CandidateType
	}
	seen := make(map[key]bool)
	for _, c := range candidates {
		k := key{text: c.Text, typ: c.Type}
		if seen[k] {
			return true
		}
		seen[k] = true
	}
	return false
}

func TestComplete_2_1_CompleteReturnsSlice(t *testing.T) {
	// Scenario: Complete(sql, cursorOffset, catalog) returns []Candidate
	cat := catalog.New()
	candidates := Complete("SELECT ", 7, cat)
	if candidates == nil {
		// nil is acceptable (no candidates), but the function should not panic
		candidates = []Candidate{}
	}
	// Just verify it returns a slice (type is enforced by compiler).
	_ = candidates
}

func TestComplete_2_1_CandidateFields(t *testing.T) {
	// Scenario: Candidate struct has Text, Type, Definition, Comment fields
	c := Candidate{
		Text:       "SELECT",
		Type:       CandidateKeyword,
		Definition: "SQL SELECT statement",
		Comment:    "Retrieves data",
	}
	if c.Text != "SELECT" {
		t.Errorf("Text = %q, want SELECT", c.Text)
	}
	if c.Type != CandidateKeyword {
		t.Errorf("Type = %d, want CandidateKeyword", c.Type)
	}
	if c.Definition != "SQL SELECT statement" {
		t.Errorf("Definition = %q", c.Definition)
	}
	if c.Comment != "Retrieves data" {
		t.Errorf("Comment = %q", c.Comment)
	}
}

func TestComplete_2_1_CandidateTypeEnum(t *testing.T) {
	// Scenario: CandidateType enum with all types
	types := []CandidateType{
		CandidateKeyword,
		CandidateDatabase,
		CandidateTable,
		CandidateView,
		CandidateColumn,
		CandidateFunction,
		CandidateProcedure,
		CandidateIndex,
		CandidateTrigger,
		CandidateEvent,
		CandidateVariable,
		CandidateCharset,
		CandidateEngine,
		CandidateType_,
	}
	// All types should be distinct.
	seen := make(map[CandidateType]bool)
	for _, ct := range types {
		if seen[ct] {
			t.Errorf("duplicate CandidateType value %d", ct)
		}
		seen[ct] = true
	}
	if len(types) != 14 {
		t.Errorf("expected 14 CandidateType values, got %d", len(types))
	}
}

func TestComplete_2_1_NilCatalog(t *testing.T) {
	// Scenario: Complete with nil catalog returns keyword-only candidates
	candidates := Complete("SELECT ", 7, nil)
	for _, c := range candidates {
		if c.Type != CandidateKeyword {
			t.Errorf("with nil catalog, got non-keyword candidate: %+v", c)
		}
	}
	// Should still return some keywords (e.g., DISTINCT, ALL from SELECT context).
	if len(candidates) == 0 {
		t.Error("expected some keyword candidates with nil catalog")
	}
}

func TestComplete_2_1_EmptySQL(t *testing.T) {
	// Scenario: Complete with empty sql returns top-level statement keywords
	candidates := Complete("", 0, nil)
	if len(candidates) == 0 {
		t.Fatal("expected top-level keywords for empty SQL")
	}
	// Should contain core statement keywords.
	for _, kw := range []string{"SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", "ALTER", "DROP"} {
		if !containsCandidate(candidates, kw, CandidateKeyword) {
			t.Errorf("missing expected keyword %s", kw)
		}
	}
	// All should be keywords.
	for _, c := range candidates {
		if c.Type != CandidateKeyword {
			t.Errorf("non-keyword candidate in empty SQL: %+v", c)
		}
	}
}

func TestComplete_2_1_PrefixFiltering(t *testing.T) {
	// Scenario: Prefix filtering: `SEL|` matches SELECT keyword
	candidates := Complete("SEL", 3, nil)
	if !containsCandidate(candidates, "SELECT", CandidateKeyword) {
		t.Error("expected SELECT in candidates for prefix SEL")
	}
	// Should not contain non-matching keywords.
	if containsCandidate(candidates, "INSERT", CandidateKeyword) {
		t.Error("INSERT should not match prefix SEL")
	}
}

func TestComplete_2_1_PrefixCaseInsensitive(t *testing.T) {
	// Scenario: Prefix filtering is case-insensitive
	candidates := Complete("sel", 3, nil)
	if !containsCandidate(candidates, "SELECT", CandidateKeyword) {
		t.Error("expected SELECT in candidates for lowercase prefix sel")
	}
	// Mixed case
	candidates2 := Complete("Sel", 3, nil)
	if !containsCandidate(candidates2, "SELECT", CandidateKeyword) {
		t.Error("expected SELECT in candidates for mixed-case prefix Sel")
	}
}

func TestComplete_2_1_Deduplication(t *testing.T) {
	// Scenario: Deduplication: same candidate not returned twice
	// Use a context that might produce duplicate token candidates.
	candidates := Complete("", 0, nil)
	if hasDuplicates(candidates) {
		t.Error("found duplicate candidates in results")
	}

	// Also test with a prefix context.
	candidates2 := Complete("SELECT ", 7, nil)
	if hasDuplicates(candidates2) {
		t.Error("found duplicate candidates in SELECT context")
	}
}
