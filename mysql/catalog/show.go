package catalog

import (
	"fmt"
	"strings"
)

// defaultCollationForCharset returns the default collation for common MySQL charsets.
var defaultCollationForCharset = map[string]string{
	"utf8mb4":  "utf8mb4_0900_ai_ci",
	"utf8mb3":  "utf8mb3_general_ci",
	"utf8":     "utf8mb3_general_ci",
	"latin1":   "latin1_swedish_ci",
	"ascii":    "ascii_general_ci",
	"binary":   "binary",
	"gbk":      "gbk_chinese_ci",
	"big5":     "big5_chinese_ci",
	"euckr":    "euckr_korean_ci",
	"gb2312":   "gb2312_chinese_ci",
	"sjis":     "sjis_japanese_ci",
	"cp1252":   "cp1252_general_ci",
	"ucs2":     "ucs2_general_ci",
	"utf16":    "utf16_general_ci",
	"utf16le":  "utf16le_general_ci",
	"utf32":    "utf32_general_ci",
	"cp932":    "cp932_japanese_ci",
	"eucjpms":  "eucjpms_japanese_ci",
	"gb18030":  "gb18030_chinese_ci",
	"geostd8":  "geostd8_general_ci",
	"tis620":   "tis620_thai_ci",
	"hebrew":   "hebrew_general_ci",
	"greek":    "greek_general_ci",
	"armscii8": "armscii8_general_ci",
}

// ShowCreateTable produces MySQL 8.0-compatible SHOW CREATE TABLE output.
// Returns "" if the database or table does not exist.
func (c *Catalog) ShowCreateTable(database, table string) string {
	db := c.GetDatabase(database)
	if db == nil {
		return ""
	}
	tbl := db.GetTable(table)
	if tbl == nil {
		return ""
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("CREATE TABLE `%s` (\n", tbl.Name))

	// Columns.
	parts := make([]string, 0, len(tbl.Columns)+len(tbl.Indexes)+len(tbl.Constraints))
	for _, col := range tbl.Columns {
		parts = append(parts, showColumn(col))
	}

	// Indexes.
	for _, idx := range tbl.Indexes {
		parts = append(parts, showIndex(idx))
	}

	// Constraints (FK and CHECK only — PK/UNIQUE are shown via indexes).
	for _, con := range tbl.Constraints {
		if con.Type == ConForeignKey || con.Type == ConCheck {
			parts = append(parts, showConstraint(con))
		}
	}

	b.WriteString("  ")
	b.WriteString(strings.Join(parts, ",\n  "))
	b.WriteString("\n)")

	// Table options.
	opts := showTableOptions(tbl)
	if opts != "" {
		b.WriteString(" ")
		b.WriteString(opts)
	}

	return b.String()
}

func showColumn(col *Column) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("`%s` %s", col.Name, col.ColumnType))

	// Generated column.
	if col.Generated != nil {
		mode := "VIRTUAL"
		if col.Generated.Stored {
			mode = "STORED"
		}
		b.WriteString(fmt.Sprintf(" GENERATED ALWAYS AS (%s) %s", col.Generated.Expr, mode))
		if !col.Nullable {
			b.WriteString(" NOT NULL")
		}
		if col.Comment != "" {
			b.WriteString(fmt.Sprintf(" COMMENT '%s'", escapeComment(col.Comment)))
		}
		if col.Invisible {
			b.WriteString(" /*!80023 INVISIBLE */")
		}
		return b.String()
	}

	// NOT NULL.
	if !col.Nullable {
		b.WriteString(" NOT NULL")
	}

	// DEFAULT.
	if col.Default != nil {
		b.WriteString(" DEFAULT ")
		b.WriteString(formatDefault(*col.Default, col.Nullable))
	} else if col.Nullable && !col.AutoIncrement {
		b.WriteString(" DEFAULT NULL")
	}

	// AUTO_INCREMENT.
	if col.AutoIncrement {
		b.WriteString(" AUTO_INCREMENT")
	}

	// ON UPDATE.
	if col.OnUpdate != "" {
		b.WriteString(fmt.Sprintf(" ON UPDATE %s", col.OnUpdate))
	}

	// COMMENT.
	if col.Comment != "" {
		b.WriteString(fmt.Sprintf(" COMMENT '%s'", escapeComment(col.Comment)))
	}

	// INVISIBLE.
	if col.Invisible {
		b.WriteString(" /*!80023 INVISIBLE */")
	}

	return b.String()
}

// formatDefault formats a default value for SHOW CREATE TABLE output.
func formatDefault(val string, nullable bool) string {
	if strings.EqualFold(val, "NULL") {
		return "NULL"
	}
	// Check if it's a numeric literal.
	if isNumericLiteral(val) {
		return val
	}
	// Check for special expressions (CURRENT_TIMESTAMP, etc.)
	upper := strings.ToUpper(val)
	if upper == "CURRENT_TIMESTAMP" || strings.HasPrefix(upper, "CURRENT_TIMESTAMP(") {
		return val
	}
	if upper == "NOW()" {
		return val
	}
	// Already single-quoted string — return as-is.
	if len(val) >= 2 && val[0] == '\'' && val[len(val)-1] == '\'' {
		return val
	}
	// Otherwise quote as string.
	return "'" + val + "'"
}

