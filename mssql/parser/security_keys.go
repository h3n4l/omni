// Package parser - security_keys.go implements parsing for security key/cert/credential statements.
package parser

import (
	"strings"

	nodes "github.com/bytebase/omni/mssql/ast"
)

// parseSecurityKeyStmt parses CREATE/ALTER/DROP statements for security objects:
// MASTER KEY, SYMMETRIC KEY, ASYMMETRIC KEY, CERTIFICATE, CREDENTIAL,
// COLUMN ENCRYPTION KEY, COLUMN MASTER KEY, CRYPTOGRAPHIC PROVIDER.
// Also handles OPEN/CLOSE SYMMETRIC KEY and BACKUP CERTIFICATE.
//
// These statements are parsed generically: we capture the action, object type,
// name, and skip remaining tokens as options.
func (p *Parser) parseSecurityKeyStmt(action string) *nodes.SecurityKeyStmt {
	loc := p.pos()
	stmt := &nodes.SecurityKeyStmt{
		Action: action,
		Loc:    nodes.Loc{Start: loc},
	}

	// Determine object type
	switch {
	case p.matchIdentCI("MASTER"):
		// MASTER KEY
		p.match(kwKEY)
		stmt.ObjectType = "MASTER KEY"
	case p.matchIdentCI("SYMMETRIC"):
		// SYMMETRIC KEY key_name
		p.match(kwKEY)
		stmt.ObjectType = "SYMMETRIC KEY"
		name, _ := p.parseIdentifier()
		stmt.Name = name
	case p.matchIdentCI("ASYMMETRIC"):
		// ASYMMETRIC KEY key_name
		p.match(kwKEY)
		stmt.ObjectType = "ASYMMETRIC KEY"
		name, _ := p.parseIdentifier()
		stmt.Name = name
	case p.matchIdentCI("CERTIFICATE"):
		stmt.ObjectType = "CERTIFICATE"
		name, _ := p.parseIdentifier()
		stmt.Name = name
	case p.matchIdentCI("CREDENTIAL"):
		stmt.ObjectType = "CREDENTIAL"
		name, _ := p.parseIdentifier()
		stmt.Name = name
	case p.matchIdentCI("CRYPTOGRAPHIC"):
		// CRYPTOGRAPHIC PROVIDER provider_name
		p.matchIdentCI("PROVIDER")
		stmt.ObjectType = "CRYPTOGRAPHIC PROVIDER"
		name, _ := p.parseIdentifier()
		stmt.Name = name
	default:
		stmt.Loc.End = p.pos()
		return stmt
	}

	// Consume remaining tokens as options until we hit a statement boundary
	p.skipSecurityKeyOptions(stmt)

	stmt.Loc.End = p.pos()
	return stmt
}

