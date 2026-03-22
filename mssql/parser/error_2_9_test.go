package parser

import "testing"

func TestErrorSection2_9(t *testing.T) {
	tests := []struct {
		name string
		sql  string
	}{
		{"ALTER_TABLE_no_name", "ALTER TABLE"},
		{"ALTER_TABLE_ADD_no_col", "ALTER TABLE t ADD"},
		{"ALTER_TABLE_DROP_COLUMN_no_name", "ALTER TABLE t DROP COLUMN"},
		{"ALTER_TABLE_ALTER_COLUMN_no_name", "ALTER TABLE t ALTER COLUMN"},
		{"ALTER_TABLE_ALTER_COLUMN_no_type", "ALTER TABLE t ALTER COLUMN a"},
		{"CREATE_INDEX_no_name", "CREATE INDEX"},
		{"CREATE_INDEX_ON_no_table", "CREATE INDEX ix ON"},
		{"CREATE_INDEX_open_paren_no_cols", "CREATE INDEX ix ON t ("},
		{"CREATE_UNIQUE_INDEX_no_name", "CREATE UNIQUE INDEX"},
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
