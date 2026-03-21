# PG Parser-Native C3 Completion Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Implement full-grammar SQL auto-completion directly in the omni pg recursive descent parser, achieving the same coverage as ANTLR C3 without a separate grammar artifact.

**Architecture:** The parser gains a "completion mode" where, after parsing tokens up to the cursor offset, it switches from consuming tokens to collecting all valid continuations (token candidates + rule candidates). Rule candidates (e.g. `columnref`, `relation_expr`) are then resolved against the catalog for semantic suggestions. A FIRST-set cache ensures sub-10ms response times. A two-pass strategy (standard + tricky) handles incomplete SQL.

**Tech Stack:** Pure Go, zero external dependencies. Builds on existing `pg/parser` (recursive descent), `pg/catalog` (schema metadata), and `pg/ast` (AST nodes).

---

## Task 1: Completion Types and Public API

**Files:**
- Create: `pg/completion/completion.go`
- Test: `pg/completion/completion_test.go`

**Step 1: Write the failing test**

```go
// pg/completion/completion_test.go
package completion

import (
	"testing"

	"github.com/bytebase/omni/pg/catalog"
)

func TestCompleteSelectKeyword(t *testing.T) {
	// Cursor at position 0 in empty string — should suggest statement keywords.
	candidates := Complete("", 0, nil)
	found := false
	for _, c := range candidates {
		if c.Type == CandidateKeyword && c.Text == "SELECT" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected SELECT keyword candidate")
	}
}

func TestCompleteCandidateTypes(t *testing.T) {
	// Verify candidate types are defined.
	if CandidateKeyword == CandidateTable {
		t.Fatal("candidate types must be distinct")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/completion/ -run TestComplete -v`
Expected: FAIL — package does not exist

**Step 3: Write minimal implementation**

```go
// pg/completion/completion.go
package completion

import (
	"github.com/bytebase/omni/pg/catalog"
)

// CandidateType classifies a completion suggestion.
type CandidateType int

const (
	CandidateKeyword CandidateType = iota + 1
	CandidateSchema
	CandidateTable
	CandidateView
	CandidateMaterializedView
	CandidateColumn
	CandidateFunction
	CandidateSequence
	CandidateIndex
	CandidateType_
	CandidateTrigger
	CandidatePolicy
	CandidateExtension
)

// Candidate is a single completion suggestion.
type Candidate struct {
	Text       string
	Type       CandidateType
	Definition string // optional type signature or column type
	Comment    string // optional documentation
}

// Complete returns completion candidates for the given SQL at cursorOffset.
// cat may be nil for keyword-only completion.
func Complete(sql string, cursorOffset int, cat *catalog.Catalog) []Candidate {
	// Phase 1: standard completion
	result := standardComplete(sql, cursorOffset, cat)
	if len(result) > 0 {
		return result
	}
	// Phase 2: tricky completion for incomplete SQL
	return trickyComplete(sql, cursorOffset, cat)
}

func standardComplete(sql string, cursorOffset int, cat *catalog.Catalog) []Candidate {
	return nil // implemented in task 3
}

func trickyComplete(sql string, cursorOffset int, cat *catalog.Catalog) []Candidate {
	return nil // implemented in task 10
}
```

**Step 4: Run test to verify it fails**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/completion/ -run TestComplete -v`
Expected: FAIL — TestCompleteSelectKeyword fails (no candidates returned)

**Step 5: Commit**

```bash
git add pg/completion/completion.go pg/completion/completion_test.go
git commit -m "feat(pg/completion): add completion types and public API stub"
```

---

## Task 2: Parser Completion Infrastructure

**Files:**
- Modify: `pg/parser/parser.go` (add completion fields and modify `match`/`expect`/`advance`)
- Create: `pg/parser/complete.go` (completion-specific helpers)
- Test: `pg/parser/complete_test.go`

**Step 1: Write the failing test**

```go
// pg/parser/complete_test.go
package parser

import (
	"testing"
)

func TestCollectStmtKeywords(t *testing.T) {
	// Collect candidates at offset 0 (beginning of empty input).
	candidates := Collect("", 0)
	if candidates == nil {
		t.Fatal("expected non-nil candidate set")
	}
	// Should include statement-starting keywords.
	if !candidates.HasToken(SELECT) {
		t.Error("expected SELECT token candidate")
	}
	if !candidates.HasToken(INSERT) {
		t.Error("expected INSERT token candidate")
	}
	if !candidates.HasToken(CREATE) {
		t.Error("expected CREATE token candidate")
	}
}

func TestCollectAfterSelect(t *testing.T) {
	// "SELECT " — cursor after SELECT, should suggest expression atoms + column refs.
	candidates := Collect("SELECT ", 7)
	if candidates == nil {
		t.Fatal("expected non-nil candidate set")
	}
	// Should have a columnref rule candidate (for column names).
	if !candidates.HasRule("columnref") {
		t.Error("expected columnref rule candidate")
	}
}

