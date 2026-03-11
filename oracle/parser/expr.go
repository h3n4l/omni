package parser

import (
	nodes "github.com/bytebase/omni/oracle/ast"
)

// Precedence levels for Pratt parsing.
const (
	precNone    = 0
	precOr      = 1
	precAnd     = 2
	precNot     = 3
	precIs      = 4
	precComp    = 5
	precLike    = 6
	precConcat  = 7
	precAdd     = 8
	precMul     = 9
	precUnary   = 10
	precExpon   = 11
	precPrimary = 12
)

// parseExpr parses an expression using Pratt parsing / precedence climbing.
//
// Ref: https://docs.oracle.com/en/database/oracle/oracle-database/23/sqlrf/About-SQL-Expressions.html
//
//	expression ::= or_expr
//	or_expr    ::= and_expr { OR and_expr }
//	and_expr   ::= not_expr { AND not_expr }
//	not_expr   ::= NOT not_expr | comparison_expr
//	...
func (p *Parser) parseExpr() nodes.ExprNode {
	return p.parseExprPrec(precOr)
}

// parseExprPrec parses expressions at or above the given precedence level.
func (p *Parser) parseExprPrec(minPrec int) nodes.ExprNode {
	left := p.parsePrefix()
	if left == nil {
		return nil
	}

	// Handle MODEL cell reference subscript: expr[dim1, dim2, ...]
	left = p.parseSubscriptIfPresent(left)

	for {
		prec, op, isBool := p.infixInfo()
		if prec < minPrec {
			break
		}

		if isBool {
			left = p.parseBoolInfix(left, prec, op)
		} else {
			left = p.parseBinaryInfix(left, prec, op)
		}
	}

	// Check for postfix operators: IS, BETWEEN, IN, LIKE, NOT BETWEEN/IN/LIKE
	left = p.parsePostfix(left)

	return left
}

// infixInfo returns the precedence, operator string, and whether it's a boolean op
// for the current token if it's an infix operator.
func (p *Parser) infixInfo() (int, string, bool) {
	switch p.cur.Type {
	case kwOR:
		return precOr, "OR", true
	case kwAND:
		return precAnd, "AND", true
	case '=':
		return precComp, "=", false
	case tokNOTEQ:
		return precComp, p.cur.Str, false
	case '<':
		return precComp, "<", false
	case '>':
		return precComp, ">", false
	case tokLESSEQ:
		return precComp, "<=", false
	case tokGREATEQ:
		return precComp, ">=", false
	case tokCONCAT:
		return precConcat, "||", false
	case '+':
		return precAdd, "+", false
	case '-':
		return precAdd, "-", false
	case '*':
		return precMul, "*", false
	case '/':
		return precMul, "/", false
	case tokEXPON:
		return precExpon, "**", false
	}
	return precNone, "", false
}

// parseBinaryInfix parses a binary infix expression.
func (p *Parser) parseBinaryInfix(left nodes.ExprNode, prec int, op string) nodes.ExprNode {
	locStart := p.pos()
	p.advance() // consume operator

	// Right-associative for exponentiation
	nextPrec := prec + 1
	if op == "**" {
		nextPrec = prec
	}

	right := p.parseExprPrec(nextPrec)

	return &nodes.BinaryExpr{
		Op:    op,
		Left:  left,
		Right: right,
		Loc:   nodes.Loc{Start: locStart, End: p.pos()},
	}
}

// parseBoolInfix parses AND/OR boolean expressions.
func (p *Parser) parseBoolInfix(left nodes.ExprNode, prec int, op string) nodes.ExprNode {
	start := p.pos()
	var boolop nodes.BoolExprType
	if op == "AND" {
		boolop = nodes.BOOL_AND
	} else {
		boolop = nodes.BOOL_OR
	}

	p.advance() // consume AND/OR

	right := p.parseExprPrec(prec + 1)

	// Flatten: if left is the same bool type, merge args
	if be, ok := left.(*nodes.BoolExpr); ok && be.Boolop == boolop {
		be.Args.Items = append(be.Args.Items, right)
		return be
	}

	return &nodes.BoolExpr{
		Boolop: boolop,
		Args:   &nodes.List{Items: []nodes.Node{left, right}},
		Loc:    nodes.Loc{Start: start, End: p.pos()},
	}
}

