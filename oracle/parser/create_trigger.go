package parser

import (
	nodes "github.com/bytebase/omni/oracle/ast"
)

// parseCreateTriggerStmt parses a CREATE TRIGGER statement after the TRIGGER keyword.
// The caller has already consumed CREATE [OR REPLACE].
//
// Ref: https://docs.oracle.com/en/database/oracle/oracle-database/23/sqlrf/CREATE-TRIGGER.html
//
//	TRIGGER [schema.]trigger_name
//	  { BEFORE | AFTER | INSTEAD OF }
//	  { INSERT | UPDATE [OF col [, col...]] | DELETE }
//	  [OR { INSERT | UPDATE [OF col [, col...]] | DELETE } ...]
//	  ON [schema.]table_name
//	  [FOR EACH ROW]
//	  [WHEN (condition)]
//	  { plsql_block | CALL routine_name }
func (p *Parser) parseCreateTriggerStmt(start int, orReplace bool) *nodes.CreateTriggerStmt {
	p.advance() // consume TRIGGER

	stmt := &nodes.CreateTriggerStmt{
		OrReplace: orReplace,
		Enable:    true,
		Events:    &nodes.List{},
		Loc:       nodes.Loc{Start: start},
	}

	// Trigger name
	stmt.Name = p.parseObjectName()

	// Timing: BEFORE | AFTER | INSTEAD OF
	switch p.cur.Type {
	case kwBEFORE:
		stmt.Timing = nodes.TRIGGER_BEFORE
		p.advance()
	case kwAFTER:
		stmt.Timing = nodes.TRIGGER_AFTER
		p.advance()
	case kwINSTEAD:
		stmt.Timing = nodes.TRIGGER_INSTEAD_OF
		p.advance() // consume INSTEAD
		if p.cur.Type == kwOF {
			p.advance() // consume OF
		}
	}

	// Events: INSERT | UPDATE [OF col, ...] | DELETE
	// separated by OR
	p.parseTriggerEvent(stmt)
	for p.cur.Type == kwOR {
		p.advance() // consume OR
		p.parseTriggerEvent(stmt)
	}

	// ON [schema.]table_name
	if p.cur.Type == kwON {
		p.advance() // consume ON
		stmt.Table = p.parseObjectName()
	}

	// Optional: FOR EACH ROW
	if p.cur.Type == kwFOR {
		p.advance() // consume FOR
		if p.cur.Type == kwEACH {
			p.advance() // consume EACH
			if p.cur.Type == kwROW {
				p.advance() // consume ROW
				stmt.ForEachRow = true
			}
		}
	}

	// Optional: WHEN (condition)
	if p.cur.Type == kwWHEN {
		p.advance() // consume WHEN
		if p.cur.Type == '(' {
			p.advance() // consume '('
			stmt.When = p.parseExpr()
			if p.cur.Type == ')' {
				p.advance() // consume ')'
			}
		}
	}

	// Optional: ENABLE / DISABLE
	if p.cur.Type == kwENABLE {
		stmt.Enable = true
		p.advance()
	} else if p.cur.Type == kwDISABLE {
		stmt.Enable = false
		p.advance()
	}

	// Body: CALL routine_name | plsql_block
	if p.isIdentLikeStr("CALL") {
		p.advance() // consume CALL
		// Parse the routine name as an object name, wrap as a simple expression
		callName := p.parseObjectName()
		stmt.Body = &nodes.PLSQLBlock{
			Label: "CALL:" + callName.Name,
			Loc:   callName.Loc,
		}
	} else if p.cur.Type == kwDECLARE || p.cur.Type == kwBEGIN || p.cur.Type == tokLABELOPEN {
		stmt.Body = p.parsePLSQLBlock()
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseTriggerEvent parses a single trigger event: INSERT | UPDATE [OF col, ...] | DELETE.
func (p *Parser) parseTriggerEvent(stmt *nodes.CreateTriggerStmt) {
	switch p.cur.Type {
	case kwINSERT:
		p.advance()
		stmt.Events.Items = append(stmt.Events.Items, &nodes.Integer{Ival: int64(nodes.TRIGGER_INSERT)})
	case kwUPDATE:
		p.advance()
		stmt.Events.Items = append(stmt.Events.Items, &nodes.Integer{Ival: int64(nodes.TRIGGER_UPDATE)})
		// Optional: OF col [, col...]
		if p.cur.Type == kwOF {
			p.advance() // consume OF
			// Skip column list — not stored in the AST
			for {
				p.parseIdentifier()
				if p.cur.Type != ',' {
					break
				}
				p.advance() // consume ','
			}
		}
	case kwDELETE:
		p.advance()
		stmt.Events.Items = append(stmt.Events.Items, &nodes.Integer{Ival: int64(nodes.TRIGGER_DELETE)})
	}
}
