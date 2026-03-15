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
		return p.parseCreateMaterializedZonemapStmt(start)
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
func (p *Parser) parseCreateAdminObject(start int, orReplace bool) nodes.StmtNode {
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
		return p.parseCreateDirectoryStmt(start, orReplace)
	case kwCONTEXT:
		p.advance()
		return p.parseCreateContextStmt(start, orReplace)
	case kwCLUSTER:
		p.advance()
		return p.parseCreateClusterStmt(start)
	case kwJAVA:
		p.advance()
		return p.parseCreateJavaStmt(start, orReplace)
	case kwLIBRARY:
		p.advance()
		return p.parseCreateLibraryStmt(start, orReplace)
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
				return p.parseCreateFlashbackArchiveStmt(start)
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
			return p.parseCreateAttributeDimensionStmt(start, false, false, false)
		case "HIERARCHY":
			p.advance()
			return p.parseCreateHierarchyStmt(start, false, false, false)
		case "DOMAIN":
			p.advance()
			return p.parseCreateDomainStmt(start, false, false)
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
			return p.parseCreateLockdownProfileStmt(start)
		case "OUTLINE":
			p.advance()
			return p.parseCreateOutlineStmt(start, false, false)
		case "INMEMORY":
			p.advance() // consume INMEMORY
			if p.cur.Type == kwJOIN {
				p.advance() // consume JOIN
			}
			if p.cur.Type == kwGROUP || (p.isIdentLike() && p.cur.Str == "GROUP") {
				p.advance() // consume GROUP
			}
			return p.parseCreateInmemoryJoinGroupStmt(start)
		case "ROLLBACK":
			p.advance() // consume ROLLBACK
			if p.isIdentLike() && p.cur.Str == "SEGMENT" {
				p.advance() // consume SEGMENT
			}
			return p.parseCreateRollbackSegmentStmt(start, false)
		case "EDITION":
			p.advance()
			return p.parseCreateEditionStmt(start)
		case "MLE":
			p.advance() // consume MLE
			if p.isIdentLike() && p.cur.Str == "ENV" {
				p.advance() // consume ENV
				return p.parseCreateMLEEnvStmt(start, orReplace)
			}
			if p.isIdentLike() && p.cur.Str == "MODULE" {
				p.advance() // consume MODULE
				return p.parseCreateMLEModuleStmt(start, orReplace)
			}
			return p.parseCreateMLEEnvStmt(start, orReplace)
		case "PFILE":
			p.advance()
			return p.parseCreatePfileStmt(start)
		case "SPFILE":
			p.advance()
			return p.parseCreateSpfileStmt(start)
		case "PROPERTY":
			p.advance() // consume PROPERTY
			if p.isIdentLike() && p.cur.Str == "GRAPH" {
				p.advance() // consume GRAPH
			}
			return p.parseCreatePropertyGraphStmt(start, false, false)
		case "VECTOR":
			p.advance() // consume VECTOR
			if p.cur.Type == kwINDEX {
				p.advance() // consume INDEX
			}
			return p.parseCreateVectorIndexStmt(start, false)
		case "RESTORE":
			p.advance() // consume RESTORE
			if p.isIdentLike() && p.cur.Str == "POINT" {
				p.advance() // consume POINT
			}
			return p.parseCreateRestorePointStmt(start, false)
		case "CLEAN":
			p.advance() // consume CLEAN
			if p.isIdentLike() && p.cur.Str == "RESTORE" {
				p.advance() // consume RESTORE
			}
			if p.isIdentLike() && p.cur.Str == "POINT" {
				p.advance() // consume POINT
			}
			return p.parseCreateRestorePointStmt(start, true)
		case "LOGICAL":
			p.advance() // consume LOGICAL
			if p.cur.Type == kwPARTITION || (p.isIdentLike() && p.cur.Str == "PARTITION") {
				p.advance() // consume PARTITION
			}
			if p.isIdentLike() && p.cur.Str == "TRACKING" {
				p.advance() // consume TRACKING
			}
			return p.parseCreateLogicalPartitionTrackingStmt(start)
		case "PMEM":
			p.advance() // consume PMEM
			if p.isIdentLike() && p.cur.Str == "FILESTORE" {
				p.advance() // consume FILESTORE
			}
			return p.parseCreatePmemFilestoreStmt(start)
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
		return p.parseDropSimpleStmt(nodes.OBJECT_DIRECTORY, start)
	case kwCONTEXT:
		p.advance()
		return p.parseDropSimpleStmt(nodes.OBJECT_CONTEXT, start)
	case kwCLUSTER:
		p.advance()
		return p.parseDropClusterStmt(start)
	case kwJAVA:
		p.advance()
		return p.parseDropJavaStmt(start)
	case kwLIBRARY:
		p.advance()
		return p.parseDropSimpleStmt(nodes.OBJECT_LIBRARY, start)
	default:
		if p.isIdentLike() {
			switch p.cur.Str {
			case "DIMENSION":
				p.advance()
				return p.parseDropSimpleStmt(nodes.OBJECT_DIMENSION, start)
			case "FLASHBACK":
				p.advance()
				if p.isIdentLike() && p.cur.Str == "ARCHIVE" {
					p.advance()
				}
				return p.parseDropSimpleStmt(nodes.OBJECT_FLASHBACK_ARCHIVE, start)
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
				return p.parseDropSimpleStmt(nodes.OBJECT_ANALYTIC_VIEW, start)
			case "ATTRIBUTE":
				p.advance() // consume ATTRIBUTE
				if p.isIdentLike() && p.cur.Str == "DIMENSION" {
					p.advance() // consume DIMENSION
				}
				return p.parseDropSimpleStmt(nodes.OBJECT_ATTRIBUTE_DIMENSION, start)
			case "HIERARCHY":
				p.advance()
				return p.parseDropSimpleStmt(nodes.OBJECT_HIERARCHY, start)
			case "DOMAIN":
				p.advance()
				return p.parseDropSimpleStmt(nodes.OBJECT_DOMAIN, start)
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
				return p.parseDropSimpleStmt(nodes.OBJECT_LOCKDOWN_PROFILE, start)
			case "OUTLINE":
				p.advance()
				return p.parseDropSimpleStmt(nodes.OBJECT_OUTLINE, start)
			case "INMEMORY":
				p.advance() // consume INMEMORY
				if p.cur.Type == kwJOIN {
					p.advance() // consume JOIN
				}
				if p.cur.Type == kwGROUP || (p.isIdentLike() && p.cur.Str == "GROUP") {
					p.advance() // consume GROUP
				}
				return p.parseDropSimpleStmt(nodes.OBJECT_INMEMORY_JOIN_GROUP, start)
			case "ROLLBACK":
				p.advance() // consume ROLLBACK
				if p.isIdentLike() && p.cur.Str == "SEGMENT" {
					p.advance() // consume SEGMENT
				}
				return p.parseDropSimpleStmt(nodes.OBJECT_ROLLBACK_SEGMENT, start)
			case "EDITION":
				p.advance()
				return p.parseDropEditionStmt(start)
			case "MLE":
				p.advance() // consume MLE
				if p.isIdentLike() && p.cur.Str == "ENV" {
					p.advance() // consume ENV
					return p.parseDropSimpleStmt(nodes.OBJECT_MLE_ENV, start)
				}
				if p.isIdentLike() && p.cur.Str == "MODULE" {
					p.advance() // consume MODULE
					return p.parseDropSimpleStmt(nodes.OBJECT_MLE_MODULE, start)
				}
				return p.parseDropSimpleStmt(nodes.OBJECT_MLE_ENV, start)
			case "PROPERTY":
				p.advance() // consume PROPERTY
				if p.isIdentLike() && p.cur.Str == "GRAPH" {
					p.advance() // consume GRAPH
				}
				return p.parseDropSimpleStmt(nodes.OBJECT_PROPERTY_GRAPH, start)
			case "VECTOR":
				p.advance() // consume VECTOR
				if p.cur.Type == kwINDEX {
					p.advance() // consume INDEX
				}
				return p.parseDropSimpleStmt(nodes.OBJECT_VECTOR_INDEX, start)
			case "RESTORE":
				p.advance() // consume RESTORE
				if p.isIdentLike() && p.cur.Str == "POINT" {
					p.advance() // consume POINT
				}
				return p.parseDropSimpleStmt(nodes.OBJECT_RESTORE_POINT, start)
			case "LOGICAL":
				p.advance() // consume LOGICAL
				if p.cur.Type == kwPARTITION || (p.isIdentLike() && p.cur.Str == "PARTITION") {
					p.advance() // consume PARTITION
				}
				if p.isIdentLike() && p.cur.Str == "TRACKING" {
					p.advance() // consume TRACKING
				}
				return p.parseDropSimpleStmt(nodes.OBJECT_LOGICAL_PARTITION_TRACKING, start)
			case "PMEM":
				p.advance() // consume PMEM
				if p.isIdentLike() && p.cur.Str == "FILESTORE" {
					p.advance() // consume FILESTORE
				}
				return p.parseDropSimpleStmt(nodes.OBJECT_PMEM_FILESTORE, start)
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
// parseCreateClusterStmt parses a CREATE CLUSTER statement.
//
// BNF: oracle/parser/bnf/CREATE-CLUSTER.bnf
//
//	CREATE CLUSTER [ IF NOT EXISTS ] [ schema. ] cluster
//	    [ SHARING = { METADATA | NONE } ]
//	    ( column datatype [ COLLATE column_collation_name ] [ SORT ]
//	      [, column datatype [ COLLATE column_collation_name ] [ SORT ] ]... )
//	    [ physical_attributes_clause ]
//	    [ SIZE size_clause ]
//	    [ TABLESPACE tablespace ]
//	    [ { INDEX
//	      | HASHKEYS integer [ HASH IS expr ] }
//	    ]
//	    [ SINGLE TABLE ]
//	    [ parallel_clause ]
//	    [ { NOROWDEPENDENCIES | ROWDEPENDENCIES } ]
//	    [ { CACHE | NOCACHE } ]
//	    [ cluster_range_partitions ] ;
//
//	physical_attributes_clause:
//	    [ PCTFREE integer ]
//	    [ PCTUSED integer ]
//	    [ INITRANS integer ]
//	    [ storage_clause ]
//
//	parallel_clause:
//	    { NOPARALLEL | PARALLEL [ integer ] }
//
//	cluster_range_partitions:
//	    PARTITION BY RANGE ( column [, column ]... )
//	    ( PARTITION [ partition ]
//	        VALUES LESS THAN ( { value | MAXVALUE } [, { value | MAXVALUE } ]... )
//	        [ table_partition_description ]
//	      [, PARTITION [ partition ]
//	          VALUES LESS THAN ( { value | MAXVALUE } [, { value | MAXVALUE } ]... )
//	          [ table_partition_description ] ]...
//	    )
func (p *Parser) parseCreateClusterStmt(start int) *nodes.CreateClusterStmt {
	stmt := &nodes.CreateClusterStmt{
		Loc: nodes.Loc{Start: start},
	}

	// Optional IF NOT EXISTS
	if p.cur.Type == kwIF {
		next := p.peekNext()
		if next.Type == kwNOT {
			p.advance() // IF
			p.advance() // NOT
			if p.cur.Type == kwEXISTS {
				p.advance() // EXISTS
			}
		}
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
// BNF: oracle/parser/bnf/CREATE-DIMENSION.bnf
//
//	CREATE DIMENSION [ schema. ] dimension
//	    level_clause [ level_clause ]...
//	    { hierarchy_clause
//	    | dimension_join_clause
//	    | attribute_clause
//	    | extended_attribute_clause
//	    }... ;
//
//	level_clause:
//	    LEVEL level IS ( [ schema. ] table. column [, [ schema. ] table. column ]... )
//	    [ SKIP WHEN NULL ]
//
//	hierarchy_clause:
//	    HIERARCHY hierarchy (
//	        child_level CHILD OF parent_level
//	        [ CHILD OF parent_level ]...
//	        [ dimension_join_clause ]...
//	    )
//
//	dimension_join_clause:
//	    JOIN KEY ( [ [ schema. ] table. ] child_key_column
//	        [, [ [ schema. ] table. ] child_key_column ]... )
//	    REFERENCES parent_level
//
//	attribute_clause:
//	    ATTRIBUTE level DETERMINES ( [ schema. ] table. column
//	        [, [ schema. ] table. column ]... )
//
//	extended_attribute_clause:
//	    ATTRIBUTE attribute LEVEL level DETERMINES ( [ schema. ] table. column
//	        [, [ schema. ] table. column ]... )
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

// ---------------------------------------------------------------------------
// ALTER CLUSTER
// ---------------------------------------------------------------------------

// parseAlterClusterStmt parses an ALTER CLUSTER statement.
//
// BNF: oracle/parser/bnf/ALTER-CLUSTER.bnf
//
//	ALTER CLUSTER [ IF EXISTS ] [ schema . ] cluster
//	    { physical_attributes_clause
//	    | SIZE integer
//	    | MODIFY PARTITION partition allocate_extent_clause
//	    | allocate_extent_clause
//	    | deallocate_unused_clause
//	    | parallel_clause
//	    }
//
//	physical_attributes_clause:
//	    [ PCTUSED integer ]
//	    [ PCTFREE integer ]
//	    [ INITRANS integer ]
//	    [ STORAGE storage_clause ]
//
//	allocate_extent_clause:
//	    ALLOCATE EXTENT
//	    [ ( { SIZE size_clause
//	        | DATAFILE 'filename'
//	        | INSTANCE integer
//	        }...
//	      )
//	    ]
//
//	deallocate_unused_clause:
//	    DEALLOCATE UNUSED [ KEEP size_clause ]
//
//	parallel_clause:
//	    { PARALLEL [ integer ] | NOPARALLEL }
func (p *Parser) parseAlterClusterStmt(start int) *nodes.AlterClusterStmt {
	stmt := &nodes.AlterClusterStmt{
		Loc: nodes.Loc{Start: start},
	}

	// Optional IF EXISTS
	if p.cur.Type == kwIF {
		next := p.peekNext()
		if next.Type == kwEXISTS {
			p.advance() // IF
			p.advance() // EXISTS
			stmt.IfExists = true
		}
	}

	stmt.Name = p.parseObjectName()

	// Parse the action
	for p.cur.Type != ';' && p.cur.Type != tokEOF {
		switch {
		case p.cur.Type == kwSIZE:
			p.advance()
			stmt.Action = "SIZE"
			stmt.Size = p.parseSizeValue()

		case p.isIdentLike() && p.cur.Str == "PCTUSED":
			p.advance()
			if p.cur.Type == tokICONST {
				v := p.parseIntValue()
				stmt.PctUsed = &v
			}
			if stmt.Action == "" {
				stmt.Action = "PHYSICAL_ATTRS"
			}

		case p.cur.Type == kwPCTFREE:
			p.advance()
			if p.cur.Type == tokICONST {
				v := p.parseIntValue()
				stmt.PctFree = &v
			}
			if stmt.Action == "" {
				stmt.Action = "PHYSICAL_ATTRS"
			}

		case p.isIdentLike() && p.cur.Str == "INITRANS":
			p.advance()
			if p.cur.Type == tokICONST {
				v := p.parseIntValue()
				stmt.InitTrans = &v
			}
			if stmt.Action == "" {
				stmt.Action = "PHYSICAL_ATTRS"
			}

		case p.cur.Type == kwSTORAGE:
			p.advance()
			if p.cur.Type == '(' {
				p.skipParens()
			}
			if stmt.Action == "" {
				stmt.Action = "PHYSICAL_ATTRS"
			}

		case p.isIdentLike() && p.cur.Str == "MODIFY":
			p.advance() // MODIFY
			stmt.Action = "MODIFY_PARTITION"
			if p.cur.Type == kwPARTITION || (p.isIdentLike() && p.cur.Str == "PARTITION") {
				p.advance() // PARTITION
			}
			stmt.ModifyPartition = p.parseIdentifier()
			// allocate_extent_clause follows
			if p.isIdentLike() && p.cur.Str == "ALLOCATE" {
				p.advance() // ALLOCATE
				if p.isIdentLike() && p.cur.Str == "EXTENT" {
					p.advance() // EXTENT
				}
				if p.cur.Type == '(' {
					p.skipParens()
				}
			}

		case p.isIdentLike() && p.cur.Str == "ALLOCATE":
			p.advance() // ALLOCATE
			stmt.Action = "ALLOCATE_EXTENT"
			if p.isIdentLike() && p.cur.Str == "EXTENT" {
				p.advance() // EXTENT
			}
			if p.cur.Type == '(' {
				p.skipParens()
			}

		case p.isIdentLike() && p.cur.Str == "DEALLOCATE":
			p.advance() // DEALLOCATE
			stmt.Action = "DEALLOCATE_UNUSED"
			if p.isIdentLike() && p.cur.Str == "UNUSED" {
				p.advance() // UNUSED
			}
			if p.isIdentLike() && p.cur.Str == "KEEP" {
				p.advance() // KEEP
				p.parseSizeValue()
			}

		case p.isIdentLike() && p.cur.Str == "PARALLEL":
			p.advance()
			stmt.Action = "PARALLEL"
			if p.cur.Type == tokICONST {
				stmt.Parallel = p.cur.Str
				p.advance()
			} else {
				stmt.Parallel = "PARALLEL"
			}

		case p.isIdentLike() && p.cur.Str == "NOPARALLEL":
			p.advance()
			stmt.Action = "PARALLEL"
			stmt.Parallel = "NOPARALLEL"

		default:
			p.advance()
		}
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// ---------------------------------------------------------------------------
// ALTER DIMENSION
// ---------------------------------------------------------------------------

// parseAlterDimensionStmt parses an ALTER DIMENSION statement.
//
// BNF: oracle/parser/bnf/ALTER-DIMENSION.bnf
//
//	ALTER DIMENSION [ schema . ] dimension
//	    { ADD level_clause
//	    | ADD hierarchy_clause
//	    | ADD attribute_clause
//	    | ADD extended_attribute_clause
//	    | DROP level_clause
//	    | DROP hierarchy_clause
//	    | DROP attribute_clause
//	    | DROP extended_attribute_clause
//	    | COMPILE
//	    }
//
//	level_clause:
//	    LEVEL level IS ( table . column [, table . column ]... )
//	    [ SKIP WHEN expression ]
//
//	hierarchy_clause:
//	    HIERARCHY hierarchy_name ( child_level CHILD OF parent_level
//	      [ JOIN KEY child_key_column REFERENCES parent_level ]
//	      [, child_level CHILD OF parent_level
//	        [ JOIN KEY child_key_column REFERENCES parent_level ] ]...
//	    )
//
//	attribute_clause:
//	    ATTRIBUTE level DETERMINES ( dependent_column [, dependent_column ]... )
//
//	extended_attribute_clause:
//	    ATTRIBUTE attribute_name OF level_name
//	      DETERMINES ( dependent_column [, dependent_column ]... )
func (p *Parser) parseAlterDimensionStmt(start int) *nodes.AlterDimensionStmt {
	stmt := &nodes.AlterDimensionStmt{
		Loc: nodes.Loc{Start: start},
	}

	stmt.Name = p.parseObjectName()

	// Parse actions
	for p.cur.Type != ';' && p.cur.Type != tokEOF {
		switch {
		case p.isIdentLike() && p.cur.Str == "COMPILE":
			p.advance()
			stmt.Compile = true

		case p.cur.Type == kwADD:
			p.advance() // ADD
			switch {
			case p.cur.Type == kwLEVEL:
				stmt.AddLevels = append(stmt.AddLevels, p.parseDimensionLevel())
			case p.isIdentLike() && p.cur.Str == "HIERARCHY":
				stmt.AddHierarchies = append(stmt.AddHierarchies, p.parseDimensionHierarchy())
			case p.isIdentLike() && p.cur.Str == "ATTRIBUTE":
				stmt.AddAttributes = append(stmt.AddAttributes, p.parseDimensionAttribute())
			default:
				p.advance()
			}

		case p.cur.Type == kwDROP:
			p.advance() // DROP
			switch {
			case p.cur.Type == kwLEVEL:
				p.advance() // LEVEL
				stmt.DropLevels = append(stmt.DropLevels, p.parseIdentifier())
			case p.isIdentLike() && p.cur.Str == "HIERARCHY":
				p.advance() // HIERARCHY
				stmt.DropHierarchies = append(stmt.DropHierarchies, p.parseIdentifier())
			case p.isIdentLike() && p.cur.Str == "ATTRIBUTE":
				p.advance() // ATTRIBUTE
				stmt.DropAttributes = append(stmt.DropAttributes, p.parseIdentifier())
			default:
				p.advance()
			}

		default:
			p.advance()
		}
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// ---------------------------------------------------------------------------
// CREATE MATERIALIZED ZONEMAP
// ---------------------------------------------------------------------------

// parseCreateMaterializedZonemapStmt parses a CREATE MATERIALIZED ZONEMAP statement.
//
// BNF: oracle/parser/bnf/CREATE-MATERIALIZED-ZONEMAP.bnf
//
//	CREATE MATERIALIZED ZONEMAP [ IF NOT EXISTS ]
//	    [ schema. ] zonemap_name
//	    [ zonemap_attributes ]
//	    [ zonemap_refresh_clause ]
//	    [ { ENABLE | DISABLE } PRUNING ]
//	    { create_zonemap_on_table | create_zonemap_as_subquery }
//
//	create_zonemap_on_table:
//	    ON [ schema. ] table ( column [, column ]... )
//
//	create_zonemap_as_subquery:
//	    [ ( column_alias [, column_alias ]... ) ]
//	    AS query_block
//
//	zonemap_attributes:
//	    [ TABLESPACE tablespace_name ]
//	    [ SCALE integer ]
//	    [ PCTFREE integer ]
//	    [ PCTUSED integer ]
//	    [ { CACHE | NOCACHE } ]
//
//	zonemap_refresh_clause:
//	    REFRESH
//	    [ { FAST | COMPLETE | FORCE } ]
//	    [ { ON DEMAND
//	      | ON COMMIT
//	      | ON LOAD
//	      | ON DATA MOVEMENT
//	      | ON LOAD DATA MOVEMENT
//	      }
//	    ]
func (p *Parser) parseCreateMaterializedZonemapStmt(start int) *nodes.CreateMaterializedZonemapStmt {
	stmt := &nodes.CreateMaterializedZonemapStmt{
		Loc: nodes.Loc{Start: start},
	}

	// Optional IF NOT EXISTS
	if p.cur.Type == kwIF {
		next := p.peekNext()
		if next.Type == kwNOT {
			p.advance() // IF
			p.advance() // NOT
			if p.cur.Type == kwEXISTS {
				p.advance() // EXISTS
			}
			stmt.IfNotExists = true
		}
	}

	stmt.Name = p.parseObjectName()

	// Parse options until ON, AS, (, or ; / EOF
	for p.cur.Type != ';' && p.cur.Type != tokEOF {
		switch {
		// zonemap_attributes
		case p.cur.Type == kwTABLESPACE:
			p.advance()
			stmt.Tablespace = p.parseIdentifier()

		case p.isIdentLike() && p.cur.Str == "SCALE":
			p.advance()
			if p.cur.Type == tokICONST {
				v := p.parseIntValue()
				stmt.Scale = &v
			}

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

		case p.cur.Type == kwCACHE:
			p.advance()
			stmt.Cache = true

		case p.cur.Type == kwNOCACHE:
			p.advance()
			stmt.NoCache = true

		// zonemap_refresh_clause
		case p.isIdentLike() && p.cur.Str == "REFRESH":
			p.advance() // REFRESH
			p.parseZonemapRefresh(stmt, nil)

		// ENABLE/DISABLE PRUNING
		case p.cur.Type == kwENABLE:
			p.advance()
			if p.isIdentLike() && p.cur.Str == "PRUNING" {
				p.advance()
				stmt.EnablePruning = true
			}

		case p.cur.Type == kwDISABLE:
			p.advance()
			if p.isIdentLike() && p.cur.Str == "PRUNING" {
				p.advance()
				stmt.DisablePruning = true
			}

		// create_zonemap_on_table
		case p.cur.Type == kwON:
			p.advance() // ON
			stmt.OnTable = p.parseObjectName()
			if p.cur.Type == '(' {
				p.advance()
				for p.cur.Type != ')' && p.cur.Type != tokEOF {
					stmt.OnColumns = append(stmt.OnColumns, p.parseIdentifier())
					if p.cur.Type == ',' {
						p.advance()
					}
				}
				if p.cur.Type == ')' {
					p.advance()
				}
			}

		// create_zonemap_as_subquery: ( column_alias, ... ) AS query_block
		case p.cur.Type == '(' && stmt.OnTable == nil && stmt.AsQuery == nil:
			// column aliases before AS
			p.advance()
			for p.cur.Type != ')' && p.cur.Type != tokEOF {
				stmt.ColumnAliases = append(stmt.ColumnAliases, p.parseIdentifier())
				if p.cur.Type == ',' {
					p.advance()
				}
			}
			if p.cur.Type == ')' {
				p.advance()
			}

		case p.cur.Type == kwAS:
			p.advance() // AS
			stmt.AsQuery = p.parseSelectStmt()

		default:
			p.advance()
		}
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseZonemapRefresh parses the refresh clause for a zonemap.
// It sets refresh fields on either a CreateMaterializedZonemapStmt or AlterMaterializedZonemapStmt.
func (p *Parser) parseZonemapRefresh(create *nodes.CreateMaterializedZonemapStmt, alter *nodes.AlterMaterializedZonemapStmt) {
	// Optional method: FAST | COMPLETE | FORCE
	method := ""
	if p.isIdentLike() {
		switch p.cur.Str {
		case "FAST", "COMPLETE", "FORCE":
			method = p.cur.Str
			p.advance()
		}
	}

	// Optional ON ...
	refreshOn := ""
	if p.cur.Type == kwON {
		p.advance() // ON
		if p.isIdentLike() {
			switch p.cur.Str {
			case "DEMAND":
				refreshOn = "ON DEMAND"
				p.advance()
			case "COMMIT":
				refreshOn = "ON COMMIT"
				p.advance()
			case "LOAD":
				p.advance()
				if p.isIdentLike() && p.cur.Str == "DATA" {
					p.advance() // DATA
					if p.isIdentLike() && p.cur.Str == "MOVEMENT" {
						p.advance() // MOVEMENT
					}
					refreshOn = "ON LOAD DATA MOVEMENT"
				} else {
					refreshOn = "ON LOAD"
				}
			case "DATA":
				p.advance() // DATA
				if p.isIdentLike() && p.cur.Str == "MOVEMENT" {
					p.advance() // MOVEMENT
				}
				refreshOn = "ON DATA MOVEMENT"
			case "STATEMENT":
				refreshOn = "ON STATEMENT"
				p.advance()
			}
		}
	}

	if create != nil {
		create.RefreshMethod = method
		create.RefreshOn = refreshOn
	}
	if alter != nil {
		alter.RefreshMethod = method
		alter.RefreshOn = refreshOn
	}
}

// ---------------------------------------------------------------------------
// ALTER MATERIALIZED ZONEMAP
// ---------------------------------------------------------------------------

// parseAlterMaterializedZonemapStmt parses an ALTER MATERIALIZED ZONEMAP statement.
//
// BNF: oracle/parser/bnf/ALTER-MATERIALIZED-ZONEMAP.bnf
//
//	ALTER MATERIALIZED ZONEMAP [ IF EXISTS ] [ schema. ] zonemap_name
//	    { alter_zonemap_attributes
//	    | zonemap_refresh_clause
//	    | { ENABLE | DISABLE } PRUNING
//	    | COMPILE
//	    | REBUILD
//	    | UNUSABLE
//	    } ;
//
//	alter_zonemap_attributes:
//	    [ PCTFREE integer ]
//	    [ PCTUSED integer ]
//	    [ { CACHE | NOCACHE } ]
//
//	zonemap_refresh_clause:
//	    REFRESH
//	    [ { FAST | COMPLETE | FORCE } ]
//	    [ { ON COMMIT | ON DEMAND | ON LOAD | ON DATA MOVEMENT | ON STATEMENT } ]
func (p *Parser) parseAlterMaterializedZonemapStmt(start int) *nodes.AlterMaterializedZonemapStmt {
	stmt := &nodes.AlterMaterializedZonemapStmt{
		Loc: nodes.Loc{Start: start},
	}

	// Optional IF EXISTS
	if p.cur.Type == kwIF {
		next := p.peekNext()
		if next.Type == kwEXISTS {
			p.advance() // IF
			p.advance() // EXISTS
			stmt.IfExists = true
		}
	}

	stmt.Name = p.parseObjectName()

	// Parse action
	for p.cur.Type != ';' && p.cur.Type != tokEOF {
		switch {
		case p.cur.Type == kwPCTFREE:
			p.advance()
			if p.cur.Type == tokICONST {
				v := p.parseIntValue()
				stmt.PctFree = &v
			}
			if stmt.Action == "" {
				stmt.Action = "ATTRS"
			}

		case p.isIdentLike() && p.cur.Str == "PCTUSED":
			p.advance()
			if p.cur.Type == tokICONST {
				v := p.parseIntValue()
				stmt.PctUsed = &v
			}
			if stmt.Action == "" {
				stmt.Action = "ATTRS"
			}

		case p.cur.Type == kwCACHE:
			p.advance()
			stmt.Cache = true
			if stmt.Action == "" {
				stmt.Action = "ATTRS"
			}

		case p.cur.Type == kwNOCACHE:
			p.advance()
			stmt.NoCache = true
			if stmt.Action == "" {
				stmt.Action = "ATTRS"
			}

		case p.isIdentLike() && p.cur.Str == "REFRESH":
			p.advance()
			stmt.Action = "REFRESH"
			p.parseZonemapRefresh(nil, stmt)

		case p.cur.Type == kwENABLE:
			p.advance()
			if p.isIdentLike() && p.cur.Str == "PRUNING" {
				p.advance()
			}
			stmt.Action = "ENABLE_PRUNING"

		case p.cur.Type == kwDISABLE:
			p.advance()
			if p.isIdentLike() && p.cur.Str == "PRUNING" {
				p.advance()
			}
			stmt.Action = "DISABLE_PRUNING"

		case p.isIdentLike() && p.cur.Str == "COMPILE":
			p.advance()
			stmt.Action = "COMPILE"

		case p.isIdentLike() && p.cur.Str == "REBUILD":
			p.advance()
			stmt.Action = "REBUILD"

		case p.isIdentLike() && p.cur.Str == "UNUSABLE":
			p.advance()
			stmt.Action = "UNUSABLE"

		default:
			p.advance()
		}
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// ---------------------------------------------------------------------------
// CREATE INMEMORY JOIN GROUP
// ---------------------------------------------------------------------------

// parseCreateInmemoryJoinGroupStmt parses a CREATE INMEMORY JOIN GROUP statement.
//
// BNF: oracle/parser/bnf/CREATE-INMEMORY-JOIN-GROUP.bnf
//
//	CREATE INMEMORY JOIN GROUP [ IF NOT EXISTS ] [ schema. ] join_group
//	    ( [ schema. ] table ( column )
//	      [, [ schema. ] table ( column ) ]... ) ;
func (p *Parser) parseCreateInmemoryJoinGroupStmt(start int) *nodes.CreateInmemoryJoinGroupStmt {
	stmt := &nodes.CreateInmemoryJoinGroupStmt{
		Loc: nodes.Loc{Start: start},
	}

	// Optional IF NOT EXISTS
	if p.cur.Type == kwIF {
		next := p.peekNext()
		if next.Type == kwNOT {
			p.advance() // IF
			p.advance() // NOT
			if p.cur.Type == kwEXISTS {
				p.advance() // EXISTS
			}
			stmt.IfNotExists = true
		}
	}

	stmt.Name = p.parseObjectName()

	// Parse member list: ( table(col) [, table(col) ]... )
	if p.cur.Type == '(' {
		p.advance()
		for p.cur.Type != ')' && p.cur.Type != tokEOF {
			member := p.parseJoinGroupMember()
			stmt.Members = append(stmt.Members, member)
			if p.cur.Type == ',' {
				p.advance()
			}
		}
		if p.cur.Type == ')' {
			p.advance()
		}
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseJoinGroupMember parses a table(column) member of a join group.
func (p *Parser) parseJoinGroupMember() *nodes.JoinGroupMember {
	m := &nodes.JoinGroupMember{
		Loc: nodes.Loc{Start: p.pos()},
	}
	m.Table = p.parseObjectName()
	if p.cur.Type == '(' {
		p.advance()
		m.Column = p.parseIdentifier()
		if p.cur.Type == ')' {
			p.advance()
		}
	}
	m.Loc.End = p.pos()
	return m
}

// ---------------------------------------------------------------------------
// ALTER INMEMORY JOIN GROUP
// ---------------------------------------------------------------------------

// parseAlterInmemoryJoinGroupStmt parses an ALTER INMEMORY JOIN GROUP statement.
//
// BNF: oracle/parser/bnf/ALTER-INMEMORY-JOIN-GROUP.bnf
//
//	ALTER INMEMORY JOIN GROUP [ IF EXISTS ] [ schema. ] join_group
//	    { ADD | REMOVE } ( [ schema. ] table ( column ) ) ;
func (p *Parser) parseAlterInmemoryJoinGroupStmt(start int) *nodes.AlterInmemoryJoinGroupStmt {
	stmt := &nodes.AlterInmemoryJoinGroupStmt{
		Loc: nodes.Loc{Start: start},
	}

	// Optional IF EXISTS
	if p.cur.Type == kwIF {
		next := p.peekNext()
		if next.Type == kwEXISTS {
			p.advance() // IF
			p.advance() // EXISTS
			stmt.IfExists = true
		}
	}

	stmt.Name = p.parseObjectName()

	// ADD or REMOVE
	if p.cur.Type == kwADD {
		p.advance()
		stmt.Action = "ADD"
	} else if p.isIdentLike() && p.cur.Str == "REMOVE" {
		p.advance()
		stmt.Action = "REMOVE"
	}

	// ( table(column) )
	if p.cur.Type == '(' {
		p.advance()
		stmt.Member = p.parseJoinGroupMember()
		if p.cur.Type == ')' {
			p.advance()
		}
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// ---------------------------------------------------------------------------
// DROP CLUSTER (with INCLUDING TABLES / CASCADE CONSTRAINTS)
// ---------------------------------------------------------------------------

// parseDropClusterStmt parses a DROP CLUSTER statement.
//
// BNF: oracle/parser/bnf/DROP-CLUSTER.bnf
//
//	DROP CLUSTER [ IF EXISTS ] [ schema. ] cluster
//	    [ INCLUDING TABLES [ CASCADE CONSTRAINTS ] ]
func (p *Parser) parseDropClusterStmt(start int) *nodes.DropStmt {
	stmt := &nodes.DropStmt{
		ObjectType: nodes.OBJECT_CLUSTER,
		Names:      &nodes.List{},
		Loc:        nodes.Loc{Start: start},
	}

	// Optional IF EXISTS
	if p.cur.Type == kwIF {
		next := p.peekNext()
		if next.Type == kwEXISTS {
			p.advance() // IF
			p.advance() // EXISTS
			stmt.IfExists = true
		}
	}

	name := p.parseObjectName()
	stmt.Names.Items = append(stmt.Names.Items, name)

	// Optional INCLUDING TABLES
	if p.isIdentLike() && p.cur.Str == "INCLUDING" {
		p.advance() // INCLUDING
		if p.cur.Type == kwTABLE || (p.isIdentLike() && p.cur.Str == "TABLES") {
			p.advance() // TABLES
		}
		// Optional CASCADE CONSTRAINTS
		if p.cur.Type == kwCASCADE {
			p.advance() // CASCADE
			if p.isIdentLike() && p.cur.Str == "CONSTRAINTS" {
				p.advance() // CONSTRAINTS
			}
			stmt.Cascade = true
		}
	}

	stmt.Loc.End = p.pos()
	return stmt
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
		return p.parseAlterClusterStmt(start)
	case kwJAVA:
		p.advance()
		return p.parseAlterJavaStmt(start)
	case kwLIBRARY:
		p.advance()
		return p.parseAlterLibraryStmt(start)
	default:
		if p.isIdentLike() {
			switch p.cur.Str {
			case "DIMENSION":
				p.advance()
				return p.parseAlterDimensionStmt(start)
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
				return p.parseAlterAttributeDimensionStmt(start)
			case "HIERARCHY":
				p.advance()
				return p.parseAlterHierarchyStmt(start)
			case "DOMAIN":
				p.advance()
				return p.parseAlterDomainStmt(start, false)
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
				return p.parseAlterLockdownProfileStmt(start)
			case "OUTLINE":
				p.advance()
				return p.parseAlterOutlineStmt(start)
			case "INMEMORY":
				p.advance() // consume INMEMORY
				if p.cur.Type == kwJOIN {
					p.advance() // consume JOIN
				}
				if p.cur.Type == kwGROUP || (p.isIdentLike() && p.cur.Str == "GROUP") {
					p.advance() // consume GROUP
				}
				return p.parseAlterInmemoryJoinGroupStmt(start)
			case "FLASHBACK":
				p.advance() // consume FLASHBACK
				if p.isIdentLike() && p.cur.Str == "ARCHIVE" {
					p.advance() // consume ARCHIVE
				}
				return p.parseAlterFlashbackArchiveStmt(start)
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
				return p.parseAlterRollbackSegmentStmt(start)
			case "MATERIALIZED":
				p.advance() // consume MATERIALIZED (already a keyword, handled above for MVIEW)
				if p.isIdentLike() && p.cur.Str == "ZONEMAP" {
					p.advance() // consume ZONEMAP
					return p.parseAlterMaterializedZonemapStmt(start)
				}
				return nil
			}
		}
		return nil
	}
}

// ---------------------------------------------------------------------------
// CREATE ATTRIBUTE DIMENSION
// ---------------------------------------------------------------------------

// parseCreateAttributeDimensionStmt parses a CREATE ATTRIBUTE DIMENSION statement.
//
// BNF: oracle/parser/bnf/CREATE-ATTRIBUTE-DIMENSION.bnf
//
//  CREATE [ OR REPLACE ] [ { FORCE | NOFORCE } ] ATTRIBUTE DIMENSION
//      [ IF NOT EXISTS ] [ schema. ] attr_dimension
//      [ SHARING = { METADATA | NONE } ]
//      [ classification_clause ]
//      [ DIMENSION TYPE { STANDARD | TIME } ]
//      attr_dim_using_clause
//      attributes_clause
//      attr_dim_level_clause [ attr_dim_level_clause ]...
//      [ all_clause ] ;
//
//  classification_clause:
//      { CAPTION 'caption'
//      | DESCRIPTION 'description'
//      | CLASSIFICATION classification_name [ LANGUAGE language ] VALUE 'value'
//      } [ classification_clause ]
//
//  attr_dim_using_clause:
//      USING source_clause [, source_clause ]...
//
//  source_clause:
//      [ REMOTE ] [ schema. ] { table | view } [ [ AS ] alias ]
//      | join_path_clause
//
//  join_path_clause:
//      JOIN PATH join_path_name ON join_condition
//
//  join_condition:
//      join_condition_elem [ AND join_condition_elem ]...
//
//  join_condition_elem:
//      [ table_alias. ] column = [ table_alias. ] column
//
//  attributes_clause:
//      ATTRIBUTES ( attr_dim_attribute_clause [, attr_dim_attribute_clause ]... )
//
//  attr_dim_attribute_clause:
//      column [ AS alias ]
//      [ classification_clause ]
//
//  attr_dim_level_clause:
//      LEVEL level_name
//      [ LEVEL TYPE { STANDARD | YEARS | HALF_YEARS | QUARTERS | MONTHS | WEEKS | DAYS | HOURS | MINUTES | SECONDS } ]
//      [ classification_clause ]
//      key_clause
//      [ alternate_key_clause ]
//      [ MEMBER NAME expression ]
//      [ MEMBER CAPTION expression ]
//      [ MEMBER DESCRIPTION expression ]
//      [ dim_order_clause ]
//      [ DETERMINES ( attribute [, attribute ]... ) ]
//
//  key_clause:
//      KEY { attribute [ NOT NULL | SKIP WHEN NULL ]
//          | ( attribute [, attribute ]... ) }
//
//  alternate_key_clause:
//      ALTERNATE KEY { attribute
//                    | ( attribute [, attribute ]... ) }
//
//  dim_order_clause:
//      ORDER BY { attribute [ ASC | DESC ]
//               | ( attribute [ ASC | DESC ] [, attribute [ ASC | DESC ] ]... ) }
//
//  all_clause:
//      ALL [ MEMBER NAME expression ]
//          [ MEMBER CAPTION expression ]
//          [ MEMBER DESCRIPTION expression ]
func (p *Parser) parseCreateAttributeDimensionStmt(start int, orReplace, force, noForce bool) *nodes.CreateAttributeDimensionStmt {
	stmt := &nodes.CreateAttributeDimensionStmt{
		OrReplace: orReplace,
		Force:     force,
		NoForce:   noForce,
		Loc:       nodes.Loc{Start: start},
	}

	// IF NOT EXISTS
	if p.cur.Type == kwIF && p.peekNext().Type == kwNOT {
		p.advance()
		p.advance()
		if p.cur.Type == kwEXISTS {
			p.advance()
		}
		stmt.IfNotExists = true
	}

	// name
	stmt.Name = p.parseObjectName()

	// SHARING = { METADATA | NONE }
	if p.isIdentLikeStr("SHARING") {
		p.advance()
		if p.cur.Type == '=' {
			p.advance()
		}
		if p.isIdentLike() {
			stmt.Sharing = p.cur.Str
			p.advance()
		}
	}

	// classification_clause(s) at top level
	stmt.Classifications = p.parseClassificationClauses()

	// DIMENSION TYPE { STANDARD | TIME }
	if p.isIdentLike() && p.cur.Str == "DIMENSION" {
		next := p.peekNext()
		if next.Type == kwTYPE {
			p.advance() // consume DIMENSION
			p.advance() // consume TYPE
			if p.isIdentLike() {
				stmt.DimensionType = p.cur.Str
				p.advance()
			}
		}
	}

	// USING source_clause [, source_clause]...
	if p.cur.Type == kwUSING {
		p.advance()
		stmt.Sources = &nodes.List{}
		for {
			src := p.parseAttrDimSourceClause()
			if src != nil {
				stmt.Sources.Items = append(stmt.Sources.Items, src)
			}
			if p.cur.Type != ',' {
				break
			}
			p.advance() // consume comma
		}
	}

	// ATTRIBUTES ( ... )
	if p.isIdentLikeStr("ATTRIBUTES") {
		p.advance()
		stmt.Attributes = &nodes.List{}
		if p.cur.Type == '(' {
			p.advance()
			for p.cur.Type != ')' && p.cur.Type != tokEOF {
				attr := p.parseAttrDimAttributeClause()
				if attr != nil {
					stmt.Attributes.Items = append(stmt.Attributes.Items, attr)
				}
				if p.cur.Type == ',' {
					p.advance()
				}
			}
			if p.cur.Type == ')' {
				p.advance()
			}
		}
	}

	// LEVEL clauses (one or more)
	stmt.Levels = &nodes.List{}
	for p.cur.Type == kwLEVEL {
		lvl := p.parseAttrDimLevelClause()
		if lvl != nil {
			stmt.Levels.Items = append(stmt.Levels.Items, lvl)
		}
	}

	// ALL clause
	if p.cur.Type == kwALL {
		p.advance()
		allc := &nodes.AttrDimAllClause{
			Loc: nodes.Loc{Start: p.pos()},
		}
		allc.MemberName, allc.MemberCaption, allc.MemberDesc = p.parseMemberExprs()
		allc.Loc.End = p.pos()
		stmt.AllClause = allc
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseClassificationClauses parses zero or more classification_clause items.
func (p *Parser) parseClassificationClauses() *nodes.List {
	var items []nodes.Node
	for {
		if p.isIdentLikeStr("CAPTION") {
			p.advance()
			if p.cur.Type == tokSCONST {
				items = append(items, &nodes.DDLOption{Key: "CAPTION", Value: p.cur.Str})
				p.advance()
			}
		} else if p.isIdentLikeStr("DESCRIPTION") {
			p.advance()
			if p.cur.Type == tokSCONST {
				items = append(items, &nodes.DDLOption{Key: "DESCRIPTION", Value: p.cur.Str})
				p.advance()
			}
		} else if p.isIdentLikeStr("CLASSIFICATION") {
			p.advance()
			opt := &nodes.DDLOption{Key: "CLASSIFICATION"}
			if p.isIdentLike() || p.cur.Type == tokSCONST {
				opt.Value = p.cur.Str
				p.advance()
			}
			// [ LANGUAGE language ]
			if p.isIdentLikeStr("LANGUAGE") {
				p.advance()
				if p.isIdentLike() || p.cur.Type == tokSCONST {
					opt.Value += " LANGUAGE " + p.cur.Str
					p.advance()
				}
			}
			// VALUE 'value'
			if p.isIdentLikeStr("VALUE") {
				p.advance()
				if p.cur.Type == tokSCONST {
					opt.Value += " VALUE " + p.cur.Str
					p.advance()
				}
			}
			items = append(items, opt)
		} else {
			break
		}
	}
	if len(items) == 0 {
		return nil
	}
	return &nodes.List{Items: items}
}

// parseAttrDimSourceClause parses a source_clause in an attribute dimension USING clause.
func (p *Parser) parseAttrDimSourceClause() *nodes.AttrDimSourceClause {
	src := &nodes.AttrDimSourceClause{Loc: nodes.Loc{Start: p.pos()}}

	// JOIN PATH join_path_name ON join_condition
	if p.cur.Type == kwJOIN {
		p.advance() // consume JOIN
		if p.isIdentLikeStr("PATH") {
			p.advance() // consume PATH
		}
		src.IsJoinPath = true
		src.JoinPathName = p.parseIdentifier()
		if p.cur.Type == kwON {
			p.advance()
		}
		src.JoinCondition = &nodes.List{}
		for {
			elem := p.parseAttrDimJoinCondElem()
			if elem != nil {
				src.JoinCondition.Items = append(src.JoinCondition.Items, elem)
			}
			if p.cur.Type != kwAND {
				break
			}
			p.advance() // consume AND
		}
		src.Loc.End = p.pos()
		return src
	}

	// [ REMOTE ]
	if p.isIdentLikeStr("REMOTE") {
		src.Remote = true
		p.advance()
	}

	// [ schema. ] table/view
	src.Name = p.parseObjectName()

	// [ [ AS ] alias ]
	if p.cur.Type == kwAS {
		p.advance()
		src.Alias = p.parseIdentifier()
	} else if p.isIdentLike() && p.cur.Str != "ATTRIBUTES" && p.cur.Str != "JOIN" &&
		p.cur.Str != "DIMENSION" && p.cur.Str != "SHARING" && p.cur.Str != "CAPTION" &&
		p.cur.Str != "DESCRIPTION" && p.cur.Str != "CLASSIFICATION" &&
		p.cur.Type != ',' && p.cur.Type != ';' && p.cur.Type != tokEOF {
		src.Alias = p.parseIdentifier()
	}

	src.Loc.End = p.pos()
	return src
}

// parseAttrDimJoinCondElem parses a join condition element: [table.]col = [table.]col
func (p *Parser) parseAttrDimJoinCondElem() *nodes.AttrDimJoinCondElem {
	elem := &nodes.AttrDimJoinCondElem{Loc: nodes.Loc{Start: p.pos()}}
	// Left side
	name1 := p.parseIdentifier()
	if p.cur.Type == '.' {
		p.advance()
		elem.LeftTable = name1
		elem.LeftCol = p.parseIdentifier()
	} else {
		elem.LeftCol = name1
	}
	// =
	if p.cur.Type == '=' {
		p.advance()
	}
	// Right side
	name2 := p.parseIdentifier()
	if p.cur.Type == '.' {
		p.advance()
		elem.RightTable = name2
		elem.RightCol = p.parseIdentifier()
	} else {
		elem.RightCol = name2
	}
	elem.Loc.End = p.pos()
	return elem
}

// parseAttrDimAttributeClause parses an attribute in the ATTRIBUTES clause.
func (p *Parser) parseAttrDimAttributeClause() *nodes.AttrDimAttribute {
	attr := &nodes.AttrDimAttribute{Loc: nodes.Loc{Start: p.pos()}}
	attr.Column = p.parseIdentifier()
	if p.cur.Type == kwAS {
		p.advance()
		attr.Alias = p.parseIdentifier()
	}
	attr.Classifications = p.parseClassificationClauses()
	attr.Loc.End = p.pos()
	return attr
}

// parseAttrDimLevelClause parses a LEVEL clause in CREATE ATTRIBUTE DIMENSION.
func (p *Parser) parseAttrDimLevelClause() *nodes.AttrDimLevel {
	lvl := &nodes.AttrDimLevel{Loc: nodes.Loc{Start: p.pos()}}
	p.advance() // consume LEVEL

	lvl.Name = p.parseIdentifier()

	// LEVEL TYPE { STANDARD | YEARS | ... }
	if p.cur.Type == kwLEVEL {
		next := p.peekNext()
		if next.Type == kwTYPE {
			p.advance() // consume LEVEL
			p.advance() // consume TYPE
			if p.isIdentLike() {
				lvl.LevelType = p.cur.Str
				p.advance()
			}
		}
	}

	// classification_clause(s)
	lvl.Classifications = p.parseClassificationClauses()

	// KEY clause
	if p.cur.Type == kwKEY {
		p.advance()
		lvl.KeyAttrs = &nodes.List{}
		if p.cur.Type == '(' {
			p.advance()
			for p.cur.Type != ')' && p.cur.Type != tokEOF {
				lvl.KeyAttrs.Items = append(lvl.KeyAttrs.Items, &nodes.String{Str: p.parseIdentifier()})
				if p.cur.Type == ',' {
					p.advance()
				}
			}
			if p.cur.Type == ')' {
				p.advance()
			}
		} else {
			lvl.KeyAttrs.Items = append(lvl.KeyAttrs.Items, &nodes.String{Str: p.parseIdentifier()})
			// NOT NULL | SKIP WHEN NULL
			if p.cur.Type == kwNOT && p.peekNext().Type == kwNULL {
				lvl.KeyNotNull = true
				p.advance()
				p.advance()
			} else if p.cur.Type == kwSKIP {
				p.advance() // consume SKIP
				if p.cur.Type == kwWHEN {
					p.advance() // consume WHEN
				}
				if p.cur.Type == kwNULL {
					p.advance() // consume NULL
				}
				lvl.KeySkipWhenNull = true
			}
		}
	}

	// ALTERNATE KEY
	if p.isIdentLikeStr("ALTERNATE") {
		p.advance() // consume ALTERNATE
		if p.cur.Type == kwKEY {
			p.advance() // consume KEY
		}
		lvl.AltKeyAttrs = &nodes.List{}
		if p.cur.Type == '(' {
			p.advance()
			for p.cur.Type != ')' && p.cur.Type != tokEOF {
				lvl.AltKeyAttrs.Items = append(lvl.AltKeyAttrs.Items, &nodes.String{Str: p.parseIdentifier()})
				if p.cur.Type == ',' {
					p.advance()
				}
			}
			if p.cur.Type == ')' {
				p.advance()
			}
		} else {
			lvl.AltKeyAttrs.Items = append(lvl.AltKeyAttrs.Items, &nodes.String{Str: p.parseIdentifier()})
		}
	}

	// MEMBER NAME / CAPTION / DESCRIPTION
	lvl.MemberName, lvl.MemberCaption, lvl.MemberDesc = p.parseMemberExprs()

	// ORDER BY
	if p.cur.Type == kwORDER {
		p.advance() // consume ORDER
		if p.cur.Type == kwBY {
			p.advance() // consume BY
		}
		lvl.OrderByAttrs = &nodes.List{}
		if p.cur.Type == '(' {
			p.advance()
			for p.cur.Type != ')' && p.cur.Type != tokEOF {
				item := &nodes.AttrDimOrderByItem{Loc: nodes.Loc{Start: p.pos()}}
				item.Attribute = p.parseIdentifier()
				if p.cur.Type == kwASC {
					item.Direction = "ASC"
					p.advance()
				} else if p.cur.Type == kwDESC {
					item.Direction = "DESC"
					p.advance()
				}
				item.Loc.End = p.pos()
				lvl.OrderByAttrs.Items = append(lvl.OrderByAttrs.Items, item)
				if p.cur.Type == ',' {
					p.advance()
				}
			}
			if p.cur.Type == ')' {
				p.advance()
			}
		} else {
			item := &nodes.AttrDimOrderByItem{Loc: nodes.Loc{Start: p.pos()}}
			item.Attribute = p.parseIdentifier()
			if p.cur.Type == kwASC {
				item.Direction = "ASC"
				p.advance()
			} else if p.cur.Type == kwDESC {
				item.Direction = "DESC"
				p.advance()
			}
			item.Loc.End = p.pos()
			lvl.OrderByAttrs.Items = append(lvl.OrderByAttrs.Items, item)
		}
	}

	// DETERMINES ( attribute [, attribute ]... )
	if p.isIdentLikeStr("DETERMINES") {
		p.advance()
		lvl.Determines = &nodes.List{}
		if p.cur.Type == '(' {
			p.advance()
			for p.cur.Type != ')' && p.cur.Type != tokEOF {
				lvl.Determines.Items = append(lvl.Determines.Items, &nodes.String{Str: p.parseIdentifier()})
				if p.cur.Type == ',' {
					p.advance()
				}
			}
			if p.cur.Type == ')' {
				p.advance()
			}
		}
	}

	lvl.Loc.End = p.pos()
	return lvl
}

// parseMemberExprs parses MEMBER NAME/CAPTION/DESCRIPTION expression triples.
func (p *Parser) parseMemberExprs() (name, caption, desc nodes.ExprNode) {
	for p.isIdentLikeStr("MEMBER") {
		p.advance() // consume MEMBER
		switch {
		case p.cur.Type == kwNAME || p.isIdentLikeStr("NAME"):
			p.advance()
			name = p.parseExpr()
		case p.isIdentLikeStr("CAPTION"):
			p.advance()
			caption = p.parseExpr()
		case p.isIdentLikeStr("DESCRIPTION"):
			p.advance()
			desc = p.parseExpr()
		default:
			return
		}
	}
	return
}

// ---------------------------------------------------------------------------
// ALTER ATTRIBUTE DIMENSION
// ---------------------------------------------------------------------------

// parseAlterAttributeDimensionStmt parses an ALTER ATTRIBUTE DIMENSION statement.
//
// BNF: oracle/parser/bnf/ALTER-ATTRIBUTE-DIMENSION.bnf
//
//  ALTER ATTRIBUTE DIMENSION [ IF EXISTS ] [ schema . ] attr_dim_name
//      {
//          RENAME TO new_attr_dim_name
//        | COMPILE
//      }
func (p *Parser) parseAlterAttributeDimensionStmt(start int) *nodes.AlterAttributeDimensionStmt {
	stmt := &nodes.AlterAttributeDimensionStmt{
		Loc: nodes.Loc{Start: start},
	}

	// IF EXISTS
	if p.cur.Type == kwIF && p.peekNext().Type == kwEXISTS {
		stmt.IfExists = true
		p.advance()
		p.advance()
	}

	stmt.Name = p.parseObjectName()

	switch {
	case p.isIdentLikeStr("COMPILE"):
		stmt.Action = "COMPILE"
		p.advance()
	case p.cur.Type == kwRENAME || p.isIdentLikeStr("RENAME"):
		stmt.Action = "RENAME"
		p.advance() // consume RENAME
		if p.cur.Type == kwTO {
			p.advance() // consume TO
		}
		stmt.NewName = p.parseObjectName()
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// ---------------------------------------------------------------------------
// CREATE HIERARCHY
// ---------------------------------------------------------------------------

// parseCreateHierarchyStmt parses a CREATE HIERARCHY statement.
//
// BNF: oracle/parser/bnf/CREATE-HIERARCHY.bnf
//
//  CREATE [ OR REPLACE ] [ { FORCE | NOFORCE } ] HIERARCHY
//      [ IF NOT EXISTS ] [ schema. ] hierarchy
//      [ SHARING = { METADATA | NONE } ]
//      [ classification_clause ]...
//      hier_using_clause
//      ( level_hier_clause )
//      [ hier_attrs_clause ] ;
//
//  classification_clause:
//      { CAPTION 'caption'
//      | DESCRIPTION 'description'
//      | CLASSIFICATION classification_name VALUE 'classification_value'
//          [ LANGUAGE language ] }
//
//  hier_using_clause:
//      USING [ schema. ] attr_dimension
//
//  level_hier_clause:
//      level_name [ classification_clause ]...
//          [ CHILD OF level_hier_clause ]
//
//  hier_attrs_clause:
//      HIERARCHICAL ATTRIBUTES ( hier_attr_clause [, hier_attr_clause ]... )
//
//  hier_attr_clause:
//      hier_attr_name [ classification_clause ]...
//
//  hier_attr_name:
//      { HIER_ORDER | DEPTH | IS_LEAF | IS_ROOT
//      | MEMBER_NAME | MEMBER_UNIQUE_NAME | MEMBER_CAPTION | MEMBER_DESCRIPTION
//      | PARENT_LEVEL_NAME | PARENT_UNIQUE_NAME }
func (p *Parser) parseCreateHierarchyStmt(start int, orReplace, force, noForce bool) *nodes.CreateHierarchyStmt {
	stmt := &nodes.CreateHierarchyStmt{
		OrReplace: orReplace,
		Force:     force,
		NoForce:   noForce,
		Loc:       nodes.Loc{Start: start},
	}

	// IF NOT EXISTS
	if p.cur.Type == kwIF && p.peekNext().Type == kwNOT {
		p.advance()
		p.advance()
		if p.cur.Type == kwEXISTS {
			p.advance()
		}
		stmt.IfNotExists = true
	}

	stmt.Name = p.parseObjectName()

	// SHARING = { METADATA | NONE }
	if p.isIdentLikeStr("SHARING") {
		p.advance()
		if p.cur.Type == '=' {
			p.advance()
		}
		if p.isIdentLike() {
			stmt.Sharing = p.cur.Str
			p.advance()
		}
	}

	// classification_clause(s)
	stmt.Classifications = p.parseClassificationClauses()

	// USING [ schema. ] attr_dimension
	if p.cur.Type == kwUSING {
		p.advance()
		stmt.UsingAttrDim = p.parseObjectName()
	}

	// ( level_hier_clause )
	if p.cur.Type == '(' {
		p.advance()
		stmt.LevelHier = p.parseHierLevelClause()
		if p.cur.Type == ')' {
			p.advance()
		}
	}

	// HIERARCHICAL ATTRIBUTES ( ... )
	if p.isIdentLikeStr("HIERARCHICAL") {
		p.advance() // consume HIERARCHICAL
		if p.isIdentLikeStr("ATTRIBUTES") {
			p.advance() // consume ATTRIBUTES
		}
		if p.cur.Type == '(' {
			p.advance()
			stmt.HierAttrs = &nodes.List{}
			for p.cur.Type != ')' && p.cur.Type != tokEOF {
				ha := &nodes.HierAttr{Loc: nodes.Loc{Start: p.pos()}}
				ha.Name = p.parseIdentifier()
				ha.Classifications = p.parseClassificationClauses()
				ha.Loc.End = p.pos()
				stmt.HierAttrs.Items = append(stmt.HierAttrs.Items, ha)
				if p.cur.Type == ',' {
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

// parseHierLevelClause parses a level_hier_clause recursively.
func (p *Parser) parseHierLevelClause() *nodes.HierLevelClause {
	lvl := &nodes.HierLevelClause{Loc: nodes.Loc{Start: p.pos()}}
	lvl.Name = p.parseIdentifier()
	lvl.Classifications = p.parseClassificationClauses()

	// CHILD OF level_hier_clause
	if p.isIdentLikeStr("CHILD") {
		p.advance() // consume CHILD
		if p.cur.Type == kwOF {
			p.advance() // consume OF
		}
		lvl.ChildOf = p.parseHierLevelClause()
	}

	lvl.Loc.End = p.pos()
	return lvl
}

// ---------------------------------------------------------------------------
// ALTER HIERARCHY
// ---------------------------------------------------------------------------

// parseAlterHierarchyStmt parses an ALTER HIERARCHY statement.
//
// BNF: oracle/parser/bnf/ALTER-HIERARCHY.bnf
//
//  ALTER HIERARCHY [ IF EXISTS ] [ schema. ] hierarchy_name
//      { RENAME TO new_hier_name
//      | COMPILE
//      } ;
func (p *Parser) parseAlterHierarchyStmt(start int) *nodes.AlterHierarchyStmt {
	stmt := &nodes.AlterHierarchyStmt{
		Loc: nodes.Loc{Start: start},
	}

	// IF EXISTS
	if p.cur.Type == kwIF && p.peekNext().Type == kwEXISTS {
		stmt.IfExists = true
		p.advance()
		p.advance()
	}

	stmt.Name = p.parseObjectName()

	switch {
	case p.isIdentLikeStr("COMPILE"):
		stmt.Action = "COMPILE"
		p.advance()
	case p.cur.Type == kwRENAME || p.isIdentLikeStr("RENAME"):
		stmt.Action = "RENAME"
		p.advance()
		if p.cur.Type == kwTO {
			p.advance()
		}
		stmt.NewName = p.parseObjectName()
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// ---------------------------------------------------------------------------
// CREATE DOMAIN
// ---------------------------------------------------------------------------

// parseCreateDomainStmt parses a CREATE DOMAIN statement.
//
// BNF: oracle/parser/bnf/create-domain.bnf
//
//  CREATE [ OR REPLACE ] [ USECASE ] DOMAIN [ IF NOT EXISTS ] [ schema. ] domain_name
//      AS { create_single_column_domain
//         | create_multi_column_domain
//         | create_flexible_domain } ;
//
//  create_single_column_domain:
//      datatype [ STRICT ]
//      [ DEFAULT [ ON NULL ] default_expression ]
//      [ { constraint_clause }... ]
//      [ COLLATE collation_name ]
//      [ DISPLAY display_expression ]
//      [ ORDER order_expression ]
//      [ annotations_clause ]
//
//  create_single_column_domain:  -- ENUM variant
//      ENUM ( enum_list ) [ STRICT ]
//      [ DEFAULT [ ON NULL ] default_expression ]
//      [ { constraint_clause }... ]
//      [ COLLATE collation_name ]
//      [ DISPLAY display_expression ]
//      [ ORDER order_expression ]
//      [ annotations_clause ]
//
//  enum_list:
//      enum_item_list [, enum_item_list ]...
//
//  enum_item_list:
//      name [ = enum_alias_list ] [ = value ]
//
//  enum_alias_list:
//      alias [ = alias ]...
//
//  column_properties_clause:
//      [ DEFAULT [ ON NULL ] default_expression ]
//      [ { constraint_clause }... ]
//      [ COLLATE collation_name ]
//      [ DISPLAY display_expression ]
//      [ ORDER order_expression ]
//
//  create_multi_column_domain:
//      ( column_name AS datatype [ annotations_clause ]
//        [, column_name AS datatype [ annotations_clause ] ]... )
//      [ { constraint_clause }... ]
//      [ COLLATE collation_name ]
//      [ DISPLAY display_expression ]
//      [ ORDER order_expression ]
//      [ annotations_clause ]
//
//  create_flexible_domain:
//      FLEXIBLE DOMAIN [ schema. ] domain_name
//          ( column_name [, column_name ]... )
//      CHOOSE DOMAIN USING ( domain_discriminant_column datatype
//          [, domain_discriminant_column datatype ]... )
//      FROM { DECODE ( expr , comparison_expr , result_expr
//                      [, comparison_expr , result_expr ]... )
//           | CASE { WHEN condition THEN result_expr }... END }
//
//  result_expr:
//      [ schema. ] domain_name ( column_name [, column_name ]... )
//
//  default_clause:
//      DEFAULT [ ON NULL ] default_expression
//
//  constraint_clause:
//      [ CONSTRAINT constraint_name ]
//      { NOT NULL | NULL | CHECK ( condition ) }
//      [ constraint_state ]
//
//  annotations_clause:
//      ANNOTATIONS ( annotation [, annotation ]... )
func (p *Parser) parseCreateDomainStmt(start int, orReplace, usecase bool) *nodes.CreateDomainStmt {
	stmt := &nodes.CreateDomainStmt{
		OrReplace: orReplace,
		Usecase:   usecase,
		Loc:       nodes.Loc{Start: start},
	}

	// IF NOT EXISTS
	if p.cur.Type == kwIF && p.peekNext().Type == kwNOT {
		p.advance()
		p.advance()
		if p.cur.Type == kwEXISTS {
			p.advance()
		}
		stmt.IfNotExists = true
	}

	stmt.Name = p.parseObjectName()

	// AS
	if p.cur.Type == kwAS {
		p.advance()
	}

	// Determine variant: flexible (FLEXIBLE), enum (ENUM), multi-column '(', or single-column (datatype)
	switch {
	case p.isIdentLikeStr("FLEXIBLE"):
		stmt.DomainType = "FLEXIBLE"
		p.advance() // consume FLEXIBLE
		// DOMAIN [schema.]domain_name (column_name, ...)
		if p.isIdentLike() && p.cur.Str == "DOMAIN" {
			p.advance()
		}
		stmt.FlexDomainName = p.parseObjectName()
		if p.cur.Type == '(' {
			p.advance()
			stmt.FlexColumns = &nodes.List{}
			for p.cur.Type != ')' && p.cur.Type != tokEOF {
				stmt.FlexColumns.Items = append(stmt.FlexColumns.Items, &nodes.String{Str: p.parseIdentifier()})
				if p.cur.Type == ',' {
					p.advance()
				}
			}
			if p.cur.Type == ')' {
				p.advance()
			}
		}
		// CHOOSE DOMAIN USING ( ... )
		if p.isIdentLikeStr("CHOOSE") {
			p.advance()
			if p.isIdentLike() && p.cur.Str == "DOMAIN" {
				p.advance()
			}
			if p.cur.Type == kwUSING {
				p.advance()
			}
			if p.cur.Type == '(' {
				p.advance()
				stmt.ChooseUsing = &nodes.List{}
				for p.cur.Type != ')' && p.cur.Type != tokEOF {
					col := &nodes.DomainColumn{Loc: nodes.Loc{Start: p.pos()}}
					col.Name = p.parseIdentifier()
					col.DataType = p.parseTypeName()
					col.Loc.End = p.pos()
					stmt.ChooseUsing.Items = append(stmt.ChooseUsing.Items, col)
					if p.cur.Type == ',' {
						p.advance()
					}
				}
				if p.cur.Type == ')' {
					p.advance()
				}
			}
		}
		// FROM { DECODE(...) | CASE ... END }
		if p.cur.Type == kwFROM {
			p.advance()
			stmt.ChooseExpr = p.parseExpr()
		}

	case p.isIdentLikeStr("ENUM"):
		stmt.DomainType = "ENUM"
		p.advance() // consume ENUM
		// ( enum_list )
		if p.cur.Type == '(' {
			p.advance()
			stmt.EnumItems = &nodes.List{}
			for p.cur.Type != ')' && p.cur.Type != tokEOF {
				item := &nodes.DomainEnumItem{Loc: nodes.Loc{Start: p.pos()}}
				item.Name = p.parseIdentifier()
				// [ = alias [= alias]... ] [ = value ]
				for p.cur.Type == '=' {
					p.advance()
					if p.cur.Type == tokICONST || p.cur.Type == tokFCONST || p.cur.Type == tokSCONST {
						item.Value = p.parseExpr()
					} else if p.isIdentLike() || p.cur.Type == tokIDENT {
						item.Aliases = append(item.Aliases, p.parseIdentifier())
					} else {
						item.Value = p.parseExpr()
					}
				}
				item.Loc.End = p.pos()
				stmt.EnumItems.Items = append(stmt.EnumItems.Items, item)
				if p.cur.Type == ',' {
					p.advance()
				}
			}
			if p.cur.Type == ')' {
				p.advance()
			}
		}
		// STRICT
		if p.isIdentLikeStr("STRICT") {
			stmt.Strict = true
			p.advance()
		}
		p.parseDomainProperties(stmt)

	case p.cur.Type == '(':
		// Multi-column domain: ( column_name AS datatype [, ...] )
		stmt.DomainType = "MULTI"
		p.advance() // consume (
		stmt.Columns = &nodes.List{}
		for p.cur.Type != ')' && p.cur.Type != tokEOF {
			col := &nodes.DomainColumn{Loc: nodes.Loc{Start: p.pos()}}
			col.Name = p.parseIdentifier()
			if p.cur.Type == kwAS {
				p.advance()
			}
			col.DataType = p.parseTypeName()
			// annotations_clause per column
			if p.isIdentLikeStr("ANNOTATIONS") {
				col.Annotations = p.parseDomainAnnotations()
			}
			col.Loc.End = p.pos()
			stmt.Columns.Items = append(stmt.Columns.Items, col)
			if p.cur.Type == ',' {
				p.advance()
			}
		}
		if p.cur.Type == ')' {
			p.advance()
		}
		p.parseDomainProperties(stmt)

	default:
		// Single-column domain: datatype [STRICT] [properties...]
		stmt.DomainType = "SINGLE"
		stmt.DataType = p.parseTypeName()
		if p.isIdentLikeStr("STRICT") {
			stmt.Strict = true
			p.advance()
		}
		p.parseDomainProperties(stmt)
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseDomainProperties parses the common properties after a domain type definition.
func (p *Parser) parseDomainProperties(stmt *nodes.CreateDomainStmt) {
	// DEFAULT [ON NULL] expr
	if p.cur.Type == kwDEFAULT {
		p.advance()
		if p.cur.Type == kwON && p.peekNext().Type == kwNULL {
			stmt.DefaultOnNull = true
			p.advance()
			p.advance()
		}
		stmt.Default = p.parseExpr()
	}

	// constraint_clause(s)
	stmt.Constraints = p.parseDomainConstraints()

	// COLLATE collation_name
	if p.isIdentLikeStr("COLLATE") {
		p.advance()
		if p.isIdentLike() || p.cur.Type == tokSCONST {
			stmt.Collation = p.cur.Str
			p.advance()
		}
	}

	// DISPLAY display_expression
	if p.isIdentLikeStr("DISPLAY") {
		p.advance()
		stmt.Display = p.parseExpr()
	}

	// ORDER order_expression
	if p.cur.Type == kwORDER {
		p.advance()
		stmt.Order = p.parseExpr()
	}

	// ANNOTATIONS ( ... )
	if p.isIdentLikeStr("ANNOTATIONS") {
		stmt.Annotations = p.parseDomainAnnotations()
	}
}

// parseDomainConstraints parses zero or more constraint_clause items.
func (p *Parser) parseDomainConstraints() *nodes.List {
	var items []nodes.Node
	for {
		var c *nodes.DomainConstraint
		if p.cur.Type == kwCONSTRAINT {
			c = &nodes.DomainConstraint{Loc: nodes.Loc{Start: p.pos()}}
			p.advance()
			c.Name = p.parseIdentifier()
		}
		if p.cur.Type == kwNOT && p.peekNext().Type == kwNULL {
			if c == nil {
				c = &nodes.DomainConstraint{Loc: nodes.Loc{Start: p.pos()}}
			}
			c.Type = "NOT_NULL"
			p.advance()
			p.advance()
		} else if p.cur.Type == kwNULL {
			if c == nil {
				c = &nodes.DomainConstraint{Loc: nodes.Loc{Start: p.pos()}}
			}
			c.Type = "NULL"
			p.advance()
		} else if p.cur.Type == kwCHECK {
			if c == nil {
				c = &nodes.DomainConstraint{Loc: nodes.Loc{Start: p.pos()}}
			}
			c.Type = "CHECK"
			p.advance()
			if p.cur.Type == '(' {
				p.advance()
				c.CheckExpr = p.parseExpr()
				if p.cur.Type == ')' {
					p.advance()
				}
			}
		} else if c != nil {
			// CONSTRAINT name without a recognized type — shouldn't happen, keep going
			c.Type = "UNKNOWN"
		} else {
			break
		}
		c.Loc.End = p.pos()
		items = append(items, c)
	}
	if len(items) == 0 {
		return nil
	}
	return &nodes.List{Items: items}
}

// parseDomainAnnotations parses ANNOTATIONS ( ... ) for domains.
func (p *Parser) parseDomainAnnotations() *nodes.List {
	p.advance() // consume ANNOTATIONS
	result := &nodes.List{}
	if p.cur.Type == '(' {
		p.advance()
		for p.cur.Type != ')' && p.cur.Type != tokEOF {
			key := p.parseIdentifier()
			var val string
			if p.cur.Type == tokSCONST {
				val = p.cur.Str
				p.advance()
			}
			result.Items = append(result.Items, &nodes.DDLOption{Key: key, Value: val})
			if p.cur.Type == ',' {
				p.advance()
			}
		}
		if p.cur.Type == ')' {
			p.advance()
		}
	}
	return result
}

// ---------------------------------------------------------------------------
// ALTER DOMAIN
// ---------------------------------------------------------------------------

// parseAlterDomainStmt parses an ALTER DOMAIN statement.
//
// BNF: oracle/parser/bnf/alter-domain.bnf
//
//  ALTER [ USECASE ] DOMAIN [ IF EXISTS ] [ schema. ] domain_name
//      { ADD DISPLAY display_expression
//      | MODIFY DISPLAY display_expression
//      | DROP DISPLAY
//      | ADD ORDER order_expression
//      | MODIFY ORDER order_expression
//      | DROP ORDER
//      | annotations_clause
//      } ;
func (p *Parser) parseAlterDomainStmt(start int, usecase bool) *nodes.AlterDomainStmt {
	stmt := &nodes.AlterDomainStmt{
		Usecase: usecase,
		Loc:     nodes.Loc{Start: start},
	}

	// IF EXISTS
	if p.cur.Type == kwIF && p.peekNext().Type == kwEXISTS {
		stmt.IfExists = true
		p.advance()
		p.advance()
	}

	stmt.Name = p.parseObjectName()

	// Action
	switch {
	case p.isIdentLikeStr("ADD"):
		p.advance()
		if p.isIdentLikeStr("DISPLAY") {
			stmt.Action = "ADD_DISPLAY"
			p.advance()
			stmt.Display = p.parseExpr()
		} else if p.cur.Type == kwORDER {
			stmt.Action = "ADD_ORDER"
			p.advance()
			stmt.Order = p.parseExpr()
		}
	case p.isIdentLikeStr("MODIFY"):
		p.advance()
		if p.isIdentLikeStr("DISPLAY") {
			stmt.Action = "MODIFY_DISPLAY"
			p.advance()
			stmt.Display = p.parseExpr()
		} else if p.cur.Type == kwORDER {
			stmt.Action = "MODIFY_ORDER"
			p.advance()
			stmt.Order = p.parseExpr()
		}
	case p.isIdentLikeStr("DROP"):
		p.advance()
		if p.isIdentLikeStr("DISPLAY") {
			stmt.Action = "DROP_DISPLAY"
			p.advance()
		} else if p.cur.Type == kwORDER {
			stmt.Action = "DROP_ORDER"
			p.advance()
		}
	case p.isIdentLikeStr("ANNOTATIONS"):
		stmt.Action = "ANNOTATIONS"
		stmt.Annotations = p.parseDomainAnnotations()
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// ---------------------------------------------------------------------------
// CREATE PROPERTY GRAPH
// ---------------------------------------------------------------------------

// parseCreatePropertyGraphStmt parses a CREATE PROPERTY GRAPH statement.
// Called after CREATE [OR REPLACE] [IF NOT EXISTS] PROPERTY GRAPH has been consumed.
//
// BNF:
//
//	CREATE [ OR REPLACE ] PROPERTY GRAPH [ IF NOT EXISTS ]
//	    [ schema. ] graph_name
//	    vertex_tables_clause
//	    [ edge_tables_clause ]
//	    [ graph_options ] ;
func (p *Parser) parseCreatePropertyGraphStmt(start int, orReplace, ifNotExists bool) *nodes.CreatePropertyGraphStmt {
	stmt := &nodes.CreatePropertyGraphStmt{
		OrReplace:   orReplace,
		IfNotExists: ifNotExists,
		Loc:         nodes.Loc{Start: start},
	}

	// IF NOT EXISTS (may also be parsed here if not consumed by caller)
	if !ifNotExists && p.cur.Type == kwIF {
		next := p.peekNext()
		if next.Type == kwNOT {
			p.advance() // consume IF
			p.advance() // consume NOT
			if p.cur.Type == kwEXISTS {
				p.advance() // consume EXISTS
				stmt.IfNotExists = true
			}
		}
	}

	// Graph name
	stmt.Name = p.parseObjectName()

	// VERTEX TABLES ( ... )
	if p.isIdentLikeStr("VERTEX") {
		p.advance() // consume VERTEX
		if p.isIdentLikeStr("TABLES") {
			p.advance() // consume TABLES
		}
		if p.cur.Type == '(' {
			p.advance() // consume (
			for p.cur.Type != ')' && p.cur.Type != tokEOF {
				vtd := p.parseGraphTableDef()
				stmt.VertexTables = append(stmt.VertexTables, vtd)
				if p.cur.Type == ',' {
					p.advance()
				}
			}
			if p.cur.Type == ')' {
				p.advance()
			}
		}
	}

	// EDGE TABLES ( ... )
	if p.isIdentLikeStr("EDGE") {
		p.advance() // consume EDGE
		if p.isIdentLikeStr("TABLES") {
			p.advance() // consume TABLES
		}
		if p.cur.Type == '(' {
			p.advance() // consume (
			for p.cur.Type != ')' && p.cur.Type != tokEOF {
				etd := p.parseGraphEdgeDef()
				stmt.EdgeTables = append(stmt.EdgeTables, etd)
				if p.cur.Type == ',' {
					p.advance()
				}
			}
			if p.cur.Type == ')' {
				p.advance()
			}
		}
	}

	// OPTIONS ( ... )
	if p.isIdentLikeStr("OPTIONS") {
		p.advance() // consume OPTIONS
		if p.cur.Type == '(' {
			p.advance() // consume (
			opts := &nodes.GraphOptions{}
			for p.cur.Type != ')' && p.cur.Type != tokEOF {
				if p.isIdentLikeStr("ENFORCED") {
					opts.Mode = "ENFORCED"
					p.advance()
					if p.isIdentLikeStr("MODE") {
						p.advance()
					}
				} else if p.isIdentLikeStr("TRUSTED") {
					opts.Mode = "TRUSTED"
					p.advance()
					if p.isIdentLikeStr("MODE") {
						p.advance()
					}
				} else if p.isIdentLikeStr("ALLOW") {
					p.advance()
					opts.MixedPropTypes = "ALLOW"
					// skip MIXED PROPERTY TYPES
					for p.isIdentLike() && p.cur.Type != ')' {
						p.advance()
					}
				} else if p.isIdentLikeStr("DISALLOW") {
					p.advance()
					opts.MixedPropTypes = "DISALLOW"
					// skip MIXED PROPERTY TYPES
					for p.isIdentLike() && p.cur.Type != ')' {
						p.advance()
					}
				} else {
					p.advance()
				}
				if p.cur.Type == ',' {
					p.advance()
				}
			}
			if p.cur.Type == ')' {
				p.advance()
			}
			stmt.Options = opts
		}
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseGraphTableDef parses a vertex table definition.
func (p *Parser) parseGraphTableDef() *nodes.GraphTableDef {
	def := &nodes.GraphTableDef{
		Loc: nodes.Loc{Start: p.pos()},
	}

	// [schema.]table_name
	def.Name = p.parseObjectName()

	// AS graph_element_name
	if p.cur.Type == kwAS {
		p.advance()
		if p.isIdentLike() || p.cur.Type == tokIDENT || p.cur.Type == tokQIDENT {
			def.Alias = p.cur.Str
			p.advance()
		}
	}

	// KEY ( col, ... )
	if p.cur.Type == kwKEY {
		p.advance()
		def.KeyColumns = p.parseParenIdentList()
	}

	// Label and properties
	p.parseGraphLabelAndProperties(def)

	def.Loc.End = p.pos()
	return def
}

// parseGraphEdgeDef parses an edge table definition.
func (p *Parser) parseGraphEdgeDef() *nodes.GraphEdgeDef {
	def := &nodes.GraphEdgeDef{
		Loc: nodes.Loc{Start: p.pos()},
	}

	// [schema.]table_name
	def.Name = p.parseObjectName()

	// AS graph_element_name
	if p.cur.Type == kwAS {
		p.advance()
		if p.isIdentLike() || p.cur.Type == tokIDENT || p.cur.Type == tokQIDENT {
			def.Alias = p.cur.Str
			p.advance()
		}
	}

	// KEY ( col, ... )
	if p.cur.Type == kwKEY {
		p.advance()
		def.KeyColumns = p.parseParenIdentList()
	}

	// SOURCE [ KEY ( col, ... ) REFERENCES ] vertex_table_ref
	if p.isIdentLikeStr("SOURCE") {
		p.advance()
		if p.cur.Type == kwKEY {
			p.advance()
			def.SourceKeyColumns = p.parseParenIdentList()
			if p.cur.Type == kwREFERENCES {
				p.advance()
			}
		}
		// vertex_table_reference: name ( col, ... )
		if p.isIdentLike() || p.cur.Type == tokIDENT || p.cur.Type == tokQIDENT {
			def.SourceRef = p.cur.Str
			p.advance()
			if p.cur.Type == '(' {
				def.SourceRefColumns = p.parseParenIdentList()
			}
		}
	}

	// DESTINATION [ KEY ( col, ... ) REFERENCES ] vertex_table_ref
	if p.isIdentLikeStr("DESTINATION") {
		p.advance()
		if p.cur.Type == kwKEY {
			p.advance()
			def.DestKeyColumns = p.parseParenIdentList()
			if p.cur.Type == kwREFERENCES {
				p.advance()
			}
		}
		// vertex_table_reference: name ( col, ... )
		if p.isIdentLike() || p.cur.Type == tokIDENT || p.cur.Type == tokQIDENT {
			def.DestRef = p.cur.Str
			p.advance()
			if p.cur.Type == '(' {
				def.DestRefColumns = p.parseParenIdentList()
			}
		}
	}

	// Label and properties (reuse vertex table logic for label/properties part)
	vtd := &nodes.GraphTableDef{}
	p.parseGraphLabelAndProperties(vtd)
	def.Labels = vtd.Labels
	def.Properties = vtd.Properties
	def.PropColumns = vtd.PropColumns
	def.PropAliases = vtd.PropAliases
	def.DefaultLabel = vtd.DefaultLabel

	def.Loc.End = p.pos()
	return def
}

// parseGraphLabelAndProperties parses the label and properties clauses for graph tables.
func (p *Parser) parseGraphLabelAndProperties(def *nodes.GraphTableDef) {
	// LABEL label_name [ LABEL label_name ]...
	for p.isIdentLikeStr("LABEL") {
		p.advance()
		if p.isIdentLike() || p.cur.Type == tokIDENT || p.cur.Type == tokQIDENT {
			def.Labels = append(def.Labels, p.cur.Str)
			p.advance()
		}
	}

	// PROPERTIES clause
	if p.isIdentLikeStr("PROPERTIES") {
		p.advance()
		// ARE ALL COLUMNS [EXCEPT ...]
		if p.isIdentLikeStr("ARE") {
			p.advance()
		}
		if p.cur.Type == kwALL {
			p.advance()
			// ALL COLUMNS EXCEPT (...)
			if p.isIdentLikeStr("COLUMNS") {
				p.advance()
			}
			if p.cur.Type == kwEXCEPT || p.isIdentLikeStr("EXCEPT") {
				p.advance()
				def.Properties = "ALL_EXCEPT"
				if p.cur.Type == '(' {
					def.PropColumns = p.parseParenIdentList()
				}
			} else {
				def.Properties = "ALL"
			}
		} else if p.cur.Type == '(' {
			def.Properties = "LIST"
			p.advance()
			for p.cur.Type != ')' && p.cur.Type != tokEOF {
				if p.isIdentLike() || p.cur.Type == tokIDENT || p.cur.Type == tokQIDENT {
					def.PropColumns = append(def.PropColumns, p.cur.Str)
					p.advance()
				}
				if p.cur.Type == kwAS {
					p.advance()
					if p.isIdentLike() || p.cur.Type == tokIDENT || p.cur.Type == tokQIDENT {
						def.PropAliases = append(def.PropAliases, p.cur.Str)
						p.advance()
					}
				}
				if p.cur.Type == ',' {
					p.advance()
				}
			}
			if p.cur.Type == ')' {
				p.advance()
			}
		}
	} else if p.isIdentLikeStr("NO") {
		p.advance()
		if p.isIdentLikeStr("PROPERTIES") {
			p.advance()
			def.Properties = "NO"
		}
	}

	// DEFAULT LABEL
	if p.cur.Type == kwDEFAULT {
		p.advance()
		if p.isIdentLikeStr("LABEL") {
			p.advance()
			def.DefaultLabel = true
		}
	}
}

// parseParenIdentList parses ( ident, ident, ... ) and returns the list of identifiers.
func (p *Parser) parseParenIdentList() []string {
	var result []string
	if p.cur.Type != '(' {
		return result
	}
	p.advance() // consume (
	for p.cur.Type != ')' && p.cur.Type != tokEOF {
		if p.isIdentLike() || p.cur.Type == tokIDENT || p.cur.Type == tokQIDENT {
			result = append(result, p.cur.Str)
			p.advance()
		} else {
			p.advance()
		}
		if p.cur.Type == ',' {
			p.advance()
		}
	}
	if p.cur.Type == ')' {
		p.advance()
	}
	return result
}

// ---------------------------------------------------------------------------
// CREATE VECTOR INDEX
// ---------------------------------------------------------------------------

// parseCreateVectorIndexStmt parses a CREATE VECTOR INDEX statement.
// Called after CREATE VECTOR INDEX has been consumed.
//
// BNF:
//
//	CREATE VECTOR INDEX [ IF NOT EXISTS ] [ schema. ] index_name
//	    ON [ schema. ] table_name ( column_name )
//	    [ INCLUDE ( column_name [, column_name ]... ) ]
//	    [ vector_index_organization_clause ]
//	    [ DISTANCE metric_name ]
//	    [ WITH TARGET ACCURACY integer [ PARAMETERS ( ... ) ] ]
//	    [ vector_index_hnsw_replication_clause ]
//	    [ ONLINE ]
//	    [ PARALLEL integer ] ;
func (p *Parser) parseCreateVectorIndexStmt(start int, ifNotExists bool) *nodes.CreateVectorIndexStmt {
	stmt := &nodes.CreateVectorIndexStmt{
		IfNotExists: ifNotExists,
		Loc:         nodes.Loc{Start: start},
	}

	// IF NOT EXISTS
	if !ifNotExists && p.cur.Type == kwIF {
		next := p.peekNext()
		if next.Type == kwNOT {
			p.advance() // consume IF
			p.advance() // consume NOT
			if p.cur.Type == kwEXISTS {
				p.advance() // consume EXISTS
				stmt.IfNotExists = true
			}
		}
	}

	// Index name
	stmt.Name = p.parseObjectName()

	// ON [schema.]table_name (column_name)
	if p.cur.Type == kwON {
		p.advance()
		stmt.TableName = p.parseObjectName()
		if p.cur.Type == '(' {
			p.advance()
			if p.isIdentLike() || p.cur.Type == tokIDENT || p.cur.Type == tokQIDENT {
				stmt.Column = p.cur.Str
				p.advance()
			}
			if p.cur.Type == ')' {
				p.advance()
			}
		}
	}

	// INCLUDE ( col, ... )
	if p.cur.Type == kwINCLUDE {
		p.advance()
		stmt.IncludeColumns = p.parseParenIdentList()
	}

	// ORGANIZATION { INMEMORY NEIGHBOR GRAPH | NEIGHBOR PARTITIONS }
	if p.isIdentLikeStr("ORGANIZATION") {
		p.advance()
		if p.isIdentLikeStr("INMEMORY") {
			p.advance()
			stmt.Organization = "INMEMORY_NEIGHBOR_GRAPH"
			// consume NEIGHBOR GRAPH
			if p.isIdentLikeStr("NEIGHBOR") {
				p.advance()
			}
			if p.isIdentLikeStr("GRAPH") {
				p.advance()
			}
		} else if p.isIdentLikeStr("NEIGHBOR") {
			p.advance()
			stmt.Organization = "NEIGHBOR_PARTITIONS"
			if p.isIdentLikeStr("PARTITIONS") {
				p.advance()
			}
		}
	}

	// DISTANCE metric_name
	if p.isIdentLikeStr("DISTANCE") {
		p.advance()
		if p.isIdentLike() || p.cur.Type == tokIDENT || p.cur.Type == tokQIDENT {
			stmt.Distance = p.cur.Str
			p.advance()
		}
	}

	// WITH TARGET ACCURACY integer [ PARAMETERS (...) ]
	if p.cur.Type == kwWITH {
		p.advance()
		if p.isIdentLikeStr("TARGET") {
			p.advance()
		}
		if p.isIdentLikeStr("ACCURACY") {
			p.advance()
		}
		if p.cur.Type == tokICONST {
			stmt.TargetAccuracy = p.parseIntValue()
		}
		// PARAMETERS ( ... )
		if p.isIdentLikeStr("PARAMETERS") {
			p.advance()
			if p.cur.Type == '(' {
				p.advance()
				// TYPE HNSW | IVF
				if p.cur.Type == kwTYPE || p.isIdentLikeStr("TYPE") {
					p.advance()
					if p.isIdentLikeStr("HNSW") {
						stmt.ParameterType = "HNSW"
						p.advance()
					} else if p.isIdentLikeStr("IVF") {
						stmt.ParameterType = "IVF"
						p.advance()
					}
				}
				// Parse remaining HNSW/IVF parameters
				for p.cur.Type != ')' && p.cur.Type != tokEOF {
					if p.cur.Type == ',' {
						p.advance()
					}
					if p.isIdentLikeStr("NEIGHBORS") || p.isIdentLikeStr("M") {
						p.advance()
						if p.cur.Type == tokICONST {
							stmt.Neighbors = p.parseIntValue()
						}
					} else if p.isIdentLikeStr("EFCONSTRUCTION") {
						p.advance()
						if p.cur.Type == tokICONST {
							stmt.EfConstruction = p.parseIntValue()
						}
					} else if p.isIdentLikeStr("RESCORE_FACTOR") {
						p.advance()
						if p.cur.Type == tokICONST {
							stmt.RescoreFactor = p.parseIntValue()
						}
					} else if p.isIdentLikeStr("QUANTIZATION") {
						p.advance()
						if p.isIdentLike() || p.cur.Type == tokIDENT {
							stmt.Quantization = p.cur.Str
							p.advance()
						}
					} else if p.isIdentLikeStr("NEIGHBOR") {
						p.advance()
						if p.isIdentLikeStr("PARTITIONS") {
							p.advance()
						}
						if p.cur.Type == tokICONST {
							stmt.NeighborParts = p.parseIntValue()
						}
					} else if p.isIdentLikeStr("SAMPLES_PER_PARTITION") {
						p.advance()
						if p.cur.Type == tokICONST {
							stmt.SamplesPerPart = p.parseIntValue()
						}
					} else if p.isIdentLikeStr("MIN_VECTORS_PER_PARTITION") {
						p.advance()
						if p.cur.Type == tokICONST {
							stmt.MinVecsPerPart = p.parseIntValue()
						}
					} else {
						p.advance()
					}
				}
				if p.cur.Type == ')' {
					p.advance()
				}
			}
		}
	}

	// Replication clause: DUPLICATE ALL | DISTRIBUTE [AUTO | BY ...]
	if p.isIdentLikeStr("DUPLICATE") {
		p.advance()
		if p.cur.Type == kwALL {
			p.advance()
			stmt.Replication = "DUPLICATE_ALL"
		}
	} else if p.isIdentLikeStr("DISTRIBUTE") {
		p.advance()
		stmt.Replication = "DISTRIBUTE"
		if p.isIdentLikeStr("AUTO") {
			p.advance()
			stmt.Replication = "DISTRIBUTE_AUTO"
		} else if p.cur.Type == kwBY {
			p.advance()
			if p.isIdentLikeStr("ROWID") {
				p.advance()
				if p.isIdentLikeStr("RANGE") {
					p.advance()
				}
				stmt.Replication = "DISTRIBUTE_BY_ROWID_RANGE"
			} else if p.cur.Type == kwPARTITION {
				p.advance()
				stmt.Replication = "DISTRIBUTE_BY_PARTITION"
			} else if p.isIdentLikeStr("SUBPARTITION") {
				p.advance()
				stmt.Replication = "DISTRIBUTE_BY_SUBPARTITION"
			}
		}
	}

	// ONLINE
	if p.cur.Type == kwONLINE {
		p.advance()
		stmt.Online = true
	}

	// PARALLEL integer
	if p.cur.Type == kwPARALLEL {
		p.advance()
		if p.cur.Type == tokICONST {
			stmt.Parallel = p.parseIntValue()
		}
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// ---------------------------------------------------------------------------
// CREATE / ALTER LOCKDOWN PROFILE
// ---------------------------------------------------------------------------

// parseCreateLockdownProfileStmt parses a CREATE LOCKDOWN PROFILE statement.
// Called after CREATE LOCKDOWN PROFILE has been consumed.
//
// BNF:
//
//	CREATE LOCKDOWN PROFILE profile_name
//	    [ USING base_profile_name | INCLUDING base_profile_name ]
func (p *Parser) parseCreateLockdownProfileStmt(start int) *nodes.CreateLockdownProfileStmt {
	stmt := &nodes.CreateLockdownProfileStmt{
		Loc: nodes.Loc{Start: start},
	}

	// profile_name
	if p.isIdentLike() || p.cur.Type == tokIDENT || p.cur.Type == tokQIDENT {
		stmt.Name = p.cur.Str
		p.advance()
	}

	// USING base_profile_name
	if p.isIdentLikeStr("USING") {
		p.advance()
		if p.isIdentLike() || p.cur.Type == tokIDENT || p.cur.Type == tokQIDENT {
			stmt.Using = p.cur.Str
			p.advance()
		}
	}

	// INCLUDING base_profile_name
	if p.isIdentLikeStr("INCLUDING") {
		p.advance()
		if p.isIdentLike() || p.cur.Type == tokIDENT || p.cur.Type == tokQIDENT {
			stmt.Including = p.cur.Str
			p.advance()
		}
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseAlterLockdownProfileStmt parses an ALTER LOCKDOWN PROFILE statement.
// Called after ALTER LOCKDOWN PROFILE has been consumed.
//
// BNF:
//
//	ALTER LOCKDOWN PROFILE profile_name
//	    { lockdown_features | lockdown_options | lockdown_statements }
//	    [ USERS = { ALL | COMMON | LOCAL } ] ;
func (p *Parser) parseAlterLockdownProfileStmt(start int) *nodes.AlterLockdownProfileStmt {
	stmt := &nodes.AlterLockdownProfileStmt{
		Loc: nodes.Loc{Start: start},
	}

	// profile_name
	if p.isIdentLike() || p.cur.Type == tokIDENT || p.cur.Type == tokQIDENT {
		stmt.Name = p.cur.Str
		p.advance()
	}

	// { DISABLE | ENABLE }
	if p.cur.Type == kwDISABLE {
		stmt.Action = "DISABLE"
		p.advance()
	} else if p.cur.Type == kwENABLE {
		stmt.Action = "ENABLE"
		p.advance()
	}

	// { FEATURE = (...) | OPTION = (...) | STATEMENT = (...) }
	if p.isIdentLikeStr("FEATURE") {
		stmt.RuleType = "FEATURE"
		p.advance()
		if p.cur.Type == '=' {
			p.advance()
		}
		stmt.RuleItems, stmt.AllItems, stmt.ExceptItems = p.parseLockdownItemList()
	} else if p.cur.Type == kwOPTION || p.isIdentLikeStr("OPTION") {
		stmt.RuleType = "OPTION"
		p.advance()
		if p.cur.Type == '=' {
			p.advance()
		}
		stmt.RuleItems, stmt.AllItems, stmt.ExceptItems = p.parseLockdownItemList()
	} else if p.isIdentLikeStr("STATEMENT") {
		stmt.RuleType = "STATEMENT"
		p.advance()
		if p.cur.Type == '=' {
			p.advance()
		}
		stmt.RuleItems, stmt.AllItems, stmt.ExceptItems = p.parseLockdownItemList()

		// CLAUSE = (...)
		if p.isIdentLikeStr("CLAUSE") {
			p.advance()
			if p.cur.Type == '=' {
				p.advance()
			}
			stmt.Clauses, stmt.ClauseAll, stmt.ClauseExceptItems = p.parseLockdownItemList()

			// OPTION = (...)
			if p.cur.Type == kwOPTION || p.isIdentLikeStr("OPTION") {
				p.advance()
				if p.cur.Type == '=' {
					p.advance()
				}
				stmt.ClauseOptions, _, _ = p.parseLockdownItemList()

				// VALUE / MINVALUE / MAXVALUE
				if p.isIdentLikeStr("VALUE") {
					p.advance()
					if p.cur.Type == '=' {
						p.advance()
					}
					stmt.ValueItems, _, _ = p.parseLockdownItemList()
				}
				if p.cur.Type == kwMINVALUE || p.isIdentLikeStr("MINVALUE") {
					p.advance()
					if p.cur.Type == '=' {
						p.advance()
					}
					if p.isIdentLike() || p.cur.Type == tokIDENT || p.cur.Type == tokICONST || p.cur.Type == tokSCONST {
						stmt.MinValue = p.cur.Str
						p.advance()
					}
				}
				if p.cur.Type == kwMAXVALUE || p.isIdentLikeStr("MAXVALUE") {
					p.advance()
					if p.cur.Type == '=' {
						p.advance()
					}
					if p.isIdentLike() || p.cur.Type == tokIDENT || p.cur.Type == tokICONST || p.cur.Type == tokSCONST {
						stmt.MaxValue = p.cur.Str
						p.advance()
					}
				}
			}
		}
	}

	// USERS = { ALL | COMMON | LOCAL }
	if p.isIdentLikeStr("USERS") {
		p.advance()
		if p.cur.Type == '=' {
			p.advance()
		}
		if p.cur.Type == kwALL {
			stmt.Users = "ALL"
			p.advance()
		} else if p.isIdentLikeStr("COMMON") {
			stmt.Users = "COMMON"
			p.advance()
		} else if p.isIdentLikeStr("LOCAL") {
			stmt.Users = "LOCAL"
			p.advance()
		}
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseLockdownItemList parses ( item, item, ... ) or ( ALL ) or ( ALL EXCEPT = ( ... ) ).
func (p *Parser) parseLockdownItemList() (items []string, allItems bool, exceptItems []string) {
	if p.cur.Type != '(' {
		return
	}
	p.advance() // consume (
	if p.cur.Type == kwALL {
		p.advance()
		// ALL EXCEPT = ( ... )
		if p.cur.Type == kwEXCEPT || p.isIdentLikeStr("EXCEPT") {
			p.advance()
			if p.cur.Type == '=' {
				p.advance()
			}
			if p.cur.Type == '(' {
				p.advance()
				for p.cur.Type != ')' && p.cur.Type != tokEOF {
					if p.isIdentLike() || p.cur.Type == tokIDENT || p.cur.Type == tokQIDENT || p.cur.Type == tokSCONST {
						exceptItems = append(exceptItems, p.cur.Str)
						p.advance()
					} else {
						p.advance()
					}
					if p.cur.Type == ',' {
						p.advance()
					}
				}
				if p.cur.Type == ')' {
					p.advance()
				}
			}
		}
		allItems = true
	} else {
		for p.cur.Type != ')' && p.cur.Type != tokEOF {
			if p.isIdentLike() || p.cur.Type == tokIDENT || p.cur.Type == tokQIDENT || p.cur.Type == tokSCONST {
				items = append(items, p.cur.Str)
				p.advance()
			} else {
				p.advance()
			}
			if p.cur.Type == ',' {
				p.advance()
			}
		}
	}
	if p.cur.Type == ')' {
		p.advance()
	}
	return
}

// ---------------------------------------------------------------------------
// CREATE / ALTER OUTLINE
// ---------------------------------------------------------------------------

// parseCreateOutlineStmt parses a CREATE OUTLINE statement.
// Called after CREATE [OR REPLACE] [PUBLIC|PRIVATE] OUTLINE has been consumed.
//
// BNF:
//
//	CREATE [ OR REPLACE ] [ PUBLIC | PRIVATE ] OUTLINE [ outline ]
//	    [ FOR CATEGORY category ]
//	    { FROM [ PRIVATE ] source_outline | ON statement } ;
func (p *Parser) parseCreateOutlineStmt(start int, orReplace, public bool) *nodes.CreateOutlineStmt {
	stmt := &nodes.CreateOutlineStmt{
		OrReplace: orReplace,
		Public:    public,
		Loc:       nodes.Loc{Start: start},
	}

	// Optional outline name — the name comes before FOR/FROM/ON.
	// We need to check if the current token is an identifier that is NOT a keyword
	// like FOR, FROM, ON which start the next clause.
	if p.cur.Type != kwFOR && p.cur.Type != kwFROM && p.cur.Type != kwON &&
		p.cur.Type != ';' && p.cur.Type != tokEOF {
		if p.isIdentLike() || p.cur.Type == tokIDENT || p.cur.Type == tokQIDENT {
			stmt.Name = p.cur.Str
			p.advance()
		}
	}

	// FOR CATEGORY category
	if p.cur.Type == kwFOR {
		p.advance()
		if p.isIdentLikeStr("CATEGORY") {
			p.advance()
		}
		if p.isIdentLike() || p.cur.Type == tokIDENT || p.cur.Type == tokQIDENT {
			stmt.Category = p.cur.Str
			p.advance()
		}
	}

	// FROM [ PRIVATE ] source_outline
	if p.cur.Type == kwFROM {
		p.advance()
		if p.cur.Type == kwPRIVATE {
			stmt.FromPrivate = true
			p.advance()
		}
		if p.isIdentLike() || p.cur.Type == tokIDENT || p.cur.Type == tokQIDENT {
			stmt.FromSource = p.cur.Str
			p.advance()
		}
	}

	// ON statement — skip to semicolon
	if p.cur.Type == kwON {
		p.advance()
		for p.cur.Type != ';' && p.cur.Type != tokEOF {
			p.advance()
		}
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseAlterOutlineStmt parses an ALTER OUTLINE statement.
// Called after ALTER OUTLINE has been consumed.
//
// BNF:
//
//	ALTER OUTLINE [ PUBLIC | PRIVATE ] outline
//	    { REBUILD | RENAME TO new_outline_name | CHANGE CATEGORY TO new_category_name | { ENABLE | DISABLE } } ;
func (p *Parser) parseAlterOutlineStmt(start int) *nodes.AlterOutlineStmt {
	stmt := &nodes.AlterOutlineStmt{
		Loc: nodes.Loc{Start: start},
	}

	// PUBLIC | PRIVATE
	if p.cur.Type == kwPUBLIC {
		stmt.Public = true
		p.advance()
	} else if p.cur.Type == kwPRIVATE {
		stmt.Private = true
		p.advance()
	}

	// outline name
	if p.isIdentLike() || p.cur.Type == tokIDENT || p.cur.Type == tokQIDENT {
		stmt.Name = p.cur.Str
		p.advance()
	}

	// Action
	if p.isIdentLikeStr("REBUILD") {
		stmt.Action = "REBUILD"
		p.advance()
	} else if p.cur.Type == kwRENAME {
		stmt.Action = "RENAME"
		p.advance()
		if p.cur.Type == kwTO {
			p.advance()
		}
		if p.isIdentLike() || p.cur.Type == tokIDENT || p.cur.Type == tokQIDENT {
			stmt.NewName = p.cur.Str
			p.advance()
		}
	} else if p.isIdentLikeStr("CHANGE") {
		stmt.Action = "CHANGE_CATEGORY"
		p.advance()
		if p.isIdentLikeStr("CATEGORY") {
			p.advance()
		}
		if p.cur.Type == kwTO {
			p.advance()
		}
		if p.isIdentLike() || p.cur.Type == tokIDENT || p.cur.Type == tokQIDENT {
			stmt.Category = p.cur.Str
			p.advance()
		}
	} else if p.cur.Type == kwENABLE {
		stmt.Action = "ENABLE"
		p.advance()
	} else if p.cur.Type == kwDISABLE {
		stmt.Action = "DISABLE"
		p.advance()
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// ---------------------------------------------------------------------------
// Batch 105 — small objects bundle
// ---------------------------------------------------------------------------

// parseCreateJavaStmt parses a CREATE JAVA statement.
// Called after CREATE [OR REPLACE] [IF NOT EXISTS] JAVA has been consumed.
//
// BNF:
//
//	CREATE [ OR REPLACE | IF NOT EXISTS ] [ AND { RESOLVE | COMPILE } ] [ NOFORCE ]
//	    JAVA { SOURCE | CLASS | RESOURCE }
//	    [ NAMED [ schema. ] primary_name ]
//	    [ SHARING = { METADATA | NONE } ]
//	    [ invoker_rights_clause ]
//	    [ resolver_clause ]
//	    { USING { BFILE ( directory_object_name , server_file_name )
//	            | { CLOB | BLOB | BFILE } subquery
//	            | key_for_BLOB }
//	    | AS source_char }
func (p *Parser) parseCreateJavaStmt(start int, orReplace bool) nodes.StmtNode {
	stmt := &nodes.AdminDDLStmt{
		Action:     "CREATE",
		ObjectType: nodes.OBJECT_JAVA,
		OrReplace:  orReplace,
		Loc:        nodes.Loc{Start: start},
	}
	opts := &nodes.List{}

	// [ AND { RESOLVE | COMPILE } ]
	if p.isIdentLike() && p.cur.Str == "AND" {
		p.advance()
		if p.isIdentLike() && (p.cur.Str == "RESOLVE" || p.cur.Str == "COMPILE") {
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: "AND", Value: p.cur.Str})
			p.advance()
		}
	}

	// [ NOFORCE ]
	if p.isIdentLike() && p.cur.Str == "NOFORCE" {
		opts.Items = append(opts.Items, &nodes.DDLOption{Key: "NOFORCE"})
		p.advance()
	}

	// { SOURCE | CLASS | RESOURCE }
	if p.isIdentLike() && (p.cur.Str == "SOURCE" || p.cur.Str == "CLASS" || p.cur.Str == "RESOURCE") {
		opts.Items = append(opts.Items, &nodes.DDLOption{Key: "JAVA_TYPE", Value: p.cur.Str})
		p.advance()
	}

	// [ NAMED [ schema. ] primary_name ]
	if p.isIdentLike() && p.cur.Str == "NAMED" {
		p.advance()
		stmt.Name = p.parseObjectName()
	}

	// Parse remaining clauses until ; or EOF
	for p.cur.Type != ';' && p.cur.Type != tokEOF {
		if p.isIdentLike() && p.cur.Str == "SHARING" {
			p.advance()
			if p.cur.Type == '=' {
				p.advance()
			}
			if p.isIdentLike() || p.cur.Type == tokIDENT {
				opts.Items = append(opts.Items, &nodes.DDLOption{Key: "SHARING", Value: p.cur.Str})
				p.advance()
			}
		} else if p.isIdentLike() && p.cur.Str == "AUTHID" {
			p.advance()
			if p.isIdentLike() || p.cur.Type == tokIDENT {
				opts.Items = append(opts.Items, &nodes.DDLOption{Key: "AUTHID", Value: p.cur.Str})
				p.advance()
			}
		} else if p.isIdentLike() && p.cur.Str == "RESOLVER" {
			p.advance()
			if p.cur.Type == '(' {
				p.skipParenthesized()
			}
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: "RESOLVER"})
		} else if p.isIdentLike() && p.cur.Str == "USING" {
			p.advance()
			val := ""
			if p.isIdentLike() || p.cur.Type == tokIDENT {
				val = p.cur.Str
				p.advance()
			}
			if p.cur.Type == '(' {
				p.skipParenthesized()
			}
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: "USING", Value: val})
		} else if p.cur.Type == kwAS {
			p.advance()
			// source_char — typically a string constant
			if p.cur.Type == tokSCONST {
				opts.Items = append(opts.Items, &nodes.DDLOption{Key: "AS", Value: p.cur.Str})
				p.advance()
			}
		} else {
			p.advance()
		}
	}

	if len(opts.Items) > 0 {
		stmt.Options = opts
	}
	stmt.Loc.End = p.pos()
	return stmt
}

// parseAlterJavaStmt parses an ALTER JAVA statement.
// Called after ALTER JAVA has been consumed.
//
// BNF:
//
//	ALTER JAVA [ IF EXISTS ] { SOURCE | CLASS } [ schema. ] object_name
//	    [ RESOLVER ( ( match_string schema_name ) [, ...] ) ]
//	    [ invoker_rights_clause ]
//	    { RESOLVE | COMPILE } ;
func (p *Parser) parseAlterJavaStmt(start int) nodes.StmtNode {
	stmt := &nodes.AdminDDLStmt{
		Action:     "ALTER",
		ObjectType: nodes.OBJECT_JAVA,
		Loc:        nodes.Loc{Start: start},
	}
	opts := &nodes.List{}

	// [ IF EXISTS ]
	if p.cur.Type == kwIF {
		next := p.peekNext()
		if next.Type == kwEXISTS {
			p.advance()
			p.advance()
			stmt.IfExists = true
		}
	}

	// { SOURCE | CLASS }
	if p.isIdentLike() && (p.cur.Str == "SOURCE" || p.cur.Str == "CLASS") {
		opts.Items = append(opts.Items, &nodes.DDLOption{Key: "JAVA_TYPE", Value: p.cur.Str})
		p.advance()
	}

	stmt.Name = p.parseObjectName()

	// Parse remaining clauses
	for p.cur.Type != ';' && p.cur.Type != tokEOF {
		if p.isIdentLike() && p.cur.Str == "RESOLVER" {
			p.advance()
			if p.cur.Type == '(' {
				p.skipParenthesized()
			}
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: "RESOLVER"})
		} else if p.isIdentLike() && p.cur.Str == "AUTHID" {
			p.advance()
			if p.isIdentLike() || p.cur.Type == tokIDENT {
				opts.Items = append(opts.Items, &nodes.DDLOption{Key: "AUTHID", Value: p.cur.Str})
				p.advance()
			}
		} else if p.isIdentLike() && (p.cur.Str == "RESOLVE" || p.cur.Str == "COMPILE") {
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: p.cur.Str})
			p.advance()
		} else {
			p.advance()
		}
	}

	if len(opts.Items) > 0 {
		stmt.Options = opts
	}
	stmt.Loc.End = p.pos()
	return stmt
}

// parseCreateLibraryStmt parses a CREATE LIBRARY statement.
// Called after CREATE [OR REPLACE] [IF NOT EXISTS] [EDITIONABLE|NONEDITIONABLE] LIBRARY has been consumed.
//
// BNF:
//
//	CREATE [ OR REPLACE | IF NOT EXISTS ] [ EDITIONABLE | NONEDITIONABLE ]
//	    LIBRARY [ schema. ] library_name
//	    { IS | AS } library_path
//	    [ AGENT agent_dblink ]
//	    [ CREDENTIAL credential_name ]
//	    [ SHARING = { METADATA | NONE } ]
func (p *Parser) parseCreateLibraryStmt(start int, orReplace bool) nodes.StmtNode {
	stmt := &nodes.AdminDDLStmt{
		Action:     "CREATE",
		ObjectType: nodes.OBJECT_LIBRARY,
		OrReplace:  orReplace,
		Loc:        nodes.Loc{Start: start},
	}
	opts := &nodes.List{}

	stmt.Name = p.parseObjectName()

	// { IS | AS } library_path
	if p.cur.Type == kwIS || p.cur.Type == kwAS {
		p.advance()
	}
	if p.cur.Type == tokSCONST {
		opts.Items = append(opts.Items, &nodes.DDLOption{Key: "PATH", Value: p.cur.Str})
		p.advance()
	}

	// Parse optional clauses
	for p.cur.Type != ';' && p.cur.Type != tokEOF {
		if p.isIdentLike() && p.cur.Str == "AGENT" {
			p.advance()
			if p.isIdentLike() || p.cur.Type == tokIDENT || p.cur.Type == tokSCONST {
				opts.Items = append(opts.Items, &nodes.DDLOption{Key: "AGENT", Value: p.cur.Str})
				p.advance()
			}
		} else if p.isIdentLike() && p.cur.Str == "CREDENTIAL" {
			p.advance()
			if p.isIdentLike() || p.cur.Type == tokIDENT {
				opts.Items = append(opts.Items, &nodes.DDLOption{Key: "CREDENTIAL", Value: p.cur.Str})
				p.advance()
			}
		} else if p.isIdentLike() && p.cur.Str == "SHARING" {
			p.advance()
			if p.cur.Type == '=' {
				p.advance()
			}
			if p.isIdentLike() || p.cur.Type == tokIDENT {
				opts.Items = append(opts.Items, &nodes.DDLOption{Key: "SHARING", Value: p.cur.Str})
				p.advance()
			}
		} else {
			p.advance()
		}
	}

	if len(opts.Items) > 0 {
		stmt.Options = opts
	}
	stmt.Loc.End = p.pos()
	return stmt
}

// parseAlterLibraryStmt parses an ALTER LIBRARY statement.
// Called after ALTER LIBRARY has been consumed.
//
// BNF:
//
//	ALTER LIBRARY [ IF EXISTS ] [ schema. ] library_name
//	    { library_compile_clause }
//	    [ EDITIONABLE | NONEDITIONABLE ] ;
//
//	library_compile_clause:
//	    COMPILE [ DEBUG ] [ compiler_parameters_clause ] [ REUSE SETTINGS ]
func (p *Parser) parseAlterLibraryStmt(start int) nodes.StmtNode {
	stmt := &nodes.AdminDDLStmt{
		Action:     "ALTER",
		ObjectType: nodes.OBJECT_LIBRARY,
		Loc:        nodes.Loc{Start: start},
	}
	opts := &nodes.List{}

	// [ IF EXISTS ]
	if p.cur.Type == kwIF {
		next := p.peekNext()
		if next.Type == kwEXISTS {
			p.advance()
			p.advance()
			stmt.IfExists = true
		}
	}

	stmt.Name = p.parseObjectName()

	// Parse remaining clauses
	for p.cur.Type != ';' && p.cur.Type != tokEOF {
		if p.isIdentLike() && p.cur.Str == "COMPILE" {
			p.advance()
			opt := &nodes.DDLOption{Key: "COMPILE"}
			if p.isIdentLike() && p.cur.Str == "DEBUG" {
				opt.Value = "DEBUG"
				p.advance()
			}
			opts.Items = append(opts.Items, opt)
			// compiler_parameters_clause: name = value [, ...]
			for p.isIdentLike() && p.cur.Str != "REUSE" &&
				p.cur.Type != ';' && p.cur.Type != tokEOF {
				if p.cur.Str == "EDITIONABLE" || p.cur.Str == "NONEDITIONABLE" {
					break
				}
				paramName := p.cur.Str
				p.advance()
				if p.cur.Type == '=' {
					p.advance()
					paramVal := ""
					if p.isIdentLike() || p.cur.Type == tokIDENT || p.cur.Type == tokICONST || p.cur.Type == tokSCONST {
						paramVal = p.cur.Str
						p.advance()
					}
					opts.Items = append(opts.Items, &nodes.DDLOption{Key: paramName, Value: paramVal})
				}
			}
			// [ REUSE SETTINGS ]
			if p.isIdentLike() && p.cur.Str == "REUSE" {
				p.advance()
				if p.isIdentLike() && p.cur.Str == "SETTINGS" {
					p.advance()
				}
				opts.Items = append(opts.Items, &nodes.DDLOption{Key: "REUSE SETTINGS"})
			}
		} else if p.isIdentLike() && (p.cur.Str == "EDITIONABLE" || p.cur.Str == "NONEDITIONABLE") {
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: p.cur.Str})
			p.advance()
		} else {
			p.advance()
		}
	}

	if len(opts.Items) > 0 {
		stmt.Options = opts
	}
	stmt.Loc.End = p.pos()
	return stmt
}

// parseCreateDirectoryStmt parses a CREATE DIRECTORY statement.
// Called after CREATE [OR REPLACE] [IF NOT EXISTS] DIRECTORY has been consumed.
//
// BNF:
//
//	CREATE [ OR REPLACE | IF NOT EXISTS ] DIRECTORY directory
//	    [ SHARING = { METADATA | NONE } ]
//	    AS 'path_name' ;
func (p *Parser) parseCreateDirectoryStmt(start int, orReplace bool) nodes.StmtNode {
	stmt := &nodes.AdminDDLStmt{
		Action:     "CREATE",
		ObjectType: nodes.OBJECT_DIRECTORY,
		OrReplace:  orReplace,
		Loc:        nodes.Loc{Start: start},
	}
	opts := &nodes.List{}

	stmt.Name = p.parseObjectName()

	// [ SHARING = { METADATA | NONE } ]
	if p.isIdentLike() && p.cur.Str == "SHARING" {
		p.advance()
		if p.cur.Type == '=' {
			p.advance()
		}
		if p.isIdentLike() || p.cur.Type == tokIDENT {
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: "SHARING", Value: p.cur.Str})
			p.advance()
		}
	}

	// AS 'path_name'
	if p.cur.Type == kwAS {
		p.advance()
		if p.cur.Type == tokSCONST {
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: "AS", Value: p.cur.Str})
			p.advance()
		}
	}

	if len(opts.Items) > 0 {
		stmt.Options = opts
	}
	stmt.Loc.End = p.pos()
	return stmt
}

// parseCreateContextStmt parses a CREATE CONTEXT statement.
// Called after CREATE [OR REPLACE] CONTEXT has been consumed.
//
// BNF:
//
//	CREATE [ OR REPLACE ] CONTEXT namespace
//	    USING [ schema. ] package
//	    [ SHARING = { METADATA | NONE } ]
//	    [ INITIALIZED { EXTERNALLY | GLOBALLY } ]
//	    [ ACCESSED GLOBALLY ] ;
func (p *Parser) parseCreateContextStmt(start int, orReplace bool) nodes.StmtNode {
	stmt := &nodes.AdminDDLStmt{
		Action:     "CREATE",
		ObjectType: nodes.OBJECT_CONTEXT,
		OrReplace:  orReplace,
		Loc:        nodes.Loc{Start: start},
	}
	opts := &nodes.List{}

	stmt.Name = p.parseObjectName()

	// USING [ schema. ] package
	if p.isIdentLike() && p.cur.Str == "USING" {
		p.advance()
		usingName := p.parseObjectName()
		if usingName != nil {
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: "USING", Value: usingName.Name})
		}
	}

	// Parse remaining clauses
	for p.cur.Type != ';' && p.cur.Type != tokEOF {
		if p.isIdentLike() && p.cur.Str == "SHARING" {
			p.advance()
			if p.cur.Type == '=' {
				p.advance()
			}
			if p.isIdentLike() || p.cur.Type == tokIDENT {
				opts.Items = append(opts.Items, &nodes.DDLOption{Key: "SHARING", Value: p.cur.Str})
				p.advance()
			}
		} else if p.isIdentLike() && p.cur.Str == "INITIALIZED" {
			p.advance()
			if p.isIdentLike() && (p.cur.Str == "EXTERNALLY" || p.cur.Str == "GLOBALLY") {
				opts.Items = append(opts.Items, &nodes.DDLOption{Key: "INITIALIZED", Value: p.cur.Str})
				p.advance()
			}
		} else if p.isIdentLike() && p.cur.Str == "ACCESSED" {
			p.advance()
			if p.isIdentLike() && p.cur.Str == "GLOBALLY" {
				p.advance()
			}
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: "ACCESSED GLOBALLY"})
		} else {
			p.advance()
		}
	}

	if len(opts.Items) > 0 {
		stmt.Options = opts
	}
	stmt.Loc.End = p.pos()
	return stmt
}

// parseCreateMLEEnvStmt parses a CREATE MLE ENV statement.
// Called after CREATE [OR REPLACE] [IF NOT EXISTS] [PURE] MLE ENV has been consumed.
//
// BNF:
//
//	CREATE [ OR REPLACE | IF NOT EXISTS ] [ PURE ] MLE ENV
//	    [ schema. ] environment_name
//	    [ CLONE [ schema. ] existing_environment ]
//	    [ imports_clause ]
//	    [ language_options_clause ]
//
//	imports_clause: IMPORTS ( import_item [, import_item ]... )
//	import_item: import_name MODULE [ schema. ] module_name
//	language_options_clause: LANGUAGE OPTIONS language_options_string
func (p *Parser) parseCreateMLEEnvStmt(start int, orReplace bool) nodes.StmtNode {
	stmt := &nodes.AdminDDLStmt{
		Action:     "CREATE",
		ObjectType: nodes.OBJECT_MLE_ENV,
		OrReplace:  orReplace,
		Loc:        nodes.Loc{Start: start},
	}
	opts := &nodes.List{}

	stmt.Name = p.parseObjectName()

	// [ CLONE [ schema. ] existing_environment ]
	if p.isIdentLike() && p.cur.Str == "CLONE" {
		p.advance()
		cloneName := p.parseObjectName()
		if cloneName != nil {
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: "CLONE", Value: cloneName.Name})
		}
	}

	// Parse remaining clauses
	for p.cur.Type != ';' && p.cur.Type != tokEOF {
		if p.isIdentLike() && p.cur.Str == "IMPORTS" {
			p.advance()
			if p.cur.Type == '(' {
				p.skipParenthesized()
			}
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: "IMPORTS"})
		} else if p.isIdentLike() && p.cur.Str == "LANGUAGE" {
			p.advance()
			if p.isIdentLike() && p.cur.Str == "OPTIONS" {
				p.advance()
			}
			val := ""
			if p.cur.Type == tokSCONST {
				val = p.cur.Str
				p.advance()
			}
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: "LANGUAGE OPTIONS", Value: val})
		} else {
			p.advance()
		}
	}

	if len(opts.Items) > 0 {
		stmt.Options = opts
	}
	stmt.Loc.End = p.pos()
	return stmt
}

// parseCreateMLEModuleStmt parses a CREATE MLE MODULE statement.
// Called after CREATE [OR REPLACE] [IF NOT EXISTS] MLE MODULE has been consumed.
//
// BNF:
//
//	CREATE [ OR REPLACE | IF NOT EXISTS ] MLE MODULE
//	    [ schema. ] module_name
//	    LANGUAGE JAVASCRIPT
//	    [ VERSION version_string ]
//	    { USING { CLOB ( subquery ) | BLOB ( subquery ) | BFILE ( subquery )
//	            | BFILE ( directory_object_name , server_file_name ) }
//	    | AS source_code }
func (p *Parser) parseCreateMLEModuleStmt(start int, orReplace bool) nodes.StmtNode {
	stmt := &nodes.AdminDDLStmt{
		Action:     "CREATE",
		ObjectType: nodes.OBJECT_MLE_MODULE,
		OrReplace:  orReplace,
		Loc:        nodes.Loc{Start: start},
	}
	opts := &nodes.List{}

	stmt.Name = p.parseObjectName()

	// LANGUAGE JAVASCRIPT
	if p.isIdentLike() && p.cur.Str == "LANGUAGE" {
		p.advance()
		if p.isIdentLike() && p.cur.Str == "JAVASCRIPT" {
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: "LANGUAGE", Value: "JAVASCRIPT"})
			p.advance()
		}
	}

	// [ VERSION version_string ]
	if p.isIdentLike() && p.cur.Str == "VERSION" {
		p.advance()
		if p.cur.Type == tokSCONST {
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: "VERSION", Value: p.cur.Str})
			p.advance()
		}
	}

	// USING or AS
	for p.cur.Type != ';' && p.cur.Type != tokEOF {
		if p.isIdentLike() && p.cur.Str == "USING" {
			p.advance()
			val := ""
			if p.isIdentLike() || p.cur.Type == tokIDENT {
				val = p.cur.Str
				p.advance()
			}
			if p.cur.Type == '(' {
				p.skipParenthesized()
			}
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: "USING", Value: val})
		} else if p.cur.Type == kwAS {
			p.advance()
			if p.cur.Type == tokSCONST {
				opts.Items = append(opts.Items, &nodes.DDLOption{Key: "AS", Value: p.cur.Str})
				p.advance()
			}
		} else {
			p.advance()
		}
	}

	if len(opts.Items) > 0 {
		stmt.Options = opts
	}
	stmt.Loc.End = p.pos()
	return stmt
}

// parseCreatePfileStmt parses a CREATE PFILE statement.
// Called after CREATE PFILE has been consumed.
//
// BNF:
//
//	CREATE PFILE [ = 'pfile_name' ]
//	    FROM { SPFILE [ = 'spfile_name' ] | MEMORY } ;
func (p *Parser) parseCreatePfileStmt(start int) nodes.StmtNode {
	stmt := &nodes.AdminDDLStmt{
		Action:     "CREATE",
		ObjectType: nodes.OBJECT_PFILE,
		Loc:        nodes.Loc{Start: start},
	}
	opts := &nodes.List{}

	// [ = 'pfile_name' ] or [ 'pfile_name' ]
	if p.cur.Type == '=' {
		p.advance()
	}
	if p.cur.Type == tokSCONST {
		opts.Items = append(opts.Items, &nodes.DDLOption{Key: "PFILE", Value: p.cur.Str})
		p.advance()
	}

	// FROM
	if p.cur.Type == kwFROM {
		p.advance()
	}

	// { SPFILE [ = 'spfile_name' ] | MEMORY }
	if p.isIdentLike() && p.cur.Str == "SPFILE" {
		p.advance()
		val := ""
		if p.cur.Type == '=' {
			p.advance()
		}
		if p.cur.Type == tokSCONST {
			val = p.cur.Str
			p.advance()
		}
		opts.Items = append(opts.Items, &nodes.DDLOption{Key: "FROM", Value: "SPFILE"})
		if val != "" {
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: "SPFILE", Value: val})
		}
	} else if p.isIdentLike() && p.cur.Str == "MEMORY" {
		opts.Items = append(opts.Items, &nodes.DDLOption{Key: "FROM", Value: "MEMORY"})
		p.advance()
	}

	if len(opts.Items) > 0 {
		stmt.Options = opts
	}
	stmt.Loc.End = p.pos()
	return stmt
}

// parseCreateSpfileStmt parses a CREATE SPFILE statement.
// Called after CREATE SPFILE has been consumed.
//
// BNF:
//
//	CREATE SPFILE [ = 'spfile_name' ]
//	    FROM { PFILE [ = 'pfile_name' ] [ AS COPY ] | MEMORY } ;
func (p *Parser) parseCreateSpfileStmt(start int) nodes.StmtNode {
	stmt := &nodes.AdminDDLStmt{
		Action:     "CREATE",
		ObjectType: nodes.OBJECT_SPFILE,
		Loc:        nodes.Loc{Start: start},
	}
	opts := &nodes.List{}

	// [ = 'spfile_name' ] or [ 'spfile_name' ]
	if p.cur.Type == '=' {
		p.advance()
	}
	if p.cur.Type == tokSCONST {
		opts.Items = append(opts.Items, &nodes.DDLOption{Key: "SPFILE", Value: p.cur.Str})
		p.advance()
	}

	// FROM
	if p.cur.Type == kwFROM {
		p.advance()
	}

	// { PFILE [ = 'pfile_name' ] [ AS COPY ] | MEMORY }
	if p.isIdentLike() && p.cur.Str == "PFILE" {
		p.advance()
		val := ""
		if p.cur.Type == '=' {
			p.advance()
		}
		if p.cur.Type == tokSCONST {
			val = p.cur.Str
			p.advance()
		}
		opts.Items = append(opts.Items, &nodes.DDLOption{Key: "FROM", Value: "PFILE"})
		if val != "" {
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: "PFILE", Value: val})
		}
		// [ AS COPY ]
		if p.cur.Type == kwAS {
			p.advance()
			if p.isIdentLike() && p.cur.Str == "COPY" {
				p.advance()
				opts.Items = append(opts.Items, &nodes.DDLOption{Key: "AS COPY"})
			}
		}
	} else if p.isIdentLike() && p.cur.Str == "MEMORY" {
		opts.Items = append(opts.Items, &nodes.DDLOption{Key: "FROM", Value: "MEMORY"})
		p.advance()
	}

	if len(opts.Items) > 0 {
		stmt.Options = opts
	}
	stmt.Loc.End = p.pos()
	return stmt
}

// parseCreateFlashbackArchiveStmt parses a CREATE FLASHBACK ARCHIVE statement.
// Called after CREATE FLASHBACK ARCHIVE has been consumed.
//
// BNF:
//
//	CREATE FLASHBACK ARCHIVE [ DEFAULT ] flashback_archive
//	    TABLESPACE tablespace_name
//	    [ flashback_archive_quota ]
//	    [ { NO OPTIMIZE DATA | OPTIMIZE DATA } ]
//	    flashback_archive_retention ;
//
//	flashback_archive_quota: QUOTA integer { K | M | G | T | P | E }
//	flashback_archive_retention: RETENTION integer { DAY | DAYS | MONTH | MONTHS | YEAR | YEARS }
func (p *Parser) parseCreateFlashbackArchiveStmt(start int) nodes.StmtNode {
	stmt := &nodes.AdminDDLStmt{
		Action:     "CREATE",
		ObjectType: nodes.OBJECT_FLASHBACK_ARCHIVE,
		Loc:        nodes.Loc{Start: start},
	}
	opts := &nodes.List{}

	// [ DEFAULT ]
	if p.isIdentLike() && p.cur.Str == "DEFAULT" {
		opts.Items = append(opts.Items, &nodes.DDLOption{Key: "DEFAULT"})
		p.advance()
	}

	stmt.Name = p.parseObjectName()

	// Parse remaining clauses
	for p.cur.Type != ';' && p.cur.Type != tokEOF {
		if p.cur.Type == kwTABLESPACE {
			p.advance()
			if p.isIdentLike() || p.cur.Type == tokIDENT || p.cur.Type == tokQIDENT {
				opts.Items = append(opts.Items, &nodes.DDLOption{Key: "TABLESPACE", Value: p.cur.Str})
				p.advance()
			}
		} else if p.isIdentLike() && p.cur.Str == "QUOTA" {
			p.advance()
			size := p.parseSizeClause()
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: "QUOTA", Value: size})
		} else if p.isIdentLike() && p.cur.Str == "NO" {
			p.advance()
			if p.isIdentLike() && p.cur.Str == "OPTIMIZE" {
				p.advance()
				if p.isIdentLike() && p.cur.Str == "DATA" {
					p.advance()
				}
				opts.Items = append(opts.Items, &nodes.DDLOption{Key: "NO OPTIMIZE DATA"})
			}
		} else if p.isIdentLike() && p.cur.Str == "OPTIMIZE" {
			p.advance()
			if p.isIdentLike() && p.cur.Str == "DATA" {
				p.advance()
			}
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: "OPTIMIZE DATA"})
		} else if p.isIdentLike() && p.cur.Str == "RETENTION" {
			p.advance()
			val := ""
			if p.cur.Type == tokICONST {
				val = p.cur.Str
				p.advance()
			}
			if p.isIdentLike() {
				val += " " + p.cur.Str
				p.advance()
			}
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: "RETENTION", Value: val})
		} else {
			p.advance()
		}
	}

	if len(opts.Items) > 0 {
		stmt.Options = opts
	}
	stmt.Loc.End = p.pos()
	return stmt
}

// parseAlterFlashbackArchiveStmt parses an ALTER FLASHBACK ARCHIVE statement.
// Called after ALTER FLASHBACK ARCHIVE has been consumed.
//
// BNF:
//
//	ALTER FLASHBACK ARCHIVE flashback_archive_name
//	    { SET DEFAULT
//	    | { ADD | MODIFY } TABLESPACE tablespace_name [ flashback_archive_quota ]
//	    | REMOVE TABLESPACE tablespace_name
//	    | MODIFY RETENTION flashback_archive_retention
//	    | PURGE { ALL | BEFORE SCN scn_value | BEFORE TIMESTAMP timestamp_value }
//	    | [ NO ] OPTIMIZE DATA
//	    } ;
func (p *Parser) parseAlterFlashbackArchiveStmt(start int) nodes.StmtNode {
	stmt := &nodes.AdminDDLStmt{
		Action:     "ALTER",
		ObjectType: nodes.OBJECT_FLASHBACK_ARCHIVE,
		Loc:        nodes.Loc{Start: start},
	}
	opts := &nodes.List{}

	stmt.Name = p.parseObjectName()

	// Parse action clause
	for p.cur.Type != ';' && p.cur.Type != tokEOF {
		if p.cur.Type == kwSET {
			p.advance()
			if p.isIdentLike() && p.cur.Str == "DEFAULT" {
				p.advance()
			}
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: "SET DEFAULT"})
		} else if p.isIdentLike() && (p.cur.Str == "ADD" || p.cur.Str == "MODIFY") {
			action := p.cur.Str
			p.advance()
			if p.cur.Type == kwTABLESPACE {
				p.advance()
				tsName := ""
				if p.isIdentLike() || p.cur.Type == tokIDENT || p.cur.Type == tokQIDENT {
					tsName = p.cur.Str
					p.advance()
				}
				opts.Items = append(opts.Items, &nodes.DDLOption{Key: action + " TABLESPACE", Value: tsName})
				// optional quota
				if p.isIdentLike() && p.cur.Str == "QUOTA" {
					p.advance()
					size := p.parseSizeClause()
					opts.Items = append(opts.Items, &nodes.DDLOption{Key: "QUOTA", Value: size})
				}
			} else if p.isIdentLike() && p.cur.Str == "RETENTION" {
				p.advance()
				val := ""
				if p.cur.Type == tokICONST {
					val = p.cur.Str
					p.advance()
				}
				if p.isIdentLike() {
					val += " " + p.cur.Str
					p.advance()
				}
				opts.Items = append(opts.Items, &nodes.DDLOption{Key: "MODIFY RETENTION", Value: val})
			}
		} else if p.isIdentLike() && p.cur.Str == "REMOVE" {
			p.advance()
			if p.cur.Type == kwTABLESPACE {
				p.advance()
			}
			tsName := ""
			if p.isIdentLike() || p.cur.Type == tokIDENT || p.cur.Type == tokQIDENT {
				tsName = p.cur.Str
				p.advance()
			}
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: "REMOVE TABLESPACE", Value: tsName})
		} else if p.isIdentLike() && p.cur.Str == "PURGE" {
			p.advance()
			val := ""
			if p.isIdentLike() && p.cur.Str == "ALL" {
				val = "ALL"
				p.advance()
			} else if p.isIdentLike() && p.cur.Str == "BEFORE" {
				p.advance()
				if p.isIdentLike() && p.cur.Str == "SCN" {
					p.advance()
					val = "BEFORE SCN"
					if p.cur.Type == tokICONST {
						val += " " + p.cur.Str
						p.advance()
					}
				} else if p.isIdentLike() && p.cur.Str == "TIMESTAMP" {
					p.advance()
					val = "BEFORE TIMESTAMP"
					// Skip the timestamp expression
					for p.cur.Type != ';' && p.cur.Type != tokEOF {
						p.advance()
					}
				}
			}
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: "PURGE", Value: val})
		} else if p.isIdentLike() && p.cur.Str == "NO" {
			p.advance()
			if p.isIdentLike() && p.cur.Str == "OPTIMIZE" {
				p.advance()
				if p.isIdentLike() && p.cur.Str == "DATA" {
					p.advance()
				}
				opts.Items = append(opts.Items, &nodes.DDLOption{Key: "NO OPTIMIZE DATA"})
			}
		} else if p.isIdentLike() && p.cur.Str == "OPTIMIZE" {
			p.advance()
			if p.isIdentLike() && p.cur.Str == "DATA" {
				p.advance()
			}
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: "OPTIMIZE DATA"})
		} else {
			p.advance()
		}
	}

	if len(opts.Items) > 0 {
		stmt.Options = opts
	}
	stmt.Loc.End = p.pos()
	return stmt
}

