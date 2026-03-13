package parser

import (
	nodes "github.com/bytebase/omni/oracle/ast"
)

// parseAlterStmt dispatches ALTER statements based on the next keyword.
//
// Ref: https://docs.oracle.com/en/database/oracle/oracle-database/23/sqlrf/SQL-Statements-ALTER-ANALYTIC-VIEW-to-ALTER-SYSTEM.html
//
//	ALTER SESSION SET param = value [ param = value ... ]
//	ALTER SYSTEM  SET param = value [ param = value ... ]
//	ALTER SYSTEM  KILL SESSION 'sid,serial#'
//	ALTER INDEX   name ...
//	ALTER VIEW    name ...
//	ALTER SEQUENCE name ...
func (p *Parser) parseAlterStmt() nodes.StmtNode {
	start := p.pos()
	p.advance() // consume ALTER

	switch p.cur.Type {
	case kwSESSION:
		return p.parseAlterSessionStmt(start)
	case kwSYSTEM:
		return p.parseAlterSystemStmt(start)
	case kwINDEX:
		return p.parseAlterIndexStmt(start)
	case kwVIEW:
		return p.parseAlterViewStmt(start)
	case kwSEQUENCE:
		return p.parseAlterSequenceStmt(start)
	case kwTABLE:
		return p.parseAlterTableStmt(start)
	case kwPROCEDURE:
		return p.parseAlterGeneric(start, nodes.OBJECT_PROCEDURE)
	case kwFUNCTION:
		return p.parseAlterGeneric(start, nodes.OBJECT_FUNCTION)
	case kwTRIGGER:
		return p.parseAlterGeneric(start, nodes.OBJECT_TRIGGER)
	case kwTYPE:
		return p.parseAlterGeneric(start, nodes.OBJECT_TYPE)
	case kwPACKAGE:
		return p.parseAlterGeneric(start, nodes.OBJECT_PACKAGE)
	case kwMATERIALIZED:
		// Check for MATERIALIZED ZONEMAP vs MATERIALIZED VIEW
		p.advance() // consume MATERIALIZED
		if p.isIdentLike() && p.cur.Str == "ZONEMAP" {
			p.advance() // consume ZONEMAP
			return p.parseAdminDDLStmt("ALTER", nodes.OBJECT_MATERIALIZED_ZONEMAP, start)
		}
		// MATERIALIZED VIEW - consume VIEW and parse
		if p.cur.Type == kwVIEW {
			p.advance() // consume VIEW
		}
		stmt := &nodes.AdminDDLStmt{
			Action:     "ALTER",
			ObjectType: nodes.OBJECT_MATERIALIZED_VIEW,
			Loc:        nodes.Loc{Start: start},
		}
		stmt.Name = p.parseObjectName()
		p.skipToSemicolon()
		stmt.Loc.End = p.pos()
		return stmt
	case kwDATABASE:
		// Distinguish ALTER DATABASE LINK, ALTER DATABASE DICTIONARY, ALTER DATABASE
		next := p.peekNext()
		if next.Type == kwLINK {
			return p.parseAlterGeneric(start, nodes.OBJECT_DATABASE_LINK)
		}
		p.advance() // consume DATABASE
		if p.isIdentLikeStr("DICTIONARY") {
			p.advance() // consume DICTIONARY
			return p.parseAlterDatabaseDictionaryStmt(start)
		}
		return p.parseAlterDatabaseStmt(start)
	case kwSYNONYM:
		return p.parseAlterGeneric(start, nodes.OBJECT_SYNONYM)
	case kwAUDIT:
		// ALTER AUDIT POLICY
		p.advance() // consume AUDIT
		if p.isIdentLikeStr("POLICY") {
			p.advance() // consume POLICY
		}
		return p.parseAdminDDLStmt("ALTER", nodes.OBJECT_AUDIT_POLICY, start)
	case kwJSON:
		// ALTER JSON RELATIONAL DUALITY VIEW
		p.advance() // consume JSON
		if p.isIdentLike() && p.cur.Str == "RELATIONAL" {
			p.advance() // consume RELATIONAL
		}
		if p.isIdentLike() && p.cur.Str == "DUALITY" {
			p.advance() // consume DUALITY
		}
		if p.cur.Type == kwVIEW {
			p.advance() // consume VIEW
		}
		return p.parseAdminDDLStmt("ALTER", nodes.OBJECT_JSON_DUALITY_VIEW, start)
	case kwFLASHBACK:
		// ALTER FLASHBACK ARCHIVE
		p.advance() // consume FLASHBACK
		if p.isIdentLike() && p.cur.Str == "ARCHIVE" {
			p.advance() // consume ARCHIVE
		}
		return p.parseAdminDDLStmt("ALTER", nodes.OBJECT_FLASHBACK_ARCHIVE, start)
	case kwUSER, kwROLE, kwPROFILE,
		kwTABLESPACE, kwCLUSTER, kwJAVA, kwLIBRARY:
		if adminStmt := p.parseAlterAdminObject(start); adminStmt != nil {
			return adminStmt
		}
		p.skipToSemicolon()
		return nil
	default:
		// Check for DIMENSION (identifier)
		if p.isIdentLike() {
			if adminStmt := p.parseAlterAdminObject(start); adminStmt != nil {
				return adminStmt
			}
		}
		// Unknown ALTER target — skip to semicolon or EOF.
		p.skipToSemicolon()
		return nil
	}
}

