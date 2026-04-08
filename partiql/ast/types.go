package ast

// ---------------------------------------------------------------------------
// PartiQL type references — implement TypeName.
//
// One flat TypeRef covers all PartiQL types. Discrimination is by Name
// (uppercase canonical: INT, DECIMAL, TIME, BAG, ANY, ...) rather than a
// 25-arm Go enum. The grammar treats types as a name token with optional
// modifiers (Args for parameterized types, WithTimeZone for TIME/TIMESTAMP),
// so the AST mirrors that shape.
//
// Grammar: PartiQLParser.g4 — sourced from rules:
//   type#TypeAtomic    (lines 675–680)
//   type#TypeArgSingle (line 681)
//   type#TypeVarChar   (line 682)
//   type#TypeArgDouble (line 683)
//   type#TypeTimeZone  (line 684)
//   type#TypeCustom    (line 685)
// ---------------------------------------------------------------------------

// TypeRef represents any PartiQL type reference, used by CAST and DDL.
//
// Args carries optional precision/scale/length:
//   - DECIMAL(10,2) -> Args=[10, 2]
//   - VARCHAR(255)  -> Args=[255]
//   - FLOAT(53)     -> Args=[53]
//   - INT           -> Args=nil
//
// WithTimeZone is set for `TIME WITH TIME ZONE` and `TIMESTAMP WITH TIME ZONE`.
//
// Names covered (canonical uppercase):
//   - Numeric: INT, INTEGER, BIGINT, SMALLINT, REAL, DOUBLE PRECISION,
//     DECIMAL, NUMERIC, FLOAT
//   - Boolean: BOOL, BOOLEAN
//   - Null/missing: NULL, MISSING
//   - String: STRING, SYMBOL, VARCHAR, CHAR, CHARACTER
//   - Binary: BLOB, CLOB
//   - Temporal: DATE, TIME, TIMESTAMP
//   - Collection: STRUCT, TUPLE, LIST, BAG, SEXP
//   - Wildcard: ANY
type TypeRef struct {
	Name         string
	Args         []int
	WithTimeZone bool
	Loc          Loc
}

func (*TypeRef) nodeTag()      {}
func (n *TypeRef) GetLoc() Loc { return n.Loc }
func (*TypeRef) typeName()     {}
