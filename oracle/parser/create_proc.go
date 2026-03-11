package parser

import (
	nodes "github.com/bytebase/omni/oracle/ast"
)

// parseCreateProcedureStmt parses a CREATE [OR REPLACE] PROCEDURE statement.
//
// Ref: https://docs.oracle.com/en/database/oracle/oracle-database/23/lnpls/CREATE-PROCEDURE-statement.html
//
//	CREATE [OR REPLACE] PROCEDURE [schema.]name [(parameter [, ...])]
//	  { IS | AS }
//	  plsql_block
func (p *Parser) parseCreateProcedureStmt(start int, orReplace bool) *nodes.CreateProcedureStmt {
	p.advance() // consume PROCEDURE

	stmt := &nodes.CreateProcedureStmt{
		OrReplace: orReplace,
		Loc:       nodes.Loc{Start: start},
	}

	// Procedure name
	stmt.Name = p.parseObjectName()

	// Optional parameter list
	if p.cur.Type == '(' {
		stmt.Parameters = p.parseParameterList()
	}

	// IS | AS
	if p.cur.Type == kwIS || p.cur.Type == kwAS {
		p.advance()
	}

	// PL/SQL block body (BEGIN ... END)
	stmt.Body = p.parsePLSQLBlock()

	stmt.Loc.End = p.pos()
	return stmt
}

// parseCreateFunctionStmt parses a CREATE [OR REPLACE] FUNCTION statement.
//
// Ref: https://docs.oracle.com/en/database/oracle/oracle-database/23/lnpls/CREATE-FUNCTION-statement.html
//
//	CREATE [OR REPLACE] FUNCTION [schema.]name [(parameter [, ...])]
//	  RETURN datatype
//	  [DETERMINISTIC] [PIPELINED] [PARALLEL_ENABLE] [RESULT_CACHE]
//	  { IS | AS }
//	  plsql_block
func (p *Parser) parseCreateFunctionStmt(start int, orReplace bool) *nodes.CreateFunctionStmt {
	p.advance() // consume FUNCTION

	stmt := &nodes.CreateFunctionStmt{
		OrReplace: orReplace,
		Loc:       nodes.Loc{Start: start},
	}

	// Function name
	stmt.Name = p.parseObjectName()

	// Optional parameter list
	if p.cur.Type == '(' {
		stmt.Parameters = p.parseParameterList()
	}

	// RETURN type
	if p.cur.Type == kwRETURN {
		p.advance() // consume RETURN
		stmt.ReturnType = p.parseTypeName()
	}

	// Optional function properties (can appear in any order before IS/AS)
	p.parseFunctionProperties(stmt)

	// IS | AS
	if p.cur.Type == kwIS || p.cur.Type == kwAS {
		p.advance()
	}

	// PL/SQL block body (BEGIN ... END)
	stmt.Body = p.parsePLSQLBlock()

	stmt.Loc.End = p.pos()
	return stmt
}

// parseFunctionProperties parses optional DETERMINISTIC, PIPELINED, PARALLEL_ENABLE, RESULT_CACHE.
func (p *Parser) parseFunctionProperties(stmt *nodes.CreateFunctionStmt) {
	for {
		switch p.cur.Type {
		case kwDETERMINISTIC:
			stmt.Deterministic = true
			p.advance()
		case kwPIPELINED:
			stmt.Pipelined = true
			p.advance()
		case kwPARALLEL_ENABLE:
			stmt.Parallel = true
			p.advance()
		case kwRESULT_CACHE:
			stmt.ResultCache = true
			p.advance()
		default:
			return
		}
	}
}

