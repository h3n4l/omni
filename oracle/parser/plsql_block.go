package parser

import (
	nodes "github.com/bytebase/omni/oracle/ast"
)

// parsePLSQLBlock parses a PL/SQL block:
//
//	[<<label>>] [DECLARE declarations] BEGIN statements [EXCEPTION handlers] END [label] ;
func (p *Parser) parsePLSQLBlock() *nodes.PLSQLBlock {
	start := p.pos()
	block := &nodes.PLSQLBlock{
		Loc: nodes.Loc{Start: start},
	}

	// Optional label: <<label>>
	if p.cur.Type == tokLABELOPEN {
		p.advance() // consume <<
		block.Label = p.parseIdentifier()
		if p.cur.Type == tokLABELCLOSE {
			p.advance() // consume >>
		}
	}

	// DECLARE section (optional)
	if p.cur.Type == kwDECLARE {
		p.advance() // consume DECLARE
		block.Declarations = p.parsePLSQLDeclarations()
	}

	// BEGIN
	if p.cur.Type == kwBEGIN {
		p.advance() // consume BEGIN
	}

	// Statements
	block.Statements = p.parsePLSQLStatements()

	// EXCEPTION section (optional)
	if p.cur.Type == kwEXCEPTION {
		p.advance() // consume EXCEPTION
		block.Exceptions = p.parsePLSQLExceptionHandlers()
	}

	// END [label] ;
	if p.cur.Type == kwEND {
		p.advance() // consume END
	}
	// Optional label after END
	if p.isIdentLike() && p.cur.Type != ';' && p.cur.Type != tokEOF {
		p.advance() // consume label
	}
	if p.cur.Type == ';' {
		p.advance() // consume ;
	}

	block.Loc.End = p.pos()
	return block
}

// parsePLSQLDeclarations parses the DECLARE section of a PL/SQL block.
// Stops when BEGIN is encountered.
func (p *Parser) parsePLSQLDeclarations() *nodes.List {
	decls := &nodes.List{}

	for p.cur.Type != kwBEGIN && p.cur.Type != tokEOF {
		decl := p.parsePLSQLDeclaration()
		if decl == nil {
			break
		}
		decls.Items = append(decls.Items, decl)
	}

	return decls
}

// parsePLSQLDeclaration parses a single declaration.
func (p *Parser) parsePLSQLDeclaration() nodes.Node {
	// CURSOR declaration
	if p.cur.Type == kwCURSOR {
		return p.parsePLSQLCursorDecl()
	}

	// PRAGMA directive
	if p.cur.Type == kwPRAGMA {
		return p.parsePLSQLPragma()
	}

	// TYPE declaration
	if p.cur.Type == kwTYPE {
		next := p.peekNext()
		if next.Type != kwBODY { // not CREATE TYPE BODY
			return p.parsePLSQLTypeDecl()
		}
	}

	// Variable declaration: name [CONSTANT] type [NOT NULL] [:= | DEFAULT expr] ;
	if p.isIdentLike() {
		return p.parsePLSQLVarDecl()
	}

	return nil
}

// parsePLSQLVarDecl parses a variable declaration.
func (p *Parser) parsePLSQLVarDecl() *nodes.PLSQLVarDecl {
	start := p.pos()
	decl := &nodes.PLSQLVarDecl{
		Loc: nodes.Loc{Start: start},
	}

	decl.Name = p.parseIdentifier()

	// Optional CONSTANT
	if p.cur.Type == kwCONSTANT {
		decl.Constant = true
		p.advance()
	}

	// Type name
	decl.TypeName = p.parseTypeName()

	// Optional NOT NULL
	if p.cur.Type == kwNOT {
		next := p.peekNext()
		if next.Type == kwNULL {
			decl.NotNull = true
			p.advance() // consume NOT
			p.advance() // consume NULL
		}
	}

	// Optional default value: := expr or DEFAULT expr
	if p.cur.Type == tokASSIGN {
		p.advance() // consume :=
		decl.Default = p.parseExpr()
	} else if p.cur.Type == kwDEFAULT {
		p.advance() // consume DEFAULT
		decl.Default = p.parseExpr()
	}

	// Semicolon
	if p.cur.Type == ';' {
		p.advance()
	}

	decl.Loc.End = p.pos()
	return decl
}

