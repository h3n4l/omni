package parser

import (
	nodes "github.com/bytebase/omni/oracle/ast"
)

// parseCreateDatabaseStmt parses a CREATE DATABASE statement.
// The CREATE keyword has already been consumed, and DATABASE is the current token.
//
// Ref: https://docs.oracle.com/en/database/oracle/oracle-database/23/sqlrf/CREATE-DATABASE.html
//
//	CREATE DATABASE [ database_name ]
//	    [ USER SYS IDENTIFIED BY password ]
//	    [ USER SYSTEM IDENTIFIED BY password ]
//	    [ CONTROLFILE REUSE ]
//	    [ LOGFILE logfile_clause [, ...] ]
//	    [ MAXLOGFILES integer ]
//	    [ MAXLOGMEMBERS integer ]
//	    [ MAXLOGHISTORY integer ]
//	    [ MAXDATAFILES integer ]
//	    [ MAXINSTANCES integer ]
//	    [ { ARCHIVELOG | NOARCHIVELOG } ]
//	    [ FORCE LOGGING ]
//	    [ SET STANDBY NOLOGGING FOR { DATA AVAILABILITY | LOAD PERFORMANCE } ]
//	    [ CHARACTER SET charset ]
//	    [ NATIONAL CHARACTER SET charset ]
//	    [ SET DEFAULT { BIGFILE | SMALLFILE } TABLESPACE ]
//	    [ database_logging_clauses ]
//	    [ tablespace_clauses ]
//	    [ set_time_zone_clause ]
//	    [ [ BIGFILE | SMALLFILE ]
//	        DEFAULT TABLESPACE tablespace
//	        DATAFILE datafile_tempfile_spec
//	      | DEFAULT TEMPORARY TABLESPACE tablespace
//	        TEMPFILE datafile_tempfile_spec
//	      | UNDO TABLESPACE tablespace
//	        DATAFILE datafile_tempfile_spec ]
//	    [ enable_pluggable_database ]
func (p *Parser) parseCreateDatabaseStmt(start int) nodes.StmtNode {
	// DATABASE keyword already checked by caller but not consumed
	p.advance() // consume DATABASE

	stmt := &nodes.AdminDDLStmt{
		Action:     "CREATE",
		ObjectType: nodes.OBJECT_DATABASE,
		Loc:        nodes.Loc{Start: start},
	}

	// Optional database name — peek at the next token to see if it's an identifier
	// (not a keyword that starts a clause like USER, LOGFILE, etc.)
	if (p.isIdentLike() || p.cur.Type == tokQIDENT) && !p.isCreateDatabaseClauseKeyword() {
		stmt.Name = p.parseObjectName()
	}

	// Parse CREATE DATABASE clauses
	opts := &nodes.List{}
	for p.cur.Type != ';' && p.cur.Type != tokEOF {
		optStart := p.pos()

		switch {
		// USER SYS IDENTIFIED BY password / USER SYSTEM IDENTIFIED BY password
		case p.cur.Type == kwUSER:
			p.advance() // consume USER
			userType := ""
			if p.isIdentLike() {
				userType = p.cur.Str // SYS or SYSTEM
				p.advance()
			}
			if p.cur.Type == kwIDENTIFIED {
				p.advance() // IDENTIFIED
				if p.isIdentLike() && p.cur.Str == "BY" {
					p.advance() // BY
				}
			}
			password := ""
			if p.isIdentLike() || p.cur.Type == tokSCONST {
				password = p.cur.Str
				p.advance()
			}
			opts.Items = append(opts.Items, &nodes.DDLOption{
				Key: "USER_" + userType, Value: password,
				Loc: nodes.Loc{Start: optStart, End: p.pos()},
			})

		// CONTROLFILE REUSE
		case p.isIdentLike() && p.cur.Str == "CONTROLFILE":
			p.advance() // CONTROLFILE
			if p.isIdentLike() && p.cur.Str == "REUSE" {
				p.advance()
			}
			opts.Items = append(opts.Items, &nodes.DDLOption{
				Key: "CONTROLFILE_REUSE",
				Loc: nodes.Loc{Start: optStart, End: p.pos()},
			})

		// LOGFILE GROUP n 'file' SIZE ... [, GROUP n 'file' SIZE ...]
		case p.isIdentLike() && p.cur.Str == "LOGFILE":
			p.advance() // LOGFILE
			logItems := &nodes.List{}
			for {
				lfStart := p.pos()
				groupNum := ""
				if p.cur.Type == kwGROUP {
					p.advance() // GROUP
					if p.cur.Type == tokICONST {
						groupNum = p.cur.Str
						p.advance()
					}
				}
				// Parse file specs
				var files []*nodes.DatafileClause
				for p.cur.Type == tokSCONST {
					df := p.parseDatafileClause()
					if df != nil {
						files = append(files, df)
					}
					if p.cur.Type == ',' && (p.isIdentLike() || p.cur.Type == tokSCONST) {
						// Could be next file or next GROUP — peek
						break
					}
				}
				fileList := &nodes.List{}
				for _, f := range files {
					fileList.Items = append(fileList.Items, f)
				}
				logItems.Items = append(logItems.Items, &nodes.DDLOption{
					Key: "LOGFILE_GROUP", Value: groupNum,
					Items: fileList,
					Loc:   nodes.Loc{Start: lfStart, End: p.pos()},
				})
				// Check for comma-separated groups
				if p.cur.Type == ',' {
					p.advance()
					continue
				}
				break
			}
			opts.Items = append(opts.Items, &nodes.DDLOption{
				Key: "LOGFILE", Items: logItems,
				Loc: nodes.Loc{Start: optStart, End: p.pos()},
			})

		// MAXLOGFILES integer
		case p.isIdentLike() && p.cur.Str == "MAXLOGFILES":
			p.advance()
			val := ""
			if p.cur.Type == tokICONST {
				val = p.cur.Str
				p.advance()
			}
			opts.Items = append(opts.Items, &nodes.DDLOption{
				Key: "MAXLOGFILES", Value: val,
				Loc: nodes.Loc{Start: optStart, End: p.pos()},
			})

		// MAXLOGMEMBERS integer
		case p.isIdentLike() && p.cur.Str == "MAXLOGMEMBERS":
			p.advance()
			val := ""
			if p.cur.Type == tokICONST {
				val = p.cur.Str
				p.advance()
			}
			opts.Items = append(opts.Items, &nodes.DDLOption{
				Key: "MAXLOGMEMBERS", Value: val,
				Loc: nodes.Loc{Start: optStart, End: p.pos()},
			})

		// MAXLOGHISTORY integer
		case p.isIdentLike() && p.cur.Str == "MAXLOGHISTORY":
			p.advance()
			val := ""
			if p.cur.Type == tokICONST {
				val = p.cur.Str
				p.advance()
			}
			opts.Items = append(opts.Items, &nodes.DDLOption{
				Key: "MAXLOGHISTORY", Value: val,
				Loc: nodes.Loc{Start: optStart, End: p.pos()},
			})

		// MAXDATAFILES integer
		case p.isIdentLike() && p.cur.Str == "MAXDATAFILES":
			p.advance()
			val := ""
			if p.cur.Type == tokICONST {
				val = p.cur.Str
				p.advance()
			}
			opts.Items = append(opts.Items, &nodes.DDLOption{
				Key: "MAXDATAFILES", Value: val,
				Loc: nodes.Loc{Start: optStart, End: p.pos()},
			})

		// MAXINSTANCES integer
		case p.isIdentLike() && p.cur.Str == "MAXINSTANCES":
			p.advance()
			val := ""
			if p.cur.Type == tokICONST {
				val = p.cur.Str
				p.advance()
			}
			opts.Items = append(opts.Items, &nodes.DDLOption{
				Key: "MAXINSTANCES", Value: val,
				Loc: nodes.Loc{Start: optStart, End: p.pos()},
			})

		// ARCHIVELOG
		case p.isIdentLike() && p.cur.Str == "ARCHIVELOG":
			p.advance()
			opts.Items = append(opts.Items, &nodes.DDLOption{
				Key: "ARCHIVELOG",
				Loc: nodes.Loc{Start: optStart, End: p.pos()},
			})

		// NOARCHIVELOG
		case p.isIdentLike() && p.cur.Str == "NOARCHIVELOG":
			p.advance()
			opts.Items = append(opts.Items, &nodes.DDLOption{
				Key: "NOARCHIVELOG",
				Loc: nodes.Loc{Start: optStart, End: p.pos()},
			})

		// FORCE LOGGING
		case p.cur.Type == kwFORCE:
			p.advance() // FORCE
			if p.cur.Type == kwLOGGING {
				p.advance()
			}
			opts.Items = append(opts.Items, &nodes.DDLOption{
				Key: "FORCE_LOGGING",
				Loc: nodes.Loc{Start: optStart, End: p.pos()},
			})

		// SET STANDBY NOLOGGING FOR ... / SET DEFAULT ... TABLESPACE / SET TIME_ZONE ...
		case p.cur.Type == kwSET:
			p.advance() // SET
			if p.isIdentLike() && p.cur.Str == "STANDBY" {
				// SET STANDBY NOLOGGING FOR { DATA AVAILABILITY | LOAD PERFORMANCE }
				p.advance() // STANDBY
				if p.cur.Type == kwNOLOGGING {
					p.advance() // NOLOGGING
				}
				if p.cur.Type == kwFOR {
					p.advance() // FOR
				}
				val := ""
				if p.isIdentLike() && p.cur.Str == "DATA" {
					p.advance()
					if p.isIdentLike() && p.cur.Str == "AVAILABILITY" {
						p.advance()
					}
					val = "DATA AVAILABILITY"
				} else if p.isIdentLike() && p.cur.Str == "LOAD" {
					p.advance()
					if p.isIdentLike() && p.cur.Str == "PERFORMANCE" {
						p.advance()
					}
					val = "LOAD PERFORMANCE"
				}
				opts.Items = append(opts.Items, &nodes.DDLOption{
					Key: "SET_STANDBY_NOLOGGING", Value: val,
					Loc: nodes.Loc{Start: optStart, End: p.pos()},
				})
			} else if p.cur.Type == kwDEFAULT {
				// SET DEFAULT { BIGFILE | SMALLFILE } TABLESPACE
				p.advance() // DEFAULT
				val := ""
				if p.isIdentLike() && (p.cur.Str == "BIGFILE" || p.cur.Str == "SMALLFILE") {
					val = p.cur.Str
					p.advance()
				}
				if p.isIdentLike() && p.cur.Str == "TABLESPACE" {
					p.advance()
				}
				opts.Items = append(opts.Items, &nodes.DDLOption{
					Key: "SET_DEFAULT_TABLESPACE_TYPE", Value: val,
					Loc: nodes.Loc{Start: optStart, End: p.pos()},
				})
			} else if p.isIdentLike() && p.cur.Str == "TIME_ZONE" {
				// SET TIME_ZONE = 'value'
				p.advance() // TIME_ZONE
				if p.cur.Type == '=' {
					p.advance()
				}
				val := ""
				if p.cur.Type == tokSCONST {
					val = p.cur.Str
					p.advance()
				}
				opts.Items = append(opts.Items, &nodes.DDLOption{
					Key: "SET_TIME_ZONE", Value: val,
					Loc: nodes.Loc{Start: optStart, End: p.pos()},
				})
			} else {
				// Unknown SET clause, skip token
				p.advance()
			}

		// CHARACTER SET charset
		case p.isIdentLike() && p.cur.Str == "CHARACTER":
			p.advance() // CHARACTER
			if p.cur.Type == kwSET {
				p.advance() // SET
			}
			val := ""
			if p.isIdentLike() || p.cur.Type == tokQIDENT {
				val = p.cur.Str
				p.advance()
			}
			opts.Items = append(opts.Items, &nodes.DDLOption{
				Key: "CHARACTER_SET", Value: val,
				Loc: nodes.Loc{Start: optStart, End: p.pos()},
			})

		// NATIONAL CHARACTER SET charset
		case p.isIdentLike() && p.cur.Str == "NATIONAL":
			p.advance() // NATIONAL
			if p.isIdentLike() && p.cur.Str == "CHARACTER" {
				p.advance()
			}
			if p.cur.Type == kwSET {
				p.advance()
			}
			val := ""
			if p.isIdentLike() || p.cur.Type == tokQIDENT {
				val = p.cur.Str
				p.advance()
			}
			opts.Items = append(opts.Items, &nodes.DDLOption{
				Key: "NATIONAL_CHARACTER_SET", Value: val,
				Loc: nodes.Loc{Start: optStart, End: p.pos()},
			})

		// DATAFILE 'file' SIZE ... (standalone datafile spec, not part of tablespace)
		case p.isIdentLike() && p.cur.Str == "DATAFILE":
			p.advance() // DATAFILE
			fileList := &nodes.List{}
			for p.cur.Type == tokSCONST {
				df := p.parseDatafileClause()
				if df != nil {
					fileList.Items = append(fileList.Items, df)
				}
				if p.cur.Type == ',' {
					p.advance()
					continue
				}
				break
			}
			opts.Items = append(opts.Items, &nodes.DDLOption{
				Key: "DATAFILE", Items: fileList,
				Loc: nodes.Loc{Start: optStart, End: p.pos()},
			})

		// BIGFILE / SMALLFILE DEFAULT TABLESPACE
		case p.isIdentLike() && (p.cur.Str == "BIGFILE" || p.cur.Str == "SMALLFILE"):
			fileType := p.cur.Str
			p.advance()
			if p.cur.Type == kwDEFAULT {
				// BIGFILE/SMALLFILE DEFAULT TABLESPACE ...
				p.advance() // DEFAULT
				if p.isIdentLike() && p.cur.Str == "TABLESPACE" {
					p.advance() // TABLESPACE
				}
				tsName := ""
				if p.isIdentLike() || p.cur.Type == tokQIDENT {
					tsName = p.cur.Str
					p.advance()
				}
				// Parse DATAFILE spec
				fileList := &nodes.List{}
				if p.isIdentLike() && p.cur.Str == "DATAFILE" {
					p.advance()
					for p.cur.Type == tokSCONST {
						df := p.parseDatafileClause()
						if df != nil {
							fileList.Items = append(fileList.Items, df)
						}
						if p.cur.Type == ',' {
							p.advance()
							continue
						}
						break
					}
				}
				opts.Items = append(opts.Items, &nodes.DDLOption{
					Key: "DEFAULT_TABLESPACE", Value: fileType + " " + tsName,
					Items: fileList,
					Loc:   nodes.Loc{Start: optStart, End: p.pos()},
				})
			} else {
				// Just BIGFILE/SMALLFILE without DEFAULT — skip
				opts.Items = append(opts.Items, &nodes.DDLOption{
					Key: "FILE_TYPE", Value: fileType,
					Loc: nodes.Loc{Start: optStart, End: p.pos()},
				})
			}

		// DEFAULT TABLESPACE / DEFAULT TEMPORARY TABLESPACE
		case p.cur.Type == kwDEFAULT:
			p.advance() // DEFAULT
			if p.cur.Type == kwTEMPORARY {
				// DEFAULT TEMPORARY TABLESPACE name TEMPFILE ...
				p.advance() // TEMPORARY
				if p.isIdentLike() && p.cur.Str == "TABLESPACE" {
					p.advance() // TABLESPACE
				}
				tsName := ""
				if p.isIdentLike() || p.cur.Type == tokQIDENT {
					tsName = p.cur.Str
					p.advance()
				}
				fileList := &nodes.List{}
				if p.isIdentLike() && p.cur.Str == "TEMPFILE" {
					p.advance()
					for p.cur.Type == tokSCONST {
						df := p.parseDatafileClause()
						if df != nil {
							fileList.Items = append(fileList.Items, df)
						}
						if p.cur.Type == ',' {
							p.advance()
							continue
						}
						break
					}
				}
				opts.Items = append(opts.Items, &nodes.DDLOption{
					Key: "DEFAULT_TEMPORARY_TABLESPACE", Value: tsName,
					Items: fileList,
					Loc:   nodes.Loc{Start: optStart, End: p.pos()},
				})
			} else if p.isIdentLike() && p.cur.Str == "TABLESPACE" {
				// DEFAULT TABLESPACE name DATAFILE ...
				p.advance() // TABLESPACE
				tsName := ""
				if p.isIdentLike() || p.cur.Type == tokQIDENT {
					tsName = p.cur.Str
					p.advance()
				}
				fileList := &nodes.List{}
				if p.isIdentLike() && p.cur.Str == "DATAFILE" {
					p.advance()
					for p.cur.Type == tokSCONST {
						df := p.parseDatafileClause()
						if df != nil {
							fileList.Items = append(fileList.Items, df)
						}
						if p.cur.Type == ',' {
							p.advance()
							continue
						}
						break
					}
				}
				opts.Items = append(opts.Items, &nodes.DDLOption{
					Key: "DEFAULT_TABLESPACE", Value: tsName,
					Items: fileList,
					Loc:   nodes.Loc{Start: optStart, End: p.pos()},
				})
			} else {
				// Unknown DEFAULT clause
				p.advance()
			}

		// UNDO TABLESPACE name DATAFILE ...
		case p.isIdentLike() && p.cur.Str == "UNDO":
			p.advance() // UNDO
			if p.isIdentLike() && p.cur.Str == "TABLESPACE" {
				p.advance() // TABLESPACE
			}
			tsName := ""
			if p.isIdentLike() || p.cur.Type == tokQIDENT {
				tsName = p.cur.Str
				p.advance()
			}
			fileList := &nodes.List{}
			if p.isIdentLike() && p.cur.Str == "DATAFILE" {
				p.advance()
				for p.cur.Type == tokSCONST {
					df := p.parseDatafileClause()
					if df != nil {
						fileList.Items = append(fileList.Items, df)
					}
					if p.cur.Type == ',' {
						p.advance()
						continue
					}
					break
				}
			}
			opts.Items = append(opts.Items, &nodes.DDLOption{
				Key: "UNDO_TABLESPACE", Value: tsName,
				Items: fileList,
				Loc:   nodes.Loc{Start: optStart, End: p.pos()},
			})

		// ENABLE PLUGGABLE DATABASE
		case p.cur.Type == kwENABLE:
			p.advance() // ENABLE
			if p.isIdentLike() && p.cur.Str == "PLUGGABLE" {
				p.advance() // PLUGGABLE
				if p.cur.Type == kwDATABASE {
					p.advance() // DATABASE
				}
			}
			opts.Items = append(opts.Items, &nodes.DDLOption{
				Key: "ENABLE_PLUGGABLE_DATABASE",
				Loc: nodes.Loc{Start: optStart, End: p.pos()},
			})

		default:
			// Unknown token, advance to avoid infinite loop
			p.advance()
		}
	}

	if len(opts.Items) > 0 {
		stmt.Options = opts
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// isCreateDatabaseClauseKeyword returns true if the current token starts
// a CREATE DATABASE clause (and is not a database name).
func (p *Parser) isCreateDatabaseClauseKeyword() bool {
	switch p.cur.Type {
	case kwUSER, kwFORCE, kwSET, kwDEFAULT, kwENABLE:
		return true
	}
	if p.isIdentLike() {
		switch p.cur.Str {
		case "CONTROLFILE", "LOGFILE", "DATAFILE", "TEMPFILE",
			"MAXLOGFILES", "MAXLOGMEMBERS", "MAXLOGHISTORY",
			"MAXDATAFILES", "MAXINSTANCES",
			"ARCHIVELOG", "NOARCHIVELOG",
			"CHARACTER", "NATIONAL",
			"BIGFILE", "SMALLFILE",
			"UNDO", "PLUGGABLE":
			return true
		}
	}
	return false
}

// parseCreateControlfileStmt parses a CREATE CONTROLFILE statement.
// The CREATE keyword has already been consumed.
//
// Ref: https://docs.oracle.com/en/database/oracle/oracle-database/23/sqlrf/CREATE-CONTROLFILE.html
//
//	CREATE CONTROLFILE [ REUSE ]
//	    [ SET ] DATABASE database_name
//	    logfile_clause
//	    { RESETLOGS | NORESETLOGS }
//	    [ DATAFILE file_specification [, ...] ]
//	    [ MAXLOGFILES integer ]
//	    [ MAXLOGMEMBERS integer ]
//	    [ MAXLOGHISTORY integer ]
//	    [ MAXDATAFILES integer ]
//	    [ MAXINSTANCES integer ]
//	    [ { ARCHIVELOG | NOARCHIVELOG } ]
//	    [ FORCE LOGGING ]
//	    [ SET STANDBY NOLOGGING FOR { DATA AVAILABILITY | LOAD PERFORMANCE } ]
//	    [ character_set_clause ]
func (p *Parser) parseCreateControlfileStmt(start int) nodes.StmtNode {
	// "CONTROLFILE" identifier already consumed by caller

	stmt := &nodes.AdminDDLStmt{
		Action:     "CREATE",
		ObjectType: nodes.OBJECT_CONTROLFILE,
		Loc:        nodes.Loc{Start: start},
	}

	// Skip remaining tokens until ;/EOF — the CONTROLFILE syntax includes
	// DATABASE name, LOGFILE, DATAFILE, etc. which are consumed by skip.
	for p.cur.Type != ';' && p.cur.Type != tokEOF {
		p.advance()
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseAlterDatabaseStmt parses an ALTER DATABASE statement.
// The ALTER keyword has already been consumed, and DATABASE is the current token.
//
// Ref: https://docs.oracle.com/en/database/oracle/oracle-database/23/sqlrf/ALTER-DATABASE.html
//
//	ALTER DATABASE [ database_name ]
//	    { startup_clauses
//	    | recovery_clauses
//	    | database_file_clauses
//	    | logfile_clauses
//	    | controlfile_clauses
//	    | standby_database_clauses
//	    | default_settings_clauses
//	    | instance_clauses
//	    | security_clause
//	    | prepare_clause
//	    | drop_mirror_copy
//	    | lost_write_protection
//	    | cdb_fleet_clauses
//	    | property_clauses }
//
//	startup_clauses:
//	    MOUNT [ STANDBY | CLONE DATABASE ]
//	  | OPEN { [ READ WRITE ] [ RESETLOGS | NORESETLOGS ] [ UPGRADE | DOWNGRADE ]
//	           | READ ONLY }
//
//	recovery_clauses:
//	    { general_recovery | managed_standby_recovery }
//
//	database_file_clauses:
//	    RENAME FILE 'filename' [, ...] TO 'filename' [, ...]
//	  | create_datafile_clause
//	  | alter_datafile_clause
//	  | alter_tempfile_clause
//	  | move_datafile_clause
//
//	logfile_clauses:
//	    { ADD [ STANDBY ] LOGFILE
//	    | DROP [ STANDBY ] LOGFILE
//	    | ADD [ STANDBY ] LOGFILE MEMBER
//	    | DROP [ STANDBY ] LOGFILE MEMBER
//	    | CLEAR [ UNARCHIVED ] LOGFILE
//	    | switch_logfile_clause }
//
//	controlfile_clauses:
//	    BACKUP CONTROLFILE TO { 'filename' [ REUSE ] | TRACE [ AS 'filename' [ REUSE ] ] [ RESETLOGS | NORESETLOGS ] }
//
//	standby_database_clauses:
//	    { ACTIVATE [ PHYSICAL | LOGICAL ] STANDBY DATABASE [ FINISH APPLY ]
//	    | SET STANDBY DATABASE TO MAXIMIZE { PROTECTION | AVAILABILITY | PERFORMANCE }
//	    | register_logfile_clause
//	    | commit_switchover_clause
//	    | start_standby_clause
//	    | stop_standby_clause
//	    | convert_database_clause }
//
//	default_settings_clauses:
//	    { SET DEFAULT { BIGFILE | SMALLFILE } TABLESPACE
//	    | DEFAULT TABLESPACE tablespace
//	    | DEFAULT TEMPORARY TABLESPACE { tablespace | tablespace_group_name }
//	    | RENAME GLOBAL_NAME TO database.domain [.domain ...]
//	    | ENABLE BLOCK CHANGE TRACKING [ USING FILE 'filename' [ REUSE ] ]
//	    | DISABLE BLOCK CHANGE TRACKING
//	    | set_time_zone_clause
//	    | FLASHBACK { ON | OFF } }
//
//	security_clause:
//	    GUARD { ALL | STANDBY | NONE }
func (p *Parser) parseAlterDatabaseStmt(start int) nodes.StmtNode {
	// DATABASE already consumed by caller
	stmt := &nodes.AdminDDLStmt{
		Action:     "ALTER",
		ObjectType: nodes.OBJECT_DATABASE,
		Loc:        nodes.Loc{Start: start},
	}

	// Optional database name — if next token is an identifier (not a keyword clause starter)
	if p.isIdentLike() || p.cur.Type == tokQIDENT {
		// Check this isn't a clause keyword like MOUNT, OPEN, etc.
		if !p.isDatabaseClauseKeyword() {
			stmt.Name = p.parseObjectName()
		}
	}

	// Skip remaining tokens until ;/EOF
	for p.cur.Type != ';' && p.cur.Type != tokEOF {
		p.advance()
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseAlterDatabaseDictionaryStmt parses an ALTER DATABASE DICTIONARY statement.
// The ALTER keyword has already been consumed, DATABASE and DICTIONARY are consumed by caller.
//
// Ref: https://docs.oracle.com/en/database/oracle/oracle-database/23/sqlrf/ALTER-DATABASE-DICTIONARY.html
//
//	ALTER DATABASE DICTIONARY
//	    { ENCRYPT CREDENTIALS
//	    | REKEY CREDENTIALS
//	    | DELETE CREDENTIALS KEY }
func (p *Parser) parseAlterDatabaseDictionaryStmt(start int) nodes.StmtNode {
	// DATABASE and DICTIONARY already consumed by caller
	stmt := &nodes.AdminDDLStmt{
		Action:     "ALTER",
		ObjectType: nodes.OBJECT_DATABASE_DICTIONARY,
		Loc:        nodes.Loc{Start: start},
	}

	// Skip remaining tokens until ;/EOF
	for p.cur.Type != ';' && p.cur.Type != tokEOF {
		p.advance()
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// isDatabaseClauseKeyword returns true if the current token is a keyword
// that starts an ALTER DATABASE clause (not a database name).
func (p *Parser) isDatabaseClauseKeyword() bool {
	switch p.cur.Type {
	case kwOPEN, kwFORCE, kwSET, kwDEFAULT, kwADD, kwDROP,
		kwENABLE, kwDISABLE, kwRENAME, kwCREATE,
		kwFLASHBACK, kwNOLOGGING, kwLOGGING:
		return true
	}
	if p.isIdentLike() {
		switch p.cur.Str {
		case "MOUNT", "RECOVER", "RESETLOGS", "NORESETLOGS",
			"ARCHIVELOG", "NOARCHIVELOG", "ACTIVATE", "GUARD",
			"STANDBY", "CLEAR", "CONVERT", "DISMOUNT", "PREPARE",
			"BACKUP":
			return true
		}
	}
	return false
}