// parseAlterSessionStmt parses ALTER SESSION SET param = value [, ...].
func (p *Parser) parseAlterSessionStmt(start int) nodes.StmtNode {
	p.advance() // consume SESSION

	stmt := &nodes.AlterSessionStmt{
		Loc: nodes.Loc{Start: start},
	}

	if _, ok := p.matchKeyword(kwSET); ok {
		stmt.SetParams = p.parseSetParams()
	} else {
		// ALTER SESSION with other clauses — skip remaining tokens.
		p.skipToSemicolon()
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseAlterSystemStmt parses ALTER SYSTEM SET/KILL ... .
func (p *Parser) parseAlterSystemStmt(start int) nodes.StmtNode {
	p.advance() // consume SYSTEM

	stmt := &nodes.AlterSystemStmt{
		Loc: nodes.Loc{Start: start},
	}

	switch {
	case p.cur.Type == kwSET:
		p.advance() // consume SET
		stmt.SetParams = p.parseSetParams()
	default:
		// ALTER SYSTEM KILL SESSION or other — consume remaining tokens.
		// Capture KILL SESSION 'sid,serial#' if present.
		if p.isIdentLike() && p.cur.Str == "KILL" {
			p.advance() // consume KILL
			if p.cur.Type == kwSESSION {
				p.advance() // consume SESSION
			}
			if p.cur.Type == tokSCONST {
				stmt.Kill = p.cur.Str
				p.advance()
			}
		}
		p.skipToSemicolon()
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseSetParams parses one or more param = value pairs.
func (p *Parser) parseSetParams() *nodes.List {
	params := &nodes.List{}
	for {
		param := p.parseSetParam()
		if param == nil {
			break
		}
		params.Items = append(params.Items, param)
		// Some Oracle ALTER SESSION SET supports multiple params without commas;
		// but also handle comma separation.
		if p.cur.Type == ',' {
			p.advance()
		}
		// Stop if we hit end of statement.
		if !p.isIdentLike() {
			break
		}
	}
	return params
}

// parseSetParam parses a single name = value parameter setting.
func (p *Parser) parseSetParam() *nodes.SetParam {
	if !p.isIdentLike() {
		return nil
	}
	start := p.pos()
	name := p.parseIdentifier()
	if name == "" {
		return nil
	}

	// Expect '='
	if p.cur.Type != '=' {
		return &nodes.SetParam{
			Name: name,
			Loc:  nodes.Loc{Start: start, End: p.pos()},
		}
	}
	p.advance() // consume '='

	value := p.parseExpr()

	return &nodes.SetParam{
		Name:  name,
		Value: value,
		Loc:   nodes.Loc{Start: start, End: p.pos()},
	}
}

// parseAlterGeneric parses ALTER INDEX/VIEW/SEQUENCE/TABLE by consuming the
// object name and skipping the rest (simplified). Returns an AlterSessionStmt
// as a placeholder — in practice these would have their own AST types, but for
// now we skip the body to avoid blocking other work.
func (p *Parser) parseAlterGeneric(start int, objType nodes.ObjectType) nodes.StmtNode {
	p.advance() // consume INDEX/VIEW/SEQUENCE/etc.

	// For MATERIALIZED VIEW, consume VIEW too
	if objType == nodes.OBJECT_MATERIALIZED_VIEW && p.cur.Type == kwVIEW {
		p.advance()
	}
	// For DATABASE LINK, consume LINK too
	if objType == nodes.OBJECT_DATABASE_LINK && p.cur.Type == kwLINK {
		p.advance()
	}

	stmt := &nodes.AdminDDLStmt{
		Action:     "ALTER",
		ObjectType: objType,
		Loc:        nodes.Loc{Start: start},
	}

	// Parse the object name.
	stmt.Name = p.parseObjectName()

	// Skip remainder of the statement (clauses vary greatly by object type).
	p.skipToSemicolon()

	stmt.Loc.End = p.pos()
	return stmt
}

// parseAlterIndexStmt parses an ALTER INDEX statement.
//
// Ref: https://docs.oracle.com/en/database/oracle/oracle-database/23/sqlrf/ALTER-INDEX.html
//
//	ALTER INDEX [IF EXISTS] [schema.]index_name
//	{   REBUILD [PARTITION partition | SUBPARTITION subpartition]
//	          [TABLESPACE tablespace] [ONLINE] [REVERSE | NOREVERSE]
//	          [PARALLEL integer | NOPARALLEL] [COMPRESS integer | NOCOMPRESS]
//	          [LOGGING | NOLOGGING]
//	  | RENAME TO new_name
//	  | COALESCE [CLEANUP [ONLY]] [PARALLEL integer | NOPARALLEL]
//	  | { MONITORING | NOMONITORING } USAGE
//	  | USABLE | UNUSABLE [ONLINE]
//	  | VISIBLE | INVISIBLE
//	  | ENABLE | DISABLE | COMPILE
//	  | SHRINK SPACE [COMPACT] [CASCADE]
//	  | PARALLEL integer | NOPARALLEL
//	  | LOGGING | NOLOGGING
//	  | UPDATE BLOCK REFERENCES
//	  | INDEXING {FULL | PARTIAL}
//	}
func (p *Parser) parseAlterIndexStmt(start int) nodes.StmtNode {
	p.advance() // consume INDEX

	stmt := &nodes.AlterIndexStmt{
		Loc: nodes.Loc{Start: start},
	}

	// IF EXISTS
	if p.cur.Type == kwIF {
		next := p.peekNext()
		if next.Type == kwEXISTS {
			stmt.IfExists = true
			p.advance() // consume IF
			p.advance() // consume EXISTS
		}
	}

	// Parse index name
	stmt.Name = p.parseObjectName()

	// Parse action
	switch {
	case p.isIdentLikeStr("REBUILD"):
		stmt.Action = "REBUILD"
		p.advance() // consume REBUILD
		// Optional PARTITION/SUBPARTITION and rebuild options
		for p.cur.Type != ';' && p.cur.Type != tokEOF {
			switch {
			case p.cur.Type == kwPARTITION:
				p.advance() // consume PARTITION
				stmt.Partition = p.parseIdentifier()
			case p.isIdentLikeStr("SUBPARTITION"):
				p.advance() // consume SUBPARTITION
				stmt.Subpartition = p.parseIdentifier()
			case p.cur.Type == kwTABLESPACE:
				p.advance() // consume TABLESPACE
				stmt.Tablespace = p.parseIdentifier()
			case p.cur.Type == kwONLINE:
				stmt.Online = true
				p.advance()
			case p.cur.Type == kwREVERSE:
				stmt.Reverse = true
				p.advance()
			case p.isIdentLikeStr("NOREVERSE"):
				stmt.NoReverse = true
				p.advance()
			case p.cur.Type == kwPARALLEL:
				p.advance() // consume PARALLEL
				if p.cur.Type == tokICONST {
					stmt.Parallel = p.cur.Str
					p.advance()
				}
			case p.cur.Type == kwNOPARALLEL:
				stmt.NoParallel = true
				p.advance()
			case p.cur.Type == kwCOMPRESS:
				p.advance() // consume COMPRESS
				if p.cur.Type == tokICONST {
					stmt.Compress = p.cur.Str
					p.advance()
				} else {
					stmt.Compress = "1"
				}
			case p.cur.Type == kwNOCOMPRESS:
				stmt.NoCompress = true
				p.advance()
			case p.cur.Type == kwLOGGING:
				stmt.Logging = true
				p.advance()
			case p.cur.Type == kwNOLOGGING:
				stmt.NoLogging = true
				p.advance()
			default:
				goto done
			}
		}

	case p.cur.Type == kwRENAME:
		stmt.Action = "RENAME"
		p.advance() // consume RENAME
		if p.cur.Type == kwTO {
			p.advance() // consume TO
		}
		stmt.NewName = p.parseIdentifier()

	case p.isIdentLikeStr("COALESCE"):
		stmt.Action = "COALESCE"
		p.advance() // consume COALESCE
		if p.isIdentLikeStr("CLEANUP") {
			stmt.Cleanup = true
			p.advance() // consume CLEANUP
			if p.isIdentLikeStr("ONLY") {
				stmt.CleanupOnly = true
				p.advance() // consume ONLY
			}
		}
		// Optional parallel clause
		if p.cur.Type == kwPARALLEL {
			p.advance()
			if p.cur.Type == tokICONST {
				stmt.Parallel = p.cur.Str
				p.advance()
			}
		} else if p.cur.Type == kwNOPARALLEL {
			stmt.NoParallel = true
			p.advance()
		}

	case p.isIdentLikeStr("MONITORING"):
		stmt.Action = "MONITORING_USAGE"
		p.advance() // consume MONITORING
		if p.isIdentLikeStr("USAGE") {
			p.advance() // consume USAGE
		}

	case p.isIdentLikeStr("NOMONITORING"):
		stmt.Action = "NOMONITORING_USAGE"
		p.advance() // consume NOMONITORING
		if p.isIdentLikeStr("USAGE") {
			p.advance() // consume USAGE
		}

	case p.isIdentLikeStr("UNUSABLE"):
		stmt.Action = "UNUSABLE"
		p.advance() // consume UNUSABLE
		if p.cur.Type == kwONLINE {
			stmt.Online = true
			p.advance()
		}

	case p.isIdentLikeStr("USABLE"):
		stmt.Action = "USABLE"
		p.advance() // consume USABLE

	case p.isIdentLikeStr("VISIBLE"):
		stmt.Action = "VISIBLE"
		p.advance() // consume VISIBLE

	case p.cur.Type == kwINVISIBLE:
		stmt.Action = "INVISIBLE"
		p.advance() // consume INVISIBLE

	case p.cur.Type == kwENABLE:
		stmt.Action = "ENABLE"
		p.advance() // consume ENABLE

	case p.cur.Type == kwDISABLE:
		stmt.Action = "DISABLE"
		p.advance() // consume DISABLE

	case p.isIdentLikeStr("COMPILE"):
		stmt.Action = "COMPILE"
		p.advance() // consume COMPILE

	case p.isIdentLikeStr("SHRINK"):
		stmt.Action = "SHRINK_SPACE"
		p.advance() // consume SHRINK
		if p.isIdentLikeStr("SPACE") {
			p.advance() // consume SPACE
		}
		if p.isIdentLikeStr("COMPACT") {
			stmt.Compact = true
			p.advance()
		}
		if p.cur.Type == kwCASCADE {
			stmt.Cascade = true
			p.advance()
		}

	case p.cur.Type == kwPARALLEL:
		stmt.Action = "PARALLEL"
		p.advance() // consume PARALLEL
		if p.cur.Type == tokICONST {
			stmt.Parallel = p.cur.Str
			p.advance()
		}

	case p.cur.Type == kwNOPARALLEL:
		stmt.Action = "NOPARALLEL"
		p.advance() // consume NOPARALLEL

	case p.cur.Type == kwLOGGING:
		stmt.Action = "LOGGING"
		p.advance() // consume LOGGING

	case p.cur.Type == kwNOLOGGING:
		stmt.Action = "NOLOGGING"
		p.advance() // consume NOLOGGING

	case p.isIdentLikeStr("UPDATE"):
		stmt.Action = "UPDATE_BLOCK_REFERENCES"
		p.advance() // consume UPDATE
		if p.isIdentLikeStr("BLOCK") {
			p.advance() // consume BLOCK
		}
		if p.isIdentLikeStr("REFERENCES") {
			p.advance() // consume REFERENCES
		}

	case p.isIdentLikeStr("INDEXING"):
		p.advance() // consume INDEXING
		if p.isIdentLikeStr("FULL") {
			stmt.Action = "INDEXING"
			stmt.IndexingFull = true
			p.advance()
		} else if p.isIdentLikeStr("PARTIAL") {
			stmt.Action = "INDEXING"
			stmt.IndexingPartial = true
			p.advance()
		}

	default:
		// Unknown action — skip remaining tokens
		p.skipToSemicolon()
	}

done:
	stmt.Loc.End = p.pos()
	return stmt
}

// parseAlterViewStmt parses an ALTER VIEW statement.
//
// Ref: https://docs.oracle.com/en/database/oracle/oracle-database/23/sqlrf/ALTER-VIEW.html
//
//	ALTER VIEW [IF EXISTS] [schema.]view
//	{   COMPILE
//	  | ADD out_of_line_constraint
//	  | MODIFY CONSTRAINT constraint_name { RELY | NORELY }
//	  | DROP CONSTRAINT constraint_name
//	  | { READ ONLY | READ WRITE }
//	  | { EDITIONABLE | NONEDITIONABLE }
//	}
func (p *Parser) parseAlterViewStmt(start int) nodes.StmtNode {
	p.advance() // consume VIEW

	stmt := &nodes.AlterViewStmt{
		Loc: nodes.Loc{Start: start},
	}

	// IF EXISTS
	if p.cur.Type == kwIF {
		next := p.peekNext()
		if next.Type == kwEXISTS {
			stmt.IfExists = true
			p.advance() // consume IF
			p.advance() // consume EXISTS
		}
	}

	// Parse view name
	stmt.Name = p.parseObjectName()

	// Parse action
	switch {
	case p.isIdentLikeStr("COMPILE"), p.isIdentLikeStr("RECOMPILE"):
		stmt.Action = "COMPILE"
		p.advance()

	case p.cur.Type == kwADD:
		stmt.Action = "ADD_CONSTRAINT"
		p.advance() // consume ADD
		stmt.Constraint = p.parseTableConstraint()

	case p.cur.Type == kwMODIFY:
		p.advance() // consume MODIFY
		if p.cur.Type == kwCONSTRAINT {
			stmt.Action = "MODIFY_CONSTRAINT"
			p.advance() // consume CONSTRAINT
			stmt.ConstraintName = p.parseIdentifier()
			if p.cur.Type == kwRELY {
				stmt.Rely = true
				p.advance()
			} else if p.isIdentLikeStr("NORELY") {
				stmt.NoRely = true
				p.advance()
			}
		} else {
			p.skipToSemicolon()
		}

	case p.cur.Type == kwDROP:
		p.advance() // consume DROP
		if p.cur.Type == kwCONSTRAINT {
			stmt.Action = "DROP_CONSTRAINT"
			p.advance() // consume CONSTRAINT
			stmt.ConstraintName = p.parseIdentifier()
		} else {
			p.skipToSemicolon()
		}

	case p.cur.Type == kwREAD:
		p.advance() // consume READ
		if p.isIdentLikeStr("ONLY") {
			stmt.Action = "READ_ONLY"
			p.advance()
		} else if p.cur.Type == kwWRITE {
			stmt.Action = "READ_WRITE"
			p.advance()
		}

	case p.isIdentLikeStr("EDITIONABLE"):
		stmt.Action = "EDITIONABLE"
		p.advance()

	case p.isIdentLikeStr("NONEDITIONABLE"):
		stmt.Action = "NONEDITIONABLE"
		p.advance()

	default:
		p.skipToSemicolon()
	}

	// Skip optional trailing clauses (DISABLE NOVALIDATE on constraints, etc.)
	if stmt.Action == "ADD_CONSTRAINT" {
		// Consume DISABLE/ENABLE NOVALIDATE/VALIDATE after constraint
		for p.cur.Type != ';' && p.cur.Type != tokEOF {
			if p.cur.Type == kwDISABLE || p.cur.Type == kwENABLE || p.isIdentLikeStr("NOVALIDATE") || p.isIdentLikeStr("VALIDATE") {
				p.advance()
			} else {
				break
			}
		}
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseAlterSequenceStmt parses an ALTER SEQUENCE statement.
//
// Ref: https://docs.oracle.com/en/database/oracle/oracle-database/23/sqlrf/ALTER-SEQUENCE.html
//
//	ALTER SEQUENCE [IF EXISTS] [schema.]sequence_name
//	  [ INCREMENT BY integer ]
//	  [ MAXVALUE integer | NOMAXVALUE ]
//	  [ MINVALUE integer | NOMINVALUE ]
//	  [ CYCLE | NOCYCLE ]
//	  [ CACHE integer | NOCACHE ]
//	  [ ORDER | NOORDER ]
//	  [ KEEP | NOKEEP ]
//	  [ RESTART [ WITH integer ] ]
//	  [ SCALE [ EXTEND | NOEXTEND ] | NOSCALE ]
//	  [ SHARD [ EXTEND | NOEXTEND ] | NOSHARD ]
//	  [ GLOBAL | SESSION ]
func (p *Parser) parseAlterSequenceStmt(start int) nodes.StmtNode {
	p.advance() // consume SEQUENCE

	stmt := &nodes.AlterSequenceStmt{
		Loc: nodes.Loc{Start: start},
	}

	// IF EXISTS
	if p.cur.Type == kwIF {
		next := p.peekNext()
		if next.Type == kwEXISTS {
			stmt.IfExists = true
			p.advance() // consume IF
			p.advance() // consume EXISTS
		}
	}

	// Parse sequence name
	stmt.Name = p.parseObjectName()

	// Parse sequence options (loop, multiple may be specified)
	for p.cur.Type != ';' && p.cur.Type != tokEOF {
		switch {
		case p.cur.Type == kwINCREMENT:
			p.advance() // consume INCREMENT
			if p.cur.Type == kwBY {
				p.advance() // consume BY
			}
			stmt.IncrementBy = p.parseExpr()

		case p.cur.Type == kwMAXVALUE:
			p.advance() // consume MAXVALUE
			stmt.MaxValue = p.parseExpr()

		case p.cur.Type == kwNOMAXVALUE:
			stmt.NoMaxValue = true
			p.advance()

		case p.cur.Type == kwMINVALUE:
			p.advance() // consume MINVALUE
			stmt.MinValue = p.parseExpr()

		case p.cur.Type == kwNOMINVALUE:
			stmt.NoMinValue = true
			p.advance()

		case p.cur.Type == kwCYCLE:
			stmt.Cycle = true
			p.advance()

		case p.cur.Type == kwNOCYCLE:
			stmt.NoCycle = true
			p.advance()

		case p.cur.Type == kwCACHE:
			p.advance() // consume CACHE
			stmt.Cache = p.parseExpr()

		case p.cur.Type == kwNOCACHE:
			stmt.NoCache = true
			p.advance()

		case p.cur.Type == kwORDER:
			stmt.Order = true
			p.advance()

		case p.cur.Type == kwNOORDER:
			stmt.NoOrder = true
			p.advance()

		case p.cur.Type == kwKEEP:
			stmt.Keep = true
			p.advance()

		case p.isIdentLikeStr("NOKEEP"):
			stmt.NoKeep = true
			p.advance()

		case p.isIdentLikeStr("RESTART"):
			stmt.Restart = true
			p.advance() // consume RESTART
			if p.cur.Type == kwWITH {
				p.advance() // consume WITH
				stmt.RestartWith = p.parseExpr()
			}

		case p.isIdentLikeStr("SCALE"):
			stmt.Scale = true
			p.advance() // consume SCALE
			if p.isIdentLikeStr("EXTEND") {
				stmt.ScaleExtend = true
				p.advance()
			} else if p.isIdentLikeStr("NOEXTEND") {
				stmt.ScaleNoExtend = true
				p.advance()
			}

		case p.isIdentLikeStr("NOSCALE"):
			stmt.NoScale = true
			p.advance()

		case p.isIdentLikeStr("SHARD"):
			stmt.Shard = true
			p.advance() // consume SHARD
			if p.isIdentLikeStr("EXTEND") {
				stmt.ShardExtend = true
				p.advance()
			} else if p.isIdentLikeStr("NOEXTEND") {
				stmt.ShardNoExtend = true
				p.advance()
			}

		case p.isIdentLikeStr("NOSHARD"):
			stmt.NoShard = true
			p.advance()

		case p.cur.Type == kwGLOBAL:
			stmt.Global = true
			p.advance()

		case p.cur.Type == kwSESSION:
			stmt.Session = true
			p.advance()

		default:
			goto done
		}
	}

done:
	stmt.Loc.End = p.pos()
	return stmt
}

// skipToSemicolon advances until a semicolon or EOF is found.
// It does NOT consume the semicolon.
func (p *Parser) skipToSemicolon() {
	for p.cur.Type != ';' && p.cur.Type != tokEOF {
		p.advance()
	}
}