// parsePLSQLCursorDecl parses a cursor declaration.
//
//	CURSOR name [(params)] IS select_stmt ;
func (p *Parser) parsePLSQLCursorDecl() *nodes.PLSQLCursorDecl {
	start := p.pos()
	p.advance() // consume CURSOR

	decl := &nodes.PLSQLCursorDecl{
		Loc: nodes.Loc{Start: start},
	}

	decl.Name = p.parseIdentifier()

	// Optional parameter list
	if p.cur.Type == '(' {
		decl.Parameters = p.parsePLSQLCursorParams()
	}

	// IS
	if p.cur.Type == kwIS {
		p.advance()
	}

	// SELECT statement
	if p.cur.Type == kwSELECT {
		decl.Query = p.parseSelectStmt()
	}

	// Semicolon
	if p.cur.Type == ';' {
		p.advance()
	}

	decl.Loc.End = p.pos()
	return decl
}

// parsePLSQLCursorParams parses cursor parameter list.
func (p *Parser) parsePLSQLCursorParams() *nodes.List {
	params := &nodes.List{}
	p.advance() // consume (

	for p.cur.Type != ')' && p.cur.Type != tokEOF {
		paramStart := p.pos()
		paramDecl := &nodes.PLSQLVarDecl{
			Loc: nodes.Loc{Start: paramStart},
		}
		paramDecl.Name = p.parseIdentifier()
		// Optional IN keyword (cursor params are always IN)
		if p.cur.Type == kwIN {
			p.advance()
		}
		paramDecl.TypeName = p.parseTypeName()

		// Optional default
		if p.cur.Type == tokASSIGN {
			p.advance()
			paramDecl.Default = p.parseExpr()
		} else if p.cur.Type == kwDEFAULT {
			p.advance()
			paramDecl.Default = p.parseExpr()
		}

		paramDecl.Loc.End = p.pos()
		params.Items = append(params.Items, paramDecl)

		if p.cur.Type != ',' {
			break
		}
		p.advance() // consume ,
	}

	if p.cur.Type == ')' {
		p.advance()
	}
	return params
}

// parsePLSQLStatements parses PL/SQL statements until END, EXCEPTION, ELSIF, ELSE, or EOF.
func (p *Parser) parsePLSQLStatements() *nodes.List {
	stmts := &nodes.List{}

	for {
		// Check for terminators
		switch p.cur.Type {
		case kwEND, kwEXCEPTION, kwELSIF, kwELSE, kwWHEN, tokEOF:
			return stmts
		}

		stmt := p.parsePLSQLStatement()
		if stmt == nil {
			return stmts
		}
		stmts.Items = append(stmts.Items, stmt)
	}
}

// parsePLSQLStatement parses a single PL/SQL statement.
func (p *Parser) parsePLSQLStatement() nodes.StmtNode {
	// Handle optional label before statement
	if p.cur.Type == tokLABELOPEN {
		// This could be a labeled block or labeled loop
		return p.parsePLSQLBlock()
	}

	switch p.cur.Type {
	case kwBEGIN:
		return p.parsePLSQLBlock()

	case kwDECLARE:
		return p.parsePLSQLBlock()

	case kwIF:
		return p.parsePLSQLIf()

	case kwLOOP:
		return p.parsePLSQLBasicLoop()

	case kwWHILE:
		return p.parsePLSQLWhileLoop()

	case kwFOR:
		return p.parsePLSQLForLoop()

	case kwRETURN:
		return p.parsePLSQLReturn()

	case kwGOTO:
		return p.parsePLSQLGoto()

	case kwRAISE:
		return p.parsePLSQLRaise()

	case kwNULL:
		return p.parsePLSQLNull()

	case kwEXECUTE:
		return p.parsePLSQLExecImmediate()

	case kwOPEN:
		return p.parsePLSQLOpen()

	case kwFETCH:
		return p.parsePLSQLFetch()

	case kwCLOSE:
		return p.parsePLSQLClose()

	case kwEXIT:
		return p.parsePLSQLExit()

	case kwCONTINUE:
		return p.parsePLSQLContinue()

	case kwFORALL:
		return p.parsePLSQLForall()

	case kwPIPE:
		return p.parsePLSQLPipeRow()

	case kwCASE:
		return p.parsePLSQLCaseStmt()

	case kwMERGE:
		stmt := p.parseMergeStmt()
		if p.cur.Type == ';' {
			p.advance()
		}
		return stmt

	// DML statements
	case kwSELECT, kwWITH:
		stmt := p.parseSelectStmt()
		if p.cur.Type == ';' {
			p.advance()
		}
		return stmt

	case kwINSERT:
		stmt := p.parseInsertStmt()
		if p.cur.Type == ';' {
			p.advance()
		}
		return stmt

	case kwUPDATE:
		stmt := p.parseUpdateStmt()
		if p.cur.Type == ';' {
			p.advance()
		}
		return stmt

	case kwDELETE:
		stmt := p.parseDeleteStmt()
		if p.cur.Type == ';' {
			p.advance()
		}
		return stmt

	default:
		// Try assignment: target := expr ;
		if p.isIdentLike() || p.cur.Type == tokQIDENT {
			return p.parsePLSQLAssignOrCall()
		}
		return nil
	}
}

