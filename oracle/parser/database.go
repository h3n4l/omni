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
	if p.isIdentLike() || p.cur.Type == tokQIDENT {
		stmt.Name = p.parseObjectName()
	}

	// Skip remaining tokens until ;/EOF
	for p.cur.Type != ';' && p.cur.Type != tokEOF {
		p.advance()
	}

	stmt.Loc.End = p.pos()
	return stmt
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