// parseCreateRollbackSegmentStmt parses a CREATE ROLLBACK SEGMENT statement.
// Called after CREATE [PUBLIC] ROLLBACK SEGMENT has been consumed.
//
// BNF:
//
//	CREATE [ PUBLIC ] ROLLBACK SEGMENT rollback_segment
//	    [ TABLESPACE tablespace ]
//	    [ storage_clause ] ;
func (p *Parser) parseCreateRollbackSegmentStmt(start int, public bool) nodes.StmtNode {
	stmt := &nodes.AdminDDLStmt{
		Action:     "CREATE",
		ObjectType: nodes.OBJECT_ROLLBACK_SEGMENT,
		Loc:        nodes.Loc{Start: start},
	}
	opts := &nodes.List{}

	if public {
		opts.Items = append(opts.Items, &nodes.DDLOption{Key: "PUBLIC"})
	}

	stmt.Name = p.parseObjectName()

	// [ TABLESPACE tablespace ]
	if p.cur.Type == kwTABLESPACE {
		p.advance()
		if p.isIdentLike() || p.cur.Type == tokIDENT || p.cur.Type == tokQIDENT {
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: "TABLESPACE", Value: p.cur.Str})
			p.advance()
		}
	}

	// [ storage_clause ] and remaining
	for p.cur.Type != ';' && p.cur.Type != tokEOF {
		if p.isIdentLike() && p.cur.Str == "STORAGE" {
			p.advance()
			if p.cur.Type == '(' {
				p.skipParenthesized()
			}
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: "STORAGE"})
		} else {
			p.advance()
		}
	}

	if len(opts.Items) > 0 {
		stmt.Options = opts
	}
	stmt.Loc.End = p.pos()
	return stmt
}