// parsePLSQLAssignOrCall parses an assignment statement (target := expr ;)
// or a procedure call (name(args) ;).
func (p *Parser) parsePLSQLAssignOrCall() nodes.StmtNode {
	start := p.pos()

	// Parse the target expression (could be column ref or function call)
	target := p.parseExpr()
	if target == nil {
		return nil
	}

	// Check for := (assignment)
	if p.cur.Type == tokASSIGN {
		p.advance() // consume :=
		value := p.parseExpr()
		if p.cur.Type == ';' {
			p.advance()
		}
		return &nodes.PLSQLAssign{
			Target: target,
			Value:  value,
			Loc:    nodes.Loc{Start: start, End: p.pos()},
		}
	}

	// Otherwise it's a procedure call, consume the semicolon
	if p.cur.Type == ';' {
		p.advance()
	}

	return &nodes.PLSQLCall{
		Name: target,
		Loc:  nodes.Loc{Start: start, End: p.pos()},
	}
}

// parsePLSQLIf parses an IF/ELSIF/ELSE/END IF statement.
//
//	IF condition THEN statements [ELSIF condition THEN statements ...] [ELSE statements] END IF ;
func (p *Parser) parsePLSQLIf() *nodes.PLSQLIf {
	start := p.pos()
	p.advance() // consume IF

	ifStmt := &nodes.PLSQLIf{
		ElsIfs: &nodes.List{},
		Loc:    nodes.Loc{Start: start},
	}

	// Condition
	ifStmt.Condition = p.parseExpr()

	// THEN
	if p.cur.Type == kwTHEN {
		p.advance()
	}

	// Then body
	ifStmt.Then = p.parsePLSQLStatements()

	// ELSIF clauses
	for p.cur.Type == kwELSIF {
		elsifStart := p.pos()
		p.advance() // consume ELSIF

		elsif := &nodes.PLSQLElsIf{
			Loc: nodes.Loc{Start: elsifStart},
		}
		elsif.Condition = p.parseExpr()

		if p.cur.Type == kwTHEN {
			p.advance()
		}

		elsif.Then = p.parsePLSQLStatements()
		elsif.Loc.End = p.pos()
		ifStmt.ElsIfs.Items = append(ifStmt.ElsIfs.Items, elsif)
	}

	// ELSE clause
	if p.cur.Type == kwELSE {
		p.advance()
		ifStmt.Else = p.parsePLSQLStatements()
	}

	// END IF ;
	if p.cur.Type == kwEND {
		p.advance()
	}
	if p.cur.Type == kwIF {
		p.advance()
	}
	if p.cur.Type == ';' {
		p.advance()
	}

	ifStmt.Loc.End = p.pos()
	return ifStmt
}