// parsePrefix parses a prefix expression (unary operators and primary expressions).
func (p *Parser) parsePrefix() nodes.ExprNode {
	start := p.pos()

	switch p.cur.Type {
	case kwNOT:
		p.advance()
		operand := p.parseExprPrec(precNot)
		return &nodes.BoolExpr{
			Boolop: nodes.BOOL_NOT,
			Args:   &nodes.List{Items: []nodes.Node{operand}},
			Loc:    nodes.Loc{Start: start, End: p.pos()},
		}

	case '-':
		p.advance()
		operand := p.parseExprPrec(precUnary)
		return &nodes.UnaryExpr{
			Op:      "-",
			Operand: operand,
			Loc:     nodes.Loc{Start: start, End: p.pos()},
		}

	case '+':
		p.advance()
		operand := p.parseExprPrec(precUnary)
		return &nodes.UnaryExpr{
			Op:      "+",
			Operand: operand,
			Loc:     nodes.Loc{Start: start, End: p.pos()},
		}

	case kwPRIOR:
		p.advance()
		operand := p.parseExprPrec(precUnary)
		return &nodes.UnaryExpr{
			Op:      "PRIOR",
			Operand: operand,
			Loc:     nodes.Loc{Start: start, End: p.pos()},
		}

	case kwCONNECT_BY_ROOT:
		p.advance()
		operand := p.parseExprPrec(precUnary)
		return &nodes.UnaryExpr{
			Op:      "CONNECT_BY_ROOT",
			Operand: operand,
			Loc:     nodes.Loc{Start: start, End: p.pos()},
		}

	default:
		return p.parsePrimary()
	}
}

// parsePrimary parses a primary expression (literals, identifiers, function calls, etc).
func (p *Parser) parsePrimary() nodes.ExprNode {
	start := p.pos()

	switch p.cur.Type {
	case tokICONST:
		tok := p.advance()
		return &nodes.NumberLiteral{
			Val:  tok.Str,
			Ival: tok.Ival,
			Loc:  nodes.Loc{Start: start, End: p.pos()},
		}

	case tokFCONST:
		tok := p.advance()
		return &nodes.NumberLiteral{
			Val:     tok.Str,
			IsFloat: true,
			Loc:     nodes.Loc{Start: start, End: p.pos()},
		}

	case tokSCONST:
		tok := p.advance()
		return &nodes.StringLiteral{
			Val: tok.Str,
			Loc: nodes.Loc{Start: start, End: p.pos()},
		}

	case tokNCHARLIT:
		tok := p.advance()
		return &nodes.StringLiteral{
			Val:     tok.Str,
			IsNChar: true,
			Loc:     nodes.Loc{Start: start, End: p.pos()},
		}

	case kwNULL:
		p.advance()
		return &nodes.NullLiteral{
			Loc: nodes.Loc{Start: start, End: p.pos()},
		}

	case tokBIND:
		return p.parseBindVariable()

	case '*':
		p.advance()
		return &nodes.Star{
			Loc: nodes.Loc{Start: start, End: p.pos()},
		}

	case '(':
		return p.parseParenExpr()

	case kwCASE:
		return p.parseCaseExpr()

	case kwCAST:
		return p.parseCastExpr()

	case kwDECODE:
		return p.parseDecodeExpr()

	case kwEXISTS:
		return p.parseExistsExpr()

	default:
		// Pseudo columns
		if p.isPseudoColumn() {
			return p.parsePseudoColumn()
		}

		// Identifier — could be column ref, function call, or keyword-as-identifier
		if p.isIdentLike() {
			return p.parseIdentExpr()
		}

		return nil
	}
}

