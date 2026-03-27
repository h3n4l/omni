package mssql

import "github.com/bytebase/omni/mssql/ast"

// Statement is the result of parsing a single SQL statement.
type Statement struct {
	// Text is the SQL text including trailing semicolon if present.
	Text string
	// AST is the inner statement node. Nil for empty statements.
	AST ast.Node

	// ByteStart is the inclusive start byte offset in the original SQL.
	ByteStart int
	// ByteEnd is the exclusive end byte offset in the original SQL.
	ByteEnd int

	// Start is the start position (line:column) in the original SQL.
	Start Position
	// End is the exclusive end position (line:column) in the original SQL.
	End Position
}

// Position represents a location in source text.
type Position struct {
	// Line is 1-based line number.
	Line int
	// Column is 1-based column in bytes.
	Column int
}

// Empty returns true if this statement has no meaningful content.
func (s *Statement) Empty() bool {
	return s.AST == nil
}
