package parser

import (
	nodes "github.com/bytebase/omni/oracle/ast"
)

// parseCreateUserStmt parses a CREATE USER statement.
//
// Ref: https://docs.oracle.com/en/database/oracle/oracle-database/23/sqlrf/CREATE-USER.html
//
//	CREATE USER name IDENTIFIED BY password
//	    [DEFAULT TABLESPACE ts] [TEMPORARY TABLESPACE ts]
//	    [QUOTA { n [K|M|G|T] | UNLIMITED } ON tablespace]
//	    [PROFILE profile] [ACCOUNT { LOCK | UNLOCK }]
//	    [PASSWORD EXPIRE]
func (p *Parser) parseCreateUserStmt(start int) nodes.StmtNode {
	stmt := &nodes.CreateUserStmt{
		Loc: nodes.Loc{Start: start},
	}

	stmt.Name = p.parseObjectName()

	// IDENTIFIED BY password / IDENTIFIED EXTERNALLY / IDENTIFIED GLOBALLY
	if p.cur.Type == kwIDENTIFIED {
		p.advance()
		if p.cur.Type == kwBY {
			p.advance()
			if p.cur.Type == tokSCONST || p.isIdentLike() {
				stmt.IdentifyBy = p.cur.Str
				p.advance()
			}
		} else if p.isIdentLike() {
			stmt.IdentifyBy = p.cur.Str // EXTERNALLY or GLOBALLY
			p.advance()
		}
	}

	// Collect remaining options as strings until ;/EOF
	for p.cur.Type != ';' && p.cur.Type != tokEOF {
		stmt.Options = append(stmt.Options, p.cur.Str)
		p.advance()
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseAlterUserStmt parses an ALTER USER statement.
//
// Ref: https://docs.oracle.com/en/database/oracle/oracle-database/23/sqlrf/ALTER-USER.html
func (p *Parser) parseAlterUserStmt(start int) nodes.StmtNode {
	stmt := &nodes.AlterUserStmt{
		Loc: nodes.Loc{Start: start},
	}

	stmt.Name = p.parseObjectName()

	// Collect remaining options until ;/EOF
	for p.cur.Type != ';' && p.cur.Type != tokEOF {
		stmt.Options = append(stmt.Options, p.cur.Str)
		p.advance()
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseCreateRoleStmt parses a CREATE ROLE statement.
//
// Ref: https://docs.oracle.com/en/database/oracle/oracle-database/23/sqlrf/CREATE-ROLE.html
//
//	CREATE ROLE role [NOT IDENTIFIED | IDENTIFIED BY password | IDENTIFIED USING package | IDENTIFIED EXTERNALLY | IDENTIFIED GLOBALLY]
func (p *Parser) parseCreateRoleStmt(start int) nodes.StmtNode {
	stmt := &nodes.CreateRoleStmt{
		Loc: nodes.Loc{Start: start},
	}

	stmt.Name = p.parseObjectName()

	// Optional IDENTIFIED clause
	if p.cur.Type == kwIDENTIFIED {
		p.advance()
		if p.cur.Type == kwBY {
			p.advance()
			if p.isIdentLike() || p.cur.Type == tokSCONST {
				stmt.IdentifyBy = p.cur.Str
				p.advance()
			}
		} else if p.isIdentLike() {
			stmt.IdentifyBy = p.cur.Str
			p.advance()
		}
	} else if p.cur.Type == kwNOT {
		p.advance()
		if p.cur.Type == kwIDENTIFIED {
			p.advance()
		}
		stmt.IdentifyBy = "NOT IDENTIFIED"
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseCreateProfileStmt parses a CREATE PROFILE statement.
//
// Ref: https://docs.oracle.com/en/database/oracle/oracle-database/23/sqlrf/CREATE-PROFILE.html
//
//	CREATE PROFILE profile LIMIT { resource_parameter | password_parameter } ...
func (p *Parser) parseCreateProfileStmt(start int) nodes.StmtNode {
	stmt := &nodes.CreateProfileStmt{
		Loc: nodes.Loc{Start: start},
	}

	stmt.Name = p.parseObjectName()

	// Collect remaining clauses until ;/EOF
	for p.cur.Type != ';' && p.cur.Type != tokEOF {
		stmt.Limits = append(stmt.Limits, p.cur.Str)
		p.advance()
	}

	stmt.Loc.End = p.pos()
	return stmt
}
