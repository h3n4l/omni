// Package parser - security_principals.go implements T-SQL security principal
// statement parsing: CREATE/ALTER/DROP USER, LOGIN, ROLE, APPLICATION ROLE,
// and ADD/DROP ROLE MEMBER.
package parser

import (
	"strings"

	nodes "github.com/bytebase/omni/mssql/ast"
)

// parseSecurityUserStmt parses CREATE/ALTER/DROP USER.
//
// Ref: https://learn.microsoft.com/en-us/sql/t-sql/statements/create-user-transact-sql
//
//	CREATE USER user_name
//	    [ { FOR | FROM } LOGIN login_name ]
//	    [ WITH <option_list> [ ,... ] ]
//	ALTER USER user_name WITH <option_list> [ ,... ]
//	DROP USER [ IF EXISTS ] user_name
//
//	<option_list> ::=
//	    PASSWORD = 'password' [ OLD_PASSWORD = 'oldpassword' ]
//	  | DEFAULT_SCHEMA = schema_name
//	  | DEFAULT_LANGUAGE = { NONE | lcid | language_name | language_alias }
//	  | ALLOW_ENCRYPTED_VALUE_MODIFICATIONS = { ON | OFF }
//	  | NAME = new_user_name
//	  | LOGIN = new_login_name
//	  | SID = sid
func (p *Parser) parseSecurityUserStmt(action string) *nodes.SecurityStmt {
	loc := p.pos()
	stmt := &nodes.SecurityStmt{
		Action:     action,
		ObjectType: "USER",
		Loc:        nodes.Loc{Start: loc},
	}

	// IF EXISTS (DROP only)
	if action == "DROP" {
		if p.cur.Type == kwIF {
			next := p.peekNext()
			if next.Type == kwEXISTS {
				p.advance() // IF
				p.advance() // EXISTS
			}
		}
	}

	// user name
	if name, ok := p.parseIdentifier(); ok {
		stmt.Name = name
	}

	// FOR/FROM LOGIN login_name  (CREATE)
	var opts []nodes.Node
	if action == "CREATE" {
		if p.cur.Type == kwFOR || p.cur.Type == kwFROM {
			optLoc := p.pos()
			p.advance() // consume FOR / FROM
			if p.isIdentLike() && (matchesKeywordCI(p.cur.Str, "LOGIN") || p.cur.Type == kwLOGIN) {
				p.advance() // consume LOGIN
			}
			if name, ok := p.parseIdentifier(); ok {
				opts = append(opts, &nodes.SecurityPrincipalOption{
					Name:  "LOGIN",
					Value: name,
					Loc:   nodes.Loc{Start: optLoc, End: p.pos()},
				})
			}
		}
	}

	// WITH options
	if p.cur.Type == kwWITH {
		p.advance() // consume WITH
		opts = append(opts, p.parseSecurityPrincipalWithOptions()...)
	}

	if len(opts) > 0 {
		stmt.Options = &nodes.List{Items: opts}
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseSecurityLoginStmt parses CREATE/ALTER/DROP LOGIN.
//
// Ref: https://learn.microsoft.com/en-us/sql/t-sql/statements/create-login-transact-sql
//
//	CREATE LOGIN login_name
//	    { WITH PASSWORD = 'password' [ HASHED ] [ MUST_CHANGE ]
//	        [ , DEFAULT_DATABASE = database ]
//	        [ , DEFAULT_LANGUAGE = language ]
//	        [ , CHECK_EXPIRATION = { ON | OFF } ]
//	        [ , CHECK_POLICY = { ON | OFF } ]
//	        [ , CREDENTIAL = credential_name ]
//	        [ , SID = sid ]
//	    | FROM WINDOWS [ WITH DEFAULT_DATABASE = database [, DEFAULT_LANGUAGE = language ] ]
//	    | FROM CERTIFICATE certname
//	    | FROM ASYMMETRIC KEY asym_key_name
//	    | FROM EXTERNAL PROVIDER
//	    }
//	ALTER LOGIN login_name
//	    { ENABLE | DISABLE
//	    | WITH PASSWORD = 'password' [ OLD_PASSWORD = 'old' ] [ HASHED ] [ MUST_CHANGE ] [ , ... ]
//	    | WITH DEFAULT_DATABASE = database [, ...] }
//	DROP LOGIN login_name
func (p *Parser) parseSecurityLoginStmt(action string) *nodes.SecurityStmt {
	loc := p.pos()
	stmt := &nodes.SecurityStmt{
		Action:     action,
		ObjectType: "LOGIN",
		Loc:        nodes.Loc{Start: loc},
	}

	// login name
	if name, ok := p.parseIdentifier(); ok {
		stmt.Name = name
	}

	var opts []nodes.Node

	// ENABLE / DISABLE (ALTER LOGIN)
	if action == "ALTER" {
		if p.isIdentLike() && matchesKeywordCI(p.cur.Str, "ENABLE") {
			optLoc := p.pos()
			p.advance()
			opts = append(opts, &nodes.SecurityPrincipalOption{
				Name: "ENABLE",
				Loc:  nodes.Loc{Start: optLoc, End: p.pos()},
			})
		} else if p.isIdentLike() && matchesKeywordCI(p.cur.Str, "DISABLE") {
			optLoc := p.pos()
			p.advance()
			opts = append(opts, &nodes.SecurityPrincipalOption{
				Name: "DISABLE",
				Loc:  nodes.Loc{Start: optLoc, End: p.pos()},
			})
		}
	}

	// WITH options
	if p.cur.Type == kwWITH {
		p.advance() // consume WITH
		opts = append(opts, p.parseSecurityPrincipalWithOptions()...)
	} else if p.cur.Type == kwFROM {
		optLoc := p.pos()
		p.advance() // consume FROM
		if p.isIdentLike() {
			src := strings.ToUpper(p.cur.Str)
			switch src {
			case "WINDOWS":
				p.advance()
				opts = append(opts, &nodes.SecurityPrincipalOption{
					Name:  "FROM",
					Value: "WINDOWS",
					Loc:   nodes.Loc{Start: optLoc, End: p.pos()},
				})
			case "CERTIFICATE":
				p.advance()
				certName := ""
				if id, ok := p.parseIdentifier(); ok {
					certName = id
				}
				opts = append(opts, &nodes.SecurityPrincipalOption{
					Name:  "FROM",
					Value: "CERTIFICATE " + certName,
					Loc:   nodes.Loc{Start: optLoc, End: p.pos()},
				})
			case "ASYMMETRIC":
				p.advance() // consume ASYMMETRIC
				if p.cur.Type == kwKEY {
					p.advance() // consume KEY
				}
				keyName := ""
				if id, ok := p.parseIdentifier(); ok {
					keyName = id
				}
				opts = append(opts, &nodes.SecurityPrincipalOption{
					Name:  "FROM",
					Value: "ASYMMETRIC KEY " + keyName,
					Loc:   nodes.Loc{Start: optLoc, End: p.pos()},
				})
			case "EXTERNAL":
				p.advance() // consume EXTERNAL
				if p.isIdentLike() && matchesKeywordCI(p.cur.Str, "PROVIDER") {
					p.advance()
				}
				opts = append(opts, &nodes.SecurityPrincipalOption{
					Name:  "FROM",
					Value: "EXTERNAL PROVIDER",
					Loc:   nodes.Loc{Start: optLoc, End: p.pos()},
				})
			default:
				// unknown FROM source
				p.advance()
				opts = append(opts, &nodes.SecurityPrincipalOption{
					Name:  "FROM",
					Value: src,
					Loc:   nodes.Loc{Start: optLoc, End: p.pos()},
				})
			}
		}
		// After FROM WINDOWS, optional WITH clause
		if p.cur.Type == kwWITH {
			p.advance()
			opts = append(opts, p.parseSecurityPrincipalWithOptions()...)
		}
	}

	if len(opts) > 0 {
		stmt.Options = &nodes.List{Items: opts}
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseSecurityRoleStmt parses CREATE/ALTER/DROP ROLE and ADD/DROP ROLE MEMBER.
//
// Ref: https://learn.microsoft.com/en-us/sql/t-sql/statements/create-role-transact-sql
// Ref: https://learn.microsoft.com/en-us/sql/t-sql/statements/alter-role-transact-sql
// Ref: https://learn.microsoft.com/en-us/sql/t-sql/statements/drop-role-transact-sql
//
//	CREATE ROLE role_name [ AUTHORIZATION owner_name ]
//	ALTER  ROLE role_name { ADD MEMBER member_name | DROP MEMBER member_name | WITH NAME = new_name }
//	DROP   ROLE [ IF EXISTS ] role_name
func (p *Parser) parseSecurityRoleStmt(action string) *nodes.SecurityStmt {
	loc := p.pos()
	stmt := &nodes.SecurityStmt{
		Action:     action,
		ObjectType: "ROLE",
		Loc:        nodes.Loc{Start: loc},
	}

	// IF EXISTS (DROP only)
	if action == "DROP" {
		if p.cur.Type == kwIF {
			next := p.peekNext()
			if next.Type == kwEXISTS {
				p.advance() // IF
				p.advance() // EXISTS
			}
		}
	}

	// role name
	if name, ok := p.parseIdentifier(); ok {
		stmt.Name = name
	}

	var opts []nodes.Node

	if action == "CREATE" {
		// AUTHORIZATION owner_name
		if p.isIdentLike() && matchesKeywordCI(p.cur.Str, "AUTHORIZATION") {
			optLoc := p.pos()
			p.advance()
			if owner, ok := p.parseIdentifier(); ok {
				opts = append(opts, &nodes.SecurityPrincipalOption{
					Name:  "AUTHORIZATION",
					Value: owner,
					Loc:   nodes.Loc{Start: optLoc, End: p.pos()},
				})
			}
		}
	} else if action == "ALTER" {
		// ADD MEMBER | DROP MEMBER | WITH NAME = new_name
		if p.cur.Type == kwADD {
			optLoc := p.pos()
			p.advance() // consume ADD
			if p.isIdentLike() && matchesKeywordCI(p.cur.Str, "MEMBER") {
				p.advance() // consume MEMBER
			}
			if member, ok := p.parseIdentifier(); ok {
				opts = append(opts, &nodes.SecurityPrincipalOption{
					Name:  "ADD MEMBER",
					Value: member,
					Loc:   nodes.Loc{Start: optLoc, End: p.pos()},
				})
			}
		} else if p.cur.Type == kwDROP {
			optLoc := p.pos()
			p.advance() // consume DROP
			if p.isIdentLike() && matchesKeywordCI(p.cur.Str, "MEMBER") {
				p.advance() // consume MEMBER
			}
			if member, ok := p.parseIdentifier(); ok {
				opts = append(opts, &nodes.SecurityPrincipalOption{
					Name:  "DROP MEMBER",
					Value: member,
					Loc:   nodes.Loc{Start: optLoc, End: p.pos()},
				})
			}
		} else if p.cur.Type == kwWITH {
			p.advance() // consume WITH
			opts = append(opts, p.parseSecurityPrincipalWithOptions()...)
		}
	}

	if len(opts) > 0 {
		stmt.Options = &nodes.List{Items: opts}
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseSecurityApplicationRoleStmt parses CREATE/ALTER/DROP APPLICATION ROLE.
//
// Ref: https://learn.microsoft.com/en-us/sql/t-sql/statements/create-application-role-transact-sql
//
//	CREATE APPLICATION ROLE role_name WITH PASSWORD = 'password' [ , DEFAULT_SCHEMA = schema_name ]
//	ALTER  APPLICATION ROLE role_name WITH NAME = new_name | PASSWORD = '...' | DEFAULT_SCHEMA = ...
//	DROP   APPLICATION ROLE role_name
func (p *Parser) parseSecurityApplicationRoleStmt(action string) *nodes.SecurityStmt {
	loc := p.pos()
	// APPLICATION keyword already consumed by caller
	// Consume ROLE
	if p.isIdentLike() && matchesKeywordCI(p.cur.Str, "ROLE") {
		p.advance()
	} else if p.cur.Type == kwROLE {
		p.advance()
	}

	stmt := &nodes.SecurityStmt{
		Action:     action,
		ObjectType: "APPLICATION ROLE",
		Loc:        nodes.Loc{Start: loc},
	}

	// role name
	if name, ok := p.parseIdentifier(); ok {
		stmt.Name = name
	}

	var opts []nodes.Node

	// WITH options
	if p.cur.Type == kwWITH {
		p.advance() // consume WITH
		opts = append(opts, p.parseSecurityPrincipalWithOptions()...)
	}

	if len(opts) > 0 {
		stmt.Options = &nodes.List{Items: opts}
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseSecurityPrincipalWithOptions parses comma-separated key=value options
// after WITH has been consumed. Used by CREATE/ALTER USER, LOGIN, ROLE, APPLICATION ROLE.
//
//	option ::=
//	    PASSWORD = 'password' [ HASHED ] [ MUST_CHANGE ] [ OLD_PASSWORD = 'old' ]
//	  | DEFAULT_SCHEMA = schema_name
//	  | DEFAULT_LANGUAGE = { NONE | lcid | language_name }
//	  | DEFAULT_DATABASE = database_name
//	  | CHECK_EXPIRATION = { ON | OFF }
//	  | CHECK_POLICY = { ON | OFF }
//	  | CREDENTIAL = credential_name
//	  | SID = sid
//	  | NAME = new_name
//	  | LOGIN = login_name
//	  | ALLOW_ENCRYPTED_VALUE_MODIFICATIONS = { ON | OFF }
func (p *Parser) parseSecurityPrincipalWithOptions() []nodes.Node {
	var opts []nodes.Node

	for {
		if !p.isIdentLike() {
			break
		}

		optLoc := p.pos()
		key := strings.ToUpper(p.cur.Str)
		p.advance() // consume key

		opt := &nodes.SecurityPrincipalOption{
			Name: key,
			Loc:  nodes.Loc{Start: optLoc},
		}

		if p.cur.Type == '=' {
			p.advance() // consume =
			// Value: string constant, number, identifier, ON/OFF
			if p.cur.Type == tokSCONST || p.cur.Type == tokNSCONST {
				opt.Value = p.cur.Str
				p.advance()
			} else if p.cur.Type == tokICONST || p.cur.Type == tokFCONST {
				opt.Value = p.cur.Str
				p.advance()
			} else if p.cur.Type == kwON {
				opt.Value = "ON"
				p.advance()
			} else if p.cur.Type == kwOFF {
				opt.Value = "OFF"
				p.advance()
			} else if p.isIdentLike() {
				upper := strings.ToUpper(p.cur.Str)
				if upper == "ON" {
					opt.Value = "ON"
				} else if upper == "OFF" {
					opt.Value = "OFF"
				} else {
					opt.Value = p.cur.Str
				}
				p.advance()
			}
		}

		// PASSWORD sub-options: HASHED, MUST_CHANGE, OLD_PASSWORD
		if key == "PASSWORD" {
			for {
				if p.isIdentLike() && matchesKeywordCI(p.cur.Str, "HASHED") {
					opt.Hashed = true
					p.advance()
				} else if p.isIdentLike() && matchesKeywordCI(p.cur.Str, "MUST_CHANGE") {
					opt.MustChange = true
					p.advance()
				} else if p.isIdentLike() && matchesKeywordCI(p.cur.Str, "OLD_PASSWORD") {
					p.advance() // consume OLD_PASSWORD
					if p.cur.Type == '=' {
						p.advance()
						if p.cur.Type == tokSCONST || p.cur.Type == tokNSCONST {
							opt.OldPassword = p.cur.Str
							p.advance()
						}
					}
				} else {
					break
				}
			}
		}

		opt.Loc.End = p.pos()
		opts = append(opts, opt)

		if p.cur.Type == ',' {
			p.advance()
		} else {
			break
		}
	}

	return opts
}

// parseExecuteAsStmt parses a standalone EXECUTE AS statement (context switching).
//
// Ref: https://learn.microsoft.com/en-us/sql/t-sql/statements/execute-as-transact-sql
//
//	EXECUTE AS { LOGIN | USER | CALLER | SELF | OWNER } [ = 'name' ]
//	    [ WITH { NO REVERT | COOKIE INTO @cookie_variable } ]
func (p *Parser) parseExecuteAsStmt() *nodes.SecurityStmt {
	loc := p.pos()
	p.advance() // consume EXECUTE
	p.advance() // consume AS

	stmt := &nodes.SecurityStmt{
		Action: "EXECUTE AS",
		Loc:    nodes.Loc{Start: loc},
	}

	// { LOGIN | USER | CALLER | SELF | OWNER }
	if p.cur.Type == kwLOGIN {
		stmt.ObjectType = "LOGIN"
		p.advance()
	} else if p.cur.Type == kwUSER {
		stmt.ObjectType = "USER"
		p.advance()
	} else if p.isIdentLike() && matchesKeywordCI(p.cur.Str, "CALLER") {
		stmt.ObjectType = "CALLER"
		p.advance()
		stmt.Loc.End = p.pos()
		return stmt
	} else if p.isIdentLike() && matchesKeywordCI(p.cur.Str, "SELF") {
		stmt.ObjectType = "SELF"
		p.advance()
		stmt.Loc.End = p.pos()
		return stmt
	} else if p.isIdentLike() && matchesKeywordCI(p.cur.Str, "OWNER") {
		stmt.ObjectType = "OWNER"
		p.advance()
		stmt.Loc.End = p.pos()
		return stmt
	}

	// = 'name'
	if p.cur.Type == '=' {
		p.advance()
		if p.cur.Type == tokSCONST || p.isIdentLike() {
			stmt.Name = p.cur.Str
			p.advance()
		}
	}

	// WITH { NO REVERT | COOKIE INTO @cookie_variable }
	if p.cur.Type == kwWITH {
		p.advance()
		var opts []nodes.Node
		if p.isIdentLike() && matchesKeywordCI(p.cur.Str, "NO") {
			p.advance()
			if p.isIdentLike() && matchesKeywordCI(p.cur.Str, "REVERT") {
				p.advance()
				opts = append(opts, &nodes.String{Str: "NO REVERT"})
			}
		} else if p.isIdentLike() && matchesKeywordCI(p.cur.Str, "COOKIE") {
			p.advance()
			if p.cur.Type == kwINTO {
				p.advance()
			}
			if p.cur.Type == tokVARIABLE {
				opts = append(opts, &nodes.String{Str: "COOKIE=" + p.cur.Str})
				p.advance()
			}
		}
		if len(opts) > 0 {
			stmt.Options = &nodes.List{Items: opts}
		}
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseRevertStmt parses the REVERT statement.
//
// Ref: https://learn.microsoft.com/en-us/sql/t-sql/statements/revert-transact-sql
//
//	REVERT [ WITH COOKIE = @cookie_variable ]
func (p *Parser) parseRevertStmt() *nodes.SecurityStmt {
	loc := p.pos()
	p.advance() // consume REVERT

	stmt := &nodes.SecurityStmt{
		Action:     "REVERT",
		ObjectType: "CONTEXT",
		Loc:        nodes.Loc{Start: loc},
	}

	// WITH COOKIE = @cookie_variable
	if p.cur.Type == kwWITH {
		p.advance()
		if p.isIdentLike() && matchesKeywordCI(p.cur.Str, "COOKIE") {
			p.advance()
			if p.cur.Type == '=' {
				p.advance()
				if p.cur.Type == tokVARIABLE {
					var opts []nodes.Node
					opts = append(opts, &nodes.String{Str: "COOKIE=" + p.cur.Str})
					stmt.Options = &nodes.List{Items: opts}
					p.advance()
				}
			}
		}
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseAlterAuthorizationStmt parses ALTER AUTHORIZATION.
//
// Ref: https://learn.microsoft.com/en-us/sql/t-sql/statements/alter-authorization-transact-sql
//
//	ALTER AUTHORIZATION
//	    ON [ <class_type> :: ] entity_name
//	    TO { principal_name | SCHEMA OWNER }
//
//	<class_type> ::=
//	    { OBJECT | ASSEMBLY | ASYMMETRIC KEY | CERTIFICATE | CONTRACT
//	    | DATABASE | ENDPOINT | FULLTEXT CATALOG | FULLTEXT STOPLIST
//	    | MESSAGE TYPE | REMOTE SERVICE BINDING | ROLE | ROUTE
//	    | SCHEMA | SEARCH PROPERTY LIST | SERVER ROLE | SERVICE
//	    | SYMMETRIC KEY | TYPE | XML SCHEMA COLLECTION }
func (p *Parser) parseAlterAuthorizationStmt() *nodes.SecurityStmt {
	loc := p.pos()
	// AUTHORIZATION keyword already consumed by caller

	stmt := &nodes.SecurityStmt{
		Action:     "ALTER AUTHORIZATION",
		ObjectType: "OBJECT",
		Loc:        nodes.Loc{Start: loc},
	}

	// ON [ class_type :: ] entity_name
	if p.cur.Type == kwON {
		p.advance()
		// Collect tokens until we find TO
		var nameParts []string
		for p.cur.Type != tokEOF && p.cur.Type != ';' {
			if p.isIdentLike() && matchesKeywordCI(p.cur.Str, "TO") {
				break
			}
			if p.cur.Type == ':' {
				// Skip :: separator
				p.advance()
				continue
			}
			if p.isIdentLike() || p.cur.Type == tokSCONST || p.cur.Type == kwDATABASE || p.cur.Type == kwXML || p.cur.Type == kwROLE {
				nameParts = append(nameParts, p.cur.Str)
			} else if p.cur.Type == '.' {
				nameParts = append(nameParts, ".")
			}
			p.advance()
		}
		if len(nameParts) > 0 {
			stmt.Name = strings.Join(nameParts, " ")
		}
	}

	// TO { principal_name | SCHEMA OWNER }
	if p.isIdentLike() && matchesKeywordCI(p.cur.Str, "TO") {
		p.advance()
		var opts []nodes.Node
		if p.cur.Type == kwSCHEMA {
			p.advance()
			if p.isIdentLike() && matchesKeywordCI(p.cur.Str, "OWNER") {
				p.advance()
				opts = append(opts, &nodes.String{Str: "SCHEMA OWNER"})
			}
		} else if p.isIdentLike() || p.cur.Type == tokSCONST {
			opts = append(opts, &nodes.String{Str: p.cur.Str})
			p.advance()
		}
		if len(opts) > 0 {
			stmt.Options = &nodes.List{Items: opts}
		}
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// matchesKeywordCI returns true if s case-insensitively equals keyword.
// Helper used to check string tokens against context-sensitive keywords.
func matchesKeywordCI(s, keyword string) bool {
	if len(s) != len(keyword) {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		k := keyword[i]
		if c >= 'a' && c <= 'z' {
			c -= 32
		}
		if k >= 'a' && k <= 'z' {
			k -= 32
		}
		if c != k {
			return false
		}
	}
	return true
}
