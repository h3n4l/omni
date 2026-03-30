# Pattern: nodeloc-exhaustive-switch

## Category
migration-incomplete

## Description
NodeLoc() uses an exhaustive type switch with 249 cases covering every AST node type.
This is a maintenance burden — every time a new node type is added to parsenodes.go,
a corresponding case must be added to loc.go's NodeLoc(). If a case is missed, NodeLoc()
silently returns NoLoc() (the default branch), which is a hard-to-detect bug.

PG uses the same pattern, so this is "correct by convention," but Oracle's parser has
many more node types than PG (249 vs ~200). The risk compounds in future batches that
add new node types.

An alternative approach (reflection-based) was implemented in oracle/ast/loc.go via a
separate walker, but NodeLoc itself remains a manual switch.

## Example
- Commit: 761bf10
- File: oracle/ast/loc.go
- What: 249-case type switch for NodeLoc
- Risk: New node types silently get NoLoc() if not added to switch

## Where Else to Check
- Any future node type additions in parsenodes.go
- outfuncs.go — similar exhaustive switch for serialization
- compare_test.go — may need similar exhaustive coverage

## Prevention Rule
When adding a new AST node type to parsenodes.go, also add it to NodeLoc() in loc.go
AND to outfuncs.go. Use `grep -c 'type.*struct' parsenodes.go` vs `grep -c 'case \*' loc.go`
to verify counts match. Consider adding a compile-time or test-time check.
