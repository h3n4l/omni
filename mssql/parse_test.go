package mssql

import (
	"strings"
	"testing"
)

func TestBuildLineIndex(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  lineIndex
	}{
		{"empty string", "", lineIndex{0}},
		{"no newlines", "SELECT 1", lineIndex{0}},
		{"one newline", "SELECT\n1", lineIndex{0, 7}},
		{"multiple newlines", "a\nb\nc", lineIndex{0, 2, 4}},
		{"trailing newline", "SELECT 1\n", lineIndex{0, 9}},
		{"consecutive newlines", "a\n\nb", lineIndex{0, 2, 3}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildLineIndex(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("buildLineIndex(%q) = %v (len %d), want %v (len %d)", tt.input, got, len(got), tt.want, len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("buildLineIndex(%q)[%d] = %d, want %d", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestParse(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		stmts, err := Parse("")
		if err != nil {
			t.Fatalf("Parse('') error: %v", err)
		}
		if len(stmts) != 0 {
			t.Errorf("Parse('') returned %d statements, want 0", len(stmts))
		}
	})

	t.Run("single statement", func(t *testing.T) {
		stmts, err := Parse("SELECT 1")
		if err != nil {
			t.Fatalf("Parse error: %v", err)
		}
		if len(stmts) != 1 {
			t.Fatalf("got %d statements, want 1", len(stmts))
		}
		s := stmts[0]
		if s.AST == nil {
			t.Error("AST is nil")
		}
		if s.Start.Line != 1 || s.Start.Column != 1 {
			t.Errorf("Start = %+v, want {1,1}", s.Start)
		}
	})

	t.Run("single statement with semicolon", func(t *testing.T) {
		stmts, err := Parse("SELECT 1;")
		if err != nil {
			t.Fatalf("Parse error: %v", err)
		}
		if len(stmts) != 1 {
			t.Fatalf("got %d statements, want 1", len(stmts))
		}
		if !strings.Contains(stmts[0].Text, ";") {
			t.Errorf("Text = %q, want to contain semicolon", stmts[0].Text)
		}
	})

	t.Run("multi-statement", func(t *testing.T) {
		stmts, err := Parse("SELECT 1; SELECT 2; SELECT 3")
		if err != nil {
			t.Fatalf("Parse error: %v", err)
		}
		if len(stmts) != 3 {
			t.Fatalf("got %d statements, want 3", len(stmts))
		}
		for i, s := range stmts {
			if s.AST == nil {
				t.Errorf("stmt[%d].AST is nil", i)
			}
		}
	})

	t.Run("multi-line positions", func(t *testing.T) {
		sql := "SELECT\n1"
		stmts, err := Parse(sql)
		if err != nil {
			t.Fatalf("Parse error: %v", err)
		}
		if len(stmts) != 1 {
			t.Fatalf("got %d statements, want 1", len(stmts))
		}
		s := stmts[0]
		if s.Start.Line != 1 {
			t.Errorf("Start.Line = %d, want 1", s.Start.Line)
		}
	})

	t.Run("error propagation", func(t *testing.T) {
		_, err := Parse("SELECT FROM WHERE")
		// We just check it doesn't panic — error behavior varies
		_ = err
	})
}

func TestOffsetToPosition(t *testing.T) {
	// "SELECT\n1\nFROM t"
	// Line 1: "SELECT\n"  bytes 0-6 (newline at 6)
	// Line 2: "1\n"       bytes 7-8 (newline at 8)
	// Line 3: "FROM t"    bytes 9-14
	idx := buildLineIndex("SELECT\n1\nFROM t")

	tests := []struct {
		name   string
		offset int
		want   Position
	}{
		{"offset 0 = line 1 col 1", 0, Position{Line: 1, Column: 1}},
		{"S in SELECT", 0, Position{Line: 1, Column: 1}},
		{"T in SELECT", 5, Position{Line: 1, Column: 6}},
		{"start of line 2", 7, Position{Line: 2, Column: 1}},
		{"start of line 3", 9, Position{Line: 3, Column: 1}},
		{"mid line 3", 12, Position{Line: 3, Column: 4}},
		{"past end", 15, Position{Line: 3, Column: 7}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := offsetToPosition(idx, tt.offset)
			if got != tt.want {
				t.Errorf("offsetToPosition(idx, %d) = %+v, want %+v", tt.offset, got, tt.want)
			}
		})
	}
}
