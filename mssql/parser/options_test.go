package parser

import (
	"testing"
)

// TestOptionValidation verifies the option validation framework:
// valid options are accepted, invalid options are rejected.
func TestOptionValidation(t *testing.T) {
	// Define a test option set with a few keyword tokens and one ident-only name.
	testOpts := newOptionSet(
		kwENCRYPTION,
		kwSCHEMABINDING,
		kwRECOMPILE,
	).withIdents("CUSTOM_OPT", "MY_SETTING")

	tests := []struct {
		name  string
		input string // single token to parse
		valid bool
	}{
		// Keyword tokens that are in the set — should be valid.
		{"keyword ENCRYPTION", "ENCRYPTION", true},
		{"keyword SCHEMABINDING", "SCHEMABINDING", true},
		{"keyword RECOMPILE", "RECOMPILE", true},

		// Identifier strings in the idents set — should be valid.
		{"ident CUSTOM_OPT", "CUSTOM_OPT", true},
		{"ident MY_SETTING", "MY_SETTING", true},
		{"ident my_setting lowercase", "my_setting", true},

		// Keywords NOT in the set — should be rejected.
		{"keyword SELECT rejected", "SELECT", false},
		{"keyword FROM rejected", "FROM", false},
		{"keyword INSERT rejected", "INSERT", false},

		// Random identifier NOT in any set — should be rejected.
		{"ident BOGUS rejected", "BOGUS", false},
		{"ident random rejected", "xyzzy", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := &Parser{
				lexer: NewLexer(tc.input),
			}
			p.advance()

			got := p.isValidOption(testOpts)
			if got != tc.valid {
				t.Errorf("isValidOption(%q) = %v, want %v (token type %d)",
					tc.input, got, tc.valid, p.cur.Type)
			}

			// Also test expectOption: should succeed for valid, fail for invalid.
			p2 := &Parser{
				lexer: NewLexer(tc.input),
			}
			p2.advance()

			name, err := p2.expectOption(testOpts)
			if tc.valid {
				if err != nil {
					t.Errorf("expectOption(%q) returned unexpected error: %v", tc.input, err)
				}
				if name == "" {
					t.Errorf("expectOption(%q) returned empty name", tc.input)
				}
			} else {
				if err == nil {
					t.Errorf("expectOption(%q) should have returned error, got name=%q", tc.input, name)
				}
			}
		})
	}
}