// parseIdentExpr parses an identifier-starting expression.
// It could be a function call (name(...)), a column ref (name or table.column), etc.
func (p *Parser) parseIdentExpr() nodes.ExprNode {
	start := p.pos()
	name1 := p.parseIdentifier()
	if name1 == "" {
		return nil
	}

	// Check for function call: name(
	if p.cur.Type == '(' {
		// But first check if this looks like a schema-qualified function: name.name(
		// No — just name( is the function call case
		return p.parseFuncCall(name1, "", start)
	}

	// Check for schema.name or table.column
	if p.cur.Type == '.' {
		p.advance() // consume '.'

		// table.*
		if p.cur.Type == '*' {
			p.advance()
			return &nodes.ColumnRef{
				Table:  name1,
				Column: "*",
				Loc:    nodes.Loc{Start: start, End: p.pos()},
			}
		}

		name2 := p.parseIdentifier()
		if name2 == "" {
			return &nodes.ColumnRef{
				Column: name1,
				Loc:    nodes.Loc{Start: start, End: p.pos()},
			}
		}

		// schema.func( ?
		if p.cur.Type == '(' {
			return p.parseFuncCall(name2, name1, start)
		}

		// schema.table.column or schema.table.*
		if p.cur.Type == '.' {
			p.advance()
			if p.cur.Type == '*' {
				p.advance()
				return &nodes.ColumnRef{
					Schema: name1,
					Table:  name2,
					Column: "*",
					Loc:    nodes.Loc{Start: start, End: p.pos()},
				}
			}
			name3 := p.parseIdentifier()
			return &nodes.ColumnRef{
				Schema: name1,
				Table:  name2,
				Column: name3,
				Loc:    nodes.Loc{Start: start, End: p.pos()},
			}
		}

		// table.column
		return &nodes.ColumnRef{
			Table:  name1,
			Column: name2,
			Loc:    nodes.Loc{Start: start, End: p.pos()},
		}
	}

	// Simple column reference
	return &nodes.ColumnRef{
		Column: name1,
		Loc:    nodes.Loc{Start: start, End: p.pos()},
	}
}

// parseSubscriptIfPresent checks if the current token is '[' and parses a
// MODEL cell reference subscript: expr[dim1, dim2, ...].
// Returns the original expression if no '[' is found.
func (p *Parser) parseSubscriptIfPresent(expr nodes.ExprNode) nodes.ExprNode {
	if p.cur.Type != '[' {
		return expr
	}

	// Determine start position and function name from the expression
	var start int
	funcName := ""
	switch e := expr.(type) {
	case *nodes.ColumnRef:
		funcName = e.Column
		start = e.Loc.Start
	default:
		start = p.pos()
	}
	p.advance() // consume '['

	args := &nodes.List{}
	for {
		if p.cur.Type == ']' {
			break
		}
		arg := p.parseExpr()
		if arg == nil {
			break
		}
		args.Items = append(args.Items, arg)
		if p.cur.Type != ',' {
			break
		}
		p.advance()
	}

	if p.cur.Type == ']' {
		p.advance()
	}

	// Represent subscript access as a FuncCallExpr (reusing the existing node type).
	// This is a MODEL cell reference: measure[dim1, dim2]
	return &nodes.FuncCallExpr{
		FuncName: &nodes.ObjectName{Name: funcName, Loc: nodes.Loc{Start: start}},
		Args:     args,
		Loc:      nodes.Loc{Start: start, End: p.pos()},
	}
}

