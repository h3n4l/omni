package parser

import "testing"

func TestCollectStmtKeywords(t *testing.T) {
	cs := Collect("", 0)
	if cs == nil {
		t.Fatal("Collect returned nil")
	}

	// At the start of a statement, the parser should offer statement-starting keywords.
	want := []int{SELECT, INSERT, CREATE, ALTER, DROP, UPDATE, DELETE_P, WITH,
		SET, SHOW, GRANT, REVOKE, TRUNCATE, BEGIN_P, COMMIT, ROLLBACK}

	for _, tok := range want {
		if !cs.HasToken(tok) {
			t.Errorf("expected token %d in candidates, but not found", tok)
		}
	}
}

func TestCollectBasic(t *testing.T) {
	cs := newCandidateSet()

	cs.addToken(SELECT)
	cs.addToken(INSERT)
	cs.addToken(SELECT) // duplicate

	if len(cs.Tokens) != 2 {
		t.Errorf("expected 2 tokens, got %d", len(cs.Tokens))
	}
	if !cs.HasToken(SELECT) {
		t.Error("expected HasToken(SELECT) = true")
	}
	if cs.HasToken(UPDATE) {
		t.Error("expected HasToken(UPDATE) = false")
	}

	cs.addRule("table_ref")
	cs.addRule("table_ref") // duplicate
	cs.addRule("column_ref")

	if len(cs.Rules) != 2 {
		t.Errorf("expected 2 rules, got %d", len(cs.Rules))
	}
	if !cs.HasRule("table_ref") {
		t.Error("expected HasRule(table_ref) = true")
	}
	if cs.HasRule("expr") {
		t.Error("expected HasRule(expr) = false")
	}
}

func TestCollectAfterSelect(t *testing.T) {
	// "SELECT " — cursor after SELECT keyword
	candidates := Collect("SELECT ", 7)
	if candidates == nil {
		t.Fatal("expected non-nil candidates")
	}
	if !candidates.HasRule("columnref") {
		t.Error("expected columnref rule candidate in SELECT target list")
	}
	if !candidates.HasRule("func_name") {
		t.Error("expected func_name rule candidate in SELECT target list")
	}
	// Should also have expression-starting keywords
	if !candidates.HasToken(CASE) {
		t.Error("expected CASE token in expression context")
	}
	if !candidates.HasToken(EXISTS) {
		t.Error("expected EXISTS token in expression context")
	}
}

func TestCollectAfterFrom(t *testing.T) {
	candidates := Collect("SELECT 1 FROM ", 14)
	if candidates == nil {
		t.Fatal("expected non-nil candidates")
	}
	if !candidates.HasRule("relation_expr") {
		t.Error("expected relation_expr rule candidate after FROM")
	}
}

func TestCollectKeywordCategories(t *testing.T) {
	candidates := Collect("SELECT ", 7)
	// Unreserved keywords like NAME should be candidates (valid as identifiers)
	if !candidates.HasToken(NAME_P) {
		t.Error("expected unreserved keyword NAME_P as candidate")
	}
}
