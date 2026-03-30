package parser

import (
	"testing"
)

// --- Section 4.1: INSERT ---

func TestCollect_Insert(t *testing.T) {
	tests := []struct {
		name      string
		sql       string
		wantRules []string
		wantToks  []int
	}{
		{
			name:      "INSERT INTO |",
			sql:       "INSERT INTO ",
			wantRules: []string{"table_ref"},
		},
		{
			name:      "INSERT INTO t (|)",
			sql:       "INSERT INTO t (",
			wantRules: []string{"columnref"},
		},
		{
			name:      "INSERT INTO t (a, |)",
			sql:       "INSERT INTO t (a, ",
			wantRules: []string{"columnref"},
		},
		{
			name:     "INSERT INTO t |",
			sql:      "INSERT INTO t ",
			wantToks: []int{kwVALUES, kwSELECT, kwDEFAULT, kwEXEC, kwOUTPUT},
		},
		{
			name:      "INSERT INTO t SELECT |",
			sql:       "INSERT INTO t SELECT ",
			wantRules: []string{"columnref"},
		},
		{
			name:      "INSERT INTO t OUTPUT |",
			sql:       "INSERT INTO t OUTPUT ",
			wantRules: []string{"columnref"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := Collect(tt.sql, len(tt.sql))
			if cs == nil {
				t.Fatal("Collect returned nil")
			}
			for _, r := range tt.wantRules {
				if !cs.HasRule(r) {
					t.Errorf("missing rule candidate %q", r)
				}
			}
			for _, tok := range tt.wantToks {
				if !cs.HasToken(tok) {
					t.Errorf("missing token candidate %s (%d)", TokenName(tok), tok)
				}
			}
		})
	}
}

func TestCollect_InsertValues_NoSpecificCandidates(t *testing.T) {
	// INSERT INTO t VALUES (|) → value context, no specific rule
	cs := Collect("INSERT INTO t VALUES (", len("INSERT INTO t VALUES ("))
	if cs == nil {
		t.Fatal("Collect returned nil")
	}
	// Value context — the parser enters parseExprList which goes to parseExpr.
	// No specific columnref rule is required; just verify no panic.
}

// --- Section 4.2: UPDATE ---

func TestCollect_Update(t *testing.T) {
	tests := []struct {
		name      string
		sql       string
		wantRules []string
		wantToks  []int
	}{
		{
			name:      "UPDATE |",
			sql:       "UPDATE ",
			wantRules: []string{"table_ref"},
		},
		{
			name:      "UPDATE t SET |",
			sql:       "UPDATE t SET ",
			wantRules: []string{"columnref"},
		},
		{
			name:      "UPDATE t SET a = 1, |",
			sql:       "UPDATE t SET a = 1, ",
			wantRules: []string{"columnref"},
		},
		{
			name:      "UPDATE t SET a = 1 WHERE |",
			sql:       "UPDATE t SET a = 1 WHERE ",
			wantRules: []string{"columnref"},
		},
		{
			name:      "UPDATE t SET a = 1 OUTPUT |",
			sql:       "UPDATE t SET a = 1 OUTPUT ",
			wantRules: []string{"columnref"},
		},
		{
			name:      "UPDATE t SET a = 1 FROM t JOIN |",
			sql:       "UPDATE t SET a = 1 FROM t JOIN ",
			wantRules: []string{"table_ref"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := Collect(tt.sql, len(tt.sql))
			if cs == nil {
				t.Fatal("Collect returned nil")
			}
			for _, r := range tt.wantRules {
				if !cs.HasRule(r) {
					t.Errorf("missing rule candidate %q", r)
				}
			}
			for _, tok := range tt.wantToks {
				if !cs.HasToken(tok) {
					t.Errorf("missing token candidate %s (%d)", TokenName(tok), tok)
				}
			}
		})
	}
}

// --- Section 4.3: DELETE ---

func TestCollect_Delete(t *testing.T) {
	tests := []struct {
		name      string
		sql       string
		wantRules []string
		wantToks  []int
	}{
		{
			name:      "DELETE FROM |",
			sql:       "DELETE FROM ",
			wantRules: []string{"table_ref"},
		},
		{
			name:      "DELETE FROM t WHERE |",
			sql:       "DELETE FROM t WHERE ",
			wantRules: []string{"columnref"},
		},
		{
			name:      "DELETE FROM t OUTPUT |",
			sql:       "DELETE FROM t OUTPUT ",
			wantRules: []string{"columnref"},
		},
		{
			name:      "DELETE t FROM t JOIN |",
			sql:       "DELETE t FROM t JOIN ",
			wantRules: []string{"table_ref"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := Collect(tt.sql, len(tt.sql))
			if cs == nil {
				t.Fatal("Collect returned nil")
			}
			for _, r := range tt.wantRules {
				if !cs.HasRule(r) {
					t.Errorf("missing rule candidate %q", r)
				}
			}
			for _, tok := range tt.wantToks {
				if !cs.HasToken(tok) {
					t.Errorf("missing token candidate %s (%d)", TokenName(tok), tok)
				}
			}
		})
	}
}

// --- Section 4.4: MERGE ---

func TestCollect_Merge(t *testing.T) {
	tests := []struct {
		name      string
		sql       string
		wantRules []string
		wantToks  []int
	}{
		{
			name:      "MERGE INTO |",
			sql:       "MERGE INTO ",
			wantRules: []string{"table_ref"},
		},
		{
			name:      "MERGE INTO t USING |",
			sql:       "MERGE INTO t USING ",
			wantRules: []string{"table_ref"},
		},
		{
			name:      "MERGE INTO t USING s ON |",
			sql:       "MERGE INTO t USING s ON ",
			wantRules: []string{"columnref"},
		},
		{
			name:      "MERGE ... WHEN |",
			sql:       "MERGE INTO t USING s ON t.id = s.id WHEN ",
			wantRules: []string{"matched_keyword"},
			wantToks:  []int{kwNOT},
		},
		{
			name:     "MERGE ... WHEN MATCHED THEN |",
			sql:      "MERGE INTO t USING s ON t.id = s.id WHEN MATCHED THEN ",
			wantToks: []int{kwUPDATE, kwDELETE},
		},
		{
			name:     "MERGE ... WHEN NOT MATCHED THEN |",
			sql:      "MERGE INTO t USING s ON t.id = s.id WHEN NOT MATCHED THEN ",
			wantToks: []int{kwINSERT},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := Collect(tt.sql, len(tt.sql))
			if cs == nil {
				t.Fatal("Collect returned nil")
			}
			for _, r := range tt.wantRules {
				if !cs.HasRule(r) {
					t.Errorf("missing rule candidate %q", r)
				}
			}
			for _, tok := range tt.wantToks {
				if !cs.HasToken(tok) {
					t.Errorf("missing token candidate %s (%d)", TokenName(tok), tok)
				}
			}
		})
	}
}