// parseSecurityKeyStmtColumn parses CREATE/ALTER/DROP COLUMN { ENCRYPTION KEY | MASTER KEY }.
// Called after "COLUMN" has been matched by the dispatcher.
//
// Ref: https://learn.microsoft.com/en-us/sql/t-sql/statements/create-column-encryption-key-transact-sql
//
//	CREATE COLUMN ENCRYPTION KEY key_name
//	WITH VALUES
//	  (
//	    COLUMN_MASTER_KEY = column_master_key_name,
//	    ALGORITHM = 'algorithm_name',
//	    ENCRYPTED_VALUE = varbinary_literal
//	  )
//	[, ( ... ) ]
//
// Ref: https://learn.microsoft.com/en-us/sql/t-sql/statements/alter-column-encryption-key-transact-sql
//
//	ALTER COLUMN ENCRYPTION KEY key_name
//	  [ ADD | DROP ] VALUE
//	  (
//	    COLUMN_MASTER_KEY = column_master_key_name
//	    [, ALGORITHM = 'algorithm_name', ENCRYPTED_VALUE = varbinary_literal ]
//	  )
//
// Ref: https://learn.microsoft.com/en-us/sql/t-sql/statements/create-column-master-key-transact-sql
//
//	CREATE COLUMN MASTER KEY key_name
//	  WITH (
//	    KEY_STORE_PROVIDER_NAME = 'key_store_provider_name',
//	    KEY_PATH = 'key_path'
//	    [, ENCLAVE_COMPUTATIONS (SIGNATURE = signature)]
//	  )
//
// DROP COLUMN ENCRYPTION KEY key_name
// DROP COLUMN MASTER KEY key_name
func (p *Parser) parseSecurityKeyStmtColumn(action string) *nodes.SecurityKeyStmt {
	loc := p.pos()
	stmt := &nodes.SecurityKeyStmt{
		Action: action,
		Loc:    nodes.Loc{Start: loc},
	}

	// Determine COLUMN ENCRYPTION KEY or COLUMN MASTER KEY
	if p.matchIdentCI("ENCRYPTION") {
		p.match(kwKEY)
		stmt.ObjectType = "COLUMN ENCRYPTION KEY"
	} else if p.matchIdentCI("MASTER") {
		p.match(kwKEY)
		stmt.ObjectType = "COLUMN MASTER KEY"
	} else {
		stmt.Loc.End = p.pos()
		return stmt
	}

	// Parse key name
	name, _ := p.parseIdentifier()
	stmt.Name = name

	// For ALTER COLUMN ENCRYPTION KEY, handle ADD/DROP VALUE specially
	// because DROP is a statement-start keyword that would cause skipSecurityKeyOptions to stop.
	if action == "ALTER" && stmt.ObjectType == "COLUMN ENCRYPTION KEY" {
		// Consume ADD VALUE or DROP VALUE manually before skipping options
		var buf strings.Builder
		if p.cur.Type == kwADD {
			buf.WriteString("ADD")
			p.advance()
		} else if p.cur.Type == kwDROP {
			buf.WriteString("DROP")
			p.advance()
		}
		if p.isIdentLike() && matchesKeywordCI(p.cur.Str, "VALUE") {
			if buf.Len() > 0 {
				buf.WriteByte(' ')
			}
			buf.WriteString("VALUE")
			p.advance()
		}
		// Now consume the rest (parenthesized options)
		p.skipSecurityKeyOptions(stmt)
		// Prepend the ADD/DROP VALUE to the options
		if buf.Len() > 0 && stmt.Options != nil {
			existing := stmt.Options.Items[0].(*nodes.String).Str
			stmt.Options.Items[0] = &nodes.String{Str: buf.String() + " " + existing}
		} else if buf.Len() > 0 {
			stmt.Options = &nodes.List{Items: []nodes.Node{&nodes.String{Str: buf.String()}}}
		}
	} else {
		// Consume remaining tokens as options
		p.skipSecurityKeyOptions(stmt)
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseSecurityKeyStmtDatabaseEncryption parses CREATE/ALTER/DROP DATABASE ENCRYPTION KEY
// and CREATE/ALTER/DROP DATABASE SCOPED CREDENTIAL.
// Called after "DATABASE" has been consumed and the next token is "ENCRYPTION" or "SCOPED".
//
// Ref: https://learn.microsoft.com/en-us/sql/t-sql/statements/create-database-encryption-key-transact-sql
//
//	CREATE DATABASE ENCRYPTION KEY
//	  WITH ALGORITHM = { AES_128 | AES_192 | AES_256 | TRIPLE_DES_3KEY }
//	  ENCRYPTION BY SERVER
//	    { CERTIFICATE Encryptor_Name | ASYMMETRIC KEY Encryptor_Name }
//
// Ref: https://learn.microsoft.com/en-us/sql/t-sql/statements/alter-database-encryption-key-transact-sql
//
//	ALTER DATABASE ENCRYPTION KEY
//	  REGENERATE WITH ALGORITHM = { AES_128 | AES_192 | AES_256 | TRIPLE_DES_3KEY }
//	  | ENCRYPTION BY SERVER
//	    { CERTIFICATE Encryptor_Name | ASYMMETRIC KEY Encryptor_Name }
//
// DROP DATABASE ENCRYPTION KEY
//
// Ref: https://learn.microsoft.com/en-us/sql/t-sql/statements/create-database-scoped-credential-transact-sql
//
//	CREATE DATABASE SCOPED CREDENTIAL credential_name
//	  WITH IDENTITY = 'identity_name'
//	    [ , SECRET = 'secret' ]
//
// ALTER DATABASE SCOPED CREDENTIAL credential_name
//   WITH IDENTITY = 'identity_name'
//     [ , SECRET = 'secret' ]
//
// DROP DATABASE SCOPED CREDENTIAL credential_name
func (p *Parser) parseSecurityKeyStmtDatabaseEncryption(action string) *nodes.SecurityKeyStmt {
	loc := p.pos()
	stmt := &nodes.SecurityKeyStmt{
		Action: action,
		Loc:    nodes.Loc{Start: loc},
	}

	if p.matchIdentCI("ENCRYPTION") {
		// DATABASE ENCRYPTION KEY
		p.match(kwKEY)
		stmt.ObjectType = "DATABASE ENCRYPTION KEY"
		// No name for database encryption key
	} else if p.matchIdentCI("SCOPED") {
		// DATABASE SCOPED CREDENTIAL credential_name
		p.matchIdentCI("CREDENTIAL")
		stmt.ObjectType = "DATABASE SCOPED CREDENTIAL"
		name, _ := p.parseIdentifier()
		stmt.Name = name
	} else {
		stmt.Loc.End = p.pos()
		return stmt
	}

	// Consume remaining tokens as options
	p.skipSecurityKeyOptions(stmt)

	stmt.Loc.End = p.pos()
	return stmt
}

// parseOpenSymmetricKeyStmt parses OPEN SYMMETRIC KEY statements.
//
//	OPEN SYMMETRIC KEY key_name DECRYPTION BY ...
func (p *Parser) parseOpenSymmetricKeyStmt() *nodes.SecurityKeyStmt {
	loc := p.pos()
	p.advance() // consume OPEN
	stmt := &nodes.SecurityKeyStmt{
		Action: "OPEN",
		Loc:    nodes.Loc{Start: loc},
	}

	if p.matchIdentCI("SYMMETRIC") {
		p.match(kwKEY)
		stmt.ObjectType = "SYMMETRIC KEY"
	}

	name, _ := p.parseIdentifier()
	stmt.Name = name

	p.skipSecurityKeyOptions(stmt)

	stmt.Loc.End = p.pos()
	return stmt
}

// parseCloseSymmetricKeyStmt parses CLOSE SYMMETRIC KEY or CLOSE ALL SYMMETRIC KEYS.
//
//	CLOSE { SYMMETRIC KEY key_name | ALL SYMMETRIC KEYS }
func (p *Parser) parseCloseSymmetricKeyStmt() *nodes.SecurityKeyStmt {
	loc := p.pos()
	p.advance() // consume CLOSE
	stmt := &nodes.SecurityKeyStmt{
		Action: "CLOSE",
		Loc:    nodes.Loc{Start: loc},
	}

	if p.cur.Type == kwALL {
		p.advance() // consume ALL
		p.matchIdentCI("SYMMETRIC")
		p.matchIdentCI("KEYS")
		stmt.ObjectType = "ALL SYMMETRIC KEYS"
	} else if p.matchIdentCI("SYMMETRIC") {
		p.match(kwKEY)
		stmt.ObjectType = "SYMMETRIC KEY"
		name, _ := p.parseIdentifier()
		stmt.Name = name
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseBackupCertificateStmt parses BACKUP CERTIFICATE cert_name TO FILE = 'path' ...
func (p *Parser) parseBackupCertificateStmt() *nodes.SecurityKeyStmt {
	loc := p.pos()
	p.advance() // consume BACKUP
	stmt := &nodes.SecurityKeyStmt{
		Action: "BACKUP",
		Loc:    nodes.Loc{Start: loc},
	}

	if p.matchIdentCI("CERTIFICATE") {
		stmt.ObjectType = "CERTIFICATE"
		name, _ := p.parseIdentifier()
		stmt.Name = name
	} else if p.matchIdentCI("MASTER") {
		p.match(kwKEY)
		stmt.ObjectType = "MASTER KEY"
	}

	p.skipSecurityKeyOptions(stmt)

	stmt.Loc.End = p.pos()
	return stmt
}

// skipSecurityKeyOptions consumes remaining tokens until a statement boundary,
// collecting option strings.
func (p *Parser) skipSecurityKeyOptions(stmt *nodes.SecurityKeyStmt) {
	var opts []nodes.Node
	var buf strings.Builder
	depth := 0

	for p.cur.Type != tokEOF && p.cur.Type != ';' {
		// Break on keywords that start new statements (unless nested in parens)
		if depth == 0 && isStmtStart(p.cur.Type) {
			break
		}
		if p.cur.Type == '(' {
			depth++
		} else if p.cur.Type == ')' {
			if depth > 0 {
				depth--
			}
		}
		if buf.Len() > 0 {
			buf.WriteByte(' ')
		}
		buf.WriteString(p.cur.Str)
		if buf.Len() == 0 {
			// For single-char tokens with empty Str
			buf.WriteRune(rune(p.cur.Type))
		}
		p.advance()
	}

	if buf.Len() > 0 {
		opts = append(opts, &nodes.String{Str: buf.String()})
	}
	if len(opts) > 0 {
		stmt.Options = &nodes.List{Items: opts}
	}
}

// isStmtStart returns true if the token type could start a new statement.
func isStmtStart(t int) bool {
	switch t {
	case kwSELECT, kwINSERT, kwUPDATE, kwDELETE, kwCREATE, kwALTER, kwDROP,
		kwGRANT, kwREVOKE, kwDENY, kwEXEC, kwEXECUTE, kwDECLARE, kwSET,
		kwIF, kwWHILE, kwBEGIN, kwRETURN, kwGOTO, kwWAITFOR,
		kwPRINT, kwRAISERROR, kwTHROW, kwUSE, kwGO,
		kwCOMMIT, kwROLLBACK, kwSAVE, kwTRUNCATE, kwMERGE,
		kwOPEN, kwFETCH, kwCLOSE, kwDEALLOCATE, kwBACKUP:
		return true
	}
	return false
}