// parseAlterRollbackSegmentStmt parses an ALTER ROLLBACK SEGMENT statement.
// Called after ALTER ROLLBACK SEGMENT has been consumed.
//
// BNF:
//
//	ALTER ROLLBACK SEGMENT rollback_segment
//	    { ONLINE | OFFLINE | storage_clause | SHRINK [ TO size_clause ] }
func (p *Parser) parseAlterRollbackSegmentStmt(start int) nodes.StmtNode {
	stmt := &nodes.AdminDDLStmt{
		Action:     "ALTER",
		ObjectType: nodes.OBJECT_ROLLBACK_SEGMENT,
		Loc:        nodes.Loc{Start: start},
	}
	opts := &nodes.List{}

	stmt.Name = p.parseObjectName()

	// Parse action
	for p.cur.Type != ';' && p.cur.Type != tokEOF {
		if p.isIdentLike() && p.cur.Str == "ONLINE" {
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: "ONLINE"})
			p.advance()
		} else if p.isIdentLike() && p.cur.Str == "OFFLINE" {
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: "OFFLINE"})
			p.advance()
		} else if p.isIdentLike() && p.cur.Str == "SHRINK" {
			p.advance()
			val := ""
			if p.cur.Type == kwTO {
				p.advance()
				val = p.parseSizeClause()
			}
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: "SHRINK", Value: val})
		} else if p.isIdentLike() && p.cur.Str == "STORAGE" {
			p.advance()
			if p.cur.Type == '(' {
				p.skipParenthesized()
			}
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: "STORAGE"})
		} else {
			p.advance()
		}
	}

	if len(opts.Items) > 0 {
		stmt.Options = opts
	}
	stmt.Loc.End = p.pos()
	return stmt
}

