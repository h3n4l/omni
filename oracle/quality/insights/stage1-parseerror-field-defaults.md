# Pattern: parseerror-field-defaults

## Category
missing-field

## Description
ParseError was originally a minimal struct with only `Message` and `Position`.
PG's ParseError has `Severity`, `Code`, `Message`, and `Position`, with defaults
of "ERROR" and "42601" for syntax errors. The impl commit (761bf10) added
Severity and Code fields with default handling in Error().

However, all existing call sites that create ParseError still omit Severity and
Code fields, relying on defaults. This is acceptable for syntax errors but will
become a problem when WARNING or NOTICE severity errors are needed, or when
different SQLSTATE codes (e.g., "42000" for access violation) are required.

## Example
- Commit: 761bf10
- File: oracle/parser/parser.go
- What: ParseError had only Message + Position
- Fix: Added Severity and Code fields with default fallbacks

## Where Else to Check
- All `&ParseError{` construction sites in oracle/parser/ — currently none set Severity/Code
- Future error types in completion or catalog stages
- Oracle uses ORA- error codes, not SQLSTATE — may need mapping

## Prevention Rule
When creating ParseError instances, explicitly set Severity and Code when they differ
from the defaults ("ERROR" / "42601"). Do not rely on defaults for non-syntax errors.
