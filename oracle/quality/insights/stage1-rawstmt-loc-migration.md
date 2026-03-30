# Pattern: rawstmt-loc-migration

## Category
migration-incomplete

## Description
The RawStmt struct originally used separate `StmtLocation int` and `StmtLen int` fields
to track position. PG's convention is a single `Loc ast.Loc` field with `Start` and `End`.
The impl commit (761bf10) changed `StmtLocation`/`StmtLen` to `Loc ast.Loc`, and updated
the parser to compute `End = Start + length` rather than storing length separately.

This is a clean migration, but the pattern generalizes: any struct that stores position as
"offset + length" rather than "start + end" is a migration candidate. The "length" approach
is error-prone because callers must compute end = offset + length every time.

## Example
- Commit: 761bf10
- File: oracle/ast/parsenodes.go, oracle/parser/parser.go
- What: RawStmt had `StmtLocation int` + `StmtLen int`
- Fix: Changed to `Loc ast.Loc` with `Start` and `End` fields

## Where Else to Check
- Any other AST node that uses offset+length pair instead of Loc
- Any helper functions that compute end from offset+length

## Prevention Rule
When adding position tracking to any AST node, always use `Loc ast.Loc` (Start/End pair),
never separate offset + length fields. This matches PG convention and avoids off-by-one errors.