// parseCreateEditionStmt parses a CREATE EDITION statement.
// Called after CREATE [IF NOT EXISTS] EDITION has been consumed.
//
// BNF:
//
//	CREATE EDITION [ IF NOT EXISTS ] edition
//	    [ AS CHILD OF parent_edition ] ;
func (p *Parser) parseCreateEditionStmt(start int) nodes.StmtNode {
	stmt := &nodes.AdminDDLStmt{
		Action:     "CREATE",
		ObjectType: nodes.OBJECT_EDITION,
		Loc:        nodes.Loc{Start: start},
	}
	opts := &nodes.List{}

	// [ IF NOT EXISTS ] — may have been consumed by the caller already,
	// but check here for cases where EDITION is dispatched through parseCreateAdminObject.
	if p.cur.Type == kwIF {
		next := p.peekNext()
		if next.Type == kwNOT {
			p.advance() // IF
			p.advance() // NOT
			if p.cur.Type == kwEXISTS {
				p.advance() // EXISTS
			}
		}
	}

	stmt.Name = p.parseObjectName()

	// [ AS CHILD OF parent_edition ]
	if p.cur.Type == kwAS {
		p.advance()
		if p.isIdentLike() && p.cur.Str == "CHILD" {
			p.advance()
		}
		if p.isIdentLike() && p.cur.Str == "OF" {
			p.advance()
		}
		if p.isIdentLike() || p.cur.Type == tokIDENT || p.cur.Type == tokQIDENT {
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: "AS CHILD OF", Value: p.cur.Str})
			p.advance()
		}
	}

	if len(opts.Items) > 0 {
		stmt.Options = opts
	}
	stmt.Loc.End = p.pos()
	return stmt
}

