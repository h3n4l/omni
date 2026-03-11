package parser

import (
	nodes "github.com/bytebase/omni/oracle/ast"
)

// parseSetRoleStmt parses a SET ROLE statement.
//
// Ref: https://docs.oracle.com/en/database/oracle/oracle-database/23/sqlrf/SET-ROLE.html
//
//	SET ROLE { role [IDENTIFIED BY password] [,...] | ALL [EXCEPT role [,...]] | NONE }
func (p *Parser) parseSetRoleStmt() nodes.StmtNode {
	start := p.pos()
	p.advance() // consume SET
	p.advance() // consume ROLE

	stmt := &nodes.SetRoleStmt{
		Loc: nodes.Loc{Start: start},
	}

	// NONE
	if p.isIdentLike() && p.cur.Str == "NONE" {
		stmt.None = true
		p.advance()
		stmt.Loc.End = p.pos()
		return stmt
	}

	// ALL [EXCEPT role [,...]]
	if p.cur.Type == kwALL {
		stmt.All = true
		p.advance()
		if p.cur.Type == kwEXCEPT {
			p.advance()
			for {
				name := p.parseObjectName()
				if name != nil {
					stmt.Except = append(stmt.Except, name)
				}
				if p.cur.Type != ',' {
					break
				}
				p.advance()
			}
		}
		stmt.Loc.End = p.pos()
		return stmt
	}

	// role [IDENTIFIED BY password] [,...]
	for {
		name := p.parseObjectName()
		if name != nil {
			stmt.Roles = append(stmt.Roles, name)
		}
		// Skip optional IDENTIFIED BY password
		if p.isIdentLike() && p.cur.Str == "IDENTIFIED" {
			p.advance()
			if p.cur.Type == kwBY {
				p.advance()
			}
			// consume password (identifier or string)
			p.advance()
		}
		if p.cur.Type != ',' {
			break
		}
		p.advance()
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseSetConstraintsStmt parses a SET CONSTRAINT(S) statement.
//
// Ref: https://docs.oracle.com/en/database/oracle/oracle-database/23/sqlrf/SET-CONSTRAINT.html
//
//	SET { CONSTRAINT | CONSTRAINTS } { ALL | constraint [,...] } { IMMEDIATE | DEFERRED }
func (p *Parser) parseSetConstraintsStmt() nodes.StmtNode {
	start := p.pos()
	p.advance() // consume SET
	p.advance() // consume CONSTRAINT or CONSTRAINTS

	stmt := &nodes.SetConstraintsStmt{
		Loc: nodes.Loc{Start: start},
	}

	// ALL or specific constraints
	if p.cur.Type == kwALL {
		stmt.All = true
		p.advance()
	} else {
		for {
			name := p.parseObjectName()
			if name != nil {
				stmt.Constraints = append(stmt.Constraints, name)
			}
			if p.cur.Type != ',' {
				break
			}
			p.advance()
		}
	}

	// IMMEDIATE or DEFERRED
	if p.cur.Type == kwIMMEDIATE {
		p.advance()
	} else if p.cur.Type == kwDEFERRED {
		stmt.Deferred = true
		p.advance()
	}

	stmt.Loc.End = p.pos()
	return stmt
}
