package parser

import (
	nodes "github.com/bytebase/omni/mysql/ast"
)

// parseAnalyzeTableStmt parses an ANALYZE TABLE statement.
//
// Ref: https://dev.mysql.com/doc/refman/8.0/en/analyze-table.html
//
//	ANALYZE [NO_WRITE_TO_BINLOG | LOCAL] TABLE tbl_name [, tbl_name] ...
func (p *Parser) parseAnalyzeTableStmt() (*nodes.AnalyzeTableStmt, error) {
	start := p.pos()
	p.advance() // consume ANALYZE

	// Optional NO_WRITE_TO_BINLOG | LOCAL
	if p.cur.Type == kwLOCAL {
		p.advance()
	} else if p.cur.Type == tokIDENT && eqFold(p.cur.Str, "no_write_to_binlog") {
		p.advance()
	}

	// TABLE
	p.match(kwTABLE)

	stmt := &nodes.AnalyzeTableStmt{Loc: nodes.Loc{Start: start}}

	// Table list
	for {
		ref, err := p.parseTableRef()
		if err != nil {
			return nil, err
		}
		stmt.Tables = append(stmt.Tables, ref)
		if p.cur.Type != ',' {
			break
		}
		p.advance()
	}

	stmt.Loc.End = p.pos()
	return stmt, nil
}

// parseOptimizeTableStmt parses an OPTIMIZE TABLE statement.
//
// Ref: https://dev.mysql.com/doc/refman/8.0/en/optimize-table.html
//
//	OPTIMIZE [NO_WRITE_TO_BINLOG | LOCAL] TABLE tbl_name [, tbl_name] ...
func (p *Parser) parseOptimizeTableStmt() (*nodes.OptimizeTableStmt, error) {
	start := p.pos()
	p.advance() // consume OPTIMIZE

	if p.cur.Type == kwLOCAL {
		p.advance()
	} else if p.cur.Type == tokIDENT && eqFold(p.cur.Str, "no_write_to_binlog") {
		p.advance()
	}

	p.match(kwTABLE)

	stmt := &nodes.OptimizeTableStmt{Loc: nodes.Loc{Start: start}}

	for {
		ref, err := p.parseTableRef()
		if err != nil {
			return nil, err
		}
		stmt.Tables = append(stmt.Tables, ref)
		if p.cur.Type != ',' {
			break
		}
		p.advance()
	}

	stmt.Loc.End = p.pos()
	return stmt, nil
}

// parseCheckTableStmt parses a CHECK TABLE statement.
//
// Ref: https://dev.mysql.com/doc/refman/8.0/en/check-table.html
//
//	CHECK TABLE tbl_name [, tbl_name] ... [option] ...
func (p *Parser) parseCheckTableStmt() (*nodes.CheckTableStmt, error) {
	start := p.pos()
	p.advance() // consume CHECK

	p.match(kwTABLE)

	stmt := &nodes.CheckTableStmt{Loc: nodes.Loc{Start: start}}

	for {
		ref, err := p.parseTableRef()
		if err != nil {
			return nil, err
		}
		stmt.Tables = append(stmt.Tables, ref)
		if p.cur.Type != ',' {
			break
		}
		p.advance()
	}

	// Optional check options: FOR UPGRADE, QUICK, FAST, MEDIUM, EXTENDED, CHANGED
	for p.cur.Type == kwFOR || (p.cur.Type == tokIDENT &&
		(eqFold(p.cur.Str, "quick") || eqFold(p.cur.Str, "fast") ||
			eqFold(p.cur.Str, "medium") || eqFold(p.cur.Str, "extended") ||
			eqFold(p.cur.Str, "changed"))) {
		if p.cur.Type == kwFOR {
			p.advance()
			if p.cur.Type == tokIDENT && eqFold(p.cur.Str, "upgrade") {
				stmt.Options = append(stmt.Options, "FOR UPGRADE")
				p.advance()
			}
		} else {
			stmt.Options = append(stmt.Options, p.cur.Str)
			p.advance()
		}
	}

	stmt.Loc.End = p.pos()
	return stmt, nil
}