// parseFuncCall parses a function call after the name has been consumed.
// The opening '(' is the current token.
func (p *Parser) parseFuncCall(name, schema string, start int) nodes.ExprNode {
	fc := &nodes.FuncCallExpr{
		FuncName: &nodes.ObjectName{
			Schema: schema,
			Name:   name,
		},
		Args: &nodes.List{},
		Loc:  nodes.Loc{Start: start},
	}

	p.advance() // consume '('

	// COUNT(*) special case
	if p.cur.Type == '*' {
		p.advance()
		fc.Star = true
		if p.cur.Type == ')' {
			p.advance()
		}
		fc.Loc.End = p.pos()
		return fc
	}

	// DISTINCT
	if p.cur.Type == kwDISTINCT || p.cur.Type == kwUNIQUE {
		fc.Distinct = true
		p.advance()
	}

	// ALL
	if p.cur.Type == kwALL {
		p.advance()
	}

	// Arguments
	if p.cur.Type != ')' {
		for {
			arg := p.parseExpr()
			if arg != nil {
				fc.Args.Items = append(fc.Args.Items, arg)
			}
			if p.cur.Type != ',' {
				break
			}
			p.advance() // consume ','
		}
	}

	if p.cur.Type == ')' {
		p.advance()
	}

	fc.Loc.End = p.pos()
	return fc
}

// parseParenExpr parses a parenthesized expression or subquery.
func (p *Parser) parseParenExpr() nodes.ExprNode {
	start := p.pos()
	p.advance() // consume '('

	// For now, parse as expression (subquery handling will come in batch 4)
	inner := p.parseExpr()

	if p.cur.Type == ')' {
		p.advance()
	}

	return &nodes.ParenExpr{
		Expr: inner,
		Loc:  nodes.Loc{Start: start, End: p.pos()},
	}
}

// parseCaseExpr parses a CASE expression (simple or searched).
//
// Ref: https://docs.oracle.com/en/database/oracle/oracle-database/23/sqlrf/CASE-Expressions.html
//
//	CASE [ expr ]
//	    WHEN condition THEN result
//	    [ WHEN condition THEN result ... ]
//	    [ ELSE default ]
//	END
func (p *Parser) parseCaseExpr() nodes.ExprNode {
	start := p.pos()
	p.advance() // consume CASE

	ce := &nodes.CaseExpr{
		Whens: &nodes.List{},
		Loc:   nodes.Loc{Start: start},
	}

	// Simple CASE: CASE expr WHEN ...
	// Searched CASE: CASE WHEN ...
	if p.cur.Type != kwWHEN {
		ce.Arg = p.parseExpr()
	}

	for p.cur.Type == kwWHEN {
		whenStart := p.pos()
		p.advance() // consume WHEN
		cond := p.parseExpr()
		if p.cur.Type == kwTHEN {
			p.advance()
		}
		result := p.parseExpr()
		ce.Whens.Items = append(ce.Whens.Items, &nodes.CaseWhen{
			Condition: cond,
			Result:    result,
			Loc:       nodes.Loc{Start: whenStart, End: p.pos()},
		})
	}

	if p.cur.Type == kwELSE {
		p.advance()
		ce.Default = p.parseExpr()
	}

	if p.cur.Type == kwEND {
		p.advance()
	}

	ce.Loc.End = p.pos()
	return ce
}

// parseCastExpr parses a CAST expression.
//
// Ref: https://docs.oracle.com/en/database/oracle/oracle-database/23/sqlrf/CAST.html
//
//	CAST ( expr AS datatype )
func (p *Parser) parseCastExpr() nodes.ExprNode {
	start := p.pos()
	p.advance() // consume CAST

	if p.cur.Type == '(' {
		p.advance()
	}

	arg := p.parseExpr()

	if p.cur.Type == kwAS {
		p.advance()
	}

	typeName := p.parseTypeName()

	if p.cur.Type == ')' {
		p.advance()
	}

	return &nodes.CastExpr{
		Arg:      arg,
		TypeName: typeName,
		Loc:      nodes.Loc{Start: start, End: p.pos()},
	}
}

