# Pattern: loc-sentinel-ambiguity

## Category
sentinel-mismatch

## Description
The Loc struct has an ambiguity in its End field comment: "0 means unknown/unset, -1 means
unknown". This creates confusion — is End=0 a valid position or a sentinel? The comment on
the struct says "0 means unknown/unset" for End, while Start uses -1 for unknown. But
NoLoc() returns {-1, -1}, and the eval test for LocSentinel asserts that Loc{0,0} is a
valid position (not unknown).

This dual-sentinel issue (End=0 as "unknown/unset" in comments vs End=-1 as NoLoc) means
code checking "is this Loc valid?" must handle both conventions. This is a latent bug
source for Stage 2 (Loc completeness) where every node's Loc must be validated.

## Example
- Commit: 761bf10
- File: oracle/ast/node.go
- What: End field comment says "0 means unknown/unset, -1 means unknown"
- Issue: Two sentinel values for the same concept

## Where Else to Check
- Stage 2 Loc validation logic: must decide if End=0 is valid or sentinel
- ListSpan() — currently checks only -1, not 0
- NodeLoc() — returns NoLoc() (-1,-1) for unknown, never {0,0}
- Any future code that checks `loc.End == 0` vs `loc.End == -1`

## Prevention Rule
Use -1 as the ONLY sentinel for "unknown" in Loc fields (both Start and End). Treat End=0
as a valid position (start of source). Update the Loc End field comment to remove "0 means
unknown/unset" — it contradicts the NoLoc() convention. Stage 2 impl worker must address this.
