package parser

import (
	nodes "github.com/bytebase/omni/mysql/ast"
)

// parseSetStmt parses a SET statement.
//
// Ref: https://dev.mysql.com/doc/refman/8.0/en/set-variable.html
//
//	SET [GLOBAL | SESSION | LOCAL] var = expr [, var = expr] ...
//	SET NAMES charset [COLLATE collation]
//	SET CHARACTER SET charset
func (p *Parser) parseSetStmt() (*nodes.SetStmt, error) {
	start := p.pos()
	p.advance() // consume SET

	stmt := &nodes.SetStmt{Loc: nodes.Loc{Start: start}}

	// Check for NAMES special form
	if p.cur.Type == tokIDENT && eqFold(p.cur.Str, "names") {
		p.advance() // consume NAMES
		// Parse charset name
		charset, charsetLoc, err := p.parseIdentifier()
		if err != nil {
			return nil, err
		}
		// Build assignment: NAMES = charset
		stmt.Assignments = append(stmt.Assignments, &nodes.Assignment{
			Loc:    nodes.Loc{Start: charsetLoc},
			Column: &nodes.ColumnRef{Loc: nodes.Loc{Start: charsetLoc}, Column: "NAMES"},
			Value:  &nodes.StringLit{Loc: nodes.Loc{Start: charsetLoc}, Value: charset},
		})
		// Optional COLLATE
		if _, ok := p.match(kwCOLLATE); ok {
			collation, collLoc, err := p.parseIdentifier()
			if err != nil {
				return nil, err
			}
			stmt.Assignments = append(stmt.Assignments, &nodes.Assignment{
				Loc:    nodes.Loc{Start: collLoc},
				Column: &nodes.ColumnRef{Loc: nodes.Loc{Start: collLoc}, Column: "COLLATE"},
				Value:  &nodes.StringLit{Loc: nodes.Loc{Start: collLoc}, Value: collation},
			})
		}
		stmt.Loc.End = p.pos()
		return stmt, nil
	}

	// Check for CHARACTER SET special form
	if p.cur.Type == kwCHARACTER {
		p.advance() // consume CHARACTER
		if _, ok := p.match(kwSET); ok {
			charset, charsetLoc, err := p.parseIdentifier()
			if err != nil {
				return nil, err
			}
			stmt.Assignments = append(stmt.Assignments, &nodes.Assignment{
				Loc:    nodes.Loc{Start: charsetLoc},
				Column: &nodes.ColumnRef{Loc: nodes.Loc{Start: charsetLoc}, Column: "CHARACTER SET"},
				Value:  &nodes.StringLit{Loc: nodes.Loc{Start: charsetLoc}, Value: charset},
			})
			stmt.Loc.End = p.pos()
			return stmt, nil
		}
	}

	// Check for GLOBAL / SESSION / LOCAL scope
	switch p.cur.Type {
	case kwGLOBAL:
		stmt.Scope = "GLOBAL"
		p.advance()
	case kwSESSION:
		stmt.Scope = "SESSION"
		p.advance()
	case kwLOCAL:
		stmt.Scope = "LOCAL"
		p.advance()
	}

	// Parse assignment list
	for {
		asgn, err := p.parseSetAssignment()
		if err != nil {
			return nil, err
		}
		stmt.Assignments = append(stmt.Assignments, asgn)

		if p.cur.Type != ',' {
			break
		}
		p.advance() // consume ','
	}

	stmt.Loc.End = p.pos()
	return stmt, nil
}

// parseSetAssignment parses a single SET assignment: var = expr
func (p *Parser) parseSetAssignment() (*nodes.Assignment, error) {
	start := p.pos()

	var col *nodes.ColumnRef

	// Handle @var and @@var references
	if p.isVariableRef() {
		vref, err := p.parseVariableRef()
		if err != nil {
			return nil, err
		}
		// Convert VariableRef to ColumnRef for the assignment
		prefix := "@"
		if vref.System {
			prefix = "@@"
			if vref.Scope != "" {
				prefix = "@@" + vref.Scope + "."
			}
		}
		col = &nodes.ColumnRef{
			Loc:    vref.Loc,
			Column: prefix + vref.Name,
		}
	} else {
		var err error
		col, err = p.parseColumnRef()
		if err != nil {
			return nil, err
		}
	}

	// Expect '='
	if _, err := p.expect('='); err != nil {
		return nil, err
	}

	// Parse value expression
	val, err := p.parseExpr()
	if err != nil {
		return nil, err
	}

	return &nodes.Assignment{
		Loc:    nodes.Loc{Start: start, End: p.pos()},
		Column: col,
		Value:  val,
	}, nil
}