// parseDecodeExpr parses Oracle's DECODE function.
//
// Ref: https://docs.oracle.com/en/database/oracle/oracle-database/23/sqlrf/DECODE.html
//
//	DECODE ( expr, search, result [, search, result ...] [, default] )
func (p *Parser) parseDecodeExpr() nodes.ExprNode {
	start := p.pos()
	p.advance() // consume DECODE

	if p.cur.Type == '(' {
		p.advance()
	}

	de := &nodes.DecodeExpr{
		Pairs: &nodes.List{},
		Loc:   nodes.Loc{Start: start},
	}

	// First arg is the expression to decode
	de.Arg = p.parseExpr()

	// Parse search, result pairs
	for p.cur.Type == ',' {
		p.advance() // consume ','

		search := p.parseExpr()

		if p.cur.Type != ',' {
			// This is the default value (odd argument at end)
			de.Default = search
			break
		}
		p.advance() // consume ','

		result := p.parseExpr()

		de.Pairs.Items = append(de.Pairs.Items, &nodes.DecodePair{
			Search: search,
			Result: result,
			Loc:    nodes.Loc{Start: start, End: p.pos()},
		})
	}

	if p.cur.Type == ')' {
		p.advance()
	}

	de.Loc.End = p.pos()
	return de
}

// parseExistsExpr parses an EXISTS subquery expression.
//
//	EXISTS ( subquery )
func (p *Parser) parseExistsExpr() nodes.ExprNode {
	start := p.pos()
	p.advance() // consume EXISTS

	// For now, just consume the parenthesized content
	// Full subquery parsing comes in batch 4
	if p.cur.Type == '(' {
		p.advance()
		// Skip to closing paren (placeholder until SELECT is implemented)
		depth := 1
		for depth > 0 && p.cur.Type != tokEOF {
			if p.cur.Type == '(' {
				depth++
			} else if p.cur.Type == ')' {
				depth--
				if depth == 0 {
					break
				}
			}
			p.advance()
		}
		if p.cur.Type == ')' {
			p.advance()
		}
	}

	return &nodes.ExistsExpr{
		Loc: nodes.Loc{Start: start, End: p.pos()},
	}
}

// parsePostfix parses postfix expression operators: IS, BETWEEN, IN, LIKE, NOT BETWEEN/IN/LIKE.
func (p *Parser) parsePostfix(left nodes.ExprNode) nodes.ExprNode {
	if left == nil {
		return nil
	}

	switch p.cur.Type {
	case kwIS:
		return p.parseIsExpr(left)

	case kwBETWEEN:
		return p.parseBetweenExpr(left, false)

	case kwIN:
		return p.parseInExpr(left, false)

	case kwLIKE:
		return p.parseLikeExpr(left, false, nodes.LIKE_LIKE)
	case kwLIKEC:
		return p.parseLikeExpr(left, false, nodes.LIKE_LIKEC)
	case kwLIKE2:
		return p.parseLikeExpr(left, false, nodes.LIKE_LIKE2)
	case kwLIKE4:
		return p.parseLikeExpr(left, false, nodes.LIKE_LIKE4)

	case kwNOT:
		// NOT BETWEEN, NOT IN, NOT LIKE
		next := p.peekNext()
		switch next.Type {
		case kwBETWEEN:
			p.advance() // consume NOT
			return p.parseBetweenExpr(left, true)
		case kwIN:
			p.advance() // consume NOT
			return p.parseInExpr(left, true)
		case kwLIKE:
			p.advance() // consume NOT
			return p.parseLikeExpr(left, true, nodes.LIKE_LIKE)
		case kwLIKEC:
			p.advance() // consume NOT
			return p.parseLikeExpr(left, true, nodes.LIKE_LIKEC)
		case kwLIKE2:
			p.advance() // consume NOT
			return p.parseLikeExpr(left, true, nodes.LIKE_LIKE2)
		case kwLIKE4:
			p.advance() // consume NOT
			return p.parseLikeExpr(left, true, nodes.LIKE_LIKE4)
		}
	}

	return left
}

