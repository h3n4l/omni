package parser

import (
	nodes "github.com/bytebase/omni/oracle/ast"
)

// parseCreateMaterializedOrView distinguishes between:
// - CREATE MATERIALIZED VIEW LOG ON ... (mview log)
// - CREATE MATERIALIZED VIEW ... (regular mview)
func (p *Parser) parseCreateMaterializedOrView(start int, orReplace bool) nodes.StmtNode {
	p.advance() // consume MATERIALIZED
	// Check for MATERIALIZED ZONEMAP
	if p.isIdentLike() && p.cur.Str == "ZONEMAP" {
		p.advance() // consume ZONEMAP
		return p.parseAdminDDLStmt("CREATE", nodes.OBJECT_MATERIALIZED_ZONEMAP, start)
	}
	if p.cur.Type == kwVIEW {
		next := p.peekNext()
		if next.Type == kwLOG {
			p.advance() // consume VIEW
			p.advance() // consume LOG
			return p.parseCreateMviewLogStmt(start)
		}
	}
	// It's a regular MATERIALIZED VIEW — but we already consumed MATERIALIZED.
	// parseCreateViewStmt expects to see kwMATERIALIZED. Since we consumed it,
	// we need to handle VIEW directly.
	stmt := &nodes.CreateViewStmt{
		OrReplace:    orReplace,
		Materialized: true,
		Loc:          nodes.Loc{Start: start},
	}
	if p.cur.Type == kwVIEW {
		p.advance()
	}
	// Delegate the rest to the existing view parsing logic.
	return p.finishCreateViewStmt(stmt)
}

