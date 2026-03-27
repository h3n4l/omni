package parser

import (
	"testing"
)

func TestCollect_EmptyInput(t *testing.T) {
	cs := Collect("", 0)
	if cs == nil {
		t.Fatal("Collect returned nil for empty input")
	}
}

func TestCollect_NonNilResult(t *testing.T) {
	tests := []struct {
		name   string
		sql    string
		cursor int
	}{
		{"empty", "", 0},
		{"select keyword", "SELECT", 6},
		{"partial select", "SEL", 3},
		{"cursor at start", "SELECT 1", 0},
		{"cursor mid-statement", "SELECT 1", 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := Collect(tt.sql, tt.cursor)
			if cs == nil {
				t.Fatal("Collect returned nil")
			}
		})
	}
}

func TestCandidateSet_Dedup(t *testing.T) {
	cs := newCandidateSet()
	cs.addToken(kwSELECT)
	cs.addToken(kwSELECT)
	if len(cs.Tokens) != 1 {
		t.Fatalf("expected 1 token, got %d", len(cs.Tokens))
	}
	if !cs.HasToken(kwSELECT) {
		t.Fatal("HasToken returned false for added token")
	}
}

func TestCandidateSet_RuleDedup(t *testing.T) {
	cs := newCandidateSet()
	cs.addRule("table_name")
	cs.addRule("table_name")
	if len(cs.Rules) != 1 {
		t.Fatalf("expected 1 rule, got %d", len(cs.Rules))
	}
	if !cs.HasRule("table_name") {
		t.Fatal("HasRule returned false for added rule")
	}
	if cs.HasRule("nonexistent") {
		t.Fatal("HasRule returned true for non-existent rule")
	}
}

func TestCollectMode(t *testing.T) {
	p := &Parser{}
	if p.collectMode() {
		t.Fatal("collectMode should be false when completing is false")
	}
	p.completing = true
	if p.collectMode() {
		t.Fatal("collectMode should be false when collecting is false")
	}
	p.collecting = true
	if !p.collectMode() {
		t.Fatal("collectMode should be true when both completing and collecting")
	}
}

func TestCheckCursor(t *testing.T) {
	p := &Parser{
		completing: true,
		cursorOff:  5,
		cur:        Token{Loc: 10},
	}
	p.checkCursor()
	if !p.collecting {
		t.Fatal("checkCursor should set collecting=true when cur.Loc >= cursorOff")
	}
}

func TestCheckCursor_NotYet(t *testing.T) {
	p := &Parser{
		completing: true,
		cursorOff:  10,
		cur:        Token{Loc: 5},
	}
	p.checkCursor()
	if p.collecting {
		t.Fatal("checkCursor should not set collecting when cur.Loc < cursorOff")
	}
}

func TestExistingParseUnaffected(t *testing.T) {
	// Ensure that normal parsing (completing=false) still works.
	sql := "SELECT 1"
	_, err := Parse(sql)
	if err != nil {
		t.Fatalf("Parse failed with completion fields present: %v", err)
	}
}

func TestAddTokenCandidate_NilCandidates(t *testing.T) {
	// Should not panic when candidates is nil.
	p := &Parser{}
	p.addTokenCandidate(kwSELECT)
	p.addRuleCandidate("table_name")
}

func TestErrCollecting(t *testing.T) {
	if errCollecting == nil {
		t.Fatal("errCollecting should not be nil")
	}
	if errCollecting.Message != "collecting" {
		t.Fatalf("expected message 'collecting', got %q", errCollecting.Message)
	}
}

// --- Section 1.2: TokenName & Basic Keyword Collection ---

func TestTokenName_SingleCharTokens(t *testing.T) {
	tests := []struct {
		tokenType int
		want      string
	}{
		{';', ";"},
		{'(', "("},
		{')', ")"},
		{',', ","},
		{'.', "."},
		{'=', "="},
		{'+', "+"},
		{'-', "-"},
		{'*', "*"},
		{'/', "/"},
		{'%', "%"},
	}
	for _, tt := range tests {
		got := TokenName(tt.tokenType)
		if got != tt.want {
			t.Errorf("TokenName(%d) = %q, want %q", tt.tokenType, got, tt.want)
		}
	}
}

