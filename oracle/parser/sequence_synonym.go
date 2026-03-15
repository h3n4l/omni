package parser

import (
	nodes "github.com/bytebase/omni/oracle/ast"
)

// parseCreateSequenceStmt parses a CREATE SEQUENCE statement.
// The CREATE keyword has already been consumed. The current token is SEQUENCE.
//
// BNF: oracle/parser/bnf/CREATE-SEQUENCE.bnf
//
//	CREATE SEQUENCE [ schema. ] sequence
//	    [ IF NOT EXISTS ]
//	    [ SHARING = { METADATA | DATA | NONE } ]
//	    [ INCREMENT BY integer ]
//	    [ START WITH integer ]
//	    [ { MAXVALUE integer | NOMAXVALUE } ]
//	    [ { MINVALUE integer | NOMINVALUE } ]
//	    [ { CYCLE | NOCYCLE } ]
//	    [ { CACHE integer | NOCACHE } ]
//	    [ { ORDER | NOORDER } ]
//	    [ { KEEP | NOKEEP } ]
//	    [ { SCALE { EXTEND | NOEXTEND } | NOSCALE } ]
//	    [ { SESSION | GLOBAL } ]
//	    [ SHARD ] ;
func (p *Parser) parseCreateSequenceStmt(start int) *nodes.CreateSequenceStmt {
	stmt := &nodes.CreateSequenceStmt{
		Loc: nodes.Loc{Start: start},
	}

	// SEQUENCE keyword
	if p.cur.Type == kwSEQUENCE {
		p.advance()
	}

	// Sequence name
	stmt.Name = p.parseObjectName()

	// Parse sequence options
	p.parseSequenceOptions(stmt)

	stmt.Loc.End = p.pos()
	return stmt
}

// parseSequenceOptions parses the various options for CREATE SEQUENCE.
func (p *Parser) parseSequenceOptions(stmt *nodes.CreateSequenceStmt) {
	for {
		switch p.cur.Type {
		case kwINCREMENT:
			p.advance()
			if p.cur.Type == kwBY {
				p.advance()
			}
			stmt.IncrementBy = p.parseExpr()
		case kwSTART:
			p.advance()
			if p.cur.Type == kwWITH {
				p.advance()
			}
			stmt.StartWith = p.parseExpr()
		case kwMAXVALUE:
			p.advance()
			stmt.MaxValue = p.parseExpr()
		case kwNOMAXVALUE:
			stmt.NoMaxValue = true
			p.advance()
		case kwMINVALUE:
			p.advance()
			stmt.MinValue = p.parseExpr()
		case kwNOMINVALUE:
			stmt.NoMinValue = true
			p.advance()
		case kwCYCLE:
			stmt.Cycle = true
			p.advance()
		case kwNOCYCLE:
			stmt.NoCycle = true
			p.advance()
		case kwCACHE:
			p.advance()
			stmt.Cache = p.parseExpr()
		case kwNOCACHE:
			stmt.NoCache = true
			p.advance()
		case kwORDER:
			stmt.Order = true
			p.advance()
		case kwNOORDER:
			stmt.NoOrder = true
			p.advance()
		default:
			return
		}
	}
}

// parseCreateSynonymStmt parses a CREATE [OR REPLACE] [PUBLIC] SYNONYM statement.
// The CREATE keyword has already been consumed. The caller has already parsed
// OR REPLACE and PUBLIC if present and passes them in.
//
// BNF: oracle/parser/bnf/CREATE-SYNONYM.bnf
//
//	CREATE [ OR REPLACE | IF NOT EXISTS ]
//	    [ EDITIONABLE | NONEDITIONABLE ]
//	    [ PUBLIC ] SYNONYM [ schema. ] synonym
//	    [ SHARING = { METADATA | NONE } ]
//	    FOR [ schema. ] object [ @ dblink ] ;
func (p *Parser) parseCreateSynonymStmt(start int, orReplace, public bool) *nodes.CreateSynonymStmt {
	stmt := &nodes.CreateSynonymStmt{
		OrReplace: orReplace,
		Public:    public,
		Loc:       nodes.Loc{Start: start},
	}

	// SYNONYM keyword
	if p.cur.Type == kwSYNONYM {
		p.advance()
	}

	// Synonym name
	stmt.Name = p.parseObjectName()

	// FOR target
	if p.cur.Type == kwFOR {
		p.advance()
	}
	stmt.Target = p.parseObjectName()

	stmt.Loc.End = p.pos()
	return stmt
}

// parseCreateDatabaseLinkStmt parses a CREATE [PUBLIC] DATABASE LINK statement.
// The CREATE keyword has already been consumed. The caller has already parsed
// PUBLIC if present and passes it in.
//
// BNF: oracle/parser/bnf/CREATE-DATABASE-LINK.bnf
//
//	CREATE [ SHARED ] [ PUBLIC ] DATABASE LINK [ IF NOT EXISTS ] dblink
//	    [ CONNECT TO
//	        { CURRENT_USER
//	        | user IDENTIFIED BY password [ dblink_authentication ]
//	        }
//	    | CONNECT WITH credential
//	    ]
//	    [ dblink_authentication ]
//	    USING 'connect_string' ;
//
//	dblink_authentication:
//	    AUTHENTICATED BY user IDENTIFIED BY password
func (p *Parser) parseCreateDatabaseLinkStmt(start int, public bool) *nodes.CreateDatabaseLinkStmt {
	stmt := &nodes.CreateDatabaseLinkStmt{
		Public: public,
		Loc:    nodes.Loc{Start: start},
	}

	// DATABASE keyword
	if p.cur.Type == kwDATABASE {
		p.advance()
	}

	// LINK keyword
	if p.cur.Type == kwLINK {
		p.advance()
	}

	// Link name
	stmt.Name = p.parseIdentifier()

	// CONNECT TO user IDENTIFIED BY password
	if p.cur.Type == kwCONNECT {
		p.advance()
		if p.cur.Type == kwTO {
			p.advance()
		}
		stmt.ConnectTo = p.parseIdentifier()

		if p.cur.Type == kwIDENTIFIED {
			p.advance()
			if p.cur.Type == kwBY {
				p.advance()
			}
			stmt.Identified = p.parseIdentifier()
		}
	}

	// USING 'connect_string'
	if p.cur.Type == kwUSING {
		p.advance()
		if p.cur.Type == tokSCONST {
			stmt.Using = p.cur.Str
			p.advance()
		}
	}

	stmt.Loc.End = p.pos()
	return stmt
}
