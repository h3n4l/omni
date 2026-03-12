// Package parser - endpoint.go implements T-SQL ENDPOINT statement parsing.
package parser

import (
	"strings"

	nodes "github.com/bytebase/omni/mssql/ast"
)

// parseCreateEndpointStmt parses CREATE ENDPOINT.
//
// Ref: https://learn.microsoft.com/en-us/sql/t-sql/statements/create-endpoint-transact-sql
//
//	CREATE ENDPOINT endPointName [ AUTHORIZATION login ]
//	    [ STATE = { STARTED | STOPPED | DISABLED } ]
//	    AS { TCP | HTTP } (
//	        <protocol_specific_arguments>
//	    )
//	    FOR { TSQL | SERVICE_BROKER | DATABASE_MIRRORING | SOAP } (
//	        <language_specific_arguments>
//	    )
func (p *Parser) parseCreateEndpointStmt() *nodes.SecurityStmt {
	loc := p.pos()
	// ENDPOINT keyword already consumed by caller

	stmt := &nodes.SecurityStmt{
		Action:     "CREATE",
		ObjectType: "ENDPOINT",
		Loc:        nodes.Loc{Start: loc},
	}

	if p.isIdentLike() || p.cur.Type == tokSCONST {
		stmt.Name = p.cur.Str
		p.advance()
	}

	stmt.Options = p.parseEndpointOptions()

	stmt.Loc.End = p.pos()
	return stmt
}

// parseAlterEndpointStmt parses ALTER ENDPOINT.
//
// Ref: https://learn.microsoft.com/en-us/sql/t-sql/statements/alter-endpoint-transact-sql
//
//	ALTER ENDPOINT endPointName [ AUTHORIZATION login ]
//	    [ STATE = { STARTED | STOPPED | DISABLED } ]
//	    [ AS { TCP | HTTP } (
//	        <protocol_specific_arguments>
//	    ) ]
//	    [ FOR { TSQL | SERVICE_BROKER | DATABASE_MIRRORING | SOAP } (
//	        <language_specific_arguments>
//	    ) ]
func (p *Parser) parseAlterEndpointStmt() *nodes.SecurityStmt {
	loc := p.pos()
	// ENDPOINT keyword already consumed by caller

	stmt := &nodes.SecurityStmt{
		Action:     "ALTER",
		ObjectType: "ENDPOINT",
		Loc:        nodes.Loc{Start: loc},
	}

	if p.isIdentLike() || p.cur.Type == tokSCONST {
		stmt.Name = p.cur.Str
		p.advance()
	}

	stmt.Options = p.parseEndpointOptions()

	stmt.Loc.End = p.pos()
	return stmt
}

// parseDropEndpointStmt parses DROP ENDPOINT.
//
// Ref: https://learn.microsoft.com/en-us/sql/t-sql/statements/drop-endpoint-transact-sql
//
//	DROP ENDPOINT endPointName
func (p *Parser) parseDropEndpointStmt() *nodes.SecurityStmt {
	loc := p.pos()
	// ENDPOINT keyword already consumed by caller

	stmt := &nodes.SecurityStmt{
		Action:     "DROP",
		ObjectType: "ENDPOINT",
		Loc:        nodes.Loc{Start: loc},
	}

	if p.isIdentLike() || p.cur.Type == tokSCONST {
		stmt.Name = p.cur.Str
		p.advance()
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseEndpointOptions consumes AUTHORIZATION, STATE, AS, FOR clauses.
func (p *Parser) parseEndpointOptions() *nodes.List {
	var opts []nodes.Node

	// Consume rest of statement
	for p.cur.Type != ';' && p.cur.Type != tokEOF && p.cur.Type != kwGO {
		if p.cur.Type == '(' {
			p.advance()
			depth := 1
			for depth > 0 && p.cur.Type != tokEOF {
				if p.cur.Type == '(' {
					depth++
				} else if p.cur.Type == ')' {
					depth--
				}
				if depth > 0 {
					p.advance()
				}
			}
			p.match(')')
		} else if p.cur.Type == kwAUTHORIZATION {
			p.advance()
			if p.isIdentLike() {
				opts = append(opts, &nodes.String{Str: "AUTHORIZATION=" + p.cur.Str})
				p.advance()
			}
		} else if p.isIdentLike() || p.cur.Type == kwAS || p.cur.Type == kwFOR ||
			p.cur.Type == kwON || p.cur.Type == kwOFF {
			optStr := strings.ToUpper(p.cur.Str)
			p.advance()
			if p.cur.Type == '=' {
				p.advance()
				if p.isIdentLike() || p.cur.Type == tokSCONST || p.cur.Type == tokICONST {
					optStr += "=" + strings.ToUpper(p.cur.Str)
					p.advance()
				}
			}
			opts = append(opts, &nodes.String{Str: optStr})
		} else {
			p.advance()
		}
	}

	if len(opts) == 0 {
		return nil
	}
	return &nodes.List{Items: opts}
}
