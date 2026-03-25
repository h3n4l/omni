package catalog

import (
	"fmt"

	nodes "github.com/bytebase/omni/pg/ast"
	pgparser "github.com/bytebase/omni/pg/parser"
)

// LoadSDL parses a declarative schema definition (SDL) and loads it into a new
// Catalog. SDL only accepts CREATE/COMMENT/GRANT statements — DML and
// destructive DDL (DROP, ALTER TABLE ADD/DROP COLUMN, TRUNCATE, etc.) are
// rejected with a clear error.
//
// Currently statements are executed sequentially via ProcessUtility.
// Dependency resolution (reordering) will be added in later phases.
func LoadSDL(sql string) (*Catalog, error) {
	c := New()

	list, err := pgparser.Parse(sql)
	if err != nil {
		return nil, err
	}
	if list == nil || len(list.Items) == 0 {
		return c, nil
	}

	// Unwrap RawStmt wrappers and collect bare statements.
	stmts := make([]nodes.Node, 0, len(list.Items))
	for _, item := range list.Items {
		if raw, ok := item.(*nodes.RawStmt); ok {
			stmts = append(stmts, raw.Stmt)
		} else {
			stmts = append(stmts, item)
		}
	}

	// Validate all statements before execution.
	if err := validateSDL(stmts); err != nil {
		return nil, err
	}

	// Execute sequentially (dependency resolution comes in 1.2/1.3).
	for _, stmt := range stmts {
		if err := c.ProcessUtility(stmt); err != nil {
			return c, err
		}
	}

	return c, nil
}

// validateSDL checks that every statement in the list is allowed in SDL.
// Returns the first disallowed statement as an error.
func validateSDL(stmts []nodes.Node) error {
	for _, stmt := range stmts {
		if err := validateSDLStmt(stmt); err != nil {
			return err
		}
	}
	return nil
}

// validateSDLStmt validates a single statement for SDL compliance.
func validateSDLStmt(stmt nodes.Node) error {
	switch s := stmt.(type) {
	// ---- Allowed DDL statements ----
	case *nodes.CreateStmt:
		return nil
	case *nodes.ViewStmt:
		return nil
	case *nodes.CreateFunctionStmt:
		return nil
	case *nodes.IndexStmt:
		return nil
	case *nodes.CreateSeqStmt:
		return nil
	case *nodes.CreateSchemaStmt:
		return nil
	case *nodes.CreateEnumStmt:
		return nil
	case *nodes.CreateDomainStmt:
		return nil
	case *nodes.CompositeTypeStmt:
		return nil
	case *nodes.CreateRangeStmt:
		return nil
	case *nodes.CreateExtensionStmt:
		return nil
	case *nodes.CreateTrigStmt:
		return nil
	case *nodes.CreatePolicyStmt:
		return nil
	case *nodes.CreateTableAsStmt:
		return nil
	case *nodes.CreateCastStmt:
		return nil
	case *nodes.CreateForeignTableStmt:
		return nil
	case *nodes.CommentStmt:
		return nil
	case *nodes.GrantStmt:
		return nil
	case *nodes.AlterSeqStmt:
		return nil
	case *nodes.AlterEnumStmt:
		return nil
	case *nodes.VariableSetStmt:
		return nil
	case *nodes.DefineStmt:
		return nil

	// ---- ALTER TABLE: only RLS commands allowed ----
	case *nodes.AlterTableStmt:
		return validateAlterTableSDL(s)

	// ---- Explicitly rejected DML ----
	case *nodes.InsertStmt:
		return fmt.Errorf("SDL does not allow INSERT statements")
	case *nodes.UpdateStmt:
		return fmt.Errorf("SDL does not allow UPDATE statements")
	case *nodes.DeleteStmt:
		return fmt.Errorf("SDL does not allow DELETE statements")
	case *nodes.SelectStmt:
		return fmt.Errorf("SDL does not allow SELECT statements")

	// ---- Explicitly rejected DDL ----
	case *nodes.DropStmt:
		return fmt.Errorf("SDL does not allow DROP statements")
	case *nodes.TruncateStmt:
		return fmt.Errorf("SDL does not allow TRUNCATE statements")
	case *nodes.DoStmt:
		return fmt.Errorf("SDL does not allow DO statements")

	// ---- Anything else ----
	default:
		return fmt.Errorf("SDL does not allow %T statements", stmt)
	}
}

// validateAlterTableSDL checks that an ALTER TABLE only contains RLS-related
// subcommands (ENABLE/DISABLE/FORCE/NO FORCE ROW LEVEL SECURITY).
func validateAlterTableSDL(s *nodes.AlterTableStmt) error {
	if s.Cmds == nil {
		return nil
	}
	for _, item := range s.Cmds.Items {
		cmd, ok := item.(*nodes.AlterTableCmd)
		if !ok {
			return fmt.Errorf("SDL does not allow ALTER TABLE with unknown command type")
		}
		subtype := nodes.AlterTableType(cmd.Subtype)
		switch subtype {
		case nodes.AT_EnableRowSecurity,
			nodes.AT_DisableRowSecurity,
			nodes.AT_ForceRowSecurity,
			nodes.AT_NoForceRowSecurity:
			// allowed
		default:
			return fmt.Errorf("SDL does not allow ALTER TABLE ADD/DROP COLUMN")
		}
	}
	return nil
}