// parseShowStmt parses a SHOW statement.
//
// Ref: https://dev.mysql.com/doc/refman/8.0/en/show.html
func (p *Parser) parseShowStmt() (*nodes.ShowStmt, error) {
	start := p.pos()
	p.advance() // consume SHOW

	stmt := &nodes.ShowStmt{Loc: nodes.Loc{Start: start}}

	switch p.cur.Type {
	case kwDATABASES:
		stmt.Type = "DATABASES"
		p.advance()
		if err := p.parseShowLikeOrWhere(stmt); err != nil {
			return nil, err
		}

	case kwTABLES:
		stmt.Type = "TABLES"
		p.advance()
		// Optional FROM db
		if _, ok := p.match(kwFROM); ok {
			ref, err := p.parseTableRef()
			if err != nil {
				return nil, err
			}
			stmt.From = ref
		}
		if err := p.parseShowLikeOrWhere(stmt); err != nil {
			return nil, err
		}

	case kwFULL:
		p.advance() // consume FULL
		if p.cur.Type == kwCOLUMNS {
			stmt.Type = "FULL COLUMNS"
			p.advance()
			// FROM tbl
			if _, err := p.expect(kwFROM); err != nil {
				return nil, err
			}
			ref, err := p.parseTableRef()
			if err != nil {
				return nil, err
			}
			stmt.From = ref
			// Optional FROM db
			if _, ok := p.match(kwFROM); ok {
				dbRef, err := p.parseTableRef()
				if err != nil {
					return nil, err
				}
				// Merge: set schema on From
				stmt.From.Schema = dbRef.Name
			}
			if err := p.parseShowLikeOrWhere(stmt); err != nil {
				return nil, err
			}
		}

	case kwCOLUMNS:
		stmt.Type = "COLUMNS"
		p.advance()
		// FROM tbl
		if _, err := p.expect(kwFROM); err != nil {
			return nil, err
		}
		ref, err := p.parseTableRef()
		if err != nil {
			return nil, err
		}
		stmt.From = ref
		// Optional FROM db
		if _, ok := p.match(kwFROM); ok {
			dbRef, err := p.parseTableRef()
			if err != nil {
				return nil, err
			}
			stmt.From.Schema = dbRef.Name
		}
		if err := p.parseShowLikeOrWhere(stmt); err != nil {
			return nil, err
		}

	case kwCREATE:
		p.advance() // consume CREATE
		switch p.cur.Type {
		case kwTABLE:
			stmt.Type = "CREATE TABLE"
			p.advance()
			ref, err := p.parseTableRef()
			if err != nil {
				return nil, err
			}
			stmt.From = ref
		case kwDATABASE:
			stmt.Type = "CREATE DATABASE"
			p.advance()
			ref, err := p.parseTableRef()
			if err != nil {
				return nil, err
			}
			stmt.From = ref
		case kwVIEW:
			stmt.Type = "CREATE VIEW"
			p.advance()
			ref, err := p.parseTableRef()
			if err != nil {
				return nil, err
			}
			stmt.From = ref
		}

	case kwINDEX:
		stmt.Type = "INDEX"
		p.advance()
		// FROM tbl
		if _, err := p.expect(kwFROM); err != nil {
			return nil, err
		}
		ref, err := p.parseTableRef()
		if err != nil {
			return nil, err
		}
		stmt.From = ref
		// Optional FROM db
		if _, ok := p.match(kwFROM); ok {
			dbRef, err := p.parseTableRef()
			if err != nil {
				return nil, err
			}
			stmt.From.Schema = dbRef.Name
		}

	case kwGLOBAL, kwSESSION:
		scope := "GLOBAL"
		if p.cur.Type == kwSESSION {
			scope = "SESSION"
		}
		p.advance()
		if p.cur.Type == kwVARIABLES {
			stmt.Type = scope + " VARIABLES"
			p.advance()
		} else if p.cur.Type == kwSTATUS {
			stmt.Type = scope + " STATUS"
			p.advance()
		}
		if err := p.parseShowLikeOrWhere(stmt); err != nil {
			return nil, err
		}

	case kwVARIABLES:
		stmt.Type = "VARIABLES"
		p.advance()
		if err := p.parseShowLikeOrWhere(stmt); err != nil {
			return nil, err
		}

	case kwSTATUS:
		stmt.Type = "STATUS"
		p.advance()
		if err := p.parseShowLikeOrWhere(stmt); err != nil {
			return nil, err
		}

	case kwWARNINGS:
		stmt.Type = "WARNINGS"
		p.advance()
		// Optional LIMIT
		if _, ok := p.match(kwLIMIT); ok {
			// Just skip the count for now
			p.advance()
		}

	case kwERRORS:
		stmt.Type = "ERRORS"
		p.advance()
		// Optional LIMIT
		if _, ok := p.match(kwLIMIT); ok {
			p.advance()
		}

	default:
		// Handle GRANTS and PROCESSLIST as identifier-based keywords
		if p.cur.Type == kwGRANT || (p.cur.Type == tokIDENT && eqFold(p.cur.Str, "grants")) {
			stmt.Type = "GRANTS"
			p.advance()
			// Optional FOR user
			if _, ok := p.match(kwFOR); ok {
				name, nameLoc, err := p.parseIdentifier()
				if err != nil {
					return nil, err
				}
				stmt.From = &nodes.TableRef{
					Loc:  nodes.Loc{Start: nameLoc},
					Name: name,
				}
				stmt.From.Loc.End = p.pos()
			}
		} else if p.cur.Type == kwPROCESSLIST || (p.cur.Type == tokIDENT && eqFold(p.cur.Str, "processlist")) {
			stmt.Type = "PROCESSLIST"
			p.advance()
		}
	}

	stmt.Loc.End = p.pos()
	return stmt, nil
}