// parseDropEditionStmt parses a DROP EDITION statement.
// Called after DROP EDITION has been consumed.
//
// BNF:
//
//	DROP EDITION [ IF EXISTS ] edition [ CASCADE ] ;
func (p *Parser) parseDropEditionStmt(start int) *nodes.DropStmt {
	stmt := &nodes.DropStmt{
		ObjectType: nodes.OBJECT_EDITION,
		Names:      &nodes.List{},
		Loc:        nodes.Loc{Start: start},
	}

	// [ IF EXISTS ]
	if p.cur.Type == kwIF {
		next := p.peekNext()
		if next.Type == kwEXISTS {
			p.advance()
			p.advance()
			stmt.IfExists = true
		}
	}

	name := p.parseObjectName()
	if name != nil {
		stmt.Names.Items = append(stmt.Names.Items, name)
	}

	// [ CASCADE ]
	if p.cur.Type == kwCASCADE {
		stmt.Cascade = true
		p.advance()
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseCreateRestorePointStmt parses a CREATE RESTORE POINT statement.
// Called after CREATE [CLEAN] RESTORE POINT has been consumed.
//
// BNF:
//
//	CREATE [ CLEAN ] RESTORE POINT restore_point
//	    [ FOR PLUGGABLE DATABASE pdb_name ]
//	    [ AS OF { TIMESTAMP | SCN } expr ]
//	    [ PRESERVE ]
//	    [ GUARANTEE FLASHBACK DATABASE ] ;
func (p *Parser) parseCreateRestorePointStmt(start int, clean bool) nodes.StmtNode {
	stmt := &nodes.AdminDDLStmt{
		Action:     "CREATE",
		ObjectType: nodes.OBJECT_RESTORE_POINT,
		Loc:        nodes.Loc{Start: start},
	}
	opts := &nodes.List{}

	if clean {
		opts.Items = append(opts.Items, &nodes.DDLOption{Key: "CLEAN"})
	}

	stmt.Name = p.parseObjectName()

	// Parse remaining clauses
	for p.cur.Type != ';' && p.cur.Type != tokEOF {
		if p.isIdentLike() && p.cur.Str == "FOR" {
			p.advance()
			if p.isIdentLike() && p.cur.Str == "PLUGGABLE" {
				p.advance()
			}
			if p.cur.Type == kwDATABASE {
				p.advance()
			}
			if p.isIdentLike() || p.cur.Type == tokIDENT || p.cur.Type == tokQIDENT {
				opts.Items = append(opts.Items, &nodes.DDLOption{Key: "FOR PLUGGABLE DATABASE", Value: p.cur.Str})
				p.advance()
			}
		} else if p.cur.Type == kwAS {
			p.advance()
			if p.isIdentLike() && p.cur.Str == "OF" {
				p.advance()
			}
			if p.isIdentLike() && (p.cur.Str == "TIMESTAMP" || p.cur.Str == "SCN") {
				asOfType := p.cur.Str
				p.advance()
				opts.Items = append(opts.Items, &nodes.DDLOption{Key: "AS OF", Value: asOfType})
				// Skip the expression
				for p.cur.Type != ';' && p.cur.Type != tokEOF &&
					!(p.isIdentLike() && (p.cur.Str == "PRESERVE" || p.cur.Str == "GUARANTEE")) {
					p.advance()
				}
			}
		} else if p.isIdentLike() && p.cur.Str == "PRESERVE" {
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: "PRESERVE"})
			p.advance()
		} else if p.isIdentLike() && p.cur.Str == "GUARANTEE" {
			p.advance()
			if p.cur.Type == kwFLASHBACK {
				p.advance()
			}
			if p.cur.Type == kwDATABASE {
				p.advance()
			}
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: "GUARANTEE FLASHBACK DATABASE"})
		} else {
			p.advance()
		}
	}

	if len(opts.Items) > 0 {
		stmt.Options = opts
	}
	stmt.Loc.End = p.pos()
	return stmt
}

