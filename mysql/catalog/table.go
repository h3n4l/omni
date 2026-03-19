package catalog

type Table struct {
	Name          string
	Database      *Database
	Columns       []*Column
	colByName     map[string]int // lowered name -> index
	Indexes       []*Index
	Constraints   []*Constraint
	Engine        string
	Charset       string
	Collation     string
	Comment       string
	AutoIncrement int64
	Temporary     bool
	RowFormat     string
	KeyBlockSize  int
	Partitioning  *PartitionInfo
}

// PartitionInfo holds partition metadata for a table.
type PartitionInfo struct {
	Type       string // RANGE, LIST, HASH, KEY
	Linear     bool   // LINEAR HASH or LINEAR KEY
	Expr       string // partition expression (for RANGE/LIST/HASH)
	Columns    []string // partition columns (for RANGE COLUMNS/LIST COLUMNS/KEY)
	Algorithm  int    // ALGORITHM={1|2} for KEY partitioning
	NumParts   int    // PARTITIONS num
	Partitions []*PartitionDefInfo
	SubType    string // subpartition type (HASH or KEY, "" if none)
	SubLinear  bool   // LINEAR for subpartition
	SubExpr    string // subpartition expression
	SubColumns []string // subpartition columns
	SubAlgo    int    // subpartition ALGORITHM
	NumSubParts int   // SUBPARTITIONS num
}

// PartitionDefInfo holds a single partition definition.
type PartitionDefInfo struct {
	Name          string
	ValueExpr     string // "LESS THAN (...)" or "IN (...)" or ""
	Engine        string // ENGINE option for this partition
	Comment       string // COMMENT option for this partition
	SubPartitions []*SubPartitionDefInfo
}

// SubPartitionDefInfo holds a single subpartition definition.
type SubPartitionDefInfo struct {
	Name    string
	Engine  string
	Comment string
}

type Column struct {
	Position       int
	Name           string
	DataType       string // normalized (int, varchar, etc.)
	ColumnType     string // full type string (varchar(100), int unsigned)
	Nullable       bool
	Default        *string
	DefaultDropped bool // true when ALTER COLUMN DROP DEFAULT was used
	AutoIncrement  bool
	Charset        string
	Collation      string
	Comment        string
	OnUpdate       string
	Generated      *GeneratedColumnInfo
	Invisible      bool
}

type GeneratedColumnInfo struct {
	Expr   string
	Stored bool
}

type View struct {
	Name        string
	Database    *Database
	Definition  string
	Algorithm   string
	Definer     string
	SqlSecurity string
	CheckOption string
	Columns     []string
}

func (t *Table) GetColumn(name string) *Column {
	idx, ok := t.colByName[toLower(name)]
	if !ok {
		return nil
	}
	return t.Columns[idx]
}
