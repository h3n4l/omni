package catalog

import nodes "github.com/bytebase/omni/mysql/ast"

func (c *Catalog) createView(stmt *nodes.CreateViewStmt) error   { return nil }
func (c *Catalog) dropView(stmt *nodes.DropViewStmt) error       { return nil }
func (c *Catalog) renameTable(stmt *nodes.RenameTableStmt) error { return nil }