// parseCreateLogicalPartitionTrackingStmt parses a CREATE LOGICAL PARTITION TRACKING statement.
// Called after CREATE LOGICAL PARTITION TRACKING has been consumed.
//
// BNF:
//
//	CREATE LOGICAL PARTITION TRACKING ON [ schema. ] table_name
//	    PARTITION BY { RANGE | INTERVAL } ( column_name )
//	    ( partition_definition [, partition_definition ]... )
//
//	partition_definition: PARTITION partition_name VALUES LESS THAN ( value )
func (p *Parser) parseCreateLogicalPartitionTrackingStmt(start int) nodes.StmtNode {
	stmt := &nodes.AdminDDLStmt{
		Action:     "CREATE",
		ObjectType: nodes.OBJECT_LOGICAL_PARTITION_TRACKING,
		Loc:        nodes.Loc{Start: start},
	}
	opts := &nodes.List{}

	// ON [ schema. ] table_name
	if p.cur.Type == kwON {
		p.advance()
	}
	stmt.Name = p.parseObjectName()

	// PARTITION BY { RANGE | INTERVAL } ( column_name )
	if p.cur.Type == kwPARTITION || (p.isIdentLike() && p.cur.Str == "PARTITION") {
		p.advance()
	}
	if p.cur.Type == kwBY {
		p.advance()
	}
	if p.cur.Type == kwRANGE || (p.isIdentLike() && (p.cur.Str == "RANGE" || p.cur.Str == "INTERVAL")) {
		opts.Items = append(opts.Items, &nodes.DDLOption{Key: "PARTITION BY", Value: p.cur.Str})
		p.advance()
	}
	if p.cur.Type == '(' {
		p.skipParenthesized()
	}

	// ( partition_definition [, ...] )
	if p.cur.Type == '(' {
		p.skipParenthesized()
		opts.Items = append(opts.Items, &nodes.DDLOption{Key: "PARTITIONS"})
	}

	// Skip remaining
	for p.cur.Type != ';' && p.cur.Type != tokEOF {
		p.advance()
	}

	if len(opts.Items) > 0 {
		stmt.Options = opts
	}
	stmt.Loc.End = p.pos()
	return stmt
}