// parseCreatePackageStmt parses a CREATE [OR REPLACE] PACKAGE [BODY] statement.
//
// Ref: https://docs.oracle.com/en/database/oracle/oracle-database/23/lnpls/CREATE-PACKAGE-statement.html
//
//	CREATE [OR REPLACE] PACKAGE [schema.]name { IS | AS }
//	  declarations...
//	END [name] ;
//
//	CREATE [OR REPLACE] PACKAGE BODY [schema.]name { IS | AS }
//	  declarations...
//	  [BEGIN statements [EXCEPTION handlers]]
//	END [name] ;
func (p *Parser) parseCreatePackageStmt(start int, orReplace bool) *nodes.CreatePackageStmt {
	p.advance() // consume PACKAGE

	stmt := &nodes.CreatePackageStmt{
		OrReplace: orReplace,
		Loc:       nodes.Loc{Start: start},
	}

	// Check for BODY keyword
	if p.cur.Type == kwBODY {
		stmt.IsBody = true
		p.advance() // consume BODY
	}

	// Package name
	stmt.Name = p.parseObjectName()

	// IS | AS
	if p.cur.Type == kwIS || p.cur.Type == kwAS {
		p.advance()
	}

	// Package declarations/body - collect everything until END
	stmt.Body = p.parsePackageBody()

	// END [name] ;
	if p.cur.Type == kwEND {
		p.advance() // consume END
	}
	// Optional package name after END
	if p.isIdentLike() && p.cur.Type != ';' && p.cur.Type != tokEOF {
		p.advance() // consume name
	}
	if p.cur.Type == ';' {
		p.advance() // consume ;
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parsePackageBody parses the declarations inside a package specification or body.
// Stops when END is encountered.
func (p *Parser) parsePackageBody() *nodes.List {
	decls := &nodes.List{}

	for p.cur.Type != kwEND && p.cur.Type != tokEOF {
		// Skip standalone semicolons
		if p.cur.Type == ';' {
			p.advance()
			continue
		}

		// PROCEDURE declaration/definition in package
		if p.cur.Type == kwPROCEDURE {
			decl := p.parsePackageProcDecl()
			if decl != nil {
				decls.Items = append(decls.Items, decl)
			}
			continue
		}

		// FUNCTION declaration/definition in package
		if p.cur.Type == kwFUNCTION {
			decl := p.parsePackageFuncDecl()
			if decl != nil {
				decls.Items = append(decls.Items, decl)
			}
			continue
		}

		// BEGIN section (package body initialization)
		if p.cur.Type == kwBEGIN {
			break
		}

		// Variable/type/cursor declarations
		decl := p.parsePLSQLDeclaration()
		if decl == nil {
			// If we can't parse anything, skip a token to avoid infinite loop
			p.advance()
			continue
		}
		decls.Items = append(decls.Items, decl)
	}

	return decls
}

// parsePackageProcDecl parses a PROCEDURE declaration or definition inside a package.
//
//	PROCEDURE name [(params)] ;                    -- specification
//	PROCEDURE name [(params)] IS|AS body ;         -- body definition
func (p *Parser) parsePackageProcDecl() *nodes.CreateProcedureStmt {
	start := p.pos()
	p.advance() // consume PROCEDURE

	stmt := &nodes.CreateProcedureStmt{
		Loc: nodes.Loc{Start: start},
	}

	stmt.Name = p.parseObjectName()

	// Optional parameter list
	if p.cur.Type == '(' {
		stmt.Parameters = p.parseParameterList()
	}

	// Check for IS|AS (definition) or ; (declaration)
	if p.cur.Type == kwIS || p.cur.Type == kwAS {
		p.advance()
		stmt.Body = p.parsePLSQLBlock()
	} else if p.cur.Type == ';' {
		p.advance()
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parsePackageFuncDecl parses a FUNCTION declaration or definition inside a package.
//
//	FUNCTION name [(params)] RETURN type ;                    -- specification
//	FUNCTION name [(params)] RETURN type IS|AS body ;         -- body definition
func (p *Parser) parsePackageFuncDecl() *nodes.CreateFunctionStmt {
	start := p.pos()
	p.advance() // consume FUNCTION

	stmt := &nodes.CreateFunctionStmt{
		Loc: nodes.Loc{Start: start},
	}

	stmt.Name = p.parseObjectName()

	// Optional parameter list
	if p.cur.Type == '(' {
		stmt.Parameters = p.parseParameterList()
	}

	// RETURN type
	if p.cur.Type == kwRETURN {
		p.advance()
		stmt.ReturnType = p.parseTypeName()
	}

	// Optional function properties
	p.parseFunctionProperties(stmt)

	// Check for IS|AS (definition) or ; (declaration)
	if p.cur.Type == kwIS || p.cur.Type == kwAS {
		p.advance()
		stmt.Body = p.parsePLSQLBlock()
	} else if p.cur.Type == ';' {
		p.advance()
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parseParameterList parses a parenthesized parameter list: ( param1, param2, ... )
func (p *Parser) parseParameterList() *nodes.List {
	params := &nodes.List{}
	p.advance() // consume '('

	for p.cur.Type != ')' && p.cur.Type != tokEOF {
		param := p.parseParameter()
		if param != nil {
			params.Items = append(params.Items, param)
		}

		if p.cur.Type != ',' {
			break
		}
		p.advance() // consume ','
	}

	if p.cur.Type == ')' {
		p.advance() // consume ')'
	}

	return params
}

// parseParameter parses a single parameter declaration.
//
//	name [IN | OUT | IN OUT] [NOCOPY] type [{:= | DEFAULT} expr]
func (p *Parser) parseParameter() *nodes.Parameter {
	start := p.pos()
	param := &nodes.Parameter{
		Loc: nodes.Loc{Start: start},
	}

	// Parameter name
	param.Name = p.parseIdentifier()
	if param.Name == "" {
		return nil
	}

	// Optional mode: IN, OUT, IN OUT
	mode := p.parseParameterMode()
	param.Mode = mode

	// Type name
	param.TypeName = p.parseTypeName()

	// Optional default value: := expr or DEFAULT expr
	if p.cur.Type == tokASSIGN {
		p.advance() // consume :=
		param.Default = p.parseExpr()
	} else if p.cur.Type == kwDEFAULT {
		p.advance() // consume DEFAULT
		param.Default = p.parseExpr()
	}

	param.Loc.End = p.pos()
	return param
}

// parseParameterMode parses the optional IN/OUT/IN OUT/NOCOPY mode keywords.
func (p *Parser) parseParameterMode() string {
	if p.cur.Type == kwIN {
		next := p.peekNext()
		if next.Type == kwOUT {
			p.advance() // consume IN
			p.advance() // consume OUT
			// Optional NOCOPY after IN OUT
			if p.cur.Type == kwNOCOPY {
				p.advance()
				return "IN OUT NOCOPY"
			}
			return "IN OUT"
		}
		p.advance() // consume IN
		return "IN"
	}
	if p.cur.Type == kwOUT {
		p.advance() // consume OUT
		// Optional NOCOPY after OUT
		if p.cur.Type == kwNOCOPY {
			p.advance()
			return "OUT NOCOPY"
		}
		return "OUT"
	}
	return ""
}
