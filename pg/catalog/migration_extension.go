package catalog

import (
	"fmt"
	"sort"
)

// generateExtensionDDL produces CREATE EXTENSION, DROP EXTENSION, and
// ALTER EXTENSION SET SCHEMA operations from the diff.
func generateExtensionDDL(from, to *Catalog, diff *SchemaDiff) []MigrationOp {
	var ops []MigrationOp

	for _, entry := range diff.Extensions {
		switch entry.Action {
		case DiffAdd:
			ops = append(ops, MigrationOp{
				Type:          OpCreateExtension,
				ObjectName:    entry.Name,
				SQL:           fmt.Sprintf("CREATE EXTENSION %s", quoteIdentAlways(entry.Name)),
				Transactional: true,
			})

		case DiffDrop:
			ops = append(ops, MigrationOp{
				Type:          OpDropExtension,
				ObjectName:    entry.Name,
				SQL:           fmt.Sprintf("DROP EXTENSION %s", quoteIdentAlways(entry.Name)),
				Transactional: true,
			})

		case DiffModify:
			// Schema change: resolve SchemaOID from the target catalog.
			if entry.From != nil && entry.To != nil && entry.From.SchemaOID != entry.To.SchemaOID {
				schemaName := ""
				if s := to.schemas[entry.To.SchemaOID]; s != nil {
					schemaName = s.Name
				}
				if schemaName != "" {
					ops = append(ops, MigrationOp{
						Type:          OpAlterExtension,
						ObjectName:    entry.Name,
						SQL:           fmt.Sprintf("ALTER EXTENSION %s SET SCHEMA %s", quoteIdentAlways(entry.Name), quoteIdentAlways(schemaName)),
						Transactional: true,
					})
				}
			}
		}
	}

	// Deterministic ordering: drops first, then creates, then alters; within each group by name.
	sort.Slice(ops, func(i, j int) bool {
		oi, oj := opTypeOrder(ops[i].Type), opTypeOrder(ops[j].Type)
		if oi != oj {
			return oi < oj
		}
		return ops[i].ObjectName < ops[j].ObjectName
	})

	return ops
}

// opTypeOrder returns a sort key for extension op types: drops < creates < alters.
func opTypeOrder(t MigrationOpType) int {
	switch t {
	case OpDropExtension:
		return 0
	case OpCreateExtension:
		return 1
	case OpAlterExtension:
		return 2
	default:
		return 3
	}
}