// parseCreatePmemFilestoreStmt parses a CREATE PMEM FILESTORE statement.
// Called after CREATE PMEM FILESTORE has been consumed.
//
// BNF:
//
//	CREATE PMEM FILESTORE filestore_name
//	    MOUNTPOINT 'file_path'
//	    BACKINGFILE 'backing_file_path' SIZE size_value BLOCKSIZE blocksize_value
//	    [ AUTOEXTEND { ON | OFF } [ NEXT size_value ] [ MAXSIZE { UNLIMITED | size_value } ] ] ;
func (p *Parser) parseCreatePmemFilestoreStmt(start int) nodes.StmtNode {
	stmt := &nodes.AdminDDLStmt{
		Action:     "CREATE",
		ObjectType: nodes.OBJECT_PMEM_FILESTORE,
		Loc:        nodes.Loc{Start: start},
	}
	opts := &nodes.List{}

	stmt.Name = p.parseObjectName()

	// Parse clauses
	for p.cur.Type != ';' && p.cur.Type != tokEOF {
		if p.isIdentLike() && p.cur.Str == "MOUNTPOINT" {
			p.advance()
			if p.cur.Type == tokSCONST {
				opts.Items = append(opts.Items, &nodes.DDLOption{Key: "MOUNTPOINT", Value: p.cur.Str})
				p.advance()
			}
		} else if p.isIdentLike() && p.cur.Str == "BACKINGFILE" {
			p.advance()
			if p.cur.Type == tokSCONST {
				opts.Items = append(opts.Items, &nodes.DDLOption{Key: "BACKINGFILE", Value: p.cur.Str})
				p.advance()
			}
		} else if p.isIdentLike() && p.cur.Str == "SIZE" {
			p.advance()
			size := p.parseSizeClause()
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: "SIZE", Value: size})
		} else if p.isIdentLike() && p.cur.Str == "BLOCKSIZE" {
			p.advance()
			size := p.parseSizeClause()
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: "BLOCKSIZE", Value: size})
		} else if p.isIdentLike() && p.cur.Str == "AUTOEXTEND" {
			p.advance()
			val := ""
			if p.cur.Type == kwON || (p.isIdentLike() && p.cur.Str == "ON") {
				val = "ON"
				p.advance()
			} else if p.isIdentLike() && p.cur.Str == "OFF" {
				val = "OFF"
				p.advance()
			}
			opts.Items = append(opts.Items, &nodes.DDLOption{Key: "AUTOEXTEND", Value: val})
			// [ NEXT size_value ]
			if p.isIdentLike() && p.cur.Str == "NEXT" {
				p.advance()
				nextSize := p.parseSizeClause()
				opts.Items = append(opts.Items, &nodes.DDLOption{Key: "NEXT", Value: nextSize})
			}
			// [ MAXSIZE { UNLIMITED | size_value } ]
			if p.isIdentLike() && p.cur.Str == "MAXSIZE" {
				p.advance()
				maxVal := ""
				if p.isIdentLike() && p.cur.Str == "UNLIMITED" {
					maxVal = "UNLIMITED"
					p.advance()
				} else {
					maxVal = p.parseSizeClause()
				}
				opts.Items = append(opts.Items, &nodes.DDLOption{Key: "MAXSIZE", Value: maxVal})
			}
		} else {
			p.advance()
		}
	}

	if len(opts.Items) > 0 {
		stmt.Options = opts
	}
	stmt.Loc.End = p.pos()
	return stmt
}

// parseDropJavaStmt parses a DROP JAVA statement.
// Called after DROP JAVA has been consumed.
//
// BNF:
//
//	DROP JAVA { SOURCE | CLASS | RESOURCE } [ IF EXISTS ] [ schema. ] object_name ;
func (p *Parser) parseDropJavaStmt(start int) *nodes.DropStmt {
	stmt := &nodes.DropStmt{
		ObjectType: nodes.OBJECT_JAVA,
		Names:      &nodes.List{},
		Loc:        nodes.Loc{Start: start},
	}

	// { SOURCE | CLASS | RESOURCE }
	if p.isIdentLike() && (p.cur.Str == "SOURCE" || p.cur.Str == "CLASS" || p.cur.Str == "RESOURCE") {
		p.advance()
	}

	// [ IF EXISTS ]
	if p.cur.Type == kwIF {
		next := p.peekNext()
		if next.Type == kwEXISTS {
			p.advance()
			p.advance()
			stmt.IfExists = true
		}
	}

	name := p.parseObjectName()
	if name != nil {
		stmt.Names.Items = append(stmt.Names.Items, name)
	}

	stmt.Loc.End = p.pos()
	return stmt
}