// parsePLSQLBasicLoop parses a basic LOOP/END LOOP statement.
//
//	LOOP statements END LOOP [label] ;
func (p *Parser) parsePLSQLBasicLoop() *nodes.PLSQLLoop {
	start := p.pos()
	p.advance() // consume LOOP

	loop := &nodes.PLSQLLoop{
		Type: nodes.LOOP_BASIC,
		Loc:  nodes.Loc{Start: start},
	}

	loop.Statements = p.parsePLSQLStatements()

	// END LOOP [label] ;
	p.consumeEndLoop()
	if p.cur.Type == ';' {
		p.advance()
	}

	loop.Loc.End = p.pos()
	return loop
}

// parsePLSQLWhileLoop parses a WHILE LOOP statement.
//
//	WHILE condition LOOP statements END LOOP [label] ;
func (p *Parser) parsePLSQLWhileLoop() *nodes.PLSQLLoop {
	start := p.pos()
	p.advance() // consume WHILE

	loop := &nodes.PLSQLLoop{
		Type: nodes.LOOP_WHILE,
		Loc:  nodes.Loc{Start: start},
	}

	loop.Condition = p.parseExpr()

	if p.cur.Type == kwLOOP {
		p.advance()
	}

	loop.Statements = p.parsePLSQLStatements()

	p.consumeEndLoop()
	if p.cur.Type == ';' {
		p.advance()
	}

	loop.Loc.End = p.pos()
	return loop
}

// parsePLSQLForLoop parses a FOR LOOP statement.
//
//	FOR var IN [REVERSE] lower..upper LOOP statements END LOOP [label] ;
//	FOR rec IN cursor [(args)] LOOP statements END LOOP [label] ;
func (p *Parser) parsePLSQLForLoop() *nodes.PLSQLLoop {
	start := p.pos()
	p.advance() // consume FOR

	loop := &nodes.PLSQLLoop{
		Loc: nodes.Loc{Start: start},
	}

	// Iterator variable
	loop.Iterator = p.parseIdentifier()

	// IN
	if p.cur.Type == kwIN {
		p.advance()
	}

	// REVERSE (optional)
	if p.cur.Type == kwREVERSE {
		loop.Reverse = true
		p.advance()
	}

	// Determine: numeric range (lower..upper) or cursor FOR loop
	// Parse first expression
	expr1 := p.parseExpr()

	if p.cur.Type == tokDOTDOT {
		// Numeric FOR loop: lower..upper
		loop.Type = nodes.LOOP_FOR
		loop.LowerBound = expr1
		p.advance() // consume ..
		loop.UpperBound = p.parseExpr()
	} else {
		// Cursor FOR loop: cursor_name [(args)]
		loop.Type = nodes.LOOP_CURSOR_FOR
		// Extract cursor name from expr1
		if colRef, ok := expr1.(*nodes.ColumnRef); ok {
			loop.CursorName = colRef.Column
		}
		// Optional cursor arguments
		if p.cur.Type == '(' {
			loop.CursorArgs = p.parsePLSQLArgList()
		}
	}

	// LOOP
	if p.cur.Type == kwLOOP {
		p.advance()
	}

	loop.Statements = p.parsePLSQLStatements()

	p.consumeEndLoop()
	if p.cur.Type == ';' {
		p.advance()
	}

	loop.Loc.End = p.pos()
	return loop
}

// consumeEndLoop consumes END LOOP [label].
func (p *Parser) consumeEndLoop() {
	if p.cur.Type == kwEND {
		p.advance()
	}
	if p.cur.Type == kwLOOP {
		p.advance()
	}
	// Optional label after END LOOP
	if p.isIdentLike() && p.cur.Type != ';' && p.cur.Type != tokEOF {
		p.advance()
	}
}

// parsePLSQLReturn parses a RETURN [expr] ; statement.
func (p *Parser) parsePLSQLReturn() *nodes.PLSQLReturn {
	start := p.pos()
	p.advance() // consume RETURN

	ret := &nodes.PLSQLReturn{
		Loc: nodes.Loc{Start: start},
	}

	// Optional expression (not followed by ;)
	if p.cur.Type != ';' && p.cur.Type != tokEOF {
		ret.Expr = p.parseExpr()
	}

	if p.cur.Type == ';' {
		p.advance()
	}

	ret.Loc.End = p.pos()
	return ret
}

