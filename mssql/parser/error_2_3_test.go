package parser

import "testing"

func TestErrorSection2_3(t *testing.T) {
	tests := []struct {
		name string
		sql  string
	}{
		{"CAST_open_paren", "SELECT CAST("},
		{"CAST_AS_no_type", "SELECT CAST(1 AS"},
		{"TRY_CAST_open_paren", "SELECT TRY_CAST("},
		{"CONVERT_open_paren", "SELECT CONVERT("},
		{"TRY_CONVERT_open_paren", "SELECT TRY_CONVERT("},
		{"COALESCE_open_paren", "SELECT COALESCE("},
		{"NULLIF_open_paren", "SELECT NULLIF("},
		{"IIF_open_paren", "SELECT IIF("},
		{"CASE_WHEN_no_condition", "SELECT CASE WHEN"},
		{"CASE_WHEN_THEN_no_result", "SELECT CASE WHEN 1=1 THEN"},
		{"EXISTS_open_paren", "SELECT EXISTS ("},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.sql)
			if err == nil {
				t.Errorf("Parse(%q) expected error, got nil", tt.sql)
			}
		})
	}
}
