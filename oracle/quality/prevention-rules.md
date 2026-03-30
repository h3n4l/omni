# Prevention Rules

Accumulated rules from Insight Worker analysis across all stages.
Impl Workers MUST read this file before starting work.

## Rules

### Rule 1: Use Loc{Start,End} not offset+length (from Stage 1)
- **Pattern:** A struct tracks source position with separate offset and length integer fields.
- **Prevention:** Always use `Loc ast.Loc` with `Start` (inclusive) and `End` (exclusive) fields. Never use offset+length pairs.
- **Example:** RawStmt was changed from `StmtLocation int` + `StmtLen int` to `Loc ast.Loc`.

### Rule 2: Set ParseError Severity/Code explicitly for non-syntax errors (from Stage 1)
- **Pattern:** Creating `&ParseError{Message: ..., Position: ...}` without setting Severity or Code, even when the error is not a standard syntax error.
- **Prevention:** Always set Severity and Code when they differ from defaults ("ERROR" / "42601"). Future error categories (semantic errors, warnings) need distinct codes.
- **Example:** An access-violation error should use Code "42000", not the default "42601".

### Rule 3: Use -1 as the only sentinel for unknown Loc (from Stage 1)
- **Pattern:** Loc.End = 0 is treated as "unknown/unset" in some contexts, while NoLoc() uses -1. This creates ambiguity.
- **Prevention:** Use -1 as the ONLY sentinel for unknown positions. Loc{0,0} is a valid position (start of source). When checking if a Loc is valid, compare against -1 only.
- **Example:** `if loc.End == -1 { /* unknown */ }` is correct. `if loc.End == 0 { /* unknown */ }` is wrong — position 0 is valid.

### Rule 4: Keep NodeLoc switch in sync with parsenodes.go (from Stage 1)
- **Pattern:** Adding a new AST node type to parsenodes.go without adding a corresponding case to NodeLoc() in loc.go.
- **Prevention:** After adding any new struct type with a Loc field to parsenodes.go, immediately add a `case *NewType:` to NodeLoc(). Verify: `grep -c 'type.*struct' parsenodes.go` must equal `grep -c 'case \*' loc.go`.
- **Example:** Adding `CreateFooStmt` to parsenodes.go requires adding `case *CreateFooStmt: return v.Loc` to loc.go.