func TestTokenName_Keywords(t *testing.T) {
	tests := []struct {
		tokenType int
		want      string
	}{
		{kwSELECT, "SELECT"},
		{kwINSERT, "INSERT"},
		{kwUPDATE, "UPDATE"},
		{kwDELETE, "DELETE"},
		{kwCREATE, "CREATE"},
		{kwALTER, "ALTER"},
		{kwDROP, "DROP"},
		{kwFROM, "FROM"},
		{kwWHERE, "WHERE"},
		{kwWITH, "WITH"},
		{kwMERGE, "MERGE"},
		{kwTRY_CAST, "TRY_CAST"},
		{kwCURRENT_TIMESTAMP, "CURRENT_TIMESTAMP"},
		{kwXACT_ABORT, "XACT_ABORT"},
		{kwIDENTITY_INSERT, "IDENTITY_INSERT"},
	}
	for _, tt := range tests {
		got := TokenName(tt.tokenType)
		if got != tt.want {
			t.Errorf("TokenName(%d) = %q, want %q", tt.tokenType, got, tt.want)
		}
	}
}

func TestTokenName_AllKeywordsCovered(t *testing.T) {
	// Verify that every keyword in keywordMap has a round-trip through TokenName.
	for name, tok := range keywordMap {
		got := TokenName(tok)
		want := ""
		for _, ch := range name {
			if ch >= 'a' && ch <= 'z' {
				want += string(rune(ch - 32))
			} else {
				want += string(ch)
			}
		}
		if got != want {
			t.Errorf("TokenName(%d) = %q, want %q (keyword %q)", tok, got, want, name)
		}
	}
}

func TestTokenName_NonKeyword(t *testing.T) {
	// Non-keyword tokens should return "".
	if got := TokenName(tokIDENT); got != "" {
		t.Errorf("TokenName(tokIDENT) = %q, want empty", got)
	}
	if got := TokenName(tokICONST); got != "" {
		t.Errorf("TokenName(tokICONST) = %q, want empty", got)
	}
	if got := TokenName(tokSCONST); got != "" {
		t.Errorf("TokenName(tokSCONST) = %q, want empty", got)
	}
	if got := TokenName(tokEOF); got != "" {
		t.Errorf("TokenName(tokEOF=0) = %q, want empty", got)
	}
}

func TestCollect_EmptyInput_TopLevelKeywords(t *testing.T) {
	cs := Collect("", 0)
	if cs == nil {
		t.Fatal("Collect returned nil")
	}
	// Must contain key top-level statement keywords.
	required := []int{kwSELECT, kwINSERT, kwUPDATE, kwDELETE, kwCREATE, kwALTER, kwDROP}
	for _, kw := range required {
		if !cs.HasToken(kw) {
			t.Errorf("empty input: missing top-level keyword %s", TokenName(kw))
		}
	}
}

func TestCollect_AfterSemicolon_TopLevelKeywords(t *testing.T) {
	sql := "SELECT 1; "
	cs := Collect(sql, len(sql))
	if cs == nil {
		t.Fatal("Collect returned nil")
	}
	required := []int{kwSELECT, kwINSERT, kwUPDATE, kwDELETE, kwCREATE, kwALTER, kwDROP}
	for _, kw := range required {
		if !cs.HasToken(kw) {
			t.Errorf("after semicolon: missing top-level keyword %s", TokenName(kw))
		}
	}
}

func TestCollect_CursorAtEOF_EmptyString(t *testing.T) {
	cs := Collect("", 0)
	if cs == nil {
		t.Fatal("Collect returned nil")
	}
	// Verify a broader set of top-level keywords.
	topLevel := []int{
		kwSELECT, kwINSERT, kwUPDATE, kwDELETE, kwCREATE, kwALTER, kwDROP,
		kwMERGE, kwWITH, kwEXEC, kwDECLARE, kwSET, kwIF, kwWHILE,
		kwBEGIN, kwPRINT, kwGRANT, kwREVOKE, kwDENY, kwTRUNCATE, kwUSE,
		kwBACKUP, kwRESTORE,
	}
	for _, kw := range topLevel {
		if !cs.HasToken(kw) {
			t.Errorf("EOF empty string: missing top-level keyword %s", TokenName(kw))
		}
	}
}