func TestCollectAfterFrom(t *testing.T) {
	// "SELECT 1 FROM " — cursor after FROM, should suggest table names.
	candidates := Collect("SELECT 1 FROM ", 14)
	if candidates == nil {
		t.Fatal("expected non-nil candidate set")
	}
	if !candidates.HasRule("relation_expr") {
		t.Error("expected relation_expr rule candidate")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/parser/ -run TestCollect -v`
Expected: FAIL — `Collect` not defined

**Step 3: Write the implementation**

First, add completion fields to Parser struct in `parser.go`:

Add these fields to the `Parser` struct (after the existing `hasNext` field):

```go
// pg/parser/parser.go — add to Parser struct
	// Completion mode fields.
	completing   bool          // true when collecting completion candidates
	cursorOff    int           // byte offset of the cursor in source
	candidates   *CandidateSet // collected candidates
	collecting   bool          // true once we've passed the cursor
	collectDepth int           // recursion depth in collect mode
	maxCollect   int           // max exploration depth (default 4)
```

Then create the completion helpers file:

```go
// pg/parser/complete.go
package parser

// CandidateSet holds raw completion candidates from the parser.
type CandidateSet struct {
	Tokens []int             // token type candidates (keywords, operators)
	Rules  []RuleCandidate   // grammar rule candidates (for semantic resolution)
	seen   map[int]bool      // dedup tokens
	seenR  map[string]bool   // dedup rules
}

// RuleCandidate represents a grammar rule that is valid at the cursor position.
type RuleCandidate struct {
	Rule string // e.g. "columnref", "relation_expr", "func_name"
}

func newCandidateSet() *CandidateSet {
	return &CandidateSet{
		seen:  make(map[int]bool),
		seenR: make(map[string]bool),
	}
}

func (cs *CandidateSet) addToken(tokenType int) {
	if cs.seen[tokenType] {
		return
	}
	cs.seen[tokenType] = true
	cs.Tokens = append(cs.Tokens, tokenType)
}

func (cs *CandidateSet) addRule(rule string) {
	if cs.seenR[rule] {
		return
	}
	cs.seenR[rule] = true
	cs.Rules = append(cs.Rules, RuleCandidate{Rule: rule})
}

// HasToken reports whether the candidate set contains the given token type.
func (cs *CandidateSet) HasToken(tokenType int) bool {
	return cs.seen[tokenType]
}

// HasRule reports whether the candidate set contains the given rule name.
func (cs *CandidateSet) HasRule(rule string) bool {
	return cs.seenR[rule]
}

// Collect runs the parser in completion mode and returns raw candidates.
func Collect(sql string, cursorOffset int) *CandidateSet {
	p := &Parser{
		lexer:      NewLexer(sql),
		completing: true,
		cursorOff:  cursorOffset,
		candidates: newCandidateSet(),
		maxCollect: 4,
	}
	p.advance()

	// If cursor is at position 0 and cur token starts at or after cursor,
	// we're already at collection point.
	p.checkCursor()

	// Parse — the parser will switch to collect mode when it hits the cursor.
	p.parseStmt()

	return p.candidates
}

// collectMode reports whether the parser is currently collecting candidates
// (cursor has been reached).
func (p *Parser) collectMode() bool {
	return p.completing && p.collecting
}

// checkCursor checks if we've reached the cursor position and switches
// to collection mode if so.
func (p *Parser) checkCursor() {
	if !p.completing || p.collecting {
		return
	}
	// Cursor is at or before current token start — start collecting.
	if p.cursorOff <= p.cur.Loc {
		p.collecting = true
	}
	// Cursor is inside current token (user is typing).
	if p.cur.Loc < p.cursorOff && p.cursorOff <= p.cur.End {
		p.collecting = true
	}
}

// addTokenCandidate is called from match/expect in collect mode.
func (p *Parser) addTokenCandidate(tokenType int) {
	if p.candidates != nil {
		p.candidates.addToken(tokenType)
	}
}

// addRuleCandidate records that a grammar rule is valid at this position.
func (p *Parser) addRuleCandidate(rule string) {
	if p.candidates != nil {
		p.candidates.addRule(rule)
	}
}

// errCollecting is a sentinel error used in collect mode.
// It signals that we're collecting, not that there's a real error.
var errCollecting = &ParseError{Message: "collecting"}
```

Now modify `advance()`, `match()`, and `expect()` in `parser.go`:

In `advance()`, add cursor checking after the token reclassification block (before `return p.prev`):

```go
	// Check if we've reached the cursor for completion.
	if p.completing && !p.collecting {
		p.checkCursor()
	}
```

In `match()`, add collect mode handling at the top:

```go
func (p *Parser) match(types ...int) (Token, bool) {
	if p.collectMode() {
		for _, t := range types {
			p.addTokenCandidate(t)
		}
		return Token{}, false
	}
	for _, t := range types {
		if p.cur.Type == t {
			return p.advance(), true
		}
	}
	return Token{}, false
}
```

In `expect()`, add collect mode handling at the top:

```go
func (p *Parser) expect(tokenType int) (Token, error) {
	if p.collectMode() {
		p.addTokenCandidate(tokenType)
		return Token{}, errCollecting
	}
	if p.cur.Type == tokenType {
		return p.advance(), nil
	}
	return Token{}, &ParseError{
		Message:  "unexpected token",
		Position: p.cur.Loc,
	}
}
```

**Step 4: Run test to verify it passes**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/parser/ -run TestCollect -v`
Expected: TestCollectStmtKeywords PASS (parseStmt switch adds all case tokens via match)

Note: TestCollectAfterSelect and TestCollectAfterFrom may not pass yet — they need the preferred rule handling in tasks 3-4.

**Step 5: Commit**

```bash
git add pg/parser/parser.go pg/parser/complete.go pg/parser/complete_test.go
git commit -m "feat(pg/parser): add completion mode infrastructure to parser"
```

---

## Task 3: Collect-Mode for parseStmt Switch Dispatch

**Files:**
- Modify: `pg/parser/parser.go` (add collect mode to `parseStmt()` and `parseCreateDispatch()`)
- Modify: `pg/parser/complete_test.go`

**Step 1: Write the failing test**

```go
// Add to pg/parser/complete_test.go
func TestCollectStmtKeywordsComplete(t *testing.T) {
	candidates := Collect("", 0)
	// Verify all major statement-starting keywords.
	expected := []int{
		SELECT, VALUES, TABLE, WITH, INSERT, UPDATE, DELETE_P, MERGE,
		CREATE, ALTER, DROP, GRANT, REVOKE, SET, SHOW, RESET,
		BEGIN_P, COMMIT, ROLLBACK, PREPARE, EXECUTE, DEALLOCATE,
		TRUNCATE, LOCK_P, VACUUM, ANALYZE, CLUSTER, REINDEX,
		COPY, EXPLAIN, DO, COMMENT, REFRESH, DECLARE, FETCH,
		CLOSE, CHECKPOINT, DISCARD, LISTEN, UNLISTEN, NOTIFY,
		LOAD, CALL, REASSIGN, IMPORT_P, SECURITY, MOVE,
	}
	for _, tok := range expected {
		if !candidates.HasToken(tok) {
			t.Errorf("missing token candidate: %d", tok)
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/parser/ -run TestCollectStmtKeywordsComplete -v`
Expected: FAIL — parseStmt returns nil without adding all tokens in collect mode

**Step 3: Modify parseStmt to add collect-mode block at the top**

In `parser.go`, add this block at the beginning of `parseStmt()`:

```go
func (p *Parser) parseStmt() nodes.Node {
	if p.collectMode() {
		// In collect mode, emit all statement-starting keywords as candidates.
		stmtTokens := []int{
			SELECT, VALUES, TABLE, WITH, INSERT, UPDATE, DELETE_P, MERGE,
			CREATE, COMMENT, SECURITY, ALTER, REFRESH,
			BEGIN_P, START, COMMIT, END_P, ABORT_P, SAVEPOINT, RELEASE, ROLLBACK,
			PREPARE, EXECUTE, DEALLOCATE, SET, SHOW, RESET,
			GRANT, REVOKE, DROP, TRUNCATE, LOCK_P, DECLARE, FETCH, MOVE, CLOSE,
			VACUUM, ANALYZE, ANALYSE, CLUSTER, REINDEX,
			COPY, IMPORT_P, EXPLAIN, DO, CHECKPOINT, DISCARD,
			LISTEN, UNLISTEN, NOTIFY, LOAD, CALL, REASSIGN,
		}
		for _, t := range stmtTokens {
			p.addTokenCandidate(t)
		}
		return nil
	}
	// ... existing switch ...
```

**Step 4: Run tests**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/parser/ -run TestCollect -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pg/parser/parser.go pg/parser/complete_test.go
git commit -m "feat(pg/parser): emit all statement keywords in collect mode"
```

---

## Task 4: Preferred Rules — columnref, relation_expr, func_name

**Files:**
- Modify: `pg/parser/name.go` (add collect mode to `parseColumnRef`, `parseQualifiedName`, `parseFuncName`)
- Modify: `pg/parser/complete.go` (add preferred rules config)
- Modify: `pg/parser/complete_test.go`

**Step 1: Write the failing test**

```go
// Add to pg/parser/complete_test.go
func TestCollectColumnRef(t *testing.T) {
	// "SELECT " — after SELECT keyword, target_list calls parseCExpr which
	// eventually calls parseColumnRef. Should emit columnref rule.
	candidates := Collect("SELECT ", 7)
	if !candidates.HasRule("columnref") {
		t.Error("expected columnref rule candidate in SELECT target list")
	}
}

func TestCollectFromRelation(t *testing.T) {
	// "SELECT 1 FROM " — FROM clause should emit relation_expr rule.
	candidates := Collect("SELECT 1 FROM ", 14)
	if !candidates.HasRule("relation_expr") {
		t.Error("expected relation_expr rule candidate after FROM")
	}
}

func TestCollectFuncName(t *testing.T) {
	// Func name rule should be emitted where functions are valid.
	candidates := Collect("SELECT ", 7)
	if !candidates.HasRule("func_name") {
		t.Error("expected func_name rule candidate in SELECT target list")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/parser/ -run TestCollect -v`
Expected: FAIL — no rule candidates emitted

**Step 3: Implement preferred rules**

In `pg/parser/name.go`, add collect mode at the top of `parseColumnRef`:

```go
func (p *Parser) parseColumnRef() (nodes.Node, error) {
	if p.collectMode() {
		p.addRuleCandidate("columnref")
		return nil, errCollecting
	}
	// ... existing code ...
```

In `pg/parser/name.go`, add collect mode at the top of `parseQualifiedName`:

```go
func (p *Parser) parseQualifiedName() (*nodes.List, error) {
	if p.collectMode() {
		p.addRuleCandidate("relation_expr")
		return nil, errCollecting
	}
	// ... existing code ...
```

In `pg/parser/name.go`, add collect mode at the top of `parseFuncName`:

```go
func (p *Parser) parseFuncName() (*nodes.List, error) {
	if p.collectMode() {
		p.addRuleCandidate("func_name")
		return nil, errCollecting
	}
	// ... existing code ...
```

Now the tricky part: we need the parser to actually reach these functions during collect mode. Currently `parseStmt()` returns nil immediately. We need it to also explore the path to the cursor.

The key insight: collect mode should only trigger at the **cursor position**, not everywhere. The parser should parse normally up to the cursor, then switch to collect mode. The `parseStmt` collect block was for when the cursor is at position 0 (no tokens consumed). For other positions, the parser parses normally until `p.collecting` becomes true.

Update `parseStmt()` to only use the early-return collect block when the cursor hasn't been parsed past:

```go
func (p *Parser) parseStmt() nodes.Node {
	if p.collectMode() {
		// Cursor is at statement start — suggest all statement keywords.
		stmtTokens := []int{ /* ... same list ... */ }
		for _, t := range stmtTokens {
			p.addTokenCandidate(t)
		}
		return nil
	}
	// ... existing switch ...
```

The `match()` and `expect()` changes from Task 2 will automatically collect tokens at any point during parsing where the cursor falls. When `parseSimpleSelectCore()` reaches `parseTargetList()`, which calls `parseCExpr()`, which calls `parseColumnRefOrFuncCall()` → `parseColumnRef()` — if collect mode is active, the columnref rule is emitted.

We also need `parseSimpleSelectCore` to handle collect mode in its FROM clause path. When the parser has consumed `SELECT 1 FROM` and hits collect mode at the next token, `parseFromListFull` will be called, which calls a from-item parser, which eventually reaches `parseQualifiedName`.

We need to make sure nil returns from collect-mode parse functions don't crash the caller. Add nil-safety to `parseSimpleSelectCore` and similar:

In `select.go`, the FROM clause already checks `if p.cur.Type == FROM`. When `collecting` becomes true after parsing `FROM`, the calls to `parseFromListFull` → inner parsers will hit collect mode and emit rule candidates. The key is that callers need to tolerate nil returns.

For `parseCExprInner`, the default branch already checks `p.isColId()` — we need to add collect mode there too. Add at the top of `parseCExprInner` in `expr.go`:

```go
func (p *Parser) parseCExprInner() nodes.Node {
	if p.collectMode() {
		// Expression atoms: emit all c_expr starting tokens.
		exprTokens := []int{
			ICONST, FCONST, SCONST, BCONST, XCONST,
			TRUE_P, FALSE_P, NULL_P, PARAM,
			'(', EXISTS, ARRAY, CASE, ROW, DEFAULT,
			CAST, COALESCE, GREATEST, LEAST, NULLIF,
			EXTRACT, NORMALIZE, OVERLAY, POSITION, SUBSTRING, TRIM,
			GROUPING, COLLATION, TREAT,
			JSON, JSON_OBJECT, JSON_ARRAY, JSON_SCALAR, JSON_SERIALIZE,
			JSON_QUERY, JSON_EXISTS, JSON_VALUE, JSON_OBJECTAGG, JSON_ARRAYAGG,
			XMLCONCAT, XMLELEMENT, XMLEXISTS, XMLFOREST, XMLPARSE, XMLPI, XMLROOT, XMLSERIALIZE,
			CURRENT_DATE, CURRENT_TIME, CURRENT_TIMESTAMP, LOCALTIME, LOCALTIMESTAMP,
			CURRENT_ROLE, CURRENT_USER, SESSION_USER, USER, CURRENT_CATALOG, CURRENT_SCHEMA, SYSTEM_USER,
			NOT, '+', '-',
		}
		for _, t := range exprTokens {
			p.addTokenCandidate(t)
		}
		// Also emit preferred rules for identifiers.
		p.addRuleCandidate("columnref")
		p.addRuleCandidate("func_name")
		return nil
	}
	// ... existing switch ...
```

For the FROM clause, add collect mode to `parseRelationExpr` (or wherever the from-item parser first tries to parse a table name). Find the from-list entry point and add:

```go
// In the from-item parser (e.g., parseTableRef or parseFromListItem):
if p.collectMode() {
	p.addRuleCandidate("relation_expr")
	// Also suggest JOIN, subquery, LATERAL, etc.
	p.addTokenCandidate('(')
	p.addTokenCandidate(LATERAL)
	return nil
}
```

**Step 4: Run tests**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/parser/ -run TestCollect -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pg/parser/name.go pg/parser/expr.go pg/parser/select.go pg/parser/complete.go pg/parser/complete_test.go
git commit -m "feat(pg/parser): add preferred rule candidates for columnref, relation_expr, func_name"
```

---

## Task 5: Collect-Mode for Keyword Categories (isColId, isColLabel, isTypeFunctionName)

**Files:**
- Modify: `pg/parser/name.go`
- Modify: `pg/parser/complete.go`
- Modify: `pg/parser/complete_test.go`

**Step 1: Write the failing test**

```go
// Add to pg/parser/complete_test.go
func TestCollectKeywordCategories(t *testing.T) {
	// When columnref is valid, unreserved + col_name keywords should be candidates.
	candidates := Collect("SELECT ", 7)
	// "name" is an unreserved keyword — should appear as token candidate.
	if !candidates.HasToken(NAME_P) {
		t.Error("expected unreserved keyword NAME as candidate")
	}
}
```

**Step 2: Run test to verify**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/parser/ -run TestCollectKeywordCategories -v`

**Step 3: Add keyword category emission to collect mode**

In `pg/parser/complete.go`, add a helper to emit keywords by category:

```go
// addKeywordsByCategory adds all keywords matching the given categories.
func (p *Parser) addKeywordsByCategory(categories ...KeywordCategory) {
	catSet := make(map[KeywordCategory]bool, len(categories))
	for _, c := range categories {
		catSet[c] = true
	}
	for i := range Keywords {
		if catSet[Keywords[i].Category] {
			p.addTokenCandidate(Keywords[i].Token)
		}
	}
}
```

In `pg/parser/name.go`, modify `isColId` for collect mode:

```go
func (p *Parser) isColId() bool {
	if p.collectMode() {
		// ColId: IDENT | unreserved_keyword | col_name_keyword
		p.addTokenCandidate(IDENT)
		p.addKeywordsByCategory(UnreservedKeyword, ColNameKeyword)
		return true // pretend match to explore this branch
	}
	// ... existing code ...
```

Similarly for `isColLabel` and `isTypeFunctionName`:

```go
func (p *Parser) isColLabel() bool {
	if p.collectMode() {
		p.addTokenCandidate(IDENT)
		p.addKeywordsByCategory(UnreservedKeyword, ColNameKeyword, TypeFuncNameKeyword, ReservedKeyword)
		return true
	}
	// ... existing code ...
}

func (p *Parser) isTypeFunctionName() bool {
	if p.collectMode() {
		p.addTokenCandidate(IDENT)
		p.addKeywordsByCategory(UnreservedKeyword, TypeFuncNameKeyword)
		return true
	}
	// ... existing code ...
}
```

**Step 4: Run tests**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/parser/ -run TestCollect -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pg/parser/name.go pg/parser/complete.go pg/parser/complete_test.go
git commit -m "feat(pg/parser): emit keyword categories in collect mode for isColId/isColLabel/isTypeFunctionName"
```

---

## Task 6: Collect-Mode for Expression Parsing (Pratt)

**Files:**
- Modify: `pg/parser/expr.go` (add collect mode to `parseAExpr`, `aExprInfixPrec`)
- Modify: `pg/parser/complete_test.go`

**Step 1: Write the failing test**

```go
// Add to pg/parser/complete_test.go
func TestCollectWhereExprOperators(t *testing.T) {
	// "SELECT 1 WHERE x " — after an expression, should suggest operators.
	candidates := Collect("SELECT 1 WHERE x ", 18)
	// Should include comparison, boolean, and pattern operators.
	for _, tok := range []int{AND, OR, IS, '<', '>', '=', BETWEEN, IN_P, LIKE} {
		if !candidates.HasToken(tok) {
			t.Errorf("missing infix operator token: %d", tok)
		}
	}
}

func TestCollectExprAtom(t *testing.T) {
	// "SELECT 1 WHERE " — at expression start, should suggest atoms.
	candidates := Collect("SELECT 1 WHERE ", 15)
	if !candidates.HasToken(NOT) {
		t.Error("expected NOT prefix operator")
	}
	if !candidates.HasRule("columnref") {
		t.Error("expected columnref in expression context")
	}
}
```

**Step 2: Run test to verify**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/parser/ -run TestCollectWhere -v`

**Step 3: Add infix operator collection**

When the parser has consumed `x` (an expression) and hits collect mode, it's in the `for` loop of `parseAExpr`. The `aExprInfixPrec()` check runs in collect mode. Instead of the loop, we emit all operators with precedence >= minPrec.

Add a collect-mode handler to `parseAExpr`:

```go
func (p *Parser) parseAExpr(minPrec int) nodes.Node {
	if p.collectMode() {
		// Emit expression-starting tokens (atoms).
		p.parseAExprAtom()
		// Also emit all infix operators with prec >= minPrec.
		p.addInfixOperatorCandidates(minPrec)
		return nil
	}
	// ... existing Pratt loop ...
```

But wait — the cursor might be AFTER an expression (wanting infix operators) or AT the start of an expression (wanting atoms). The approach: when the for loop in parseAExpr calls `aExprInfixPrec()` and we're in collect mode, we emit all valid infix operators. The normal flow handles this because the loop re-checks `aExprInfixPrec()` after each operator, so if collect mode activates between iterations, we catch it.

Actually, the simpler approach: after `parseAExprAtom()` returns, if we're now in collect mode (the atom consumed tokens up to cursor), emit infix candidates:

In `parseAExpr`, inside the `for` loop, after `p.aExprInfixPrec()`:

```go
	for {
		if p.collectMode() {
			p.addInfixOperatorCandidates(minPrec)
			return left
		}
		prec := p.aExprInfixPrec()
		// ...
```

Add helper in `complete.go`:

```go
// addInfixOperatorCandidates adds all a_expr infix operator tokens
// with precedence >= minPrec.
func (p *Parser) addInfixOperatorCandidates(minPrec int) {
	type precToken struct {
		prec  int
		token int
	}
	ops := []precToken{
		{precOr, OR},
		{precAnd, AND},
		{precIs, IS}, {precIs, ISNULL}, {precIs, NOTNULL},
		{precComparison, '<'}, {precComparison, '>'}, {precComparison, '='},
		{precComparison, LESS_EQUALS}, {precComparison, GREATER_EQUALS}, {precComparison, NOT_EQUALS},
		{precIn, BETWEEN}, {precIn, IN_P}, {precIn, LIKE}, {precIn, ILIKE}, {precIn, SIMILAR},
		{precIn, NOT_LA},
		{precOp, Op},
		{precAdd, '+'}, {precAdd, '-'},
		{precMul, '*'}, {precMul, '/'}, {precMul, '%'},
		{precExp, '^'},
		{precAt, AT},
		{precCollate, COLLATE},
		{precTypecast, TYPECAST}, {precTypecast, '['},
	}
	for _, o := range ops {
		if o.prec >= minPrec {
			p.addTokenCandidate(o.token)
		}
	}
}
```

**Step 4: Run tests**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/parser/ -run TestCollect -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pg/parser/expr.go pg/parser/complete.go pg/parser/complete_test.go
git commit -m "feat(pg/parser): add Pratt expression operator candidates in collect mode"
```

---

## Task 7: Collect-Mode for SELECT Clauses (FROM, WHERE, GROUP BY, HAVING, ORDER BY)

**Files:**
- Modify: `pg/parser/select.go` (add collect mode to `parseSimpleSelectCore`, `parseSelectOptions`)
- Modify: `pg/parser/complete_test.go`

**Step 1: Write the failing test**

```go
// Add to pg/parser/complete_test.go
func TestCollectSelectClauses(t *testing.T) {
	// "SELECT 1 " — after target list, should suggest clause keywords.
	candidates := Collect("SELECT 1 ", 9)
	for _, tok := range []int{FROM, WHERE, GROUP_P, HAVING, ORDER, LIMIT, OFFSET, UNION, EXCEPT, INTERSECT, FOR} {
		if !candidates.HasToken(tok) {
			t.Errorf("missing clause keyword token: %d", tok)
		}
	}
}

func TestCollectAfterWhere(t *testing.T) {
	// "SELECT 1 FROM t WHERE x = 1 " — after WHERE clause, suggest more clauses.
	candidates := Collect("SELECT 1 FROM t WHERE x = 1 ", 28)
	if !candidates.HasToken(AND) {
		t.Error("expected AND after WHERE predicate")
	}
	if !candidates.HasToken(ORDER) {
		t.Error("expected ORDER BY suggestion")
	}
}
```

**Step 2: Run test to verify**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/parser/ -run TestCollectSelect -v`

**Step 3: Add collect mode to SELECT clause transitions**

In `parseSimpleSelectCore()`, after each clause is parsed, the parser checks `if p.cur.Type == NEXT_CLAUSE`. When collect mode activates between clauses, we need to emit all valid continuation keywords.

Add to `parseSimpleSelectCore`, after target list is parsed (after `stmt.TargetList = p.parseTargetList()`):

```go
	// After target list, if collecting, emit all valid continuation keywords.
	if p.collectMode() {
		p.addTokenCandidate(INTO)
		p.addTokenCandidate(FROM)
		p.addTokenCandidate(WHERE)
		p.addTokenCandidate(GROUP_P)
		p.addTokenCandidate(HAVING)
		p.addTokenCandidate(WINDOW)
		// Set operations and final clauses come from parseSelectClause/parseSelectOptions.
		p.addTokenCandidate(ORDER)
		p.addTokenCandidate(LIMIT)
		p.addTokenCandidate(OFFSET)
		p.addTokenCandidate(FETCH)
		p.addTokenCandidate(FOR)
		p.addTokenCandidate(UNION)
		p.addTokenCandidate(EXCEPT)
		p.addTokenCandidate(INTERSECT)
		// Also valid: infix operators on the last expression, semicolon.
		p.addTokenCandidate(';')
		return stmt
	}
```

Add similar blocks after FROM, WHERE, GROUP BY, HAVING:

After FROM clause (`stmt.FromClause = p.parseFromListFull()`):
```go
	if p.collectMode() {
		p.addTokenCandidate(WHERE)
		p.addTokenCandidate(GROUP_P)
		p.addTokenCandidate(HAVING)
		p.addTokenCandidate(WINDOW)
		p.addTokenCandidate(ORDER)
		p.addTokenCandidate(LIMIT)
		p.addTokenCandidate(OFFSET)
		p.addTokenCandidate(FETCH)
		p.addTokenCandidate(FOR)
		p.addTokenCandidate(UNION)
		p.addTokenCandidate(EXCEPT)
		p.addTokenCandidate(INTERSECT)
		p.addTokenCandidate(';')
		return stmt
	}
```

(Repeat pattern after each clause, removing keywords that have already been consumed.)

**Step 4: Run tests**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/parser/ -run TestCollect -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pg/parser/select.go pg/parser/complete_test.go
git commit -m "feat(pg/parser): add SELECT clause keyword candidates in collect mode"
```

---

## Task 8: Catalog Resolution Layer

**Files:**
- Modify: `pg/completion/completion.go` (implement `standardComplete` with catalog resolution)
- Create: `pg/completion/resolve.go` (rule → catalog object resolution)
- Modify: `pg/completion/completion_test.go`

**Step 1: Write the failing test**

```go
// Add to pg/completion/completion_test.go
func TestCompleteTableFromCatalog(t *testing.T) {
	cat := catalog.New()
	// Create a table in the catalog.
	_, err := cat.Exec("CREATE TABLE users (id int, name text);", nil)
	if err != nil {
		t.Fatal(err)
	}

	// "SELECT * FROM " — should suggest "users" table.
	candidates := Complete("SELECT * FROM ", 14, cat)
	found := false
	for _, c := range candidates {
		if c.Type == CandidateTable && c.Text == "users" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'users' table candidate")
	}
}

func TestCompleteColumnFromCatalog(t *testing.T) {
	cat := catalog.New()
	cat.Exec("CREATE TABLE users (id int, name text);", nil)

	// "SELECT  FROM users" with cursor at position 7 — should suggest columns.
	candidates := Complete("SELECT  FROM users", 7, cat)
	var cols []string
	for _, c := range candidates {
		if c.Type == CandidateColumn {
			cols = append(cols, c.Text)
		}
	}
	// Should include 'id' and 'name' from the users table.
	if len(cols) == 0 {
		t.Error("expected column candidates from 'users' table")
	}
}

func TestCompleteFunction(t *testing.T) {
	cat := catalog.New()
	candidates := Complete("SELECT ", 7, cat)
	// Should include built-in functions like count, sum, etc.
	var funcs []string
	for _, c := range candidates {
		if c.Type == CandidateFunction {
			funcs = append(funcs, c.Text)
		}
	}
	if len(funcs) == 0 {
		t.Error("expected function candidates")
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/completion/ -run TestComplete -v`
Expected: FAIL — standardComplete returns nil

**Step 3: Implement catalog resolution**

```go
// pg/completion/resolve.go
package completion

import (
	"sort"
	"strings"

	"github.com/bytebase/omni/pg/catalog"
	"github.com/bytebase/omni/pg/parser"
)

// resolve converts raw parser candidates into typed completion suggestions.
func resolve(cs *parser.CandidateSet, cat *catalog.Catalog, sql string, cursorOffset int) []Candidate {
	if cs == nil {
		return nil
	}
	var result []Candidate

	// 1. Token candidates → keywords.
	for _, tok := range cs.Tokens {
		name := parser.TokenName(tok)
		if name == "" {
			continue
		}
		result = append(result, Candidate{
			Text: name,
			Type: CandidateKeyword,
		})
	}

	// 2. Rule candidates → catalog objects.
	if cat != nil {
		for _, rc := range cs.Rules {
			result = append(result, resolveRule(rc.Rule, cat, sql, cursorOffset)...)
		}
	}

	// Deduplicate by (Text, Type).
	result = dedup(result)
	// Sort: keywords last, then alphabetical.
	sort.Slice(result, func(i, j int) bool {
		if result[i].Type != result[j].Type {
			return result[i].Type > result[j].Type // tables before keywords
		}
		return result[i].Text < result[j].Text
	})

	return result
}

func resolveRule(rule string, cat *catalog.Catalog, sql string, cursorOffset int) []Candidate {
	switch rule {
	case "columnref":
		return resolveColumnRef(cat, sql, cursorOffset)
	case "relation_expr":
		return resolveRelationExpr(cat)
	case "func_name":
		return resolveFuncName(cat)
	default:
		return nil
	}
}

func resolveRelationExpr(cat *catalog.Catalog) []Candidate {
	var result []Candidate
	// Add schemas.
	for _, s := range cat.UserSchemas() {
		result = append(result, Candidate{Text: s.Name, Type: CandidateSchema})
	}
	// Add tables/views from visible schemas on the search path.
	for _, s := range cat.UserSchemas() {
		for name, rel := range s.Relations {
			ct := CandidateTable
			switch rel.RelKind {
			case 'v':
				ct = CandidateView
			case 'm':
				ct = CandidateMaterializedView
			}
			result = append(result, Candidate{Text: name, Type: ct})
		}
		for name := range s.Sequences {
			result = append(result, Candidate{Text: name, Type: CandidateSequence})
		}
	}
	return result
}

func resolveColumnRef(cat *catalog.Catalog, sql string, cursorOffset int) []Candidate {
	// Extract table references from the SQL to know which columns are in scope.
	refs := extractTableRefs(sql, cursorOffset)
	var result []Candidate
	for _, ref := range refs {
		rel := cat.GetRelation(ref.Schema, ref.Table)
		if rel == nil {
			continue
		}
		for _, col := range rel.Columns {
			result = append(result, Candidate{
				Text: col.Name,
				Type: CandidateColumn,
			})
		}
		// Add table name/alias as candidate too (for table.column qualification).
		alias := ref.Table
		if ref.Alias != "" {
			alias = ref.Alias
		}
		result = append(result, Candidate{Text: alias, Type: CandidateTable})
	}
	return result
}

func resolveFuncName(cat *catalog.Catalog) []Candidate {
	// Collect unique function names from built-in procs.
	seen := make(map[string]bool)
	var result []Candidate
	for _, name := range cat.AllProcNames() {
		if seen[name] {
			continue
		}
		seen[name] = true
		result = append(result, Candidate{Text: name, Type: CandidateFunction})
	}
	return result
}

func dedup(candidates []Candidate) []Candidate {
	type key struct {
		text string
		typ  CandidateType
	}
	seen := make(map[key]bool, len(candidates))
	result := make([]Candidate, 0, len(candidates))
	for _, c := range candidates {
		k := key{strings.ToLower(c.Text), c.Type}
		if seen[k] {
			continue
		}
		seen[k] = true
		result = append(result, c)
	}
	return result
}
```

Now implement `standardComplete` in `completion.go`:

```go
func standardComplete(sql string, cursorOffset int, cat *catalog.Catalog) []Candidate {
	cs := parser.Collect(sql, cursorOffset)
	return resolve(cs, cat, sql, cursorOffset)
}
```

We need `parser.TokenName()` — add to `complete.go`:

```go
// TokenName returns the SQL keyword text for a token type, or "" if unknown.
func TokenName(tokenType int) string {
	// Single-char tokens.
	if tokenType > 0 && tokenType < 256 {
		return string(rune(tokenType))
	}
	// Look up keyword name.
	for i := range Keywords {
		if Keywords[i].Token == tokenType {
			return strings.ToUpper(Keywords[i].Name)
		}
	}
	return ""
}
```

We also need `catalog.AllProcNames()` — add to `catalog.go`:

```go
// AllProcNames returns all unique procedure/function names.
func (c *Catalog) AllProcNames() []string {
	names := make([]string, 0, len(c.procByName))
	for name := range c.procByName {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
```

**Step 4: Run tests**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/completion/ -run TestComplete -v`
Expected: PASS (at least keyword and table tests)

**Step 5: Commit**

```bash
git add pg/completion/resolve.go pg/completion/completion.go pg/completion/completion_test.go pg/parser/complete.go pg/catalog/catalog.go
git commit -m "feat(pg/completion): implement catalog resolution for tables, columns, and functions"
```

---

## Task 9: Table Reference Extraction for Column Completion

**Files:**
- Create: `pg/completion/refs.go` (extract FROM clause table references)
- Create: `pg/completion/refs_test.go`

**Step 1: Write the failing test**

```go
// pg/completion/refs_test.go
package completion

import "testing"

func TestExtractTableRefs(t *testing.T) {
	tests := []struct {
		sql    string
		offset int
		want   []string // expected table names
	}{
		{"SELECT * FROM users WHERE ", 25, []string{"users"}},
		{"SELECT * FROM users u, orders o WHERE ", 37, []string{"users", "orders"}},
		{"SELECT * FROM public.users WHERE ", 32, []string{"users"}},
		{"SELECT * FROM users JOIN orders ON ", 34, []string{"users", "orders"}},
	}
	for _, tt := range tests {
		refs := extractTableRefs(tt.sql, tt.offset)
		got := make(map[string]bool)
		for _, r := range refs {
			got[r.Table] = true
		}
		for _, w := range tt.want {
			if !got[w] {
				t.Errorf("extractTableRefs(%q, %d): missing table %q", tt.sql, tt.offset, w)
			}
		}
	}
}
```

**Step 2: Run test to verify it fails**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/completion/ -run TestExtractTableRefs -v`
Expected: FAIL

**Step 3: Implement table reference extraction**

```go
// pg/completion/refs.go
package completion

import (
	"github.com/bytebase/omni/pg/ast"
	"github.com/bytebase/omni/pg/parser"
)

// TableRef represents a table reference found in a FROM clause.
type TableRef struct {
	Schema string
	Table  string
	Alias  string
}

// extractTableRefs parses the SQL and extracts table references visible at the cursor.
func extractTableRefs(sql string, cursorOffset int) []TableRef {
	// Try parsing the full SQL. If it fails (partial SQL), try with a placeholder.
	list, err := parser.Parse(sql)
	if err != nil || list == nil {
		// Try appending a placeholder to make it parseable.
		patched := sql[:cursorOffset] + " __placeholder__"
		list, err = parser.Parse(patched)
		if err != nil || list == nil {
			return extractTableRefsLexer(sql, cursorOffset)
		}
	}

	var refs []TableRef
	for _, item := range list.Items {
		raw, ok := item.(*ast.RawStmt)
		if !ok || raw.Stmt == nil {
			continue
		}
		refs = append(refs, collectRefsFromAST(raw.Stmt)...)
	}
	return refs
}

// collectRefsFromAST walks an AST node and collects table references.
func collectRefsFromAST(node ast.Node) []TableRef {
	var refs []TableRef
	ast.Inspect(node, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.RangeVar:
			if v != nil && v.Relname != "" {
				refs = append(refs, TableRef{
					Schema: v.Schemaname,
					Table:  v.Relname,
					Alias:  aliasName(v.Alias),
				})
			}
		case *ast.JoinExpr:
			// JoinExpr children are walked automatically.
		}
		return true
	})
	return refs
}

func aliasName(a *ast.Alias) string {
	if a == nil {
		return ""
	}
	return a.Aliasname
}

// extractTableRefsLexer is a fallback that uses the lexer to find FROM ... patterns.
func extractTableRefsLexer(sql string, cursorOffset int) []TableRef {
	lex := parser.NewLexer(sql)
	var refs []TableRef
	var tokens []parser.Token

	// Tokenize up to cursor.
	for {
		tok := lex.NextToken()
		if tok.Type == 0 || tok.Loc >= cursorOffset {
			break
		}
		tokens = append(tokens, tok)
	}

	// Scan for FROM/JOIN followed by identifiers.
	for i := 0; i < len(tokens); i++ {
		if tokens[i].Type == parser.FROM || tokens[i].Type == parser.JOIN ||
			tokens[i].Type == parser.INNER || tokens[i].Type == parser.LEFT ||
			tokens[i].Type == parser.RIGHT || tokens[i].Type == parser.FULL_P ||
			tokens[i].Type == parser.CROSS {
			// Skip JOIN keyword if present after INNER/LEFT/RIGHT/FULL/CROSS
			j := i + 1
			if j < len(tokens) && tokens[j].Type == parser.JOIN {
				j++
			}
			if j < len(tokens) && tokens[j].Type == parser.IDENT {
				ref := TableRef{Table: tokens[j].Str}
				// Check for schema.table
				if j+2 < len(tokens) && tokens[j+1].Type == '.' && tokens[j+2].Type == parser.IDENT {
					ref.Schema = ref.Table
					ref.Table = tokens[j+2].Str
					j += 2
				}
				// Check for alias
				if j+1 < len(tokens) {
					next := tokens[j+1]
					if next.Type == parser.AS && j+2 < len(tokens) {
						ref.Alias = tokens[j+2].Str
					} else if next.Type == parser.IDENT {
						ref.Alias = next.Str
					}
				}
				refs = append(refs, ref)
			}
		}
	}

	return refs
}
```

Note: This requires `NewLexer` and token type constants to be exported from the parser package. They likely already are based on what we've seen (NewLexer is already exported). Check and export if needed.

**Step 4: Run tests**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/completion/ -run TestExtractTableRefs -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pg/completion/refs.go pg/completion/refs_test.go
git commit -m "feat(pg/completion): implement table reference extraction for column completion"
```

---

## Task 10: Two-Pass Strategy (Tricky Completion for Partial SQL)

**Files:**
- Modify: `pg/completion/completion.go` (implement `trickyComplete`)
- Modify: `pg/completion/completion_test.go`

**Step 1: Write the failing test**

```go
// Add to pg/completion/completion_test.go
func TestCompleteTrickyPartialSQL(t *testing.T) {
	// "SELECT * FROM " — incomplete SQL, should still suggest tables.
	cat := catalog.New()
	cat.Exec("CREATE TABLE orders (id int);", nil)

	candidates := Complete("SELECT * FROM ", 14, cat)
	found := false
	for _, c := range candidates {
		if c.Type == CandidateTable && c.Text == "orders" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'orders' table candidate for incomplete SQL")
	}
}

func TestCompleteTrickyAfterDot(t *testing.T) {
	// "SELECT users." — after dot, should suggest columns.
	cat := catalog.New()
	cat.Exec("CREATE TABLE users (id int, name text);", nil)

	candidates := Complete("SELECT users. FROM users", 13, cat)
	var cols []string
	for _, c := range candidates {
		if c.Type == CandidateColumn {
			cols = append(cols, c.Text)
		}
	}
	if len(cols) == 0 {
		t.Error("expected column candidates after table.dot")
	}
}
```

**Step 2: Run test to verify**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/completion/ -run TestCompleteTricky -v`

**Step 3: Implement tricky completion**

```go
// In pg/completion/completion.go, implement trickyComplete:

func trickyComplete(sql string, cursorOffset int, cat *catalog.Catalog) []Candidate {
	// Strategy: make the SQL parseable by inserting a placeholder at cursor.
	prefix := sql[:cursorOffset]
	suffix := ""
	if cursorOffset < len(sql) {
		suffix = sql[cursorOffset:]
	}

	// Try several placeholder strategies.
	placeholders := []string{
		prefix + "__placeholder__ " + suffix,                         // identifier placeholder
		prefix + "__placeholder__ FROM __t__ " + suffix,              // add FROM if missing
		prefix + "1 " + suffix,                                       // numeric placeholder
	}

	for _, patched := range placeholders {
		cs := parser.Collect(patched, cursorOffset)
		if cs != nil && (len(cs.Tokens) > 0 || len(cs.Rules) > 0) {
			return resolve(cs, cat, sql, cursorOffset)
		}
	}

	return nil
}
```

**Step 4: Run tests**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/completion/ -run TestComplete -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pg/completion/completion.go pg/completion/completion_test.go
git commit -m "feat(pg/completion): implement two-pass tricky completion for partial SQL"
```

---

## Task 11: FIRST-Set Cache for Performance

**Files:**
- Create: `pg/parser/cache.go`
- Modify: `pg/parser/complete.go` (use cache)
- Create: `pg/parser/cache_test.go`

**Step 1: Write the failing test**

```go
// pg/parser/cache_test.go
package parser

import (
	"testing"
	"time"
)

func TestCompletionPerformance(t *testing.T) {
	// Warm the cache.
	Collect("SELECT ", 7)

	// Second call should be fast (< 5ms).
	start := time.Now()
	for i := 0; i < 100; i++ {
		Collect("SELECT * FROM t WHERE x = ", 26)
	}
	elapsed := time.Since(start)
	avg := elapsed / 100
	if avg > 5*time.Millisecond {
		t.Errorf("completion too slow: avg %v (want < 5ms)", avg)
	}
}
```

**Step 2: Run test**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/parser/ -run TestCompletionPerformance -v`

**Step 3: Implement FIRST-set cache**

```go
// pg/parser/cache.go
package parser

import "sync"

// firstSetCache caches the candidates emitted by parse functions in collect mode.
// Key: function identity + context (e.g. "parseCExprInner", "parseAExpr:0").
// Value: snapshot of tokens and rules that function would emit.
type firstSetCache struct {
	mu sync.RWMutex
	m  map[string]*CandidateSet
}

var globalFirstSets = &firstSetCache{
	m: make(map[string]*CandidateSet),
}

func (c *firstSetCache) get(key string) *CandidateSet {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.m[key]
}

func (c *firstSetCache) set(key string, cs *CandidateSet) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.m[key] = cs
}

// cachedCollect checks the cache before running collect mode for a rule.
// If cached, it merges the cached candidates into the parser's candidate set.
// Returns true if cache was used.
func (p *Parser) cachedCollect(key string, fn func()) bool {
	if cached := globalFirstSets.get(key); cached != nil {
		// Merge cached candidates.
		for _, tok := range cached.Tokens {
			p.addTokenCandidate(tok)
		}
		for _, rule := range cached.Rules {
			p.addRuleCandidate(rule.Rule)
		}
		return true
	}

	// Run the function and capture what it adds.
	before := p.candidates.snapshot()
	fn()
	after := p.candidates

	// Store the diff.
	diff := after.diff(before)
	if diff != nil {
		globalFirstSets.set(key, diff)
	}
	return false
}
```

Add snapshot/diff methods to CandidateSet in `complete.go`:

```go
func (cs *CandidateSet) snapshot() *CandidateSet {
	snap := &CandidateSet{
		Tokens: make([]int, len(cs.Tokens)),
		Rules:  make([]RuleCandidate, len(cs.Rules)),
		seen:   make(map[int]bool, len(cs.seen)),
		seenR:  make(map[string]bool, len(cs.seenR)),
	}
	copy(snap.Tokens, cs.Tokens)
	copy(snap.Rules, cs.Rules)
	for k, v := range cs.seen {
		snap.seen[k] = v
	}
	for k, v := range cs.seenR {
		snap.seenR[k] = v
	}
	return snap
}

func (cs *CandidateSet) diff(before *CandidateSet) *CandidateSet {
	d := newCandidateSet()
	for _, tok := range cs.Tokens {
		if !before.seen[tok] {
			d.addToken(tok)
		}
	}
	for _, rule := range cs.Rules {
		if !before.seenR[rule.Rule] {
			d.addRule(rule.Rule)
		}
	}
	if len(d.Tokens) == 0 && len(d.Rules) == 0 {
		return nil
	}
	return d
}
```

Now use `cachedCollect` in the key collect-mode blocks. For example, in `parseCExprInner`:

```go
	if p.collectMode() {
		if p.cachedCollect("parseCExprInner", func() {
			// ... existing collect logic ...
		}) {
			return nil
		}
		return nil
	}
```

**Step 4: Run tests**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/parser/ -run TestCompletion -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pg/parser/cache.go pg/parser/complete.go pg/parser/cache_test.go
git commit -m "feat(pg/parser): add FIRST-set cache for completion performance"
```

---

## Task 12: CTE and Subquery Support

**Files:**
- Modify: `pg/completion/refs.go` (add CTE extraction)
- Modify: `pg/completion/refs_test.go`

**Step 1: Write the failing test**

```go
// Add to pg/completion/refs_test.go
func TestExtractCTERefs(t *testing.T) {
	sql := "WITH active_users AS (SELECT id, name FROM users WHERE active) SELECT  FROM active_users"
	// Cursor at position 73 (after "SELECT ")
	refs := extractTableRefs(sql, 73)
	found := false
	for _, r := range refs {
		if r.Table == "active_users" {
			found = true
		}
	}
	if !found {
		t.Error("expected CTE 'active_users' in table refs")
	}
}
```

**Step 2: Run test**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/completion/ -run TestExtractCTE -v`

**Step 3: Add CTE extraction to collectRefsFromAST**

```go
// Add to collectRefsFromAST in refs.go:
func collectRefsFromAST(node ast.Node) []TableRef {
	var refs []TableRef
	ast.Inspect(node, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.RangeVar:
			if v != nil && v.Relname != "" {
				refs = append(refs, TableRef{
					Schema: v.Schemaname,
					Table:  v.Relname,
					Alias:  aliasName(v.Alias),
				})
			}
		case *ast.CommonTableExpr:
			// CTE definitions are virtual tables.
			if v != nil && v.Ctename != "" {
				refs = append(refs, TableRef{
					Table: v.Ctename,
				})
			}
		}
		return true
	})
	return refs
}
```

**Step 4: Run tests**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/completion/ -run TestExtract -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pg/completion/refs.go pg/completion/refs_test.go
git commit -m "feat(pg/completion): add CTE support for table reference extraction"
```

---

## Task 13: Token Name Mapping and Prefix Filtering

**Files:**
- Modify: `pg/parser/complete.go` (add `TokenName`)
- Modify: `pg/completion/completion.go` (add prefix filtering)
- Modify: `pg/completion/completion_test.go`

**Step 1: Write the failing test**

```go
// Add to pg/completion/completion_test.go
func TestCompletePrefixFiltering(t *testing.T) {
	// "SEL" at position 3 — should only suggest keywords starting with "SEL".
	candidates := Complete("SEL", 3, nil)
	for _, c := range candidates {
		if c.Type == CandidateKeyword && c.Text == "INSERT" {
			t.Error("INSERT should be filtered out by prefix 'SEL'")
		}
	}
	found := false
	for _, c := range candidates {
		if c.Type == CandidateKeyword && c.Text == "SELECT" {
			found = true
		}
	}
	if !found {
		t.Error("expected SELECT to match prefix 'SEL'")
	}
}
```

**Step 2: Run test**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/completion/ -run TestCompletePrefixFiltering -v`

**Step 3: Implement prefix filtering**

In `pg/completion/completion.go`, add prefix extraction and filtering:

```go
// extractPrefix returns the partial token the user is typing at cursorOffset.
func extractPrefix(sql string, cursorOffset int) string {
	if cursorOffset > len(sql) {
		cursorOffset = len(sql)
	}
	i := cursorOffset
	for i > 0 && isIdentChar(sql[i-1]) {
		i--
	}
	return sql[i:cursorOffset]
}

func isIdentChar(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
		(c >= '0' && c <= '9') || c == '_'
}

// filterByPrefix filters candidates by the partial token prefix.
func filterByPrefix(candidates []Candidate, prefix string) []Candidate {
	if prefix == "" {
		return candidates
	}
	prefix = strings.ToUpper(prefix)
	var result []Candidate
	for _, c := range candidates {
		if strings.HasPrefix(strings.ToUpper(c.Text), prefix) {
			result = append(result, c)
		}
	}
	return result
}
```

Modify `Complete` to apply filtering:

```go
func Complete(sql string, cursorOffset int, cat *catalog.Catalog) []Candidate {
	prefix := extractPrefix(sql, cursorOffset)

	result := standardComplete(sql, cursorOffset, cat)
	if len(result) == 0 {
		result = trickyComplete(sql, cursorOffset, cat)
	}

	return filterByPrefix(result, prefix)
}
```

**Step 4: Run tests**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/completion/ -run TestComplete -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pg/completion/completion.go pg/completion/completion_test.go
git commit -m "feat(pg/completion): add prefix filtering for typed partial tokens"
```

---

## Task 14: Collect-Mode for CREATE, ALTER, DROP Dispatch

**Files:**
- Modify: `pg/parser/parser.go` (add collect mode to `parseCreateDispatch`)
- Modify: `pg/parser/complete_test.go`

**Step 1: Write the failing test**

```go
// Add to pg/parser/complete_test.go
func TestCollectAfterCreate(t *testing.T) {
	candidates := Collect("CREATE ", 7)
	expected := []int{
		TABLE, VIEW, INDEX, SEQUENCE, FUNCTION, PROCEDURE, DATABASE,
		ROLE, USER, SCHEMA, TYPE_P, DOMAIN_P, TRIGGER, POLICY,
		EXTENSION, MATERIALIZED, TEMPORARY, TEMP, UNLOGGED,
		UNIQUE, FOREIGN, SERVER, PUBLICATION, SUBSCRIPTION,
		RULE, AGGREGATE, OPERATOR, TEXT_P, COLLATION, STATISTICS,
		CAST, TRANSFORM, CONVERSION_P, TABLESPACE, ACCESS,
		EVENT, LANGUAGE, TRUSTED, PROCEDURAL, OR, RECURSIVE,
		GROUP_P, DEFAULT, GLOBAL, CONSTRAINT,
	}
	for _, tok := range expected {
		if !candidates.HasToken(tok) {
			t.Errorf("missing CREATE sub-keyword: %d", tok)
		}
	}
}

func TestCollectAfterAlter(t *testing.T) {
	candidates := Collect("ALTER ", 6)
	expected := []int{
		DATABASE, ROLE, USER, SERVER, GROUP_P, POLICY, PUBLICATION,
		SUBSCRIPTION, STATISTICS, OPERATOR, SCHEMA, DEFAULT,
		FUNCTION, PROCEDURE, ROUTINE, TYPE_P, DOMAIN_P, COLLATION,
		CONVERSION_P, EXTENSION, AGGREGATE, TEXT_P, LANGUAGE,
		PROCEDURAL, LARGE_P, EVENT, SYSTEM_P, TABLESPACE,
		TRIGGER, RULE, TABLE,
	}
	for _, tok := range expected {
		if !candidates.HasToken(tok) {
			t.Errorf("missing ALTER sub-keyword: %d", tok)
		}
	}
}

func TestCollectAfterDrop(t *testing.T) {
	candidates := Collect("DROP ", 5)
	// DROP dispatches to parseDropStmt which handles many object types.
	if candidates == nil || len(candidates.Tokens) == 0 {
		t.Error("expected candidates after DROP")
	}
}
```

**Step 2: Run test**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/parser/ -run TestCollectAfter -v`

**Step 3: Add collect mode to CREATE and ALTER dispatch**

In `parseCreateDispatch`, the current token is CREATE and we peek at next. In collect mode at position after CREATE, we need to emit all valid next tokens:

```go
func (p *Parser) parseCreateDispatch() nodes.Node {
	if p.collectMode() {
		// All tokens that can follow CREATE.
		createFollowTokens := []int{
			OR, VIEW, RECURSIVE, MATERIALIZED, TABLE,
			TEMPORARY, TEMP, LOCAL, UNLOGGED,
			UNIQUE, INDEX, SEQUENCE, DOMAIN_P, TYPE_P,
			AGGREGATE, OPERATOR, TEXT_P, COLLATION, STATISTICS,
			FUNCTION, PROCEDURE, DATABASE, ROLE, USER, GROUP_P,
			POLICY, TRIGGER, CONSTRAINT, EVENT, FOREIGN, SERVER,
			LANGUAGE, TRUSTED, PROCEDURAL, GLOBAL,
			PUBLICATION, SUBSCRIPTION, RULE, EXTENSION, ACCESS,
			CAST, TRANSFORM, CONVERSION_P, DEFAULT, TABLESPACE, SCHEMA,
		}
		for _, t := range createFollowTokens {
			p.addTokenCandidate(t)
		}
		return nil
	}
	// ... existing code ...
```

Similarly, add a collect-mode block at the top of the ALTER switch in `parseStmt`:

In `parseStmt`, the ALTER case already does `p.advance()` then a switch. We need to add collect mode after the advance. Since `advance()` triggers cursor checking, the switch will be reached in collect mode if the cursor is after ALTER. Add:

```go
	case ALTER:
		p.advance() // consume ALTER
		if p.collectMode() {
			alterFollowTokens := []int{
				DATABASE, ROLE, USER, SERVER, GROUP_P, POLICY,
				PUBLICATION, SUBSCRIPTION, STATISTICS, OPERATOR,
				SCHEMA, DEFAULT, FUNCTION, PROCEDURE, ROUTINE,
				TYPE_P, DOMAIN_P, COLLATION, CONVERSION_P, EXTENSION,
				AGGREGATE, TEXT_P, LANGUAGE, PROCEDURAL, LARGE_P,
				EVENT, SYSTEM_P, TABLESPACE, TRIGGER, RULE, TABLE,
				// TABLE is default case
			}
			for _, t := range alterFollowTokens {
				p.addTokenCandidate(t)
			}
			return nil
		}
		// ... existing switch ...
```

Apply the same pattern for DROP and other dispatchers.

**Step 4: Run tests**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/parser/ -run TestCollect -v`
Expected: PASS

**Step 5: Commit**

```bash
git add pg/parser/parser.go pg/parser/complete_test.go
git commit -m "feat(pg/parser): add collect mode for CREATE/ALTER/DROP dispatch"
```

---

## Task 15: Public API Integration and End-to-End Test

**Files:**
- Modify: `pg/completion/completion.go` (finalize public API)
- Create: `pg/completion/completion_integration_test.go`

**Step 1: Write the integration test**

```go
// pg/completion/completion_integration_test.go
package completion

import (
	"testing"

	"github.com/bytebase/omni/pg/catalog"
)

func TestEndToEnd(t *testing.T) {
	cat := catalog.New()
	cat.Exec(`
		CREATE TABLE users (id serial PRIMARY KEY, name text NOT NULL, email text);
		CREATE TABLE orders (id serial PRIMARY KEY, user_id int REFERENCES users(id), amount numeric);
		CREATE VIEW active_users AS SELECT * FROM users WHERE name IS NOT NULL;
	`, nil)

	tests := []struct {
		name   string
		sql    string
		offset int
		want   []string // must contain these
		wantType CandidateType
	}{
		{
			name:     "statement start",
			sql:      "",
			offset:   0,
			want:     []string{"SELECT", "INSERT", "CREATE", "ALTER", "DROP"},
			wantType: CandidateKeyword,
		},
		{
			name:     "after SELECT",
			sql:      "SELECT ",
			offset:   7,
			want:     []string{"id", "name", "email"},
			wantType: CandidateColumn,
		},
		{
			name:     "after FROM",
			sql:      "SELECT 1 FROM ",
			offset:   14,
			want:     []string{"users", "orders"},
			wantType: CandidateTable,
		},
		{
			name:     "after FROM with view",
			sql:      "SELECT 1 FROM ",
			offset:   14,
			want:     []string{"active_users"},
			wantType: CandidateView,
		},
		{
			name:     "after WHERE",
			sql:      "SELECT * FROM users WHERE ",
			offset:   26,
			want:     []string{"id", "name", "email"},
			wantType: CandidateColumn,
		},
		{
			name:     "after CREATE",
			sql:      "CREATE ",
			offset:   7,
			want:     []string{"TABLE", "VIEW", "INDEX", "FUNCTION"},
			wantType: CandidateKeyword,
		},
		{
			name:     "prefix filter",
			sql:      "SEL",
			offset:   3,
			want:     []string{"SELECT"},
			wantType: CandidateKeyword,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			candidates := Complete(tt.sql, tt.offset, cat)
			for _, w := range tt.want {
				found := false
				for _, c := range candidates {
					if c.Text == w && (tt.wantType == 0 || c.Type == tt.wantType) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("missing candidate %q (type %d) in %v", w, tt.wantType, candidateTexts(candidates))
				}
			}
		})
	}
}

func candidateTexts(cs []Candidate) []string {
	var texts []string
	for _, c := range cs {
		texts = append(texts, c.Text)
	}
	return texts
}
```

**Step 2: Run test**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/completion/ -run TestEndToEnd -v`

**Step 3: Fix any issues surfaced by integration test**

This step is iterative — fix failures as they appear. Common issues:
- Table references not extracted from partial SQL → enhance tricky completion
- Column candidates missing → ensure refs.go parses FROM clause correctly
- Nil pointer panics in collect mode → add nil guards

**Step 4: Run full test suite**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/... -v`
Expected: ALL PASS (existing parser tests + new completion tests)

**Step 5: Commit**

```bash
git add pg/completion/completion_integration_test.go
git commit -m "feat(pg/completion): add end-to-end integration tests"
```

---

## Task 16: Ensure Existing Parser Tests Still Pass

**Files:**
- No new files — verification only

**Step 1: Run all existing tests**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/parser/ -v -count=1`
Expected: ALL PASS — completion mode changes to `match`/`expect`/`advance` must not affect normal parsing

**Step 2: Run catalog tests**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/catalog/ -v -count=1`
Expected: ALL PASS

**Step 3: Run parse tests**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./pg/ -v -count=1`
Expected: ALL PASS

**Step 4: Run full project tests**

Run: `cd /Users/rebeliceyang/Github/omni && go test ./... 2>&1 | tail -20`
Expected: ALL PASS

**Step 5: Commit (no changes needed if all pass)**

If any test fails, fix and commit:
```bash
git add -u
git commit -m "fix: ensure completion mode doesn't affect normal parser behavior"
```

---

## Summary of Files Created/Modified

### New Files
| File | Purpose | ~Lines |
|---|---|---|
| `pg/completion/completion.go` | Public API, types, two-pass strategy | ~100 |
| `pg/completion/resolve.go` | Rule → catalog object resolution | ~150 |
| `pg/completion/refs.go` | Table reference extraction (AST + lexer fallback) | ~150 |
| `pg/completion/completion_test.go` | Unit tests | ~100 |
| `pg/completion/refs_test.go` | Table ref extraction tests | ~50 |
| `pg/completion/completion_integration_test.go` | End-to-end tests | ~100 |
| `pg/parser/complete.go` | CandidateSet, Collect(), helpers | ~150 |
| `pg/parser/cache.go` | FIRST-set cache | ~80 |
| `pg/parser/complete_test.go` | Parser collect-mode tests | ~100 |
| `pg/parser/cache_test.go` | Performance tests | ~30 |

### Modified Files
| File | Change |
|---|---|
| `pg/parser/parser.go` | Add completion fields to Parser struct; modify `match`, `expect`, `advance`; add collect-mode blocks to `parseStmt`, `parseCreateDispatch`, ALTER/DROP dispatch |
| `pg/parser/name.go` | Add collect mode to `parseColumnRef`, `parseQualifiedName`, `parseFuncName`, `isColId`, `isColLabel`, `isTypeFunctionName` |
| `pg/parser/expr.go` | Add collect mode to `parseCExprInner`, `parseAExpr` |
| `pg/parser/select.go` | Add collect mode clause keyword emission to `parseSimpleSelectCore` |
| `pg/catalog/catalog.go` | Add `AllProcNames()` method |

### Total Estimate
- ~1000 lines new code
- ~100 lines modified in existing files
- ~380 lines of tests