// parseIsExpr parses IS [NOT] NULL / IS [NOT] NAN / etc.
//
// Ref: https://docs.oracle.com/en/database/oracle/oracle-database/23/sqlrf/Conditions.html
//
//	expr IS [NOT] { NULL | NAN | INFINITE | EMPTY | JSON | OF ... | A SET }
func (p *Parser) parseIsExpr(left nodes.ExprNode) nodes.ExprNode {
	start := p.pos()
	p.advance() // consume IS

	not := false
	if p.cur.Type == kwNOT {
		not = true
		p.advance()
	}

	test := ""
	switch p.cur.Type {
	case kwNULL:
		test = "NULL"
		p.advance()
	default:
		if p.isIdentLike() {
			test = p.cur.Str
			p.advance()
		}
	}

	return &nodes.IsExpr{
		Expr: left,
		Test: test,
		Not:  not,
		Loc:  nodes.Loc{Start: start, End: p.pos()},
	}
}

// parseBetweenExpr parses [NOT] BETWEEN low AND high.
//
// Ref: https://docs.oracle.com/en/database/oracle/oracle-database/23/sqlrf/BETWEEN-Condition.html
//
//	expr [NOT] BETWEEN low AND high
func (p *Parser) parseBetweenExpr(left nodes.ExprNode, not bool) nodes.ExprNode {
	start := p.pos()
	p.advance() // consume BETWEEN

	// Parse low bound at higher precedence to avoid consuming AND as boolean
	low := p.parseExprPrec(precConcat)

	if p.cur.Type == kwAND {
		p.advance()
	}

	high := p.parseExprPrec(precConcat)

	return &nodes.BetweenExpr{
		Expr: left,
		Low:  low,
		High: high,
		Not:  not,
		Loc:  nodes.Loc{Start: start, End: p.pos()},
	}
}

// parseInExpr parses [NOT] IN ( list | subquery ).
//
// Ref: https://docs.oracle.com/en/database/oracle/oracle-database/23/sqlrf/IN-Condition.html
//
//	expr [NOT] IN ( expr_list | subquery )
func (p *Parser) parseInExpr(left nodes.ExprNode, not bool) nodes.ExprNode {
	start := p.pos()
	p.advance() // consume IN

	list := &nodes.List{}

	if p.cur.Type == '(' {
		p.advance()
		for p.cur.Type != ')' && p.cur.Type != tokEOF {
			item := p.parseExpr()
			if item != nil {
				list.Items = append(list.Items, item)
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

	return &nodes.InExpr{
		Expr: left,
		List: list,
		Not:  not,
		Loc:  nodes.Loc{Start: start, End: p.pos()},
	}
}

// parseLikeExpr parses [NOT] LIKE pattern [ESCAPE escape_char].
//
// Ref: https://docs.oracle.com/en/database/oracle/oracle-database/23/sqlrf/Pattern-matching-Conditions.html
//
//	expr [NOT] LIKE pattern [ ESCAPE escape_char ]
func (p *Parser) parseLikeExpr(left nodes.ExprNode, not bool, likeType nodes.LikeType) nodes.ExprNode {
	start := p.pos()
	p.advance() // consume LIKE/LIKEC/LIKE2/LIKE4

	pattern := p.parseExprPrec(precConcat)

	var escape nodes.ExprNode
	if p.cur.Type == kwESCAPE {
		p.advance()
		escape = p.parseExprPrec(precConcat)
	}

	return &nodes.LikeExpr{
		Expr:    left,
		Pattern: pattern,
		Escape:  escape,
		Not:     not,
		Type:    likeType,
		Loc:     nodes.Loc{Start: start, End: p.pos()},
	}
}