// parsePLSQLGoto parses a GOTO label ; statement.
func (p *Parser) parsePLSQLGoto() *nodes.PLSQLGoto {
	start := p.pos()
	p.advance() // consume GOTO

	g := &nodes.PLSQLGoto{
		Label: p.parseIdentifier(),
		Loc:   nodes.Loc{Start: start},
	}

	if p.cur.Type == ';' {
		p.advance()
	}

	g.Loc.End = p.pos()
	return g
}

// parsePLSQLRaise parses a RAISE [exception_name] ; statement.
func (p *Parser) parsePLSQLRaise() *nodes.PLSQLRaise {
	start := p.pos()
	p.advance() // consume RAISE

	r := &nodes.PLSQLRaise{
		Loc: nodes.Loc{Start: start},
	}

	// Optional exception name
	if p.cur.Type != ';' && p.cur.Type != tokEOF && p.isIdentLike() {
		r.Exception = p.parseIdentifier()
	}

	if p.cur.Type == ';' {
		p.advance()
	}

	r.Loc.End = p.pos()
	return r
}

// parsePLSQLNull parses a NULL ; statement.
func (p *Parser) parsePLSQLNull() *nodes.PLSQLNull {
	start := p.pos()
	p.advance() // consume NULL

	if p.cur.Type == ';' {
		p.advance()
	}

	return &nodes.PLSQLNull{
		Loc: nodes.Loc{Start: start, End: p.pos()},
	}
}