// parseRepairTableStmt parses a REPAIR TABLE statement.
//
// Ref: https://dev.mysql.com/doc/refman/8.0/en/repair-table.html
//
//	REPAIR [NO_WRITE_TO_BINLOG | LOCAL] TABLE tbl_name [, tbl_name] ... [QUICK] [EXTENDED] [USE_FRM]
func (p *Parser) parseRepairTableStmt() (*nodes.RepairTableStmt, error) {
	start := p.pos()
	p.advance() // consume REPAIR

	if p.cur.Type == kwLOCAL {
		p.advance()
	} else if p.cur.Type == tokIDENT && eqFold(p.cur.Str, "no_write_to_binlog") {
		p.advance()
	}

	p.match(kwTABLE)

	stmt := &nodes.RepairTableStmt{Loc: nodes.Loc{Start: start}}

	for {
		ref, err := p.parseTableRef()
		if err != nil {
			return nil, err
		}
		stmt.Tables = append(stmt.Tables, ref)
		if p.cur.Type != ',' {
			break
		}
		p.advance()
	}

	// Options
	for p.cur.Type == tokIDENT {
		if eqFold(p.cur.Str, "quick") {
			stmt.Quick = true
			p.advance()
		} else if eqFold(p.cur.Str, "extended") {
			stmt.Extended = true
			p.advance()
		} else if eqFold(p.cur.Str, "use_frm") {
			p.advance()
		} else {
			break
		}
	}

	stmt.Loc.End = p.pos()
	return stmt, nil
}

// parseFlushStmt parses a FLUSH statement.
//
// Ref: https://dev.mysql.com/doc/refman/8.0/en/flush.html
//
//	FLUSH [NO_WRITE_TO_BINLOG | LOCAL] flush_option [, flush_option] ...
func (p *Parser) parseFlushStmt() (*nodes.FlushStmt, error) {
	start := p.pos()
	p.advance() // consume FLUSH

	if p.cur.Type == kwLOCAL {
		p.advance()
	} else if p.cur.Type == tokIDENT && eqFold(p.cur.Str, "no_write_to_binlog") {
		p.advance()
	}

	stmt := &nodes.FlushStmt{Loc: nodes.Loc{Start: start}}

	// Flush options
	for {
		if p.cur.Type == tokEOF || p.cur.Type == ';' {
			break
		}
		if p.isIdentToken() {
			name, _, _ := p.parseIdentifier()
			stmt.Options = append(stmt.Options, name)
		} else {
			break
		}
		if p.cur.Type != ',' {
			break
		}
		p.advance()
	}

	stmt.Loc.End = p.pos()
	return stmt, nil
}

// parseResetStmt parses a RESET statement.
//
//	RESET reset_option [, reset_option] ...
func (p *Parser) parseResetStmt() (*nodes.FlushStmt, error) {
	start := p.pos()
	p.advance() // consume RESET

	stmt := &nodes.FlushStmt{Loc: nodes.Loc{Start: start}}

	for {
		if p.cur.Type == tokEOF || p.cur.Type == ';' {
			break
		}
		if p.isIdentToken() {
			name, _, _ := p.parseIdentifier()
			stmt.Options = append(stmt.Options, name)
		} else {
			break
		}
		if p.cur.Type != ',' {
			break
		}
		p.advance()
	}

	stmt.Loc.End = p.pos()
	return stmt, nil
}

// parseKillStmt parses a KILL statement.
//
// Ref: https://dev.mysql.com/doc/refman/8.0/en/kill.html
//
//	KILL [CONNECTION | QUERY] processlist_id
func (p *Parser) parseKillStmt() (*nodes.KillStmt, error) {
	start := p.pos()
	p.advance() // consume KILL

	stmt := &nodes.KillStmt{Loc: nodes.Loc{Start: start}}

	// Optional CONNECTION | QUERY
	if p.cur.Type == kwCONNECTION {
		p.advance()
	} else if p.cur.Type == kwQUERY {
		stmt.Query = true
		p.advance()
	}

	// Process ID
	expr, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	stmt.ConnectionID = expr

	stmt.Loc.End = p.pos()
	return stmt, nil
}

// parseDoStmt parses a DO statement.
//
// Ref: https://dev.mysql.com/doc/refman/8.0/en/do.html
//
//	DO expr [, expr] ...
func (p *Parser) parseDoStmt() (*nodes.DoStmt, error) {
	start := p.pos()
	p.advance() // consume DO

	stmt := &nodes.DoStmt{Loc: nodes.Loc{Start: start}}

	for {
		expr, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		stmt.Exprs = append(stmt.Exprs, expr)
		if p.cur.Type != ',' {
			break
		}
		p.advance()
	}

	stmt.Loc.End = p.pos()
	return stmt, nil
}
