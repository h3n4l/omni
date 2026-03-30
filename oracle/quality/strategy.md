# Oracle Quality Pipeline — Coverage Strategy

This file is read by Eval Workers to determine what tests to generate,
and updated by Insight Workers when blind spots are discovered.

## Stage 1: Foundation

### Scope
- Loc sentinel: all unknown positions use `Loc{Start: -1, End: -1}` (not End=0)
- Token.End: lexer tracks end byte offset for every token
- ParseError: has Severity, Code, Message, Position fields
- Parser.source: stores input SQL for tokenText extraction
- RawStmt: uses `Loc` instead of `StmtLocation`/`StmtLen`
- Utility functions: `NoLoc()`, `NodeLoc()`, `ListSpan()`

### Evaluation Strategy
- **PG behavior reference (C)**: each feature must match PG's equivalent behavior
- Eval tests verify structural properties (field existence, sentinel values, function signatures)

### Enumerable Set
- 10 infrastructure items — all must have eval tests

### Known Blind Spots
- **Loc.End dual-sentinel ambiguity:** The End field comment says "0 means unknown/unset" but NoLoc() uses -1. Stage 2 Loc validation must use -1 only. The End=0 comment in node.go should be corrected.
- **ParseError call sites omit Severity/Code:** All existing `&ParseError{}` constructions rely on defaults. No test verifies that non-default severity/code values are ever set by the parser in practice.
- **NodeLoc exhaustive switch maintenance:** No compile-time or test-time check ensures NodeLoc covers all node types. A new type added without a NodeLoc case silently returns NoLoc().
- **Unicode byte offsets untested:** All eval tests use ASCII input. Loc values are byte offsets, but multi-byte characters (UTF-8 Oracle identifiers) could cause Loc.End to be wrong if the lexer counts characters instead of bytes.
- **Token.End for whitespace/comment tokens:** The lexer skips whitespace and comments internally. Token.End tracking for comment-spanning tokens is untested.
- **Parser.source field not verified in Parse():** The eval test checks the field exists via reflect but does not verify Parse() actually sets it (it only constructs a Parser directly).

---

## Stage 2: Loc Completeness

### Scope
- Every Loc-bearing AST node (248 types) must have valid Start/End after parsing
- `sql[node.Loc.Start:node.Loc.End]` must produce a reasonable substring

### Evaluation Strategy
- **Reflection walker (automated)**: walk parsed AST, check every Loc field
- For each node type, at least one SQL statement that exercises it

### Enumerable Set
- 248 Loc-bearing struct types in `oracle/ast/parsenodes.go`
- Coverage: each type must appear in at least one eval test SQL

### Generation Strategy
- For each AST node type, find the simplest SQL that produces that node
- Parse → walk → verify Loc.Start >= 0 && Loc.End > Loc.Start

### Known Blind Spots
(initially empty)

---

## Stage 3: AST Correctness

### Scope
- Parser produces correct AST structure for all supported SQL
- Field values match SQL semantics (correct identifiers, operators, clause types)

### Evaluation Strategy
- **Oracle DB cross-validation (B)**: SQL executed on real Oracle DB; if DB accepts, parser must parse successfully
- **Structural assertions (A)**: parse result fields checked against expected values

### Enumerable Set
- BNF production rules (230+ files in oracle/parser/bnf/)
- Each rule: at least 1 test case with structural assertion

### Generation Strategy (non-enumerable)
- Oracle documentation example SQL → validated against Oracle DB
- Real-world Oracle SQL corpus → validated against Oracle DB

### Known Blind Spots
(initially empty)

---

## Stage 4: Error Quality

### Scope
- Invalid SQL detected (not silently accepted)
- Parser does not panic on any input
- Error position points to the correct token
- Error message is descriptive

### Evaluation Strategy
- **Mutation generation**: from valid SQL, systematically produce invalid variants
  - Truncation: cut at each token boundary
  - Deletion: remove one required constituent
  - Replacement: swap keyword with invalid token
  - Duplication: repeat a clause
- **Oracle DB cross-validation (B)**: if Oracle DB rejects SQL, parser should also reject

### Enumerable Set
- None (error space is infinite)

### Generation Strategy
- Take all Layer 1/2 valid SQL from Stage 3
- Apply 4 mutation types to each
- Verify: no panic, returns error, position >= 0

### Known Blind Spots
(initially empty)

---

## Stage 5: Completion

### Scope
- Parser suggests valid syntax continuations at cursor position
- Candidates include relevant keywords, identifiers, clause types

### Evaluation Strategy
- **PG behavior reference (C)**: completion infrastructure mirrors PG's pattern
- **Candidate set assertions (A)**: at cursor position X, candidates must include/exclude specific items

### Enumerable Set
- Statement types (30+) x key cursor positions (after keyword, after table, in WHERE, etc.)

### Generation Strategy (non-enumerable)
- Nested queries, CTE context, PL/SQL block context

### Known Blind Spots
(initially empty)

---

## Stage 6: Catalog / Migration

### Scope
- In-memory schema model from parsed DDL
- Migration DDL generation between schema states
- Round-trip: apply migration to source schema → matches target schema

### Evaluation Strategy
- **Oracle DB oracle verification (B)**: DDL round-trip on real Oracle container
- **Pairwise combinations**: object types x operations x properties

### Enumerable Set
- Object types: table, view, index, trigger, sequence, function, procedure, package, type, synonym
- Operations: create, alter, drop, rename, comment

### Generation Strategy (non-enumerable)
- Pairwise: cover all 2-way dimension combinations
- Property combinations generated systematically, not exhaustively

### Known Blind Spots
(initially empty)
