package pg

import (
	"testing"
)

func TestSplit(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		want     []Segment
		empties  []bool // expected Empty() for each segment
	}{
		// Basic cases.
		{
			name:  "single statement with semicolon",
			input: "SELECT 1;",
			want: []Segment{
				{Text: "SELECT 1;", ByteStart: 0, ByteEnd: 9},
			},
			empties: []bool{false},
		},
		{
			name:  "single statement without semicolon",
			input: "SELECT 1",
			want: []Segment{
				{Text: "SELECT 1", ByteStart: 0, ByteEnd: 8},
			},
			empties: []bool{false},
		},
		{
			name:  "multiple statements",
			input: "SELECT 1; SELECT 2; SELECT 3",
			want: []Segment{
				{Text: "SELECT 1;", ByteStart: 0, ByteEnd: 9},
				{Text: " SELECT 2;", ByteStart: 9, ByteEnd: 19},
				{Text: " SELECT 3", ByteStart: 19, ByteEnd: 28},
			},
			empties: []bool{false, false, false},
		},
		{
			name:  "empty input",
			input: "",
			want:  nil,
		},
		{
			name:  "whitespace only",
			input: "  \t\n  ",
			want: []Segment{
				{Text: "  \t\n  ", ByteStart: 0, ByteEnd: 6},
			},
			empties: []bool{true},
		},

		// String/identifier handling.
		{
			name:  "semicolon inside single-quoted string",
			input: "SELECT 'a;b'; SELECT 2",
			want: []Segment{
				{Text: "SELECT 'a;b';", ByteStart: 0, ByteEnd: 13},
				{Text: " SELECT 2", ByteStart: 13, ByteEnd: 22},
			},
			empties: []bool{false, false},
		},
		{
			name:  "semicolon inside double-quoted identifier",
			input: `SELECT "a;b"; SELECT 2`,
			want: []Segment{
				{Text: `SELECT "a;b";`, ByteStart: 0, ByteEnd: 13},
				{Text: " SELECT 2", ByteStart: 13, ByteEnd: 22},
			},
			empties: []bool{false, false},
		},
		{
			name:  "semicolon inside dollar-quoted string",
			input: "SELECT $$a;b$$; SELECT 2",
			want: []Segment{
				{Text: "SELECT $$a;b$$;", ByteStart: 0, ByteEnd: 15},
				{Text: " SELECT 2", ByteStart: 15, ByteEnd: 24},
			},
			empties: []bool{false, false},
		},
		{
			name:  "tagged dollar-quote",
			input: "SELECT $tag$a;b$tag$; SELECT 2",
			want: []Segment{
				{Text: "SELECT $tag$a;b$tag$;", ByteStart: 0, ByteEnd: 21},
				{Text: " SELECT 2", ByteStart: 21, ByteEnd: 30},
			},
			empties: []bool{false, false},
		},

		// Comment handling.
		{
			name:  "semicolon inside block comment",
			input: "SELECT /* ; */ 1; SELECT 2",
			want: []Segment{
				{Text: "SELECT /* ; */ 1;", ByteStart: 0, ByteEnd: 17},
				{Text: " SELECT 2", ByteStart: 17, ByteEnd: 26},
			},
			empties: []bool{false, false},
		},
		{
			name:  "semicolon inside line comment",
			input: "SELECT 1 -- ;\n; SELECT 2",
			want: []Segment{
				{Text: "SELECT 1 -- ;\n;", ByteStart: 0, ByteEnd: 15},
				{Text: " SELECT 2", ByteStart: 15, ByteEnd: 24},
			},
			empties: []bool{false, false},
		},
		{
			name:  "nested block comments",
			input: "SELECT /* /* ; */ */ 1; SELECT 2",
			want: []Segment{
				{Text: "SELECT /* /* ; */ */ 1;", ByteStart: 0, ByteEnd: 23},
				{Text: " SELECT 2", ByteStart: 23, ByteEnd: 32},
			},
			empties: []bool{false, false},
		},
		{
			name:  "comments only",
			input: "-- comment\n/* block */",
			want: []Segment{
				{Text: "-- comment\n/* block */", ByteStart: 0, ByteEnd: 22},
			},
			empties: []bool{true},
		},

		// BEGIN ATOMIC.
		{
			name:  "simple BEGIN ATOMIC END",
			input: "CREATE FUNCTION f() BEGIN ATOMIC SELECT 1; SELECT 2; END; SELECT 3",
			want: []Segment{
				{Text: "CREATE FUNCTION f() BEGIN ATOMIC SELECT 1; SELECT 2; END;", ByteStart: 0, ByteEnd: 57},
				{Text: " SELECT 3", ByteStart: 57, ByteEnd: 66},
			},
			empties: []bool{false, false},
		},
		{
			name:  "BEGIN ATOMIC with CASE END inside",
			input: "BEGIN ATOMIC CASE WHEN true THEN 1 END; END; SELECT 1",
			want: []Segment{
				{Text: "BEGIN ATOMIC CASE WHEN true THEN 1 END; END;", ByteStart: 0, ByteEnd: 44},
				{Text: " SELECT 1", ByteStart: 44, ByteEnd: 53},
			},
			empties: []bool{false, false},
		},
		{
			name:  "BEGIN ATOMIC with nested CASE",
			input: "BEGIN ATOMIC CASE WHEN x THEN CASE WHEN y THEN 1 END END; END; SELECT 1",
			want: []Segment{
				{Text: "BEGIN ATOMIC CASE WHEN x THEN CASE WHEN y THEN 1 END END; END;", ByteStart: 0, ByteEnd: 62},
				{Text: " SELECT 1", ByteStart: 62, ByteEnd: 71},
			},
			empties: []bool{false, false},
		},
		{
			name:  "case insensitive begin atomic",
			input: "begin atomic select 1; end; SELECT 2",
			want: []Segment{
				{Text: "begin atomic select 1; end;", ByteStart: 0, ByteEnd: 27},
				{Text: " SELECT 2", ByteStart: 27, ByteEnd: 36},
			},
			empties: []bool{false, false},
		},
		{
			name:  "BEGIN without ATOMIC is normal transaction",
			input: "BEGIN; SELECT 1; COMMIT;",
			want: []Segment{
				{Text: "BEGIN;", ByteStart: 0, ByteEnd: 6},
				{Text: " SELECT 1;", ByteStart: 6, ByteEnd: 16},
				{Text: " COMMIT;", ByteStart: 16, ByteEnd: 24},
			},
			empties: []bool{false, false, false},
		},
		{
			name:  "BEGIN comment ATOMIC enters block mode",
			input: "BEGIN /* comment */ ATOMIC SELECT 1; END; SELECT 2",
			want: []Segment{
				{Text: "BEGIN /* comment */ ATOMIC SELECT 1; END;", ByteStart: 0, ByteEnd: 41},
				{Text: " SELECT 2", ByteStart: 41, ByteEnd: 50},
			},
			empties: []bool{false, false},
		},
		{
			name:  "WEEKEND not matched as END",
			input: "BEGIN ATOMIC WEEKEND; END; SELECT 1",
			want: []Segment{
				{Text: "BEGIN ATOMIC WEEKEND; END;", ByteStart: 0, ByteEnd: 26},
				{Text: " SELECT 1", ByteStart: 26, ByteEnd: 35},
			},
			empties: []bool{false, false},
		},

		// Unterminated constructs.
		{
			name:  "unterminated single quote",
			input: "SELECT 'abc; SELECT 2",
			want: []Segment{
				{Text: "SELECT 'abc; SELECT 2", ByteStart: 0, ByteEnd: 21},
			},
			empties: []bool{false},
		},
		{
			name:  "unterminated double quote",
			input: `SELECT "abc; SELECT 2`,
			want: []Segment{
				{Text: `SELECT "abc; SELECT 2`, ByteStart: 0, ByteEnd: 21},
			},
			empties: []bool{false},
		},
		{
			name:  "unterminated dollar quote",
			input: "SELECT $$abc; SELECT 2",
			want: []Segment{
				{Text: "SELECT $$abc; SELECT 2", ByteStart: 0, ByteEnd: 22},
			},
			empties: []bool{false},
		},
		{
			name:  "unterminated block comment",
			input: "SELECT /* abc; SELECT 2",
			want: []Segment{
				{Text: "SELECT /* abc; SELECT 2", ByteStart: 0, ByteEnd: 23},
			},
			empties: []bool{false},
		},

		// Edge cases.
		{
			name:  "trailing content after last semicolon",
			input: "SELECT 1;  ",
			want: []Segment{
				{Text: "SELECT 1;", ByteStart: 0, ByteEnd: 9},
				{Text: "  ", ByteStart: 9, ByteEnd: 11},
			},
			empties: []bool{false, true},
		},
		{
			name:  "multiple semicolons in a row",
			input: ";;;",
			want: []Segment{
				{Text: ";", ByteStart: 0, ByteEnd: 1},
				{Text: ";", ByteStart: 1, ByteEnd: 2},
				{Text: ";", ByteStart: 2, ByteEnd: 3},
			},
			empties: []bool{true, true, true},
		},
		{
			name:  "CRLF line endings",
			input: "SELECT 1;\r\nSELECT 2;",
			want: []Segment{
				{Text: "SELECT 1;", ByteStart: 0, ByteEnd: 9},
				{Text: "\r\nSELECT 2;", ByteStart: 9, ByteEnd: 20},
			},
			empties: []bool{false, false},
		},
		{
			name:  "escaped single quotes",
			input: "SELECT 'it''s'; SELECT 2",
			want: []Segment{
				{Text: "SELECT 'it''s';", ByteStart: 0, ByteEnd: 15},
				{Text: " SELECT 2", ByteStart: 15, ByteEnd: 24},
			},
			empties: []bool{false, false},
		},
		{
			name:  "escaped double quotes",
			input: `SELECT "a""b"; SELECT 2`,
			want: []Segment{
				{Text: `SELECT "a""b";`, ByteStart: 0, ByteEnd: 14},
				{Text: " SELECT 2", ByteStart: 14, ByteEnd: 23},
			},
			empties: []bool{false, false},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Split(tt.input)

			if len(got) != len(tt.want) {
				t.Fatalf("Split(%q) returned %d segments, want %d\ngot:  %+v", tt.input, len(got), len(tt.want), got)
			}

			for i, g := range got {
				w := tt.want[i]
				if g.Text != w.Text || g.ByteStart != w.ByteStart || g.ByteEnd != w.ByteEnd {
					t.Errorf("segment[%d] = %+v, want %+v", i, g, w)
				}
			}

			for i, g := range got {
				if i < len(tt.empties) {
					if g.Empty() != tt.empties[i] {
						t.Errorf("segment[%d].Empty() = %v, want %v (text=%q)", i, g.Empty(), tt.empties[i], g.Text)
					}
				}
			}
		})
	}
}