func isNumericLiteral(s string) bool {
	if len(s) == 0 {
		return false
	}
	start := 0
	if s[0] == '-' || s[0] == '+' {
		start = 1
		if start >= len(s) {
			return false
		}
	}
	hasDot := false
	for i := start; i < len(s); i++ {
		if s[i] == '.' {
			if hasDot {
				return false
			}
			hasDot = true
		} else if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func showIndex(idx *Index) string {
	var b strings.Builder

	if idx.Primary {
		b.WriteString("PRIMARY KEY (")
	} else if idx.Unique {
		b.WriteString(fmt.Sprintf("UNIQUE KEY `%s` (", idx.Name))
	} else if idx.Fulltext {
		b.WriteString(fmt.Sprintf("FULLTEXT KEY `%s` (", idx.Name))
	} else if idx.Spatial {
		b.WriteString(fmt.Sprintf("SPATIAL KEY `%s` (", idx.Name))
	} else {
		b.WriteString(fmt.Sprintf("KEY `%s` (", idx.Name))
	}

	cols := make([]string, 0, len(idx.Columns))
	for _, ic := range idx.Columns {
		cols = append(cols, showIndexColumn(ic))
	}
	b.WriteString(strings.Join(cols, ","))
	b.WriteString(")")

	// USING clause: only for non-BTREE types, and not for PRIMARY KEY.
	if !idx.Primary && idx.IndexType != "" && !strings.EqualFold(idx.IndexType, "BTREE") {
		b.WriteString(fmt.Sprintf(" USING %s", strings.ToUpper(idx.IndexType)))
	}

	// Comment.
	if idx.Comment != "" {
		b.WriteString(fmt.Sprintf(" COMMENT '%s'", escapeComment(idx.Comment)))
	}

	// Invisible.
	if !idx.Visible {
		b.WriteString(" /*!80000 INVISIBLE */")
	}

	return b.String()
}

func showIndexColumn(ic *IndexColumn) string {
	var b strings.Builder
	if ic.Expr != "" {
		b.WriteString(fmt.Sprintf("(%s)", ic.Expr))
	} else {
		b.WriteString(fmt.Sprintf("`%s`", ic.Name))
		if ic.Length > 0 {
			b.WriteString(fmt.Sprintf("(%d)", ic.Length))
		}
	}
	if ic.Descending {
		b.WriteString(" DESC")
	}
	return b.String()
}

func showConstraint(con *Constraint) string {
	var b strings.Builder

	switch con.Type {
	case ConForeignKey:
		b.WriteString(fmt.Sprintf("CONSTRAINT `%s` FOREIGN KEY (", con.Name))
		cols := make([]string, 0, len(con.Columns))
		for _, c := range con.Columns {
			cols = append(cols, fmt.Sprintf("`%s`", c))
		}
		b.WriteString(strings.Join(cols, ","))
		b.WriteString(fmt.Sprintf(") REFERENCES `%s` (", con.RefTable))
		refCols := make([]string, 0, len(con.RefColumns))
		for _, c := range con.RefColumns {
			refCols = append(refCols, fmt.Sprintf("`%s`", c))
		}
		b.WriteString(strings.Join(refCols, ","))
		b.WriteString(")")

		// ON DELETE — omit if RESTRICT or NO ACTION (MySQL defaults).
		if con.OnDelete != "" && !isFKDefault(con.OnDelete) {
			b.WriteString(fmt.Sprintf(" ON DELETE %s", strings.ToUpper(con.OnDelete)))
		}
		// ON UPDATE — omit if RESTRICT or NO ACTION (MySQL defaults).
		if con.OnUpdate != "" && !isFKDefault(con.OnUpdate) {
			b.WriteString(fmt.Sprintf(" ON UPDATE %s", strings.ToUpper(con.OnUpdate)))
		}

	case ConCheck:
		b.WriteString(fmt.Sprintf("CONSTRAINT `%s` CHECK (%s)", con.Name, con.CheckExpr))
		if con.NotEnforced {
			b.WriteString(" /*!80016 NOT ENFORCED */")
		}
	}

	return b.String()
}

func showTableOptions(tbl *Table) string {
	var parts []string

	if tbl.Engine != "" {
		parts = append(parts, fmt.Sprintf("ENGINE=%s", tbl.Engine))
	}

	if tbl.Charset != "" {
		parts = append(parts, fmt.Sprintf("DEFAULT CHARSET=%s", tbl.Charset))
	}

	// Show collation only if not the default for the charset.
	if tbl.Collation != "" && tbl.Charset != "" {
		defColl, ok := defaultCollationForCharset[toLower(tbl.Charset)]
		if !ok || !strings.EqualFold(tbl.Collation, defColl) {
			parts = append(parts, fmt.Sprintf("COLLATE=%s", tbl.Collation))
		}
	} else if tbl.Collation != "" {
		parts = append(parts, fmt.Sprintf("COLLATE=%s", tbl.Collation))
	}

	if tbl.Comment != "" {
		parts = append(parts, fmt.Sprintf("COMMENT='%s'", escapeComment(tbl.Comment)))
	}

	return strings.Join(parts, " ")
}

// isFKDefault returns true if the action is a MySQL FK default (RESTRICT or NO ACTION).
func isFKDefault(action string) bool {
	upper := strings.ToUpper(action)
	return upper == "RESTRICT" || upper == "NO ACTION"
}

func escapeComment(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "'", "\\'")
	return s
}
