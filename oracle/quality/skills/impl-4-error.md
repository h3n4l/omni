# Impl Worker — Stage 4: Error Quality

You are an Impl Worker in the Oracle Quality Pipeline.
Your role is to write **implementation code ONLY** — never modify `*_eval_*_test.go` files.

**Working directory:** `/Users/rebeliceyang/Github/omni`

## Reference Files (Read Before Starting)

- `oracle/quality/prevention-rules.md` — **MUST read before starting any work**
- `oracle/quality/strategy.md` — Stage 4 scope
- `oracle/parser/eval_error_test.go` — eval tests you must make pass
- `oracle/parser/parser.go` — current `ParseError` and `Parser`
- `oracle/parser/lexer.go` — current `Token` and `Lexer`
- `pg/parser/parser.go` — PG's error recovery patterns (reference)

## Goal

Make **all** eval tests pass:

```bash
cd /Users/rebeliceyang/Github/omni && go test -v -count=1 ./oracle/parser/ -run "TestEvalStage4"
```

While keeping **all existing tests** passing (Stages 1-3 and existing tests):

```bash
cd /Users/rebeliceyang/Github/omni && go test -count=1 ./oracle/...
```

## Rules

1. **Implementation ONLY** — do NOT modify any `*_eval_*_test.go` file.
2. Do NOT break existing tests.
3. Read `oracle/quality/prevention-rules.md` before starting.
4. Keep changes minimal and focused — do not refactor unrelated code.

## Progress Logging (MANDATORY)

Print these markers to stdout at each step:

```
[IMPL-STAGE4] STARTED
[IMPL-STAGE4] STEP reading_eval - Reading eval test expectations
[IMPL-STAGE4] STEP reading_prevention - Reading prevention rules
[IMPL-STAGE4] STEP fix_panics - Fixing panic paths in parser
[IMPL-STAGE4] STEP fix_nil_checks - Adding nil checks for unexpected tokens
[IMPL-STAGE4] STEP fix_error_recovery - Improving error recovery
[IMPL-STAGE4] STEP fix_error_position - Ensuring error positions are valid
[IMPL-STAGE4] STEP build - Running go build
[IMPL-STAGE4] STEP test_eval - Running Stage 4 eval tests
[IMPL-STAGE4] STEP test_existing - Running all existing tests (Stages 1-3)
[IMPL-STAGE4] STEP commit - Committing changes
[IMPL-STAGE4] DONE
```

If a step fails:
```
[IMPL-STAGE4] FAIL step_name - description
[IMPL-STAGE4] RETRY - what you're fixing
```

**Do NOT skip these markers.**

## Typical Fixes

### 1. Panic Prevention — `oracle/parser/parser.go`

Find all code paths that can panic on unexpected input:
- Array/slice index out of bounds
- Nil pointer dereferences on AST nodes
- Unreachable `default` cases in switch statements that call `panic()`
- Division by zero in expression parsing

Replace panics with soft failures:
```go
// Before (panics on unexpected token)
func (p *Parser) parseExpr() nodes.Node {
    switch p.cur.Type {
    case tokenIdent:
        return p.parseIdent()
    default:
        panic(fmt.Sprintf("unexpected token: %s", p.cur.Str))
    }
}

// After (returns error via parser state)
func (p *Parser) parseExpr() nodes.Node {
    switch p.cur.Type {
    case tokenIdent:
        return p.parseIdent()
    default:
        p.addError(p.cur.Loc, "unexpected token: %s", p.cur.Str)
        return nil
    }
}
```

### 2. Nil Checks — Throughout parser

Add nil checks after every parse function call:
```go
node := p.parseExpr()
if node == nil {
    // Don't proceed — parser already recorded the error
    return nil
}
```

### 3. Error Recovery — `oracle/parser/parser.go`

Implement or improve error recovery to avoid cascading errors:
```go
// Skip tokens until we find a synchronization point
func (p *Parser) recover() {
    for p.cur.Type != tokenEOF {
        if p.cur.Type == tokenSemicolon {
            p.advance()
            return
        }
        p.advance()
    }
}
```

### 4. Error Position — `oracle/parser/parser.go`

Ensure every `ParseError` has `Position >= 0`:
```go
func (p *Parser) addError(pos int, msg string, args ...interface{}) {
    if pos < 0 {
        pos = 0 // Never report negative position
    }
    p.errors = append(p.errors, &ParseError{
        Position: pos,
        Message:  fmt.Sprintf(msg, args...),
    })
}
```

## Verification

After all implementation:

```bash
# Stage 4 eval tests must pass
cd /Users/rebeliceyang/Github/omni && go test -v -count=1 ./oracle/parser/ -run "TestEvalStage4"

# All existing tests must still pass (Stages 1-3)
cd /Users/rebeliceyang/Github/omni && go test -count=1 ./oracle/...

# Build must succeed
cd /Users/rebeliceyang/Github/omni && go build ./oracle/...
```

## Commit

After all tests pass:

```bash
git add oracle/parser/
git commit -m "feat(oracle): improve error handling — no panics, valid positions

Add nil checks, soft-fail on unexpected tokens, improve error recovery.
All mutation-based error tests pass without parser panics."
```

## Important Notes

- This stage is about **robustness**, not new features. The parser should gracefully handle ANY input without panicking.
- The mutation tests generate potentially thousands of inputs. A single panic will cause test failure — be thorough in finding all panic paths.
- Use `go test -race` to check for data races if the parser uses any shared state.
- Error position accuracy is important — the position should point to the token that caused the error, not to an arbitrary location.
- Do not suppress valid errors — the goal is to return errors gracefully, not to accept invalid SQL.