// parseAdminDDLStmt parses generic administrative DDL statements by consuming
// the action keyword (CREATE/ALTER/DROP) and object type keyword, then parsing
// the object name and skipping remaining options until semicolon/EOF.
//
// This handles: TABLESPACE, DIRECTORY, CONTEXT, CLUSTER, DIMENSION,
// FLASHBACK ARCHIVE, JAVA, LIBRARY
func (p *Parser) parseAdminDDLStmt(action string, objType nodes.ObjectType, start int) nodes.StmtNode {
	stmt := &nodes.AdminDDLStmt{
		Action:     action,
		ObjectType: objType,
		Loc:        nodes.Loc{Start: start},
	}

	stmt.Name = p.parseObjectName()

	// Skip remaining tokens until ;/EOF
	for p.cur.Type != ';' && p.cur.Type != tokEOF {
		p.advance()
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseCreateSchemaStmt parses a CREATE SCHEMA statement.
//
// Ref: https://docs.oracle.com/en/database/oracle/oracle-database/23/sqlrf/CREATE-SCHEMA.html
//
//	CREATE SCHEMA AUTHORIZATION schema_name
//	  { create_table_statement
//	  | create_view_statement
//	  | grant_statement
//	  } ...
func (p *Parser) parseCreateSchemaStmt(start int) *nodes.CreateSchemaStmt {
	stmt := &nodes.CreateSchemaStmt{
		Stmts: &nodes.List{},
		Loc:   nodes.Loc{Start: start},
	}

	// AUTHORIZATION schema_name
	if p.isIdentLikeStr("AUTHORIZATION") {
		p.advance() // consume AUTHORIZATION
	}
	if p.cur.Type == tokIDENT || p.cur.Type == tokQIDENT || p.isIdentLike() {
		stmt.SchemaName = p.cur.Str
		p.advance()
	}

	// Parse nested statements: CREATE TABLE, CREATE VIEW, GRANT
	// In Oracle syntax, nested statements do NOT have their own semicolons.
	for p.cur.Type != ';' && p.cur.Type != tokEOF {
		nestedStart := p.pos()
		switch p.cur.Type {
		case kwCREATE:
			p.advance() // consume CREATE
			switch p.cur.Type {
			case kwTABLE:
				stmt.Stmts.Items = append(stmt.Stmts.Items, p.parseCreateTableStmt(nestedStart, false, false, false, false, false))
			case kwVIEW:
				stmt.Stmts.Items = append(stmt.Stmts.Items, p.parseCreateViewStmt(nestedStart, false))
			case kwFORCE:
				stmt.Stmts.Items = append(stmt.Stmts.Items, p.parseCreateViewStmt(nestedStart, false))
			default:
				// Unknown nested CREATE — skip to next CREATE/GRANT/semicolon
				p.skipToSemicolon()
				goto done
			}
		case kwGRANT:
			stmt.Stmts.Items = append(stmt.Stmts.Items, p.parseGrantStmt())
		default:
			// Unexpected token — stop parsing nested statements
			goto done
		}
	}

done:
	stmt.Loc.End = p.pos()
	return stmt
}

// parseCreateAdminObject handles CREATE dispatches for admin DDL objects
// (called from parseCreateStmt after consuming CREATE and modifiers).
func (p *Parser) parseCreateAdminObject(start int) nodes.StmtNode {
	switch p.cur.Type {
	case kwUSER:
		p.advance()
		return p.parseCreateUserStmt(start)
	case kwROLE:
		p.advance()
		return p.parseCreateRoleStmt(start)
	case kwPROFILE:
		p.advance()
		return p.parseCreateProfileStmt(start, false)
	case kwTABLESPACE:
		p.advance()
		// Check for TABLESPACE SET
		if p.cur.Type == kwSET {
			p.advance() // consume SET
			return p.parseCreateTablespaceSetStmt(start)
		}
		return p.parseCreateTablespaceStmt(start, false, false, false, false, false)
	case kwDIRECTORY:
		p.advance()
		return p.parseAdminDDLStmt("CREATE", nodes.OBJECT_DIRECTORY, start)
	case kwCONTEXT:
		p.advance()
		return p.parseAdminDDLStmt("CREATE", nodes.OBJECT_CONTEXT, start)
	case kwCLUSTER:
		p.advance()
		return p.parseCreateClusterStmt(start)
	case kwJAVA:
		p.advance()
		return p.parseAdminDDLStmt("CREATE", nodes.OBJECT_JAVA, start)
	case kwLIBRARY:
		p.advance()
		return p.parseAdminDDLStmt("CREATE", nodes.OBJECT_LIBRARY, start)
	case kwSCHEMA:
		p.advance()
		return p.parseCreateSchemaStmt(start)
	default:
		// DIMENSION, FLASHBACK ARCHIVE, MANDATORY PROFILE handled via identifiers
		if p.isIdentLike() {
			switch p.cur.Str {
			case "DIMENSION":
				p.advance()
				return p.parseCreateDimensionStmt(start)
			case "FLASHBACK":
				p.advance()
				if p.isIdentLike() && p.cur.Str == "ARCHIVE" {
					p.advance()
				}
				return p.parseAdminDDLStmt("CREATE", nodes.OBJECT_FLASHBACK_ARCHIVE, start)
			case "MANDATORY":
				p.advance()
				if p.cur.Type == kwPROFILE {
					p.advance()
					return p.parseCreateProfileStmt(start, true)
				}
			case "DISKGROUP":
				p.advance()
				return p.parseCreateDiskgroupStmt(start)
			case "PLUGGABLE":
				p.advance() // consume PLUGGABLE
				if p.cur.Type == kwDATABASE {
					p.advance() // consume DATABASE
				}
				return p.parseCreatePluggableDatabaseStmt(start)
		case "ANALYTIC":
			p.advance() // consume ANALYTIC
			if p.cur.Type == kwVIEW {
				p.advance() // consume VIEW
			}
			return p.parseCreateAnalyticViewStmt(start, false, false, false)
		case "ATTRIBUTE":
			p.advance() // consume ATTRIBUTE
			if p.isIdentLike() && p.cur.Str == "DIMENSION" {
				p.advance() // consume DIMENSION
			}
			return p.parseAdminDDLStmt("CREATE", nodes.OBJECT_ATTRIBUTE_DIMENSION, start)
		case "HIERARCHY":
			p.advance()
			return p.parseAdminDDLStmt("CREATE", nodes.OBJECT_HIERARCHY, start)
		case "DOMAIN":
			p.advance()
			return p.parseAdminDDLStmt("CREATE", nodes.OBJECT_DOMAIN, start)
		case "INDEXTYPE":
			p.advance()
			return p.parseCreateIndextypeStmt(start, false)
		case "OPERATOR":
			p.advance()
			return p.parseCreateOperatorStmt(start, false, false)
		case "LOCKDOWN":
			p.advance() // consume LOCKDOWN
			if p.cur.Type == kwPROFILE {
				p.advance() // consume PROFILE
			}
			return p.parseAdminDDLStmt("CREATE", nodes.OBJECT_LOCKDOWN_PROFILE, start)
		case "OUTLINE":
			p.advance()
			return p.parseAdminDDLStmt("CREATE", nodes.OBJECT_OUTLINE, start)
		case "INMEMORY":
			p.advance() // consume INMEMORY
			if p.cur.Type == kwJOIN {
				p.advance() // consume JOIN
			}
			if p.cur.Type == kwGROUP || (p.isIdentLike() && p.cur.Str == "GROUP") {
				p.advance() // consume GROUP
			}
			return p.parseAdminDDLStmt("CREATE", nodes.OBJECT_INMEMORY_JOIN_GROUP, start)
		case "ROLLBACK":
			p.advance() // consume ROLLBACK
			if p.isIdentLike() && p.cur.Str == "SEGMENT" {
				p.advance() // consume SEGMENT
			}
			return p.parseAdminDDLStmt("CREATE", nodes.OBJECT_ROLLBACK_SEGMENT, start)
		case "EDITION":
			p.advance()
			return p.parseAdminDDLStmt("CREATE", nodes.OBJECT_EDITION, start)
		case "MLE":
			p.advance() // consume MLE
			if p.isIdentLike() && p.cur.Str == "ENV" {
				p.advance() // consume ENV
				return p.parseAdminDDLStmt("CREATE", nodes.OBJECT_MLE_ENV, start)
			}
			if p.isIdentLike() && p.cur.Str == "MODULE" {
				p.advance() // consume MODULE
				return p.parseAdminDDLStmt("CREATE", nodes.OBJECT_MLE_MODULE, start)
			}
			return p.parseAdminDDLStmt("CREATE", nodes.OBJECT_MLE_ENV, start)
		case "PFILE":
			p.advance()
			return p.parseAdminDDLStmt("CREATE", nodes.OBJECT_PFILE, start)
		case "SPFILE":
			p.advance()
			return p.parseAdminDDLStmt("CREATE", nodes.OBJECT_SPFILE, start)
		case "PROPERTY":
			p.advance() // consume PROPERTY
			if p.isIdentLike() && p.cur.Str == "GRAPH" {
				p.advance() // consume GRAPH
			}
			return p.parseAdminDDLStmt("CREATE", nodes.OBJECT_PROPERTY_GRAPH, start)
		case "VECTOR":
			p.advance() // consume VECTOR
			if p.cur.Type == kwINDEX {
				p.advance() // consume INDEX
			}
			return p.parseAdminDDLStmt("CREATE", nodes.OBJECT_VECTOR_INDEX, start)
		case "RESTORE":
			p.advance() // consume RESTORE
			if p.isIdentLike() && p.cur.Str == "POINT" {
				p.advance() // consume POINT
			}
			return p.parseAdminDDLStmt("CREATE", nodes.OBJECT_RESTORE_POINT, start)
		case "LOGICAL":
			p.advance() // consume LOGICAL
			if p.cur.Type == kwPARTITION || (p.isIdentLike() && p.cur.Str == "PARTITION") {
				p.advance() // consume PARTITION
			}
			if p.isIdentLike() && p.cur.Str == "TRACKING" {
				p.advance() // consume TRACKING
			}
			return p.parseAdminDDLStmt("CREATE", nodes.OBJECT_LOGICAL_PARTITION_TRACKING, start)
		case "PMEM":
			p.advance() // consume PMEM
			if p.isIdentLike() && p.cur.Str == "FILESTORE" {
				p.advance() // consume FILESTORE
			}
			return p.parseAdminDDLStmt("CREATE", nodes.OBJECT_PMEM_FILESTORE, start)
		}
	}
	return nil
	}
}

// parseDropAdminObject handles DROP dispatches for admin DDL objects
// (called from parseDropStmt for object types not handled there).
func (p *Parser) parseDropAdminObject(start int) nodes.StmtNode {
	switch p.cur.Type {
	case kwUSER:
		p.advance()
		stmt := &nodes.AdminDDLStmt{
			Action:     "DROP",
			ObjectType: nodes.OBJECT_USER,
			Loc:        nodes.Loc{Start: start},
		}
		// IF EXISTS
		if p.cur.Type == kwIF {
			next := p.peekNext()
			if next.Type == kwEXISTS {
				p.advance() // consume IF
				p.advance() // consume EXISTS
			}
		}
		stmt.Name = p.parseObjectName()
		// CASCADE
		if p.cur.Type == kwCASCADE {
			p.advance()
		}
		stmt.Loc.End = p.pos()
		return stmt
	case kwROLE:
		p.advance()
		return p.parseAdminDDLStmt("DROP", nodes.OBJECT_ROLE, start)
	case kwPROFILE:
		p.advance()
		stmt := &nodes.AdminDDLStmt{
			Action:     "DROP",
			ObjectType: nodes.OBJECT_PROFILE,
			Loc:        nodes.Loc{Start: start},
		}
		stmt.Name = p.parseObjectName()
		if p.cur.Type == kwCASCADE {
			p.advance()
		}
		stmt.Loc.End = p.pos()
		return stmt
	case kwTABLESPACE:
		p.advance()
		if p.cur.Type == kwSET {
			p.advance() // consume SET
			return p.parseDropTablespaceStmt(start, true)
		}
		return p.parseDropTablespaceStmt(start, false)
	case kwDIRECTORY:
		p.advance()
		return p.parseAdminDDLStmt("DROP", nodes.OBJECT_DIRECTORY, start)
	case kwCONTEXT:
		p.advance()
		return p.parseAdminDDLStmt("DROP", nodes.OBJECT_CONTEXT, start)
	case kwCLUSTER:
		p.advance()
		return p.parseAdminDDLStmt("DROP", nodes.OBJECT_CLUSTER, start)
	case kwJAVA:
		p.advance()
		return p.parseAdminDDLStmt("DROP", nodes.OBJECT_JAVA, start)
	case kwLIBRARY:
		p.advance()
		return p.parseAdminDDLStmt("DROP", nodes.OBJECT_LIBRARY, start)
	default:
		if p.isIdentLike() {
			switch p.cur.Str {
			case "DIMENSION":
				p.advance()
				return p.parseAdminDDLStmt("DROP", nodes.OBJECT_DIMENSION, start)
			case "FLASHBACK":
				p.advance()
				if p.isIdentLike() && p.cur.Str == "ARCHIVE" {
					p.advance()
				}
				return p.parseAdminDDLStmt("DROP", nodes.OBJECT_FLASHBACK_ARCHIVE, start)
			case "DISKGROUP":
				p.advance()
				return p.parseDropDiskgroupStmt(start)
			case "PLUGGABLE":
				p.advance() // consume PLUGGABLE
				if p.cur.Type == kwDATABASE {
					p.advance() // consume DATABASE
				}
				return p.parseDropPluggableDatabaseStmt(start)
			case "ANALYTIC":
				p.advance() // consume ANALYTIC
				if p.cur.Type == kwVIEW {
					p.advance() // consume VIEW
				}
				return p.parseAdminDDLStmt("DROP", nodes.OBJECT_ANALYTIC_VIEW, start)
			case "ATTRIBUTE":
				p.advance() // consume ATTRIBUTE
				if p.isIdentLike() && p.cur.Str == "DIMENSION" {
					p.advance() // consume DIMENSION
				}
				return p.parseAdminDDLStmt("DROP", nodes.OBJECT_ATTRIBUTE_DIMENSION, start)
			case "HIERARCHY":
				p.advance()
				return p.parseAdminDDLStmt("DROP", nodes.OBJECT_HIERARCHY, start)
			case "DOMAIN":
				p.advance()
				return p.parseAdminDDLStmt("DROP", nodes.OBJECT_DOMAIN, start)
			case "INDEXTYPE":
				p.advance()
				return p.parseDropSimpleStmt(nodes.OBJECT_INDEXTYPE, start)
			case "OPERATOR":
				p.advance()
				return p.parseDropSimpleStmt(nodes.OBJECT_OPERATOR, start)
			case "LOCKDOWN":
				p.advance() // consume LOCKDOWN
				if p.cur.Type == kwPROFILE {
					p.advance() // consume PROFILE
				}
				return p.parseAdminDDLStmt("DROP", nodes.OBJECT_LOCKDOWN_PROFILE, start)
			case "OUTLINE":
				p.advance()
				return p.parseAdminDDLStmt("DROP", nodes.OBJECT_OUTLINE, start)
			case "INMEMORY":
				p.advance() // consume INMEMORY
				if p.cur.Type == kwJOIN {
					p.advance() // consume JOIN
				}
				if p.cur.Type == kwGROUP || (p.isIdentLike() && p.cur.Str == "GROUP") {
					p.advance() // consume GROUP
				}
				return p.parseAdminDDLStmt("DROP", nodes.OBJECT_INMEMORY_JOIN_GROUP, start)
			case "ROLLBACK":
				p.advance() // consume ROLLBACK
				if p.isIdentLike() && p.cur.Str == "SEGMENT" {
					p.advance() // consume SEGMENT
				}
				return p.parseAdminDDLStmt("DROP", nodes.OBJECT_ROLLBACK_SEGMENT, start)
			case "EDITION":
				p.advance()
				return p.parseAdminDDLStmt("DROP", nodes.OBJECT_EDITION, start)
			case "MLE":
				p.advance() // consume MLE
				if p.isIdentLike() && p.cur.Str == "ENV" {
					p.advance() // consume ENV
					return p.parseAdminDDLStmt("DROP", nodes.OBJECT_MLE_ENV, start)
				}
				if p.isIdentLike() && p.cur.Str == "MODULE" {
					p.advance() // consume MODULE
					return p.parseAdminDDLStmt("DROP", nodes.OBJECT_MLE_MODULE, start)
				}
				return p.parseAdminDDLStmt("DROP", nodes.OBJECT_MLE_ENV, start)
			case "PROPERTY":
				p.advance() // consume PROPERTY
				if p.isIdentLike() && p.cur.Str == "GRAPH" {
					p.advance() // consume GRAPH
				}
				return p.parseAdminDDLStmt("DROP", nodes.OBJECT_PROPERTY_GRAPH, start)
			case "VECTOR":
				p.advance() // consume VECTOR
				if p.cur.Type == kwINDEX {
					p.advance() // consume INDEX
				}
				return p.parseAdminDDLStmt("DROP", nodes.OBJECT_VECTOR_INDEX, start)
			case "RESTORE":
				p.advance() // consume RESTORE
				if p.isIdentLike() && p.cur.Str == "POINT" {
					p.advance() // consume POINT
				}
				return p.parseAdminDDLStmt("DROP", nodes.OBJECT_RESTORE_POINT, start)
			case "LOGICAL":
				p.advance() // consume LOGICAL
				if p.cur.Type == kwPARTITION || (p.isIdentLike() && p.cur.Str == "PARTITION") {
					p.advance() // consume PARTITION
				}
				if p.isIdentLike() && p.cur.Str == "TRACKING" {
					p.advance() // consume TRACKING
				}
				return p.parseAdminDDLStmt("DROP", nodes.OBJECT_LOGICAL_PARTITION_TRACKING, start)
			case "PMEM":
				p.advance() // consume PMEM
				if p.isIdentLike() && p.cur.Str == "FILESTORE" {
					p.advance() // consume FILESTORE
				}
				return p.parseAdminDDLStmt("DROP", nodes.OBJECT_PMEM_FILESTORE, start)
			}
		}
		return nil
	}
}

// parseCreateTablespaceStmt parses a CREATE TABLESPACE statement.
//
// BNF: oracle/parser/bnf/CREATE-TABLESPACE.bnf
//
//	CREATE [ BIGFILE | SMALLFILE ]
//	    { permanent_tablespace_clause
//	    | temporary_tablespace_clause
//	    | undo_tablespace_clause
//	    } ;
//
//	permanent_tablespace_clause:
//	    TABLESPACE tablespace [ IF NOT EXISTS ]
//	    [ DATAFILE file_specification [, file_specification ] ... ]
//	    [ permanent_tablespace_attrs ]
//
//	permanent_tablespace_attrs:
//	    [ MINIMUM EXTENT size_clause ]
//	    [ BLOCKSIZE integer [ K ] ]
//	    [ logging_clause ]
//	    [ FORCE LOGGING ]
//	    [ tablespace_encryption_clause ]
//	    [ DEFAULT [ default_tablespace_params ] ]
//	    [ { ONLINE | OFFLINE } ]
//	    [ extent_management_clause ]
//	    [ segment_management_clause ]
//	    [ flashback_mode_clause ]
//	    [ lost_write_protection ]
//
//	temporary_tablespace_clause:
//	    [ LOCAL ] TEMPORARY TABLESPACE tablespace [ IF NOT EXISTS ]
//	    [ TEMPFILE file_specification [, file_specification ] ... ]
//	    [ tablespace_group_clause ]
//	    [ extent_management_clause ]
//	    [ tablespace_encryption_clause ]
//	    [ FOR { ALL | LEAF } ]
//
//	undo_tablespace_clause:
//	    UNDO TABLESPACE tablespace [ IF NOT EXISTS ]
//	    [ DATAFILE file_specification [, file_specification ] ... ]
//	    [ extent_management_clause ]
//	    [ tablespace_retention_clause ]
//	    [ tablespace_encryption_clause ]
//
//	tablespace_encryption_clause:
//	    ENCRYPTION [ tablespace_encryption_spec ] { ENCRYPT | DECRYPT }
//
//	tablespace_encryption_spec:
//	    USING 'encrypt_algorithm'
//
//	default_tablespace_params:
//	    { default_table_compression | default_index_compression
//	    | inmemory_clause | ilm_clause | storage_clause } [ ... ]
//
//	logging_clause:
//	    { LOGGING | NOLOGGING | FILESYSTEM_LIKE_LOGGING }
//
//	extent_management_clause:
//	    EXTENT MANAGEMENT { LOCAL [ { AUTOALLOCATE | UNIFORM [ SIZE size_clause ] } ] | DICTIONARY }
//
//	segment_management_clause:
//	    SEGMENT SPACE MANAGEMENT { AUTO | MANUAL }
//
//	flashback_mode_clause:
//	    FLASHBACK { ON | OFF }
//
//	tablespace_retention_clause:
//	    RETENTION { GUARANTEE | NOGUARANTEE }
//
//	tablespace_group_clause:
//	    TABLESPACE GROUP { tablespace_group_name | '' }
//
//	lost_write_protection:
//	    { ENABLE | DISABLE | SUSPEND | REMOVE } LOST WRITE PROTECTION
//
//	file_specification:
//	    [ 'filename' | 'ASM_filename' ]
//	    [ SIZE size_clause ]
//	    [ REUSE ]
//	    [ autoextend_clause ]
//
//	autoextend_clause:
//	    { AUTOEXTEND OFF | AUTOEXTEND ON [ NEXT size_clause ] [ MAXSIZE { UNLIMITED | size_clause } ] }
//
//	size_clause:
//	    integer [ K | M | G | T | P | E ]
func (p *Parser) parseCreateTablespaceStmt(start int, bigfile, smallfile, local, temporary, undo bool) *nodes.CreateTablespaceStmt {
	stmt := &nodes.CreateTablespaceStmt{
		Loc:       nodes.Loc{Start: start},
		Bigfile:   bigfile,
		Smallfile: smallfile,
		Local:     local,
		Temporary: temporary,
		Undo:      undo,
	}

	// Parse tablespace name
	stmt.Name = p.parseObjectName()

	// Optional IF NOT EXISTS
	if p.cur.Type == kwIF {
		p.advance() // IF
		if p.cur.Type == kwNOT {
			p.advance() // NOT
		}
		if p.cur.Type == kwEXISTS {
			p.advance() // EXISTS
		}
		stmt.IfNotExists = true
	}

	// Parse clauses in any order until ; or EOF
	for p.cur.Type != ';' && p.cur.Type != tokEOF {
		switch {
		case p.isIdentLike() && (p.cur.Str == "DATAFILE" || p.cur.Str == "TEMPFILE"):
			p.advance()
			// Parse one or more file specifications
			for {
				df := p.parseDatafileClause()
				if df != nil {
					stmt.Datafiles = append(stmt.Datafiles, df)
				}
				if p.cur.Type == ',' {
					p.advance()
					continue
				}
				break
			}

		case p.cur.Type == kwSIZE:
			p.advance()
			stmt.Size = p.parseSizeValue()

		case p.isIdentLike() && p.cur.Str == "AUTOEXTEND":
			stmt.Autoextend = p.parseAutoextendClause()

		case p.isIdentLike() && p.cur.Str == "REUSE":
			p.advance()
			// Attach REUSE to last datafile if exists
			if len(stmt.Datafiles) > 0 {
				stmt.Datafiles[len(stmt.Datafiles)-1].Reuse = true
			}

		case p.cur.Type == kwLOGGING:
			p.advance()
			stmt.Logging = "LOGGING"

		case p.cur.Type == kwNOLOGGING:
			p.advance()
			stmt.Logging = "NOLOGGING"

		case p.isIdentLike() && p.cur.Str == "FILESYSTEM_LIKE_LOGGING":
			p.advance()
			stmt.Logging = "FILESYSTEM_LIKE_LOGGING"

		case p.isIdentLike() && p.cur.Str == "FORCE":
			p.advance()
			if p.cur.Type == kwLOGGING {
				p.advance()
			}
			stmt.Logging = "FORCE LOGGING"

		case p.cur.Type == kwONLINE:
			p.advance()
			stmt.Online = true

		case p.cur.Type == kwOFFLINE:
			p.advance()
			stmt.Offline = true

		case p.isIdentLike() && p.cur.Str == "MINIMUM":
			p.advance() // MINIMUM
			if p.isIdentLike() && p.cur.Str == "EXTENT" {
				p.advance() // EXTENT
			}
			stmt.MinimumExtent = p.parseSizeValue()

		case p.isIdentLike() && p.cur.Str == "EXTENT":
			stmt.Extent = p.parseExtentManagementClause()

		case p.isIdentLike() && p.cur.Str == "SEGMENT":
			stmt.Segment = p.parseSegmentManagementClause()

		case p.isIdentLike() && p.cur.Str == "BLOCKSIZE":
			p.advance()
			stmt.Blocksize = p.parseSizeValue()

		case p.isIdentLike() && p.cur.Str == "RETENTION":
			p.advance()
			if p.isIdentLike() && p.cur.Str == "GUARANTEE" {
				p.advance()
				stmt.Retention = "GUARANTEE"
			} else if p.isIdentLike() && p.cur.Str == "NOGUARANTEE" {
				p.advance()
				stmt.Retention = "NOGUARANTEE"
			}

		case p.isIdentLike() && p.cur.Str == "ENCRYPTION":
			stmt.Encryption, stmt.EncryptionAlgorithm = p.parseTablespaceEncryptionClause()

		case p.cur.Type == kwDEFAULT:
			p.advance()
			stmt.DefaultParams = p.parseDefaultTablespaceParams()

		case p.isIdentLike() && p.cur.Str == "MAXSIZE":
			p.advance()
			if p.isIdentLike() && p.cur.Str == "UNLIMITED" {
				p.advance()
				stmt.MaxSize = "UNLIMITED"
			} else {
				stmt.MaxSize = p.parseSizeValue()
			}

		case p.cur.Type == kwSTORAGE:
			p.advance()
			if p.cur.Type == '(' {
				p.skipParens()
			}

		case p.isIdentLike() && p.cur.Str == "FLASHBACK":
			p.advance()
			if p.cur.Type == kwON || (p.isIdentLike() && p.cur.Str == "ON") {
				p.advance()
				stmt.Flashback = "ON"
			} else if p.isIdentLike() && p.cur.Str == "OFF" {
				p.advance()
				stmt.Flashback = "OFF"
			}

		case p.isIdentLike() && (p.cur.Str == "ENABLE" || p.cur.Str == "DISABLE" || p.cur.Str == "SUSPEND" || p.cur.Str == "REMOVE"):
			stmt.LostWriteProtection = p.parseLostWriteProtection()

		case p.cur.Type == kwTABLESPACE:
			// TABLESPACE GROUP clause (in temporary tablespace context)
			p.advance() // TABLESPACE
			if p.cur.Type == kwGROUP || (p.isIdentLike() && p.cur.Str == "GROUP") {
				p.advance() // GROUP
				if p.cur.Type == tokSCONST {
					stmt.TablespaceGroup = p.cur.Str
					p.advance()
				} else if p.isIdentLike() || p.cur.Type == tokIDENT {
					stmt.TablespaceGroup = p.cur.Str
					p.advance()
				}
			}

		case p.cur.Type == kwFOR:
			// FOR { ALL | LEAF } in temporary tablespace
			p.advance()
			if p.cur.Type == kwALL || (p.isIdentLike() && p.cur.Str == "ALL") {
				p.advance()
				stmt.ForLeaf = "ALL"
			} else if p.isIdentLike() && p.cur.Str == "LEAF" {
				p.advance()
				stmt.ForLeaf = "LEAF"
			}

		default:
			p.advance()
		}
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseExtentManagementClause parses EXTENT MANAGEMENT { LOCAL [...] | DICTIONARY }.
func (p *Parser) parseExtentManagementClause() string {
	p.advance() // EXTENT
	if p.isIdentLike() && p.cur.Str == "MANAGEMENT" {
		p.advance() // MANAGEMENT
	}
	if p.isIdentLike() && p.cur.Str == "DICTIONARY" {
		p.advance()
		return "DICTIONARY"
	}
	if p.isIdentLike() && p.cur.Str == "LOCAL" {
		p.advance() // LOCAL
	}
	if p.isIdentLike() && p.cur.Str == "AUTOALLOCATE" {
		p.advance()
		return "AUTOALLOCATE"
	} else if p.isIdentLike() && p.cur.Str == "UNIFORM" {
		p.advance()
		if p.cur.Type == kwSIZE {
			p.advance()
			return "UNIFORM SIZE " + p.parseSizeValue()
		}
		return "UNIFORM"
	}
	return "LOCAL"
}

// parseSegmentManagementClause parses SEGMENT SPACE MANAGEMENT { AUTO | MANUAL }.
func (p *Parser) parseSegmentManagementClause() string {
	p.advance() // SEGMENT
	if p.isIdentLike() && p.cur.Str == "SPACE" {
		p.advance() // SPACE
	}
	if p.isIdentLike() && p.cur.Str == "MANAGEMENT" {
		p.advance() // MANAGEMENT
	}
	if p.isIdentLike() && p.cur.Str == "AUTO" {
		p.advance()
		return "AUTO"
	} else if p.isIdentLike() && p.cur.Str == "MANUAL" {
		p.advance()
		return "MANUAL"
	}
	return "AUTO"
}

// parseTablespaceEncryptionClause parses ENCRYPTION [ USING 'algo' ] { ENCRYPT | DECRYPT }.
// Returns (encryption_summary, algorithm).
func (p *Parser) parseTablespaceEncryptionClause() (string, string) {
	p.advance() // ENCRYPTION
	algo := ""
	if p.isIdentLike() && p.cur.Str == "USING" {
		p.advance()
		if p.cur.Type == tokSCONST {
			algo = p.cur.Str
			p.advance()
		}
	}
	if p.isIdentLike() && p.cur.Str == "ENCRYPT" {
		p.advance()
		return "ENCRYPT", algo
	} else if p.isIdentLike() && p.cur.Str == "DECRYPT" {
		p.advance()
		return "DECRYPT", algo
	}
	// For ALTER TABLESPACE: ENCRYPTION ONLINE/OFFLINE/FINISH
	return "ENCRYPTION", algo
}

// parseDefaultTablespaceParams parses DEFAULT tablespace params.
// Returns a summary string of the parsed params.
func (p *Parser) parseDefaultTablespaceParams() string {
	result := ""
	for p.cur.Type != ';' && p.cur.Type != tokEOF && !p.isTablespaceClauseStart() {
		switch {
		case p.cur.Type == kwTABLE:
			p.advance() // TABLE
			if p.cur.Type == kwCOMPRESS {
				p.advance()
				result = p.appendParam(result, "TABLE COMPRESS")
				// FOR OLTP | QUERY | ARCHIVE
				if p.cur.Type == kwFOR {
					p.advance()
					if p.isIdentLike() {
						result = p.appendParam(result, p.cur.Str)
						p.advance()
						// LOW | HIGH
						if p.isIdentLike() && (p.cur.Str == "LOW" || p.cur.Str == "HIGH") {
							result = p.appendParam(result, p.cur.Str)
							p.advance()
						}
					}
				}
			} else if p.cur.Type == kwNOCOMPRESS {
				p.advance()
				result = p.appendParam(result, "TABLE NOCOMPRESS")
			} else if p.isIdentLike() && p.cur.Str == "ROW" {
				p.advance() // ROW
				if p.cur.Type == kwSTORAGE || (p.isIdentLike() && p.cur.Str == "STORE") {
					p.advance() // STORE
				}
				if p.cur.Type == kwCOMPRESS {
					p.advance()
					result = p.appendParam(result, "ROW STORE COMPRESS")
					if p.isIdentLike() && (p.cur.Str == "BASIC" || p.cur.Str == "ADVANCED") {
						result = p.appendParam(result, p.cur.Str)
						p.advance()
					}
				}
			} else if p.isIdentLike() && p.cur.Str == "COLUMN" {
				p.advance() // COLUMN
				if p.isIdentLike() && p.cur.Str == "STORE" {
					p.advance() // STORE
				}
				if p.cur.Type == kwCOMPRESS {
					p.advance()
					result = p.appendParam(result, "COLUMN STORE COMPRESS")
					if p.cur.Type == kwFOR {
						p.advance()
						if p.isIdentLike() {
							result = p.appendParam(result, p.cur.Str)
							p.advance()
							if p.isIdentLike() && (p.cur.Str == "LOW" || p.cur.Str == "HIGH") {
								result = p.appendParam(result, p.cur.Str)
								p.advance()
							}
						}
					}
				}
			}

		case p.cur.Type == kwINDEX:
			p.advance() // INDEX
			if p.cur.Type == kwCOMPRESS {
				p.advance()
				result = p.appendParam(result, "INDEX COMPRESS")
				if p.isIdentLike() && p.cur.Str == "ADVANCED" {
					p.advance()
					result = p.appendParam(result, "ADVANCED")
					if p.isIdentLike() && (p.cur.Str == "LOW" || p.cur.Str == "HIGH") {
						result = p.appendParam(result, p.cur.Str)
						p.advance()
					}
				}
			} else if p.cur.Type == kwNOCOMPRESS {
				p.advance()
				result = p.appendParam(result, "INDEX NOCOMPRESS")
			}

		case p.cur.Type == kwCOMPRESS:
			p.advance()
			result = p.appendParam(result, "COMPRESS")

		case p.cur.Type == kwNOCOMPRESS:
			p.advance()
			result = p.appendParam(result, "NOCOMPRESS")

		case p.isIdentLike() && p.cur.Str == "INMEMORY":
			p.advance()
			result = p.appendParam(result, "INMEMORY")
			// Skip inmemory attributes
			for p.isIdentLike() && (p.cur.Str == "MEMCOMPRESS" || p.cur.Str == "PRIORITY" || p.cur.Str == "DISTRIBUTE" || p.cur.Str == "DUPLICATE") {
				result = p.appendParam(result, p.cur.Str)
				p.advance()
				for p.isIdentLike() && (p.cur.Str == "FOR" || p.cur.Str == "DML" || p.cur.Str == "QUERY" || p.cur.Str == "CAPACITY" || p.cur.Str == "LOW" || p.cur.Str == "HIGH" || p.cur.Str == "NONE" || p.cur.Str == "MEDIUM" || p.cur.Str == "CRITICAL" || p.cur.Str == "AUTO" || p.cur.Str == "BY" || p.cur.Str == "ROWID" || p.cur.Str == "RANGE" || p.cur.Str == "ALL") {
					p.advance()
				}
				if p.cur.Type == kwPARTITION || (p.isIdentLike() && p.cur.Str == "SUBPARTITION") {
					p.advance()
				}
			}

		case p.isIdentLike() && p.cur.Str == "NO":
			p.advance()
			if p.isIdentLike() && p.cur.Str == "INMEMORY" {
				p.advance()
				result = p.appendParam(result, "NO INMEMORY")
			} else if p.isIdentLike() && p.cur.Str == "MEMCOMPRESS" {
				p.advance()
				result = p.appendParam(result, "NO MEMCOMPRESS")
			} else if p.isIdentLike() && p.cur.Str == "DUPLICATE" {
				p.advance()
				result = p.appendParam(result, "NO DUPLICATE")
			}

		case p.cur.Type == kwSTORAGE:
			p.advance()
			result = p.appendParam(result, "STORAGE")
			if p.cur.Type == '(' {
				p.skipParens()
			}

		default:
			p.advance()
		}
	}
	if result == "" {
		result = "DEFAULT"
	}
	return result
}

// appendParam appends a parameter to a space-separated string.
func (p *Parser) appendParam(base, param string) string {
	if base == "" {
		return param
	}
	return base + " " + param
}

// parseLostWriteProtection parses { ENABLE | DISABLE | SUSPEND | REMOVE } LOST WRITE PROTECTION.
func (p *Parser) parseLostWriteProtection() string {
	action := p.cur.Str
	p.advance() // ENABLE/DISABLE/SUSPEND/REMOVE
	if p.isIdentLike() && p.cur.Str == "LOST" {
		p.advance() // LOST
	}
	if p.isIdentLike() && p.cur.Str == "WRITE" {
		p.advance() // WRITE
	}
	if p.isIdentLike() && p.cur.Str == "PROTECTION" {
		p.advance() // PROTECTION
	}
	return action
}

// parseAlterTablespaceStmt parses an ALTER TABLESPACE statement.
//
// BNF: oracle/parser/bnf/ALTER-TABLESPACE.bnf
//
//	ALTER TABLESPACE [ IF EXISTS ] tablespace
//	    alter_tablespace_attrs
//
//	alter_tablespace_attrs:
//	    { default_tablespace_params
//	    | MINIMUM EXTENT size_clause
//	    | RESIZE size_clause
//	    | COALESCE
//	    | SHRINK SPACE [ KEEP size_clause ]
//	    | RENAME TO new_tablespace_name
//	    | BEGIN BACKUP
//	    | END BACKUP
//	    | datafile_tempfile_clauses
//	    | tablespace_logging_clauses
//	    | tablespace_group_clause
//	    | tablespace_state_clauses
//	    | autoextend_clause
//	    | flashback_mode_clause
//	    | tablespace_retention_clause
//	    | alter_tablespace_encryption
//	    | lost_write_protection
//	    }
//
//	datafile_tempfile_clauses:
//	    { ADD { DATAFILE | TEMPFILE } [ file_specification [, file_specification ]... ]
//	    | DROP { DATAFILE | TEMPFILE } { 'filename' | file_number }
//	    | SHRINK TEMPFILE { 'filename' | file_number } [ KEEP size_clause ]
//	    | RENAME DATAFILE 'filename' [, 'filename' ]... TO 'filename' [, 'filename' ]...
//	    | { DATAFILE | TEMPFILE } { ONLINE | OFFLINE }
//	    }
//
//	tablespace_logging_clauses:
//	    { logging_clause | [ NO ] FORCE LOGGING }
//
//	tablespace_state_clauses:
//	    { ONLINE | OFFLINE [ NORMAL | TEMPORARY | IMMEDIATE ]
//	    | READ ONLY | READ WRITE | PERMANENT | TEMPORARY }
//
//	alter_tablespace_encryption:
//	    ENCRYPTION
//	        { ONLINE [ tablespace_encryption_spec ] [ ts_file_name_convert ]
//	        | OFFLINE { ENCRYPT [ tablespace_encryption_spec ] | DECRYPT }
//	        | FINISH [ ENCRYPT | DECRYPT ] [ ts_file_name_convert ]
//	        }
//
//	lost_write_protection:
//	    { ENABLE | DISABLE | REMOVE | SUSPEND } LOST WRITE PROTECTION
func (p *Parser) parseAlterTablespaceStmt(start int, isSet bool) *nodes.AlterTablespaceStmt {
	stmt := &nodes.AlterTablespaceStmt{
		IsSet: isSet,
		Loc:   nodes.Loc{Start: start},
	}

	// Optional IF EXISTS (not for SET)
	if !isSet && p.cur.Type == kwIF {
		p.advance() // IF
		if p.cur.Type == kwEXISTS {
			p.advance() // EXISTS
		}
		stmt.IfExists = true
	}

	// Parse tablespace name
	stmt.Name = p.parseObjectName()

	// Parse alter clauses
	for p.cur.Type != ';' && p.cur.Type != tokEOF {
		switch {
		case p.cur.Type == kwDEFAULT:
			p.advance()
			stmt.DefaultParams = p.parseDefaultTablespaceParams()

		case p.isIdentLike() && p.cur.Str == "MINIMUM":
			p.advance() // MINIMUM
			if p.isIdentLike() && p.cur.Str == "EXTENT" {
				p.advance() // EXTENT
			}
			stmt.MinimumExtent = p.parseSizeValue()

		case p.isIdentLike() && p.cur.Str == "RESIZE":
			p.advance()
			stmt.Resize = p.parseSizeValue()

		case p.isIdentLike() && p.cur.Str == "COALESCE":
			p.advance()
			stmt.Coalesce = true

		case p.isIdentLike() && p.cur.Str == "SHRINK":
			p.advance() // SHRINK
			if p.isIdentLike() && p.cur.Str == "TEMPFILE" {
				p.advance() // TEMPFILE
				// { 'filename' | file_number }
				if p.cur.Type == tokSCONST {
					stmt.ShrinkTempfile = p.cur.Str
					p.advance()
				} else if p.cur.Type == tokICONST {
					stmt.ShrinkTempfile = p.cur.Str
					p.advance()
				}
				if p.isIdentLike() && p.cur.Str == "KEEP" {
					p.advance()
					stmt.ShrinkTempfileKeep = p.parseSizeValue()
				}
			} else {
				// SHRINK SPACE
				if p.isIdentLike() && p.cur.Str == "SPACE" {
					p.advance()
				}
				stmt.ShrinkSpace = true
				if p.isIdentLike() && p.cur.Str == "KEEP" {
					p.advance()
					stmt.ShrinkKeep = p.parseSizeValue()
				}
			}

		case p.isIdentLike() && p.cur.Str == "RENAME":
			p.advance() // RENAME
			if p.isIdentLike() && p.cur.Str == "DATAFILE" {
				p.advance() // DATAFILE
				stmt.RenameDatafile = true
				// Parse old filenames
				for {
					if p.cur.Type == tokSCONST {
						stmt.RenameFrom = append(stmt.RenameFrom, p.cur.Str)
						p.advance()
					}
					if p.cur.Type == ',' {
						p.advance()
						continue
					}
					break
				}
				// TO
				if p.cur.Type == kwTO {
					p.advance()
				}
				// Parse new filenames
				for {
					if p.cur.Type == tokSCONST {
						stmt.RenameTo2 = append(stmt.RenameTo2, p.cur.Str)
						p.advance()
					}
					if p.cur.Type == ',' {
						p.advance()
						continue
					}
					break
				}
			} else {
				// RENAME TO new_name
				if p.cur.Type == kwTO {
					p.advance()
				}
				stmt.RenameTo = p.parseIdentifier()
			}

		case p.isIdentLike() && p.cur.Str == "BEGIN":
			p.advance() // BEGIN
			if p.isIdentLike() && p.cur.Str == "BACKUP" {
				p.advance()
			}
			stmt.BeginBackup = true

		case p.isIdentLike() && p.cur.Str == "END":
			p.advance() // END
			if p.isIdentLike() && p.cur.Str == "BACKUP" {
				p.advance()
			}
			stmt.EndBackup = true

		case p.cur.Type == kwADD:
			p.advance() // ADD
			if p.isIdentLike() && p.cur.Str == "DATAFILE" {
				p.advance()
				stmt.AddDatafile = true
			} else if p.isIdentLike() && p.cur.Str == "TEMPFILE" {
				p.advance()
				stmt.AddTempfile = true
			}
			// Parse file specifications
			for {
				df := p.parseDatafileClause()
				if df != nil {
					stmt.Datafiles = append(stmt.Datafiles, df)
				}
				if p.cur.Type == ',' {
					p.advance()
					continue
				}
				break
			}

		case p.cur.Type == kwDROP:
			p.advance() // DROP
			if p.isIdentLike() && p.cur.Str == "DATAFILE" {
				p.advance()
				stmt.DropDatafile = true
			} else if p.isIdentLike() && p.cur.Str == "TEMPFILE" {
				p.advance()
				stmt.DropTempfile = true
			}
			// { 'filename' | file_number }
			if p.cur.Type == tokSCONST {
				stmt.DropFileRef = p.cur.Str
				p.advance()
			} else if p.cur.Type == tokICONST {
				stmt.DropFileRef = p.cur.Str
				p.advance()
			}

		case p.isIdentLike() && p.cur.Str == "DATAFILE":
			p.advance() // DATAFILE
			if p.cur.Type == kwONLINE {
				p.advance()
				stmt.DatafileOnline = true
			} else if p.cur.Type == kwOFFLINE {
				p.advance()
				stmt.DatafileOffline = true
			}

		case p.isIdentLike() && p.cur.Str == "TEMPFILE":
			p.advance() // TEMPFILE
			if p.cur.Type == kwONLINE {
				p.advance()
				stmt.TempfileOnline = true
			} else if p.cur.Type == kwOFFLINE {
				p.advance()
				stmt.TempfileOffline = true
			}

		case p.cur.Type == kwLOGGING:
			p.advance()
			stmt.Logging = "LOGGING"

		case p.cur.Type == kwNOLOGGING:
			p.advance()
			stmt.Logging = "NOLOGGING"

		case p.isIdentLike() && p.cur.Str == "FILESYSTEM_LIKE_LOGGING":
			p.advance()
			stmt.Logging = "FILESYSTEM_LIKE_LOGGING"

		case p.isIdentLike() && p.cur.Str == "FORCE":
			p.advance() // FORCE
			if p.cur.Type == kwLOGGING {
				p.advance()
			}
			stmt.ForceLogging = "FORCE LOGGING"

		case p.isIdentLike() && p.cur.Str == "NO":
			next := p.peekNext()
			if next.Type == kwFORCE || (next.Str == "FORCE" && (next.Type == tokIDENT || next.Type >= 2000)) {
				p.advance() // NO
				p.advance() // FORCE
				if p.cur.Type == kwLOGGING {
					p.advance()
				}
				stmt.ForceLogging = "NO FORCE LOGGING"
			} else {
				p.advance()
			}

		case p.cur.Type == kwONLINE:
			p.advance()
			stmt.Online = true

		case p.cur.Type == kwOFFLINE:
			p.advance()
			stmt.Offline = true
			// Optional NORMAL | TEMPORARY | IMMEDIATE
			if p.isIdentLike() && p.cur.Str == "NORMAL" {
				p.advance()
				stmt.OfflineMode = "NORMAL"
			} else if p.cur.Type == kwTEMPORARY {
				p.advance()
				stmt.OfflineMode = "TEMPORARY"
			} else if p.isIdentLike() && p.cur.Str == "IMMEDIATE" {
				p.advance()
				stmt.OfflineMode = "IMMEDIATE"
			}

		case p.cur.Type == kwREAD:
			p.advance() // READ
			if p.isIdentLike() && p.cur.Str == "ONLY" {
				p.advance()
				stmt.ReadOnly = true
			} else if p.isIdentLike() && p.cur.Str == "WRITE" {
				p.advance()
				stmt.ReadWrite = true
			}

		case p.isIdentLike() && p.cur.Str == "PERMANENT":
			p.advance()
			stmt.Permanent = true

		case p.cur.Type == kwTEMPORARY:
			p.advance()
			stmt.TempState = true

		case p.isIdentLike() && p.cur.Str == "AUTOEXTEND":
			stmt.Autoextend = p.parseAutoextendClause()

		case p.isIdentLike() && p.cur.Str == "FLASHBACK":
			p.advance()
			if p.cur.Type == kwON || (p.isIdentLike() && p.cur.Str == "ON") {
				p.advance()
				stmt.Flashback = "ON"
			} else if p.isIdentLike() && p.cur.Str == "OFF" {
				p.advance()
				stmt.Flashback = "OFF"
			}

		case p.isIdentLike() && p.cur.Str == "RETENTION":
			p.advance()
			if p.isIdentLike() && p.cur.Str == "GUARANTEE" {
				p.advance()
				stmt.Retention = "GUARANTEE"
			} else if p.isIdentLike() && p.cur.Str == "NOGUARANTEE" {
				p.advance()
				stmt.Retention = "NOGUARANTEE"
			}

		case p.isIdentLike() && p.cur.Str == "ENCRYPTION":
			p.advance() // ENCRYPTION
			// ALTER has ONLINE/OFFLINE/FINISH sub-clauses
			enc := "ENCRYPTION"
			if p.isIdentLike() && p.cur.Str == "ONLINE" {
				p.advance()
				enc = "ONLINE"
			} else if p.cur.Type == kwOFFLINE {
				p.advance()
				enc = "OFFLINE"
			} else if p.isIdentLike() && p.cur.Str == "FINISH" {
				p.advance()
				enc = "FINISH"
			}
			// Skip remaining encryption sub-clause tokens
			for p.cur.Type != ';' && p.cur.Type != tokEOF && !p.isAlterTablespaceClauseStart() {
				p.advance()
			}
			stmt.Encryption = enc

		case p.isIdentLike() && (p.cur.Str == "ENABLE" || p.cur.Str == "DISABLE" || p.cur.Str == "SUSPEND" || p.cur.Str == "REMOVE"):
			stmt.LostWriteProtection = p.parseLostWriteProtection()

		case p.cur.Type == kwGROUP || (p.isIdentLike() && p.cur.Str == "GROUP"):
			p.advance() // GROUP
			if p.cur.Type == tokSCONST {
				stmt.TablespaceGroup = p.cur.Str
				p.advance()
			} else if p.isIdentLike() || p.cur.Type == tokIDENT {
				stmt.TablespaceGroup = p.cur.Str
				p.advance()
			}

		default:
			p.advance()
		}
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// isAlterTablespaceClauseStart returns true if the current token starts a known ALTER TABLESPACE clause.
func (p *Parser) isAlterTablespaceClauseStart() bool {
	if p.isIdentLike() {
		switch p.cur.Str {
		case "DATAFILE", "TEMPFILE", "AUTOEXTEND", "EXTENT", "SEGMENT",
			"BLOCKSIZE", "RETENTION", "ENCRYPTION", "MAXSIZE", "FORCE",
			"MINIMUM", "RESIZE", "COALESCE", "SHRINK", "RENAME",
			"BEGIN", "END", "FLASHBACK", "ENABLE", "DISABLE",
			"SUSPEND", "REMOVE", "PERMANENT", "NO", "GROUP",
			"FILESYSTEM_LIKE_LOGGING":
			return true
		}
	}
	switch p.cur.Type {
	case kwLOGGING, kwNOLOGGING, kwONLINE, kwOFFLINE, kwDEFAULT, kwREAD, kwADD, kwDROP, kwTEMPORARY, kwGROUP:
		return true
	}
	return false
}

// isTablespaceClauseStart returns true if the current token starts a known tablespace clause.
func (p *Parser) isTablespaceClauseStart() bool {
	if p.isIdentLike() {
		switch p.cur.Str {
		case "DATAFILE", "TEMPFILE", "AUTOEXTEND", "REUSE", "EXTENT", "SEGMENT",
			"BLOCKSIZE", "RETENTION", "ENCRYPTION", "MAXSIZE", "FORCE",
			"MINIMUM", "FLASHBACK", "ENABLE", "DISABLE", "SUSPEND", "REMOVE",
			"FILESYSTEM_LIKE_LOGGING":
			return true
		}
	}
	switch p.cur.Type {
	case kwSIZE, kwLOGGING, kwNOLOGGING, kwONLINE, kwOFFLINE, kwDEFAULT, kwSTORAGE, kwTABLESPACE, kwFOR:
		return true
	}
	return false
}

// parseCreateTablespaceSetStmt parses a CREATE TABLESPACE SET statement.
//
// BNF: oracle/parser/bnf/CREATE-TABLESPACE-SET.bnf
//
//	CREATE TABLESPACE SET tablespace_set
//	    [ IN SHARDSPACE shardspace_name ]
//	    [ USING TEMPLATE
//	      ( [ DATAFILE [ file_specification ] ]
//	        [ permanent_tablespace_attrs ]
//	      )
//	    ] ;
//
//	permanent_tablespace_attrs:
//	    [ logging_clause ]
//	    [ tablespace_encryption_clause ]
//	    [ DEFAULT default_tablespace_params ]
//	    [ extent_management_clause ]
//	    [ segment_management_clause ]
//	    [ flashback_mode_clause ]
func (p *Parser) parseCreateTablespaceSetStmt(start int) *nodes.CreateTablespaceSetStmt {
	stmt := &nodes.CreateTablespaceSetStmt{
		Loc: nodes.Loc{Start: start},
	}

	stmt.Name = p.parseObjectName()

	// Optional IN SHARDSPACE
	if p.cur.Type == kwIN {
		p.advance() // IN
		if p.isIdentLike() && p.cur.Str == "SHARDSPACE" {
			p.advance() // SHARDSPACE
		}
		stmt.Shardspace = p.parseIdentifier()
	}

	// Optional USING TEMPLATE ( ... )
	if p.isIdentLike() && p.cur.Str == "USING" {
		p.advance() // USING
		if p.isIdentLike() && p.cur.Str == "TEMPLATE" {
			p.advance() // TEMPLATE
		}
		if p.cur.Type == '(' {
			p.advance() // consume (
			for p.cur.Type != ')' && p.cur.Type != tokEOF {
				switch {
				case p.isIdentLike() && p.cur.Str == "DATAFILE":
					p.advance()
					for {
						df := p.parseDatafileClause()
						if df != nil {
							stmt.Datafiles = append(stmt.Datafiles, df)
						}
						if p.cur.Type == ',' {
							p.advance()
							continue
						}
						break
					}

				case p.cur.Type == kwLOGGING:
					p.advance()
					stmt.Logging = "LOGGING"

				case p.cur.Type == kwNOLOGGING:
					p.advance()
					stmt.Logging = "NOLOGGING"

				case p.isIdentLike() && p.cur.Str == "FILESYSTEM_LIKE_LOGGING":
					p.advance()
					stmt.Logging = "FILESYSTEM_LIKE_LOGGING"

				case p.isIdentLike() && p.cur.Str == "ENCRYPTION":
					enc, _ := p.parseTablespaceEncryptionClause()
					stmt.Encryption = enc

				case p.cur.Type == kwDEFAULT:
					p.advance()
					stmt.DefaultParams = p.parseDefaultTablespaceParams()

				case p.isIdentLike() && p.cur.Str == "EXTENT":
					stmt.Extent = p.parseExtentManagementClause()

				case p.isIdentLike() && p.cur.Str == "SEGMENT":
					stmt.Segment = p.parseSegmentManagementClause()

				case p.isIdentLike() && p.cur.Str == "FLASHBACK":
					p.advance()
					if p.cur.Type == kwON || (p.isIdentLike() && p.cur.Str == "ON") {
						p.advance()
						stmt.Flashback = "ON"
					} else if p.isIdentLike() && p.cur.Str == "OFF" {
						p.advance()
						stmt.Flashback = "OFF"
					}

				default:
					p.advance()
				}
			}
			if p.cur.Type == ')' {
				p.advance()
			}
		}
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseDropTablespaceStmt parses a DROP TABLESPACE or DROP TABLESPACE SET statement.
//
// BNF: oracle/parser/bnf/DROP-TABLESPACE.bnf
//
//	DROP TABLESPACE [ IF EXISTS ] tablespace
//	    [ { DROP | KEEP } QUOTA ]
//	    [ INCLUDING CONTENTS [ { AND DATAFILES | KEEP DATAFILES } ]
//	      [ CASCADE CONSTRAINTS ] ] ;
//
// BNF: oracle/parser/bnf/DROP-TABLESPACE-SET.bnf
//
//	DROP TABLESPACE SET tablespace_set
//	    [ INCLUDING CONTENTS [ { AND DATAFILES | KEEP DATAFILES } ]
//	      [ CASCADE CONSTRAINTS ] ] ;
func (p *Parser) parseDropTablespaceStmt(start int, isSet bool) *nodes.DropTablespaceStmt {
	stmt := &nodes.DropTablespaceStmt{
		IsSet: isSet,
		Loc:   nodes.Loc{Start: start},
	}

	// Optional IF EXISTS (only for non-SET)
	if !isSet && p.cur.Type == kwIF {
		p.advance() // IF
		if p.cur.Type == kwEXISTS {
			p.advance() // EXISTS
		}
		stmt.IfExists = true
	}

	stmt.Name = p.parseObjectName()

	// Parse optional clauses
	for p.cur.Type != ';' && p.cur.Type != tokEOF {
		switch {
		case p.cur.Type == kwDROP:
			p.advance() // DROP
			if p.isIdentLike() && p.cur.Str == "QUOTA" {
				p.advance()
				stmt.DropQuota = true
			}

		case p.isIdentLike() && p.cur.Str == "KEEP":
			p.advance() // KEEP
			if p.isIdentLike() && p.cur.Str == "QUOTA" {
				p.advance()
				stmt.KeepQuota = true
			} else if p.isIdentLike() && p.cur.Str == "DATAFILES" {
				p.advance()
				stmt.KeepDatafiles = true
			}

		case p.isIdentLike() && p.cur.Str == "INCLUDING":
			p.advance() // INCLUDING
			if p.isIdentLike() && p.cur.Str == "CONTENTS" {
				p.advance()
				stmt.IncludingContents = true
			}

		case p.cur.Type == kwAND:
			p.advance() // AND
			if p.isIdentLike() && p.cur.Str == "DATAFILES" {
				p.advance()
				stmt.AndDatafiles = true
			}

		case p.cur.Type == kwCASCADE:
			p.advance() // CASCADE
			if p.cur.Type == kwCONSTRAINT || (p.isIdentLike() && p.cur.Str == "CONSTRAINTS") {
				p.advance()
				stmt.CascadeConstraints = true
			}

		default:
			p.advance()
		}
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseDatafileClause parses a single file specification (path string and optional SIZE).
func (p *Parser) parseDatafileClause() *nodes.DatafileClause {
	if p.cur.Type != tokSCONST {
		return nil
	}
	df := &nodes.DatafileClause{
		Loc: nodes.Loc{Start: p.pos()},
	}
	df.Filename = p.cur.Str
	p.advance()

	// Optional SIZE
	if p.cur.Type == kwSIZE {
		p.advance()
		df.Size = p.parseSizeValue()
	}

	// Optional REUSE
	if p.isIdentLike() && p.cur.Str == "REUSE" {
		p.advance()
		df.Reuse = true
	}

	// Optional AUTOEXTEND
	if p.isIdentLike() && p.cur.Str == "AUTOEXTEND" {
		df.Autoextend = p.parseAutoextendClause()
	}

	df.Loc.End = p.pos()
	return df
}

// parseAutoextendClause parses AUTOEXTEND ON/OFF with optional NEXT and MAXSIZE.
func (p *Parser) parseAutoextendClause() *nodes.AutoextendClause {
	ac := &nodes.AutoextendClause{
		Loc: nodes.Loc{Start: p.pos()},
	}
	p.advance() // consume AUTOEXTEND

	if p.cur.Type == kwON || (p.isIdentLike() && p.cur.Str == "ON") {
		p.advance()
		ac.On = true
		// Optional NEXT size
		if p.cur.Type == kwNEXT {
			p.advance()
			ac.Next = p.parseSizeValue()
		}
		// Optional MAXSIZE
		if p.isIdentLike() && p.cur.Str == "MAXSIZE" {
			p.advance()
			if p.isIdentLike() && p.cur.Str == "UNLIMITED" {
				p.advance()
				ac.MaxSize = "UNLIMITED"
			} else {
				ac.MaxSize = p.parseSizeValue()
			}
		}
	} else if p.isIdentLike() && p.cur.Str == "OFF" {
		p.advance()
		ac.On = false
	}

	ac.Loc.End = p.pos()
	return ac
}

// parseSizeValue parses a size value like "100M", "10G", "512", "8K".
// It combines the number and optional unit suffix into a single string.
func (p *Parser) parseSizeValue() string {
	if p.cur.Type == tokICONST || p.cur.Type == tokFCONST {
		val := p.cur.Str
		p.advance()
		// Check for size suffix (K, M, G, T, P, E) as an identifier
		if p.isIdentLike() {
			switch p.cur.Str {
			case "K", "M", "G", "T", "P", "E":
				val += p.cur.Str
				p.advance()
			}
		}
		return val
	}
	return ""
}

// skipParens skips balanced parentheses.
func (p *Parser) skipParens() {
	depth := 0
	if p.cur.Type == '(' {
		depth = 1
		p.advance()
	}
	for depth > 0 && p.cur.Type != tokEOF {
		switch p.cur.Type {
		case '(':
			depth++
		case ')':
			depth--
		}
		p.advance()
	}
}

// parseCreateClusterStmt parses a CREATE CLUSTER statement.
//
// Ref: https://docs.oracle.com/en/database/oracle/oracle-database/23/sqlrf/CREATE-CLUSTER.html
//
//	CREATE CLUSTER [ schema. ] cluster
//	    ( { column datatype [ COLLATE collation ] [ SORT ] } [, ...] )
//	    [ physical_attributes_clause ]
//	    [ SIZE size_clause ]
//	    [ TABLESPACE tablespace ]
//	    [ { INDEX
//	      | [ SINGLE TABLE ] HASHKEYS integer [ HASH IS expr ] } ]
//	    [ parallel_clause ]
//	    [ NOROWDEPENDENCIES | ROWDEPENDENCIES ]
//	    [ CACHE | NOCACHE ]
//	    [ cluster_range_partitions ]
func (p *Parser) parseCreateClusterStmt(start int) *nodes.CreateClusterStmt {
	stmt := &nodes.CreateClusterStmt{
		Loc: nodes.Loc{Start: start},
	}

	stmt.Name = p.parseObjectName()

	// Parse column list in parentheses: ( col datatype [SORT] [, ...] )
	if p.cur.Type == '(' {
		p.advance()
		for p.cur.Type != ')' && p.cur.Type != tokEOF {
			col := &nodes.ClusterColumn{
				Loc: nodes.Loc{Start: p.pos()},
			}
			col.Name = p.parseIdentifier()
			col.DataType = p.parseTypeName()
			// Optional COLLATE
			if p.isIdentLike() && p.cur.Str == "COLLATE" {
				p.advance()
				p.parseIdentifier() // collation name
			}
			// Optional SORT
			if p.cur.Type == kwORDER || (p.isIdentLike() && p.cur.Str == "SORT") {
				p.advance()
				col.Sort = true
			}
			col.Loc.End = p.pos()
			stmt.Columns = append(stmt.Columns, col)
			if p.cur.Type == ',' {
				p.advance()
			}
		}
		if p.cur.Type == ')' {
			p.advance()
		}
	}

	// Parse options until ; or EOF
	for p.cur.Type != ';' && p.cur.Type != tokEOF {
		switch {
		case p.cur.Type == kwPCTFREE:
			p.advance()
			if p.cur.Type == tokICONST {
				v := p.parseIntValue()
				stmt.PctFree = &v
			}

		case p.isIdentLike() && p.cur.Str == "PCTUSED":
			p.advance()
			if p.cur.Type == tokICONST {
				v := p.parseIntValue()
				stmt.PctUsed = &v
			}

		case p.isIdentLike() && p.cur.Str == "INITRANS":
			p.advance()
			if p.cur.Type == tokICONST {
				v := p.parseIntValue()
				stmt.InitTrans = &v
			}

		case p.cur.Type == kwSIZE:
			p.advance()
			stmt.Size = p.parseSizeValue()

		case p.cur.Type == kwTABLESPACE:
			p.advance()
			stmt.Tablespace = p.parseIdentifier()

		case p.cur.Type == kwINDEX:
			p.advance()
			stmt.IsIndex = true

		case p.isIdentLike() && p.cur.Str == "SINGLE":
			p.advance()
			if p.cur.Type == kwTABLE {
				p.advance()
			}
			stmt.SingleTable = true

		case p.isIdentLike() && p.cur.Str == "HASHKEYS":
			p.advance()
			stmt.IsHash = true
			if p.cur.Type == tokICONST {
				stmt.HashKeys = p.cur.Str
				p.advance()
			}

		case p.cur.Type == kwHASH:
			p.advance()
			if p.isIdentLike() && p.cur.Str == "IS" {
				p.advance()
				stmt.HashExpr = p.parseExpr()
			}

		case p.cur.Type == kwCACHE:
			p.advance()
			stmt.Cache = true

		case p.cur.Type == kwNOCACHE:
			p.advance()
			stmt.NoCache = true

		case p.isIdentLike() && p.cur.Str == "PARALLEL":
			p.advance()
			if p.cur.Type == tokICONST {
				stmt.Parallel = p.cur.Str
				p.advance()
			} else {
				stmt.Parallel = "PARALLEL"
			}

		case p.isIdentLike() && p.cur.Str == "NOPARALLEL":
			p.advance()
			stmt.Parallel = "NOPARALLEL"

		case p.isIdentLike() && p.cur.Str == "ROWDEPENDENCIES":
			p.advance()
			stmt.RowDep = true

		case p.isIdentLike() && p.cur.Str == "NOROWDEPENDENCIES":
			p.advance()
			stmt.NoRowDep = true

		case p.cur.Type == kwSTORAGE:
			p.advance()
			if p.cur.Type == '(' {
				// Collect storage tokens
				start := p.pos()
				p.skipParens()
				_ = start
			}

		case p.isIdentLike() && p.cur.Str == "SHARING":
			p.advance()
			if p.cur.Type == '=' {
				p.advance()
			}
			p.parseIdentifier() // METADATA, DATA, NONE, etc.

		case p.isIdentLike() && p.cur.Str == "PARTITION":
			// cluster_range_partitions - skip until end
			for p.cur.Type != ';' && p.cur.Type != tokEOF {
				p.advance()
			}

		default:
			p.advance()
		}
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseIntValue parses an integer constant and returns its value.
func (p *Parser) parseIntValue() int {
	if p.cur.Type == tokICONST {
		val := 0
		for _, c := range p.cur.Str {
			val = val*10 + int(c-'0')
		}
		p.advance()
		return val
	}
	return 0
}

// parseCreateDimensionStmt parses a CREATE DIMENSION statement.
//
// Ref: https://docs.oracle.com/en/database/oracle/oracle-database/23/sqlrf/CREATE-DIMENSION.html
//
//	CREATE DIMENSION [ schema. ] dimension
//	    { LEVEL level IS ( level_table.level_column [, ...] ) [ SKIP WHEN NULL ] } ...
//	    { HIERARCHY hierarchy (
//	        child_level CHILD OF parent_level [ CHILD OF ... ]
//	        [ JOIN KEY ( child_key_column [, ...] ) REFERENCES parent_level ] ...
//	      )
//	    | ATTRIBUTE level DETERMINES ( dependent_column [, ...] )
//	    | ATTRIBUTE attr_name LEVEL level DETERMINES ( dependent_column [, ...] )
//	    } ...
func (p *Parser) parseCreateDimensionStmt(start int) *nodes.CreateDimensionStmt {
	stmt := &nodes.CreateDimensionStmt{
		Loc: nodes.Loc{Start: start},
	}

	stmt.Name = p.parseObjectName()

	// Parse clauses: LEVEL, HIERARCHY, ATTRIBUTE
	for p.cur.Type != ';' && p.cur.Type != tokEOF {
		switch {
		case p.cur.Type == kwLEVEL:
			stmt.Levels = append(stmt.Levels, p.parseDimensionLevel())

		case p.isIdentLike() && p.cur.Str == "HIERARCHY":
			stmt.Hierarchies = append(stmt.Hierarchies, p.parseDimensionHierarchy())

		case p.isIdentLike() && p.cur.Str == "ATTRIBUTE":
			stmt.Attributes = append(stmt.Attributes, p.parseDimensionAttribute())

		default:
			p.advance()
		}
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseDimensionLevel parses a LEVEL clause.
//
//	LEVEL level IS ( level_table.level_column [, ...] ) [ SKIP WHEN NULL ]
//	-- or without parens:
//	LEVEL level IS level_table.level_column [ SKIP WHEN NULL ]
func (p *Parser) parseDimensionLevel() *nodes.DimensionLevel {
	lvl := &nodes.DimensionLevel{
		Loc: nodes.Loc{Start: p.pos()},
	}
	p.advance() // consume LEVEL

	lvl.Name = p.parseIdentifier()

	// IS
	if p.isIdentLike() && p.cur.Str == "IS" {
		p.advance()
	}

	// Column list: ( col [, ...] ) or just col
	if p.cur.Type == '(' {
		p.advance()
		for p.cur.Type != ')' && p.cur.Type != tokEOF {
			lvl.Columns = append(lvl.Columns, p.parseObjectName())
			if p.cur.Type == ',' {
				p.advance()
			}
		}
		if p.cur.Type == ')' {
			p.advance()
		}
	} else {
		// Single column reference: table.column
		lvl.Columns = append(lvl.Columns, p.parseObjectName())
	}

	// Optional SKIP WHEN NULL
	if p.cur.Type == kwSKIP {
		p.advance() // SKIP
		if p.cur.Type == kwWHEN {
			p.advance() // WHEN
		}
		if p.cur.Type == kwNULL {
			p.advance() // NULL
		}
		lvl.SkipWhenNull = true
	}

	lvl.Loc.End = p.pos()
	return lvl
}

// parseDimensionHierarchy parses a HIERARCHY clause.
//
//	HIERARCHY hierarchy_name (
//	    child_level CHILD OF parent_level [ CHILD OF ... ]
//	    [ JOIN KEY ( child_key_column [, ...] ) REFERENCES parent_level ] ...
//	)
func (p *Parser) parseDimensionHierarchy() *nodes.DimensionHierarchy {
	hier := &nodes.DimensionHierarchy{
		Loc: nodes.Loc{Start: p.pos()},
	}
	p.advance() // consume HIERARCHY

	hier.Name = p.parseIdentifier()

	// Parse ( ... )
	if p.cur.Type == '(' {
		p.advance()

		// Parse level chain: child CHILD OF parent CHILD OF ...
		// First level
		if p.isIdentLike() {
			hier.Levels = append(hier.Levels, p.parseIdentifier())
		}

		for p.cur.Type != ')' && p.cur.Type != tokEOF {
			// Check for CHILD OF
			if p.isIdentLike() && p.cur.Str == "CHILD" {
				p.advance() // CHILD
				if p.cur.Type == kwOF {
					p.advance() // OF
				}
				// Next level
				if p.isIdentLike() {
					hier.Levels = append(hier.Levels, p.parseIdentifier())
				}
			} else if p.isIdentLike() && p.cur.Str == "JOIN" {
				// JOIN KEY clause
				jk := p.parseDimensionJoinKey()
				hier.JoinKeys = append(hier.JoinKeys, jk)
			} else {
				break
			}
		}

		if p.cur.Type == ')' {
			p.advance()
		}
	}

	hier.Loc.End = p.pos()
	return hier
}

// parseDimensionJoinKey parses a JOIN KEY clause.
//
//	JOIN KEY ( child_key_column [, ...] ) REFERENCES parent_level
func (p *Parser) parseDimensionJoinKey() *nodes.DimensionJoinKey {
	jk := &nodes.DimensionJoinKey{
		Loc: nodes.Loc{Start: p.pos()},
	}
	p.advance() // consume JOIN

	if p.isIdentLike() && p.cur.Str == "KEY" {
		p.advance() // KEY
	}

	// ( child_key_column [, ...] )
	if p.cur.Type == '(' {
		p.advance()
		for p.cur.Type != ')' && p.cur.Type != tokEOF {
			jk.ChildKeys = append(jk.ChildKeys, p.parseObjectName())
			if p.cur.Type == ',' {
				p.advance()
			}
		}
		if p.cur.Type == ')' {
			p.advance()
		}
	}

	// REFERENCES parent_level
	if p.cur.Type == kwREFERENCES {
		p.advance()
		jk.ParentLevel = p.parseIdentifier()
	}

	jk.Loc.End = p.pos()
	return jk
}

// parseDimensionAttribute parses an ATTRIBUTE clause.
//
//	ATTRIBUTE level DETERMINES ( dependent_column [, ...] )
//	-- or extended form:
//	ATTRIBUTE attr_name LEVEL level DETERMINES ( dependent_column [, ...] )
func (p *Parser) parseDimensionAttribute() *nodes.DimensionAttribute {
	attr := &nodes.DimensionAttribute{
		Loc: nodes.Loc{Start: p.pos()},
	}
	p.advance() // consume ATTRIBUTE

	attr.AttrName = p.parseIdentifier()

	// Check for extended form: LEVEL level DETERMINES ...
	if p.cur.Type == kwLEVEL {
		p.advance()
		attr.LevelName = p.parseIdentifier()
	}

	// DETERMINES
	if p.isIdentLike() && p.cur.Str == "DETERMINES" {
		p.advance()
	}

	// ( dependent_column [, ...] ) or single column
	if p.cur.Type == '(' {
		p.advance()
		for p.cur.Type != ')' && p.cur.Type != tokEOF {
			attr.Columns = append(attr.Columns, p.parseObjectName())
			if p.cur.Type == ',' {
				p.advance()
			}
		}
		if p.cur.Type == ')' {
			p.advance()
		}
	} else {
		attr.Columns = append(attr.Columns, p.parseObjectName())
	}

	attr.Loc.End = p.pos()
	return attr
}

// parseAlterAdminObject handles ALTER dispatches for admin DDL objects.
func (p *Parser) parseAlterAdminObject(start int) nodes.StmtNode {
	switch p.cur.Type {
	case kwUSER:
		p.advance()
		return p.parseAlterUserStmt(start)
	case kwROLE:
		p.advance()
		return p.parseAlterRoleStmt(start)
	case kwPROFILE:
		p.advance()
		return p.parseAlterProfileStmt(start)
	case kwTABLESPACE:
		p.advance()
		if p.cur.Type == kwSET {
			p.advance() // consume SET
			return p.parseAlterTablespaceStmt(start, true)
		}
		return p.parseAlterTablespaceStmt(start, false)
	case kwCLUSTER:
		p.advance()
		return p.parseAdminDDLStmt("ALTER", nodes.OBJECT_CLUSTER, start)
	case kwJAVA:
		p.advance()
		return p.parseAdminDDLStmt("ALTER", nodes.OBJECT_JAVA, start)
	case kwLIBRARY:
		p.advance()
		return p.parseAdminDDLStmt("ALTER", nodes.OBJECT_LIBRARY, start)
	default:
		if p.isIdentLike() {
			switch p.cur.Str {
			case "DIMENSION":
				p.advance()
				return p.parseAdminDDLStmt("ALTER", nodes.OBJECT_DIMENSION, start)
			case "DISKGROUP":
				p.advance()
				return p.parseAlterDiskgroupStmt(start)
			case "PLUGGABLE":
				p.advance() // consume PLUGGABLE
				if p.cur.Type == kwDATABASE {
					p.advance() // consume DATABASE
				}
				return p.parseAlterPluggableDatabaseStmt(start)
			case "ANALYTIC":
				p.advance() // consume ANALYTIC
				if p.cur.Type == kwVIEW {
					p.advance() // consume VIEW
				}
				return p.parseAlterAnalyticViewStmt(start)
			case "ATTRIBUTE":
				p.advance() // consume ATTRIBUTE
				if p.isIdentLike() && p.cur.Str == "DIMENSION" {
					p.advance() // consume DIMENSION
				}
				return p.parseAdminDDLStmt("ALTER", nodes.OBJECT_ATTRIBUTE_DIMENSION, start)
			case "HIERARCHY":
				p.advance()
				return p.parseAdminDDLStmt("ALTER", nodes.OBJECT_HIERARCHY, start)
			case "DOMAIN":
				p.advance()
				return p.parseAdminDDLStmt("ALTER", nodes.OBJECT_DOMAIN, start)
			case "INDEXTYPE":
				p.advance()
				return p.parseAlterIndextypeStmt(start)
			case "OPERATOR":
				p.advance()
				return p.parseAlterOperatorStmt(start)
			case "LOCKDOWN":
				p.advance() // consume LOCKDOWN
				if p.cur.Type == kwPROFILE {
					p.advance() // consume PROFILE
				}
				return p.parseAdminDDLStmt("ALTER", nodes.OBJECT_LOCKDOWN_PROFILE, start)
			case "OUTLINE":
				p.advance()
				return p.parseAdminDDLStmt("ALTER", nodes.OBJECT_OUTLINE, start)
			case "INMEMORY":
				p.advance() // consume INMEMORY
				if p.cur.Type == kwJOIN {
					p.advance() // consume JOIN
				}
				if p.cur.Type == kwGROUP || (p.isIdentLike() && p.cur.Str == "GROUP") {
					p.advance() // consume GROUP
				}
				return p.parseAdminDDLStmt("ALTER", nodes.OBJECT_INMEMORY_JOIN_GROUP, start)
			case "FLASHBACK":
				p.advance() // consume FLASHBACK
				if p.isIdentLike() && p.cur.Str == "ARCHIVE" {
					p.advance() // consume ARCHIVE
				}
				return p.parseAdminDDLStmt("ALTER", nodes.OBJECT_FLASHBACK_ARCHIVE, start)
			case "RESOURCE":
				p.advance() // consume RESOURCE
				if p.isIdentLike() && p.cur.Str == "COST" {
					p.advance() // consume COST
				}
				return p.parseAlterResourceCostStmt(start)
			case "ROLLBACK":
				p.advance() // consume ROLLBACK
				if p.isIdentLike() && p.cur.Str == "SEGMENT" {
					p.advance() // consume SEGMENT
				}
				return p.parseAdminDDLStmt("ALTER", nodes.OBJECT_ROLLBACK_SEGMENT, start)
			case "MATERIALIZED":
				p.advance() // consume MATERIALIZED (already a keyword, handled above for MVIEW)
				if p.isIdentLike() && p.cur.Str == "ZONEMAP" {
					p.advance() // consume ZONEMAP
					return p.parseAdminDDLStmt("ALTER", nodes.OBJECT_MATERIALIZED_ZONEMAP, start)
				}
				return nil
			}
		}
		return nil
	}
}
