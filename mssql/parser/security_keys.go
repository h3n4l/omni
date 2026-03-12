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

// parseOpenMasterKeyStmt parses OPEN MASTER KEY DECRYPTION BY PASSWORD = 'password'.
func (p *Parser) parseOpenMasterKeyStmt() *nodes.SecurityKeyStmt {
	loc := p.pos()
	p.advance() // consume OPEN
	stmt := &nodes.SecurityKeyStmt{
		Action:     "OPEN",
		ObjectType: "MASTER KEY",
		Loc:        nodes.Loc{Start: loc},
	}

	// consume MASTER KEY
	if p.matchIdentCI("MASTER") {
		p.match(kwKEY)
	}

	p.skipSecurityKeyOptions(stmt)
	stmt.Loc.End = p.pos()
	return stmt
}

// parseCloseMasterKeyStmt parses CLOSE MASTER KEY.
func (p *Parser) parseCloseMasterKeyStmt() *nodes.SecurityKeyStmt {
	loc := p.pos()
	p.advance() // consume CLOSE
	stmt := &nodes.SecurityKeyStmt{
		Action:     "CLOSE",
		ObjectType: "MASTER KEY",
		Loc:        nodes.Loc{Start: loc},
	}

	// consume MASTER KEY
	if p.matchIdentCI("MASTER") {
		p.match(kwKEY)
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseRestoreMasterKeyStmt parses RESTORE MASTER KEY FROM FILE = 'path' ...
func (p *Parser) parseRestoreMasterKeyStmt() *nodes.SecurityKeyStmt {
	loc := p.pos()
	p.advance() // consume RESTORE
	stmt := &nodes.SecurityKeyStmt{
		Action:     "RESTORE",
		ObjectType: "MASTER KEY",
		Loc:        nodes.Loc{Start: loc},
	}

	// consume MASTER KEY
	if p.matchIdentCI("MASTER") {
		p.match(kwKEY)
	}

	p.skipSecurityKeyOptions(stmt)
	stmt.Loc.End = p.pos()
	return stmt
}

// skipSecurityKeyOptions consumes remaining tokens until a statement boundary,
// using structured key-value parsing when possible.
//
// Security key statements use several patterns:
//   - WITH ( KEY_SOURCE = ..., ALGORITHM = ..., ... )
//   - WITH SUBJECT = ..., EXPIRY_DATE = ...
//   - ENCRYPTION BY { PASSWORD = ... | CERTIFICATE ... | ASYMMETRIC KEY ... }
//   - DECRYPTION BY { PASSWORD = ... | CERTIFICATE ... | ASYMMETRIC KEY ... }
//   - WITH IDENTITY = '...', SECRET = '...'
//   - FROM FILE = '...'
//   - TO FILE = '...'
//
// We parse these as alternating name/value pairs in a List of String nodes.
func (p *Parser) skipSecurityKeyOptions(stmt *nodes.SecurityKeyStmt) {
	var items []nodes.Node
	depth := 0

	for p.cur.Type != tokEOF && p.cur.Type != ';' {
		// Break on keywords that start new statements (unless nested in parens)
		if depth == 0 && isStmtStart(p.cur.Type) {
			break
		}

		// WITH keyword: may introduce parenthesized or inline options
		if p.cur.Type == kwWITH {
			p.advance() // consume WITH
			if p.cur.Type == '(' {
				// WITH ( NAME = VALUE [, ...] )
				nested := p.parseKeyValueOptionList()
				if nested != nil {
					items = append(items, nested.Items...)
				}
				continue
			}
			// WITH inline options: SUBJECT = ..., IDENTITY = ..., etc.
			// Fall through to parse as key=value below
		}

		// ENCRYPTION BY / DECRYPTION BY / FROM / TO patterns
		if p.isIdentLike() && (strings.EqualFold(p.cur.Str, "ENCRYPTION") ||
			strings.EqualFold(p.cur.Str, "DECRYPTION")) {
			keyword := strings.ToUpper(p.cur.Str)
			p.advance()
			if p.cur.Type == kwBY {
				p.advance() // consume BY
			}
			// Collect the encryption spec: PASSWORD = '...', CERTIFICATE name, ASYMMETRIC KEY name, etc.
			var spec strings.Builder
			spec.WriteString(keyword)
			spec.WriteString(" BY")
			for p.cur.Type != tokEOF && p.cur.Type != ';' && p.cur.Type != ',' {
				if depth == 0 && isStmtStart(p.cur.Type) {
					break
				}
				if p.cur.Type == kwWITH || (p.isIdentLike() && (strings.EqualFold(p.cur.Str, "ENCRYPTION") ||
					strings.EqualFold(p.cur.Str, "DECRYPTION"))) {
					break
				}
				if p.cur.Type == '(' {
					depth++
				} else if p.cur.Type == ')' {
					if depth > 0 {
						depth--
					} else {
						break
					}
				}
				spec.WriteByte(' ')
				if p.cur.Str != "" {
					spec.WriteString(p.cur.Str)
				} else {
					spec.WriteRune(rune(p.cur.Type))
				}
				p.advance()
			}
			items = append(items, &nodes.String{Str: spec.String()})
			p.match(',')
			continue
		}

		if p.cur.Type == kwFROM || p.cur.Type == kwTO {
			keyword := strings.ToUpper(p.cur.Str)
			if keyword == "" {
				if p.cur.Type == kwFROM {
					keyword = "FROM"
				} else {
					keyword = "TO"
				}
			}
			p.advance()
			items = append(items, &nodes.String{Str: keyword})
			// Collect FILE = '...' or ASSEMBLY etc.
			continue
		}

		// General key = value parsing
		if p.isIdentLike() || p.cur.Type == tokICONST || p.cur.Type == tokSCONST {
			key := p.consumeAnyIdent()
			items = append(items, &nodes.String{Str: key})

			if p.cur.Type == '=' {
				p.advance()
				val := p.consumeAnyIdent()
				items = append(items, &nodes.String{Str: val})
			}
			p.match(',')
			continue
		}

		if p.cur.Type == '(' {
			depth++
			p.advance()
			continue
		}
		if p.cur.Type == ')' {
			if depth > 0 {
				depth--
				p.advance()
				continue
			}
			break
		}

		// Unknown token, consume to avoid infinite loop
		p.advance()
	}

	if len(items) > 0 {
		stmt.Options = &nodes.List{Items: items}
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
