package catalog

import "strings"

type Database struct {
	Name       string
	Charset    string
	Collation  string
	Tables     map[string]*Table // lowered name -> Table
	Views      map[string]*View
	Functions  map[string]*Routine // lowered name -> stored function
	Procedures map[string]*Routine // lowered name -> stored procedure
}

func newDatabase(name, charset, collation string) *Database {
	return &Database{
		Name:       name,
		Charset:    charset,
		Collation:  collation,
		Tables:     make(map[string]*Table),
		Views:      make(map[string]*View),
		Functions:  make(map[string]*Routine),
		Procedures: make(map[string]*Routine),
	}
}

func (db *Database) GetTable(name string) *Table {
	return db.Tables[toLower(name)]
}

func toLower(s string) string {
	return strings.ToLower(s)
}