// parsePLSQLExecImmediate parses EXECUTE IMMEDIATE expr [INTO vars] [USING vars] ;
func (p *Parser) parsePLSQLExecImmediate() *nodes.PLSQLExecImmediate {
	start := p.pos()
	p.advance() // consume EXECUTE

	// IMMEDIATE
	if p.cur.Type == kwIMMEDIATE {
		p.advance()
	}

	stmt := &nodes.PLSQLExecImmediate{
		Loc: nodes.Loc{Start: start},
	}

	stmt.SQL = p.parseExpr()

	// INTO
	if p.cur.Type == kwINTO {
		p.advance()
		stmt.Into = p.parsePLSQLVarList()
	}

	// USING
	if p.cur.Type == kwUSING {
		p.advance()
		stmt.Using = p.parsePLSQLVarList()
	}

	if p.cur.Type == ';' {
		p.advance()
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parsePLSQLOpen parses OPEN cursor [(args)] [FOR query] ;
func (p *Parser) parsePLSQLOpen() *nodes.PLSQLOpen {
	start := p.pos()
	p.advance() // consume OPEN

	o := &nodes.PLSQLOpen{
		Loc: nodes.Loc{Start: start},
	}

	o.Cursor = p.parseIdentifier()

	// Optional arguments
	if p.cur.Type == '(' {
		o.Args = p.parsePLSQLArgList()
	}

	// FOR query
	if p.cur.Type == kwFOR {
		p.advance()
		if p.cur.Type == kwSELECT || p.cur.Type == kwWITH {
			o.ForQuery = p.parseSelectStmt()
		}
	}

	if p.cur.Type == ';' {
		p.advance()
	}

	o.Loc.End = p.pos()
	return o
}

// parsePLSQLFetch parses FETCH cursor INTO vars [BULK COLLECT INTO vars] [LIMIT expr] ;
func (p *Parser) parsePLSQLFetch() *nodes.PLSQLFetch {
	start := p.pos()
	p.advance() // consume FETCH

	f := &nodes.PLSQLFetch{
		Loc: nodes.Loc{Start: start},
	}

	f.Cursor = p.parseIdentifier()

	// BULK COLLECT INTO
	if p.cur.Type == kwBULK {
		f.Bulk = true
		p.advance() // consume BULK
		if p.cur.Type == kwCOLLECT {
			p.advance() // consume COLLECT
		}
	}

	// INTO
	if p.cur.Type == kwINTO {
		p.advance()
		f.Into = p.parsePLSQLVarList()
	}

	// LIMIT
	if p.cur.Type == kwLIMIT {
		p.advance()
		f.Limit = p.parseExpr()
	}

	if p.cur.Type == ';' {
		p.advance()
	}

	f.Loc.End = p.pos()
	return f
}

// parsePLSQLClose parses CLOSE cursor ;
func (p *Parser) parsePLSQLClose() *nodes.PLSQLClose {
	start := p.pos()
	p.advance() // consume CLOSE

	c := &nodes.PLSQLClose{
		Cursor: p.parseIdentifier(),
		Loc:    nodes.Loc{Start: start},
	}

	if p.cur.Type == ';' {
		p.advance()
	}

	c.Loc.End = p.pos()
	return c
}

// parsePLSQLExceptionHandlers parses exception handlers.
//
//	WHEN name [OR name ...] THEN statements
func (p *Parser) parsePLSQLExceptionHandlers() *nodes.List {
	handlers := &nodes.List{}

	for p.cur.Type == kwWHEN {
		start := p.pos()
		p.advance() // consume WHEN

		handler := &nodes.ExceptionHandler{
			Exceptions: &nodes.List{},
			Loc:        nodes.Loc{Start: start},
		}

		// Exception name(s)
		name := p.parseIdentifier()
		handler.Exceptions.Items = append(handler.Exceptions.Items, &nodes.String{Str: name})

		for p.cur.Type == kwOR {
			p.advance() // consume OR
			name = p.parseIdentifier()
			handler.Exceptions.Items = append(handler.Exceptions.Items, &nodes.String{Str: name})
		}

		// THEN
		if p.cur.Type == kwTHEN {
			p.advance()
		}

		handler.Statements = p.parsePLSQLStatements()
		handler.Loc.End = p.pos()

		handlers.Items = append(handlers.Items, handler)
	}

	return handlers
}

// parsePLSQLVarList parses a comma-separated list of variable references.
func (p *Parser) parsePLSQLVarList() *nodes.List {
	list := &nodes.List{}

	for {
		expr := p.parseExpr()
		if expr != nil {
			list.Items = append(list.Items, expr)
		}
		if p.cur.Type != ',' {
			break
		}
		p.advance() // consume ,
	}

	return list
}

// parsePLSQLArgList parses a parenthesized argument list.
func (p *Parser) parsePLSQLArgList() *nodes.List {
	args := &nodes.List{}
	p.advance() // consume (

	for p.cur.Type != ')' && p.cur.Type != tokEOF {
		expr := p.parseExpr()
		if expr != nil {
			args.Items = append(args.Items, expr)
		}
		if p.cur.Type != ',' {
			break
		}
		p.advance() // consume ,
	}

	if p.cur.Type == ')' {
		p.advance()
	}

	return args
}

// parsePLSQLExit parses an EXIT [label] [WHEN condition] ; statement.
func (p *Parser) parsePLSQLExit() nodes.StmtNode {
	start := p.pos()
	p.advance() // consume EXIT

	stmt := &nodes.PLSQLExit{
		Loc: nodes.Loc{Start: start},
	}

	// Optional label (before WHEN or ;)
	if p.isIdentLike() && p.cur.Type != kwWHEN {
		stmt.Label = p.cur.Str
		p.advance()
	}

	// WHEN condition
	if p.cur.Type == kwWHEN {
		p.advance()
		stmt.Condition = p.parseExpr()
	}

	if p.cur.Type == ';' {
		p.advance()
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parsePLSQLContinue parses a CONTINUE [label] [WHEN condition] ; statement.
func (p *Parser) parsePLSQLContinue() nodes.StmtNode {
	start := p.pos()
	p.advance() // consume CONTINUE

	stmt := &nodes.PLSQLContinue{
		Loc: nodes.Loc{Start: start},
	}

	// Optional label
	if p.isIdentLike() && p.cur.Type != kwWHEN {
		stmt.Label = p.cur.Str
		p.advance()
	}

	// WHEN condition
	if p.cur.Type == kwWHEN {
		p.advance()
		stmt.Condition = p.parseExpr()
	}

	if p.cur.Type == ';' {
		p.advance()
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parsePLSQLForall parses a FORALL statement.
//
//	FORALL index IN lower..upper [SAVE EXCEPTIONS] dml_statement ;
func (p *Parser) parsePLSQLForall() nodes.StmtNode {
	start := p.pos()
	p.advance() // consume FORALL

	stmt := &nodes.PLSQLForall{
		Loc: nodes.Loc{Start: start},
	}

	// Index variable
	if p.isIdentLike() {
		stmt.Index = p.cur.Str
		p.advance()
	}

	// IN
	if p.cur.Type == kwIN {
		p.advance()
	}

	// lower..upper or VALUES OF or INDICES OF
	if p.isIdentLike() && (p.cur.Str == "VALUES" || p.cur.Str == "INDICES") {
		// VALUES OF / INDICES OF - skip to DML
		p.advance()
		if p.cur.Type == kwOF {
			p.advance()
		}
		// collection name
		stmt.Lower = p.parseExpr()
	} else {
		stmt.Lower = p.parseExpr()
		if p.cur.Type == tokDOTDOT {
			p.advance()
			stmt.Upper = p.parseExpr()
		}
	}

	// Optional SAVE EXCEPTIONS
	if p.isIdentLike() && p.cur.Str == "SAVE" {
		p.advance()
		if p.cur.Type == kwEXCEPTION || p.isIdentLike() {
			p.advance()
		}
	}

	// DML statement body
	stmt.Body = p.parsePLSQLStatement()

	stmt.Loc.End = p.pos()
	return stmt
}

// parsePLSQLPipeRow parses a PIPE ROW statement.
//
//	PIPE ROW (expression) ;
func (p *Parser) parsePLSQLPipeRow() nodes.StmtNode {
	start := p.pos()
	p.advance() // consume PIPE

	// ROW
	if p.cur.Type == kwROW || (p.isIdentLike() && p.cur.Str == "ROW") {
		p.advance()
	}

	stmt := &nodes.PLSQLPipeRow{
		Loc: nodes.Loc{Start: start},
	}

	// (expression)
	if p.cur.Type == '(' {
		p.advance()
		stmt.Row = p.parseExpr()
		if p.cur.Type == ')' {
			p.advance()
		}
	}

	if p.cur.Type == ';' {
		p.advance()
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parsePLSQLPragma parses a PRAGMA directive.
//
//	PRAGMA AUTONOMOUS_TRANSACTION ;
//	PRAGMA EXCEPTION_INIT ( exception, error_code ) ;
//	PRAGMA RESTRICT_REFERENCES ( ... ) ;
func (p *Parser) parsePLSQLPragma() *nodes.PLSQLPragma {
	start := p.pos()
	p.advance() // consume PRAGMA

	pragma := &nodes.PLSQLPragma{
		Loc: nodes.Loc{Start: start},
	}

	// Pragma name
	if p.isIdentLike() {
		pragma.Name = p.cur.Str
		p.advance()
	}

	// Optional arguments in parentheses
	if p.cur.Type == '(' {
		p.advance()
		pragma.Args = &nodes.List{}
		for p.cur.Type != ')' && p.cur.Type != tokEOF {
			arg := p.parseExpr()
			if arg != nil {
				pragma.Args.Items = append(pragma.Args.Items, arg)
			}
			if p.cur.Type != ',' {
				break
			}
			p.advance()
		}
		if p.cur.Type == ')' {
			p.advance()
		}
	}

	if p.cur.Type == ';' {
		p.advance()
	}

	pragma.Loc.End = p.pos()
	return pragma
}

// parsePLSQLCaseStmt parses a PL/SQL CASE statement (distinct from CASE expression).
//
//	CASE [expr]
//	  WHEN expr THEN statements ...
//	  [ELSE statements]
//	END CASE ;
func (p *Parser) parsePLSQLCaseStmt() nodes.StmtNode {
	start := p.pos()
	p.advance() // consume CASE

	stmt := &nodes.PLSQLCase{
		Loc: nodes.Loc{Start: start},
	}

	// Optional search expression (simple CASE vs searched CASE)
	// If next is WHEN, it's a searched CASE
	if p.cur.Type != kwWHEN {
		stmt.Expr = p.parseExpr()
	}

	// WHEN clauses
	for p.cur.Type == kwWHEN {
		whenStart := p.pos()
		p.advance() // consume WHEN
		when := &nodes.PLSQLWhen{
			Loc: nodes.Loc{Start: whenStart},
		}
		when.Expr = p.parseExpr()
		if p.cur.Type == kwTHEN {
			p.advance()
		}
		// Statements until next WHEN, ELSE, or END
		for p.cur.Type != kwWHEN && p.cur.Type != kwELSE && p.cur.Type != kwEND && p.cur.Type != tokEOF {
			s := p.parsePLSQLStatement()
			if s == nil {
				break
			}
			when.Stmts = append(when.Stmts, s)
		}
		when.Loc.End = p.pos()
		stmt.Whens = append(stmt.Whens, when)
	}

	// ELSE
	if p.cur.Type == kwELSE {
		p.advance()
		for p.cur.Type != kwEND && p.cur.Type != tokEOF {
			s := p.parsePLSQLStatement()
			if s == nil {
				break
			}
			stmt.Else = append(stmt.Else, s)
		}
	}

	// END CASE ;
	if p.cur.Type == kwEND {
		p.advance()
	}
	if p.cur.Type == kwCASE {
		p.advance()
	}
	if p.cur.Type == ';' {
		p.advance()
	}

	stmt.Loc.End = p.pos()
	return stmt
}

// parsePLSQLTypeDecl parses a PL/SQL TYPE declaration.
//
//	TYPE name IS TABLE OF type [INDEX BY type] ;
//	TYPE name IS VARRAY(n) OF type ;
//	TYPE name IS RECORD (field type [,...]) ;
//	TYPE name IS REF CURSOR [RETURN type] ;
func (p *Parser) parsePLSQLTypeDecl() *nodes.PLSQLTypeDecl {
	start := p.pos()
	p.advance() // consume TYPE

	decl := &nodes.PLSQLTypeDecl{
		Loc: nodes.Loc{Start: start},
	}

	// Type name
	if p.isIdentLike() {
		decl.Name = p.cur.Str
		p.advance()
	}

	// IS
	if p.cur.Type == kwIS {
		p.advance()
	}

	// TABLE OF / VARRAY / RECORD / REF CURSOR
	switch {
	case p.cur.Type == kwTABLE:
		decl.Kind = "TABLE"
		p.advance()
		if p.cur.Type == kwOF {
			p.advance()
		}
		decl.ElementType = p.parseTypeName()
		// INDEX BY
		if p.cur.Type == kwINDEX {
			p.advance()
			if p.cur.Type == kwBY {
				p.advance()
			}
			decl.IndexBy = p.parseTypeName()
		}

	case p.isIdentLike() && p.cur.Str == "VARRAY" || p.isIdentLike() && p.cur.Str == "VARYING":
		decl.Kind = "VARRAY"
		p.advance()
		// Optional ARRAY keyword
		if p.isIdentLike() && p.cur.Str == "ARRAY" {
			p.advance()
		}
		// (limit)
		if p.cur.Type == '(' {
			p.advance()
			decl.Limit = p.parseExpr()
			if p.cur.Type == ')' {
				p.advance()
			}
		}
		if p.cur.Type == kwOF {
			p.advance()
		}
		decl.ElementType = p.parseTypeName()

	case p.isIdentLike() && p.cur.Str == "RECORD":
		decl.Kind = "RECORD"
		p.advance()
		// (field_name type [,...])
		if p.cur.Type == '(' {
			p.advance()
			decl.Fields = &nodes.List{}
			for p.cur.Type != ')' && p.cur.Type != tokEOF {
				field := p.parsePLSQLVarDecl()
				if field != nil {
					decl.Fields.Items = append(decl.Fields.Items, field)
				}
				if p.cur.Type == ',' {
					p.advance()
				} else {
					break
				}
			}
			if p.cur.Type == ')' {
				p.advance()
			}
		}

	case p.cur.Type == kwREF:
		decl.Kind = "REF_CURSOR"
		p.advance()
		if p.cur.Type == kwCURSOR {
			p.advance()
		}
		// RETURN type
		if p.cur.Type == kwRETURN {
			p.advance()
			decl.ReturnType = p.parseTypeName()
		}
	}

	if p.cur.Type == ';' {
		p.advance()
	}

	decl.Loc.End = p.pos()
	return decl
}
