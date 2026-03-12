// Package parser - external.go implements T-SQL EXTERNAL object statement parsing.
package parser

import (
	"strings"

	nodes "github.com/bytebase/omni/mssql/ast"
)

// parseCreateExternalDataSourceStmt parses CREATE EXTERNAL DATA SOURCE.
//
// Ref: https://learn.microsoft.com/en-us/sql/t-sql/statements/create-external-data-source-transact-sql
//
//	CREATE EXTERNAL DATA SOURCE <data_source_name>
//	WITH
//	  ( [ LOCATION = '<prefix>://<path>[:<port>]' ]
//	    [ [ , ] CONNECTION_OPTIONS = '<key_value_pairs>'[,...] ]
//	    [ [ , ] CREDENTIAL = <credential_name> ]
//	    [ [ , ] PUSHDOWN = { ON | OFF } ]
//	    [ [ , ] TYPE = { HADOOP | BLOB_STORAGE } ]
//	    [ [ , ] RESOURCE_MANAGER_LOCATION = '<resource_manager>[:<port>]' ]
//	  )
//	[ ; ]
func (p *Parser) parseCreateExternalDataSourceStmt() *nodes.SecurityStmt {
	loc := p.pos()
	// EXTERNAL DATA SOURCE already consumed by caller

	stmt := &nodes.SecurityStmt{
		Action:     "CREATE",
		ObjectType: "EXTERNAL DATA SOURCE",
		Loc:        nodes.Loc{Start: loc},
	}

	// data_source_name
	if p.isIdentLike() || p.cur.Type == tokSCONST {
		stmt.Name = p.cur.Str
		p.advance()
	}

	// WITH ( options )
	stmt.Options = p.parseExternalWithOptions()

	stmt.Loc.End = p.pos()
	return stmt
}

// parseAlterExternalDataSourceStmt parses ALTER EXTERNAL DATA SOURCE.
//
// Ref: https://learn.microsoft.com/en-us/sql/t-sql/statements/alter-external-data-source-transact-sql
//
//	ALTER EXTERNAL DATA SOURCE data_source_name SET
//	    {
//	        LOCATION = '<prefix>://<path>[:<port>]' [,] |
//	        RESOURCE_MANAGER_LOCATION = '<IP address;Port>' [,] |
//	        CREDENTIAL = credential_name
//	    }
//	[ ; ]
func (p *Parser) parseAlterExternalDataSourceStmt() *nodes.SecurityStmt {
	loc := p.pos()
	// EXTERNAL DATA SOURCE already consumed by caller

	stmt := &nodes.SecurityStmt{
		Action:     "ALTER",
		ObjectType: "EXTERNAL DATA SOURCE",
		Loc:        nodes.Loc{Start: loc},
	}

	// data_source_name
	if p.isIdentLike() || p.cur.Type == tokSCONST {
		stmt.Name = p.cur.Str
		p.advance()
	}

	// SET key = value [, key = value ...]
	if p.cur.Type == kwSET {
		p.advance()
	}

	stmt.Options = p.parseExternalSetOptions()

	stmt.Loc.End = p.pos()
	return stmt
}

