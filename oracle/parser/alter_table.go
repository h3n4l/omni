package parser

import (
	nodes "github.com/bytebase/omni/oracle/ast"
)

// parseAlterTableStmt parses an ALTER TABLE statement after ALTER has been consumed.
// The caller has already consumed ALTER; this function consumes TABLE and the rest.
//
// Ref: https://docs.oracle.com/en/database/oracle/oracle-database/23/sqlrf/ALTER-TABLE.html
//
//	ALTER TABLE [ schema . ] table_name
//	  { ADD ( column_def [, ...] )
//	  | ADD ( table_constraint )
//	  | MODIFY ( column_def [, ...] )
//	  | DROP COLUMN col_name
//	  | DROP CONSTRAINT name
//	  | RENAME COLUMN old TO new
//	  | RENAME TO new_name
//	  | ... }
func (p *Parser) parseAlterTableStmt(start int) *nodes.AlterTableStmt {
	p.advance() // consume TABLE

	stmt := &nodes.AlterTableStmt{
		Actions: &nodes.List{},
		Loc:     nodes.Loc{Start: start},
	}

	// Table name
	stmt.Name = p.parseObjectName()

	// Parse one or more ALTER TABLE actions
	for p.cur.Type != ';' && p.cur.Type != tokEOF {
		cmd := p.parseAlterTableAction()
		if cmd == nil {
			break
		}
		stmt.Actions.Items = append(stmt.Actions.Items, cmd)
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseAlterTableAction parses a single ALTER TABLE action.
func (p *Parser) parseAlterTableAction() *nodes.AlterTableCmd {
	switch p.cur.Type {
	case kwADD:
		return p.parseAlterTableAdd()
	case kwMODIFY:
		return p.parseAlterTableModify()
	case kwDROP:
		return p.parseAlterTableDrop()
	case kwRENAME:
		return p.parseAlterTableRename()
	case kwTRUNCATE:
		return p.parseAlterTableTruncatePartition()
	default:
		return nil
	}
}

// parseAlterTableAdd parses ADD column or ADD constraint.
//
//	ADD ( column_def [, ...] )
//	ADD column_def
//	ADD CONSTRAINT ...
func (p *Parser) parseAlterTableAdd() *nodes.AlterTableCmd {
	start := p.pos()
	p.advance() // consume ADD

	// Check if this starts a constraint: CONSTRAINT, PRIMARY, UNIQUE, FOREIGN, CHECK
	if p.isTableConstraintStart() {
		tc := p.parseTableConstraint()
		return &nodes.AlterTableCmd{
			Action:     nodes.AT_ADD_CONSTRAINT,
			Constraint: tc,
			Loc:        nodes.Loc{Start: start, End: p.pos()},
		}
	}

	// ADD ( column_def [, ...] )
	if p.cur.Type == '(' {
		p.advance() // consume '('
		col := p.parseColumnDef()
		if p.cur.Type == ')' {
			p.advance() // consume ')'
		}
		return &nodes.AlterTableCmd{
			Action:    nodes.AT_ADD_COLUMN,
			ColumnDef: col,
			Loc:       nodes.Loc{Start: start, End: p.pos()},
		}
	}

	// ADD PARTITION
	if p.cur.Type == kwPARTITION {
		p.advance() // consume PARTITION
		name := ""
		if p.isIdentLike() {
			name = p.parseIdentifier()
		}
		// Skip partition details (VALUES, etc.)
		p.skipParenthesized()
		return &nodes.AlterTableCmd{
			Action:     nodes.AT_ADD_PARTITION,
			ColumnName: name,
			Loc:        nodes.Loc{Start: start, End: p.pos()},
		}
	}

	// ADD column_def (without parentheses)
	col := p.parseColumnDef()
	return &nodes.AlterTableCmd{
		Action:    nodes.AT_ADD_COLUMN,
		ColumnDef: col,
		Loc:       nodes.Loc{Start: start, End: p.pos()},
	}
}

// parseAlterTableModify parses MODIFY column or MODIFY CONSTRAINT.
//
//	MODIFY ( column_def [, ...] )
//	MODIFY column_def
//	MODIFY CONSTRAINT name ...
func (p *Parser) parseAlterTableModify() *nodes.AlterTableCmd {
	start := p.pos()
	p.advance() // consume MODIFY

	// MODIFY CONSTRAINT
	if p.cur.Type == kwCONSTRAINT {
		p.advance() // consume CONSTRAINT
		name := p.parseIdentifier()
		// Skip any trailing clauses (ENABLE, DISABLE, etc.)
		for p.cur.Type != ';' && p.cur.Type != tokEOF && p.cur.Type != kwADD &&
			p.cur.Type != kwMODIFY && p.cur.Type != kwDROP && p.cur.Type != kwRENAME {
			p.advance()
		}
		tc := &nodes.TableConstraint{
			Name: name,
			Loc:  nodes.Loc{Start: start, End: p.pos()},
		}
		return &nodes.AlterTableCmd{
			Action:     nodes.AT_MODIFY_CONSTRAINT,
			Constraint: tc,
			Loc:        nodes.Loc{Start: start, End: p.pos()},
		}
	}

	// MODIFY ( column_def [, ...] )
	if p.cur.Type == '(' {
		p.advance() // consume '('
		col := p.parseColumnDef()
		if p.cur.Type == ')' {
			p.advance() // consume ')'
		}
		return &nodes.AlterTableCmd{
			Action:    nodes.AT_MODIFY_COLUMN,
			ColumnDef: col,
			Loc:       nodes.Loc{Start: start, End: p.pos()},
		}
	}

	// MODIFY column_def (without parentheses)
	col := p.parseColumnDef()
	return &nodes.AlterTableCmd{
		Action:    nodes.AT_MODIFY_COLUMN,
		ColumnDef: col,
		Loc:       nodes.Loc{Start: start, End: p.pos()},
	}
}

// parseAlterTableDrop parses DROP COLUMN, DROP CONSTRAINT, or DROP PARTITION.
//
//	DROP COLUMN col_name [ CASCADE CONSTRAINTS ]
//	DROP ( col_name [, ...] )
//	DROP CONSTRAINT constraint_name
//	DROP PARTITION partition_name
func (p *Parser) parseAlterTableDrop() *nodes.AlterTableCmd {
	start := p.pos()
	p.advance() // consume DROP

	switch p.cur.Type {
	case kwCOLUMN:
		p.advance() // consume COLUMN
		name := p.parseIdentifier()
		// Optional CASCADE CONSTRAINTS
		if p.cur.Type == kwCASCADE {
			p.advance()
			if p.isIdentLikeStr("CONSTRAINTS") {
				p.advance()
			}
		}
		return &nodes.AlterTableCmd{
			Action:     nodes.AT_DROP_COLUMN,
			ColumnName: name,
			Loc:        nodes.Loc{Start: start, End: p.pos()},
		}

	case kwCONSTRAINT:
		p.advance() // consume CONSTRAINT
		name := p.parseIdentifier()
		// Optional CASCADE
		if p.cur.Type == kwCASCADE {
			p.advance()
		}
		tc := &nodes.TableConstraint{
			Name: name,
			Loc:  nodes.Loc{Start: start, End: p.pos()},
		}
		return &nodes.AlterTableCmd{
			Action:     nodes.AT_DROP_CONSTRAINT,
			Constraint: tc,
			Loc:        nodes.Loc{Start: start, End: p.pos()},
		}

	case kwPARTITION:
		p.advance() // consume PARTITION
		name := p.parseIdentifier()
		return &nodes.AlterTableCmd{
			Action:     nodes.AT_DROP_PARTITION,
			ColumnName: name,
			Loc:        nodes.Loc{Start: start, End: p.pos()},
		}

	default:
		// DROP (col_name, ...) — drop multiple columns
		if p.cur.Type == '(' {
			p.advance() // consume '('
			name := p.parseIdentifier()
			// skip rest of list
			for p.cur.Type == ',' {
				p.advance()
				p.parseIdentifier()
			}
			if p.cur.Type == ')' {
				p.advance()
			}
			return &nodes.AlterTableCmd{
				Action:     nodes.AT_DROP_COLUMN,
				ColumnName: name,
				Loc:        nodes.Loc{Start: start, End: p.pos()},
			}
		}
		// Bare DROP col_name
		name := p.parseIdentifier()
		return &nodes.AlterTableCmd{
			Action:     nodes.AT_DROP_COLUMN,
			ColumnName: name,
			Loc:        nodes.Loc{Start: start, End: p.pos()},
		}
	}
}

// parseAlterTableRename parses RENAME COLUMN ... TO ... or RENAME TO ....
//
//	RENAME COLUMN old_name TO new_name
//	RENAME TO new_table_name
func (p *Parser) parseAlterTableRename() *nodes.AlterTableCmd {
	start := p.pos()
	p.advance() // consume RENAME

	if p.cur.Type == kwCOLUMN {
		p.advance() // consume COLUMN
		oldName := p.parseIdentifier()
		if p.cur.Type == kwTO {
			p.advance() // consume TO
		}
		newName := p.parseIdentifier()
		return &nodes.AlterTableCmd{
			Action:     nodes.AT_RENAME_COLUMN,
			ColumnName: oldName,
			NewName:    newName,
			Loc:        nodes.Loc{Start: start, End: p.pos()},
		}
	}

	// RENAME TO new_name
	if p.cur.Type == kwTO {
		p.advance() // consume TO
	}
	newName := p.parseIdentifier()
	return &nodes.AlterTableCmd{
		Action:  nodes.AT_RENAME,
		NewName: newName,
		Loc:     nodes.Loc{Start: start, End: p.pos()},
	}
}

// parseAlterTableTruncatePartition parses TRUNCATE PARTITION partition_name.
func (p *Parser) parseAlterTableTruncatePartition() *nodes.AlterTableCmd {
	start := p.pos()
	p.advance() // consume TRUNCATE

	if p.cur.Type != kwPARTITION {
		// Not a recognized action — skip to next action boundary.
		return nil
	}
	p.advance() // consume PARTITION
	name := p.parseIdentifier()
	return &nodes.AlterTableCmd{
		Action:     nodes.AT_TRUNCATE_PARTITION,
		ColumnName: name,
		Loc:        nodes.Loc{Start: start, End: p.pos()},
	}
}

// skipParenthesized skips a parenthesized group if present.
func (p *Parser) skipParenthesized() {
	if p.cur.Type != '(' {
		return
	}
	depth := 1
	p.advance() // consume '('
	for depth > 0 && p.cur.Type != tokEOF {
		if p.cur.Type == '(' {
			depth++
		} else if p.cur.Type == ')' {
			depth--
		}
		p.advance()
	}
}
