package ast

// ---------------------------------------------------------------------------
// Expression nodes — all implement ExprNode.
//
// This file is built in three task groups (matching the implementation plan):
//   1. Operators & predicates (this section)
//   2. Special-form expressions: FuncCall, CaseExpr, CastExpr, etc.
//   3. Paths, variables, parameters, subqueries, collection literals,
//      window spec, path steps
//
// Grammar: PartiQLParser.g4 — sourced from rules:
//   exprOr            (lines 469–472)
//   exprAnd           (lines 474–477)
//   exprNot           (lines 479–482)
//   exprPredicate     (lines 484–492)
//   mathOp00/01/02    (lines 494–507)
//   valueExpr         (lines 509–512)
//   exprPrimary       (lines 514–534)
//   exprTerm          (lines 542–549)
//   functionCall      (lines 611–616)
//   caseExpr          (lines 557–558)
//   cast/canCast/canLosslessCast (lines 593–600)
//   extract           (lines 602–603)
//   trimFunction      (lines 605–606)
//   substring         (lines 572–575)
//   coalesce          (lines 554–555)
//   nullIf            (lines 551–552)
//   aggregate         (lines 577–580)
//   windowFunction    (lines 589–591)
//   over              (lines 276–278)
//   array/bag/tuple/pair (lines 649–659)
//   pathStep          (lines 618–623)
//   varRefExpr        (lines 635–636)
//   parameter         (lines 632–633)
// Each type below cites its specific rule#Label.
// ---------------------------------------------------------------------------

// ===========================================================================
// Operator enums
// ===========================================================================

// BinOp identifies a binary operator.
type BinOp int

const (
	BinOpInvalid BinOp = iota
	BinOpOr
	BinOpAnd
	BinOpConcat // ||
	BinOpAdd
	BinOpSub
	BinOpMul
	BinOpDiv
	BinOpMod
	BinOpEq
	BinOpNotEq
	BinOpLt
	BinOpGt
	BinOpLtEq
	BinOpGtEq
)

// String returns the canonical operator spelling.
func (op BinOp) String() string {
	switch op {
	case BinOpOr:
		return "OR"
	case BinOpAnd:
		return "AND"
	case BinOpConcat:
		return "||"
	case BinOpAdd:
		return "+"
	case BinOpSub:
		return "-"
	case BinOpMul:
		return "*"
	case BinOpDiv:
		return "/"
	case BinOpMod:
		return "%"
	case BinOpEq:
		return "="
	case BinOpNotEq:
		return "<>"
	case BinOpLt:
		return "<"
	case BinOpGt:
		return ">"
	case BinOpLtEq:
		return "<="
	case BinOpGtEq:
		return ">="
	default:
		return "INVALID"
	}
}

// UnOp identifies a unary operator.
type UnOp int

const (
	UnOpInvalid UnOp = iota
	UnOpNot
	UnOpNeg // unary -
	UnOpPos // unary +
)

func (op UnOp) String() string {
	switch op {
	case UnOpNot:
		return "NOT"
	case UnOpNeg:
		return "-"
	case UnOpPos:
		return "+"
	default:
		return "INVALID"
	}
}

// IsType identifies the right-hand side of an `IS [NOT] X` predicate.
type IsType int

const (
	IsTypeInvalid IsType = iota
	IsTypeNull
	IsTypeMissing
	IsTypeTrue
	IsTypeFalse
)

func (t IsType) String() string {
	switch t {
	case IsTypeNull:
		return "NULL"
	case IsTypeMissing:
		return "MISSING"
	case IsTypeTrue:
		return "TRUE"
	case IsTypeFalse:
		return "FALSE"
	default:
		return "INVALID"
	}
}

// ===========================================================================
// Operator nodes
// ===========================================================================

// BinaryExpr represents a binary operator application.
//
// Grammar: exprOr#Or, exprAnd#And, exprPredicate#PredicateComparison,
//
//	mathOp00 (CONCAT), mathOp01 (PLUS/MINUS), mathOp02 (PERCENT/ASTERISK/SLASH_FORWARD)
type BinaryExpr struct {
	Op    BinOp
	Left  ExprNode
	Right ExprNode
	Loc   Loc
}

func (*BinaryExpr) nodeTag()      {}
func (n *BinaryExpr) GetLoc() Loc { return n.Loc }
func (*BinaryExpr) exprNode()     {}

// UnaryExpr represents a unary operator application.
//
// Grammar: exprNot#Not, valueExpr (unary PLUS/MINUS)
type UnaryExpr struct {
	Op      UnOp
	Operand ExprNode
	Loc     Loc
}

func (*UnaryExpr) nodeTag()      {}
func (n *UnaryExpr) GetLoc() Loc { return n.Loc }
func (*UnaryExpr) exprNode()     {}

// ===========================================================================
// Predicate nodes
// ===========================================================================

// InExpr represents `expr [NOT] IN (…)` — either a parenthesized expression
// list or a subquery.
//
// Grammar: exprPredicate#PredicateIn
type InExpr struct {
	Expr     ExprNode
	List     []ExprNode // populated when the RHS is an expression list
	Subquery StmtNode   // populated when the RHS is a parenthesized SELECT
	Not      bool
	Loc      Loc
}

func (*InExpr) nodeTag()      {}
func (n *InExpr) GetLoc() Loc { return n.Loc }
func (*InExpr) exprNode()     {}

// BetweenExpr represents `expr [NOT] BETWEEN low AND high`.
//
// Grammar: exprPredicate#PredicateBetween
type BetweenExpr struct {
	Expr ExprNode
	Low  ExprNode
	High ExprNode
	Not  bool
	Loc  Loc
}

func (*BetweenExpr) nodeTag()      {}
func (n *BetweenExpr) GetLoc() Loc { return n.Loc }
func (*BetweenExpr) exprNode()     {}

// LikeExpr represents `expr [NOT] LIKE pattern [ESCAPE escape]`.
//
// Grammar: exprPredicate#PredicateLike
type LikeExpr struct {
	Expr    ExprNode
	Pattern ExprNode
	Escape  ExprNode // nil if no ESCAPE clause
	Not     bool
	Loc     Loc
}

func (*LikeExpr) nodeTag()      {}
func (n *LikeExpr) GetLoc() Loc { return n.Loc }
func (*LikeExpr) exprNode()     {}

// IsExpr represents `expr IS [NOT] (NULL|MISSING|TRUE|FALSE)`.
//
// Grammar: exprPredicate#PredicateIs
type IsExpr struct {
	Expr ExprNode
	Type IsType
	Not  bool
	Loc  Loc
}

func (*IsExpr) nodeTag()      {}
func (n *IsExpr) GetLoc() Loc { return n.Loc }
func (*IsExpr) exprNode()     {}