// parseDropExternalStmt parses DROP EXTERNAL { DATA SOURCE | TABLE | FILE FORMAT }.
//
// Ref: https://learn.microsoft.com/en-us/sql/t-sql/statements/drop-external-data-source-transact-sql
//
//	DROP EXTERNAL DATA SOURCE external_data_source_name
//	DROP EXTERNAL TABLE [ database_name . [ schema_name ] . | schema_name . ] table_name
//	DROP EXTERNAL FILE FORMAT external_file_format_name
func (p *Parser) parseDropExternalStmt() *nodes.SecurityStmt {
	loc := p.pos()
	// EXTERNAL already consumed by caller

	stmt := &nodes.SecurityStmt{
		Action: "DROP",
		Loc:    nodes.Loc{Start: loc},
	}

	// DATA SOURCE | TABLE | FILE FORMAT
	if p.isIdentLike() && matchesKeywordCI(p.cur.Str, "DATA") {
		p.advance() // consume DATA
		if p.isIdentLike() && matchesKeywordCI(p.cur.Str, "SOURCE") {
			p.advance() // consume SOURCE
		}
		stmt.ObjectType = "EXTERNAL DATA SOURCE"
	} else if p.cur.Type == kwTABLE {
		p.advance() // consume TABLE
		stmt.ObjectType = "EXTERNAL TABLE"
	} else if p.cur.Type == kwFILE || (p.isIdentLike() && matchesKeywordCI(p.cur.Str, "FILE")) {
		p.advance() // consume FILE
		if p.isIdentLike() && matchesKeywordCI(p.cur.Str, "FORMAT") {
			p.advance() // consume FORMAT
		}
		stmt.ObjectType = "EXTERNAL FILE FORMAT"
	}

	// name (possibly qualified)
	if p.isIdentLike() || p.cur.Type == tokSCONST {
		var parts []string
		parts = append(parts, p.cur.Str)
		p.advance()
		for p.cur.Type == '.' {
			p.advance() // consume '.'
			if p.isIdentLike() || p.cur.Type == tokSCONST {
				parts = append(parts, p.cur.Str)
				p.advance()
			}
		}
		stmt.Name = strings.Join(parts, ".")
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseCreateExternalTableStmt parses CREATE EXTERNAL TABLE.
//
// Ref: https://learn.microsoft.com/en-us/sql/t-sql/statements/create-external-table-transact-sql
//
//	CREATE EXTERNAL TABLE { database_name.schema_name.table_name | schema_name.table_name | table_name }
//	    ( <column_definition> [ , ...n ] )
//	    WITH (
//	        LOCATION = 'folder_or_filepath' ,
//	        DATA_SOURCE = external_data_source_name
//	        [ , FILE_FORMAT = external_file_format_name ]
//	        [ , <reject_options> [ , ...n ] ]
//	        [ , TABLE_OPTIONS = N'...' ]
//	        [ , SCHEMA_NAME = N'...' ]
//	        [ , OBJECT_NAME = N'...' ]
//	        [ , DISTRIBUTION = { SHARDED(col) | REPLICATED | ROUND_ROBIN } ]
//	    )
//	[ ; ]
//
//	<column_definition> ::=
//	    column_name <data_type>
//	        [ COLLATE collation_name ]
//	        [ NULL | NOT NULL ]
//
//	<reject_options> ::=
//	{
//	    REJECT_TYPE = { value | percentage }
//	    | REJECT_VALUE = reject_value
//	    | REJECT_SAMPLE_VALUE = reject_sample_value
//	    | REJECTED_ROW_LOCATION = '/REJECT_Directory'
//	}
func (p *Parser) parseCreateExternalTableStmt() *nodes.SecurityStmt {
	loc := p.pos()
	// EXTERNAL TABLE already consumed by caller

	stmt := &nodes.SecurityStmt{
		Action:     "CREATE",
		ObjectType: "EXTERNAL TABLE",
		Loc:        nodes.Loc{Start: loc},
	}

	// table name (possibly qualified: db.schema.table)
	if p.isIdentLike() || p.cur.Type == tokSCONST {
		var parts []string
		parts = append(parts, p.cur.Str)
		p.advance()
		for p.cur.Type == '.' {
			p.advance() // consume '.'
			if p.isIdentLike() || p.cur.Type == tokSCONST {
				parts = append(parts, p.cur.Str)
				p.advance()
			}
		}
		stmt.Name = strings.Join(parts, ".")
	}

	var opts []nodes.Node

	// ( column_definition [ , ... ] )
	if p.cur.Type == '(' {
		p.advance() // consume '('
		depth := 1
		var colBuf strings.Builder
		colBuf.WriteString("COLUMNS=(")
		for depth > 0 && p.cur.Type != tokEOF {
			if p.cur.Type == '(' {
				depth++
				colBuf.WriteString("(")
				p.advance()
			} else if p.cur.Type == ')' {
				depth--
				if depth > 0 {
					colBuf.WriteString(")")
					p.advance()
				}
			} else {
				if colBuf.Len() > len("COLUMNS=(") {
					colBuf.WriteString(" ")
				}
				colBuf.WriteString(strings.ToUpper(p.cur.Str))
				p.advance()
			}
		}
		colBuf.WriteString(")")
		p.match(')') // consume final ')'
		opts = append(opts, &nodes.String{Str: colBuf.String()})
	}

	// WITH ( options )
	withOpts := p.parseExternalWithOptions()
	if withOpts != nil {
		opts = append(opts, withOpts.Items...)
	}

	if len(opts) > 0 {
		stmt.Options = &nodes.List{Items: opts}
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseCreateExternalFileFormatStmt parses CREATE EXTERNAL FILE FORMAT.
//
// Ref: https://learn.microsoft.com/en-us/sql/t-sql/statements/create-external-file-format-transact-sql
//
//	CREATE EXTERNAL FILE FORMAT file_format_name
//	WITH (
//	    FORMAT_TYPE = { DELIMITEDTEXT | RCFILE | ORC | PARQUET | JSON | DELTA }
//	    [ , FORMAT_OPTIONS ( <format_options> [ ,...n ] ) ]
//	    [ , SERDE_METHOD = 'SerDe_method' ]  -- for RCFILE
//	    [ , DATA_COMPRESSION = 'compression_codec' ]
//	)
//	[ ; ]
//
//	<format_options> ::=
//	{
//	    FIELD_TERMINATOR = field_terminator
//	    | STRING_DELIMITER = string_delimiter
//	    | FIRST_ROW = integer
//	    | DATE_FORMAT = datetime_format
//	    | USE_TYPE_DEFAULT = { TRUE | FALSE }
//	    | ENCODING = { 'UTF8' | 'UTF16' }
//	    | PARSER_VERSION = 'parser_version'
//	}
func (p *Parser) parseCreateExternalFileFormatStmt() *nodes.SecurityStmt {
	loc := p.pos()
	// EXTERNAL FILE FORMAT already consumed by caller

	stmt := &nodes.SecurityStmt{
		Action:     "CREATE",
		ObjectType: "EXTERNAL FILE FORMAT",
		Loc:        nodes.Loc{Start: loc},
	}

	// file_format_name
	if p.isIdentLike() || p.cur.Type == tokSCONST {
		stmt.Name = p.cur.Str
		p.advance()
	}

	// WITH ( options )
	stmt.Options = p.parseExternalWithOptions()

	stmt.Loc.End = p.pos()
	return stmt
}

// parseExternalSetOptions parses SET key = value [, key = value ...] (no parentheses).
// Used by ALTER EXTERNAL DATA SOURCE.
func (p *Parser) parseExternalSetOptions() *nodes.List {
	var opts []nodes.Node

	for p.cur.Type != ';' && p.cur.Type != tokEOF && p.cur.Type != kwGO {
		// skip commas
		if p.cur.Type == ',' {
			p.advance()
			continue
		}
		if p.isIdentLike() || p.cur.Type == kwON || p.cur.Type == kwOFF {
			key := strings.ToUpper(p.cur.Str)
			p.advance()
			if p.cur.Type == '=' {
				p.advance() // consume '='
				val := p.parseExternalOptionValue()
				opts = append(opts, &nodes.String{Str: key + "=" + val})
			} else {
				opts = append(opts, &nodes.String{Str: key})
			}
		} else {
			break
		}
	}

	if len(opts) == 0 {
		return nil
	}
	return &nodes.List{Items: opts}
}

// parseExternalWithOptions parses a WITH ( key = value [, ...] ) clause,
// handling nested parentheses (e.g., FORMAT_OPTIONS(...), DISTRIBUTION = SHARDED(col)).
func (p *Parser) parseExternalWithOptions() *nodes.List {
	var opts []nodes.Node

	if p.cur.Type != kwWITH {
		return nil
	}
	p.advance() // consume WITH

	if p.cur.Type != '(' {
		return nil
	}
	p.advance() // consume '('

	for p.cur.Type != ')' && p.cur.Type != tokEOF {
		// skip commas
		if p.cur.Type == ',' {
			p.advance()
			continue
		}

		// key = value
		if p.isIdentLike() || p.cur.Type == kwON || p.cur.Type == kwOFF {
			key := strings.ToUpper(p.cur.Str)
			p.advance()
			if p.cur.Type == '=' {
				p.advance() // consume '='
				val := p.parseExternalOptionValue()
				opts = append(opts, &nodes.String{Str: key + "=" + val})
			} else if p.cur.Type == '(' {
				// FORMAT_OPTIONS ( ... )
				inner := p.parseNestedParens()
				opts = append(opts, &nodes.String{Str: key + "(" + inner + ")"})
			} else {
				opts = append(opts, &nodes.String{Str: key})
			}
		} else {
			// skip unexpected tokens
			p.advance()
		}
	}
	p.match(')') // consume ')'

	if len(opts) == 0 {
		return nil
	}
	return &nodes.List{Items: opts}
}

// parseExternalOptionValue reads one option value, which may be:
// - a string literal
// - a numeric literal
// - an identifier (e.g., ON, OFF, HADOOP, PARQUET)
// - SHARDED(column_name) or similar function-like syntax
func (p *Parser) parseExternalOptionValue() string {
	var val string
	switch {
	case p.cur.Type == tokSCONST:
		val = "'" + p.cur.Str + "'"
		p.advance()
	case p.cur.Type == tokICONST || p.cur.Type == tokFCONST:
		val = p.cur.Str
		p.advance()
	case p.cur.Type == kwON:
		val = "ON"
		p.advance()
	case p.cur.Type == kwOFF:
		val = "OFF"
		p.advance()
	case p.cur.Type == kwNULL:
		val = "NULL"
		p.advance()
	case p.isIdentLike():
		val = strings.ToUpper(p.cur.Str)
		p.advance()
		// check for function-like syntax: SHARDED(col)
		if p.cur.Type == '(' {
			inner := p.parseNestedParens()
			val += "(" + inner + ")"
		}
	default:
		val = p.cur.Str
		p.advance()
	}
	return val
}

// parseNestedParens consumes content between ( and ) including nested parens.
func (p *Parser) parseNestedParens() string {
	if p.cur.Type != '(' {
		return ""
	}
	p.advance() // consume '('
	var buf strings.Builder
	depth := 1
	first := true
	for depth > 0 && p.cur.Type != tokEOF {
		if p.cur.Type == '(' {
			depth++
			buf.WriteString("(")
			p.advance()
			first = true
			continue
		}
		if p.cur.Type == ')' {
			depth--
			if depth > 0 {
				buf.WriteString(")")
				p.advance()
			}
			continue
		}
		if !first {
			buf.WriteString(" ")
		}
		first = false
		if p.cur.Type == ',' {
			// replace comma with a comma-space representation
			buf.WriteString(",")
			p.advance()
			continue
		}
		if p.cur.Type == '=' {
			buf.WriteString("=")
			p.advance()
			continue
		}
		if p.cur.Type == tokSCONST {
			buf.WriteString("'" + p.cur.Str + "'")
		} else {
			buf.WriteString(strings.ToUpper(p.cur.Str))
		}
		p.advance()
	}
	p.match(')') // consume final ')'
	return buf.String()
}
