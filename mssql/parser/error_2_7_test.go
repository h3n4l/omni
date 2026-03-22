package parser

import "testing"

func TestErrorSection2_7(t *testing.T) {
	tests := []struct {
		name string
		sql  string
	}{
		{"INSERT_INTO_no_table", "INSERT INTO"},
		{"INSERT_INTO_VALUES_open_paren", "INSERT INTO t VALUES ("},
		{"INSERT_INTO_col_list_open_paren", "INSERT INTO t ("},
		{"UPDATE_no_table", "UPDATE"},
		{"UPDATE_SET_no_assignments", "UPDATE t SET"},
		{"UPDATE_SET_eq_no_value", "UPDATE t SET a ="},
		{"DELETE_FROM_no_table", "DELETE FROM"},
		{"MERGE_INTO_no_target", "MERGE INTO"},
		{"MERGE_USING_no_source", "MERGE INTO t USING"},
		{"MERGE_USING_ON_no_condition", "MERGE INTO t USING s ON"},
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