// parseShowLikeOrWhere parses optional LIKE or WHERE clause for SHOW statements.
func (p *Parser) parseShowLikeOrWhere(stmt *nodes.ShowStmt) error {
	if p.cur.Type == kwLIKE {
		p.advance()
		expr, err := p.parseExpr()
		if err != nil {
			return err
		}
		stmt.Like = expr
	} else if p.cur.Type == kwWHERE {
		p.advance()
		expr, err := p.parseExpr()
		if err != nil {
			return err
		}
		stmt.Where = expr
	}
	return nil
}

// parseUseStmt parses a USE statement.
//
// Ref: https://dev.mysql.com/doc/refman/8.0/en/use.html
//
//	USE db_name
func (p *Parser) parseUseStmt() (*nodes.UseStmt, error) {
	start := p.pos()
	p.advance() // consume USE

	name, _, err := p.parseIdentifier()
	if err != nil {
		return nil, err
	}

	return &nodes.UseStmt{
		Loc:      nodes.Loc{Start: start, End: p.pos()},
		Database: name,
	}, nil
}

// parseExplainStmt parses an EXPLAIN or DESCRIBE statement.
//
// Ref: https://dev.mysql.com/doc/refman/8.0/en/explain.html
//
//	EXPLAIN [FORMAT = {TRADITIONAL|JSON|TREE}] stmt
//	DESCRIBE tbl_name [col_name]
func (p *Parser) parseExplainStmt() (*nodes.ExplainStmt, error) {
	start := p.pos()
	isDescribe := p.cur.Type == kwDESCRIBE
	p.advance() // consume EXPLAIN or DESCRIBE

	stmt := &nodes.ExplainStmt{Loc: nodes.Loc{Start: start}}

	if isDescribe {
		// DESCRIBE tbl_name [col_name]
		ref, err := p.parseTableRef()
		if err != nil {
			return nil, err
		}
		// Wrap as a ShowStmt for DESCRIBE (which is equivalent to SHOW COLUMNS FROM tbl)
		showStmt := &nodes.ShowStmt{
			Loc:  nodes.Loc{Start: start},
			Type: "COLUMNS",
			From: ref,
		}
		// Optional column name
		if p.cur.Type != tokEOF && p.cur.Type != ';' {
			colExpr, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			showStmt.Like = colExpr
		}
		showStmt.Loc.End = p.pos()
		stmt.Stmt = showStmt
		stmt.Loc.End = p.pos()
		return stmt, nil
	}

	// EXPLAIN [FORMAT = value] stmt
	// Check for FORMAT = value
	if p.cur.Type == tokIDENT && eqFold(p.cur.Str, "format") {
		p.advance() // consume FORMAT
		if _, err := p.expect('='); err != nil {
			return nil, err
		}
		// Parse format value: TRADITIONAL, JSON, TREE
		formatName, _, err := p.parseIdentifier()
		if err != nil {
			return nil, err
		}
		stmt.Format = formatName
	}

	// Parse the statement (for now, only SELECT)
	if p.cur.Type == kwSELECT {
		sel, err := p.parseSelectStmt()
		if err != nil {
			return nil, err
		}
		stmt.Stmt = sel
	} else {
		// For other statements, try to parse as a table ref (EXPLAIN table_name)
		ref, err := p.parseTableRef()
		if err != nil {
			return nil, err
		}
		showStmt := &nodes.ShowStmt{
			Loc:  nodes.Loc{Start: ref.Loc.Start, End: p.pos()},
			Type: "COLUMNS",
			From: ref,
		}
		stmt.Stmt = showStmt
	}

	stmt.Loc.End = p.pos()
	return stmt, nil
}
