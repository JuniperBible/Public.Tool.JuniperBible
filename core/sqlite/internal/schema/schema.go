// Package schema provides schema management for SQLite databases.
// It tracks tables, indexes, and their metadata including type affinities.
package schema

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/FocuswithJustin/JuniperBible/core/sqlite/internal/parser"
)

// Schema represents a database schema containing tables and indexes.
// It is safe for concurrent access.
type Schema struct {
	Tables  map[string]*Table
	Indexes map[string]*Index
	mu      sync.RWMutex
}

// NewSchema creates a new empty schema.
func NewSchema() *Schema {
	return &Schema{
		Tables:  make(map[string]*Table),
		Indexes: make(map[string]*Index),
	}
}

// Table represents a database table definition.
type Table struct {
	Name         string           // Table name
	RootPage     uint32           // B-tree root page number
	SQL          string           // CREATE TABLE statement
	Columns      []*Column        // Column definitions
	PrimaryKey   []string         // Primary key column names
	WithoutRowID bool             // True for WITHOUT ROWID tables
	Strict       bool             // True for STRICT tables
	Temp         bool             // True for temporary tables
	Constraints  []TableConstraint // Table-level constraints
}

// Column represents a table column definition.
type Column struct {
	Name     string      // Column name
	Type     string      // Declared type (e.g., "INTEGER", "TEXT", "VARCHAR(100)")
	Affinity Affinity    // Type affinity
	NotNull  bool        // NOT NULL constraint
	Default  interface{} // Default value (nil if none)

	// Constraints
	PrimaryKey    bool   // Part of PRIMARY KEY
	Unique        bool   // UNIQUE constraint
	Autoincrement bool   // AUTOINCREMENT (only for INTEGER PRIMARY KEY)
	Collation     string // COLLATE clause
	Check         string // CHECK constraint expression

	// Generated columns
	Generated       bool   // GENERATED ALWAYS AS
	GeneratedExpr   string // Generation expression
	GeneratedStored bool   // STORED vs VIRTUAL
}

// TableConstraint represents a table-level constraint.
type TableConstraint struct {
	Type       ConstraintType
	Name       string
	Columns    []string
	Expression string // For CHECK constraints
}

// ConstraintType represents the type of constraint.
type ConstraintType int

const (
	ConstraintPrimaryKey ConstraintType = iota
	ConstraintUnique
	ConstraintCheck
	ConstraintForeignKey
)

// Index represents a database index definition.
type Index struct {
	Name     string   // Index name
	Table    string   // Table name this index belongs to
	RootPage uint32   // B-tree root page number
	SQL      string   // CREATE INDEX statement
	Columns  []string // Indexed column names
	Unique   bool     // True for UNIQUE indexes
	Partial  bool     // True for partial indexes (WHERE clause)
	Where    string   // WHERE clause for partial indexes
}

// GetTable retrieves a table by name.
// Returns the table and true if found, nil and false otherwise.
func (s *Schema) GetTable(name string) (*Table, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// SQLite table names are case-insensitive
	lowerName := strings.ToLower(name)
	for tableName, table := range s.Tables {
		if strings.ToLower(tableName) == lowerName {
			return table, true
		}
	}
	return nil, false
}

// GetIndex retrieves an index by name.
// Returns the index and true if found, nil and false otherwise.
func (s *Schema) GetIndex(name string) (*Index, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lowerName := strings.ToLower(name)
	for indexName, index := range s.Indexes {
		if strings.ToLower(indexName) == lowerName {
			return index, true
		}
	}
	return nil, false
}

// ListTables returns a sorted list of all table names.
func (s *Schema) ListTables() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.Tables))
	for name := range s.Tables {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ListIndexes returns a sorted list of all index names.
func (s *Schema) ListIndexes() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.Indexes))
	for name := range s.Indexes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// GetTableIndexes returns all indexes for a given table.
func (s *Schema) GetTableIndexes(tableName string) []*Index {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var indexes []*Index
	lowerTableName := strings.ToLower(tableName)
	for _, idx := range s.Indexes {
		if strings.ToLower(idx.Table) == lowerTableName {
			indexes = append(indexes, idx)
		}
	}

	// Sort by name for consistency
	sort.Slice(indexes, func(i, j int) bool {
		return indexes[i].Name < indexes[j].Name
	})

	return indexes
}

// DropTable removes a table and all its indexes from the schema.
func (s *Schema) DropTable(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	lowerName := strings.ToLower(name)

	// Find the actual table name (case-insensitive)
	var actualName string
	for tableName := range s.Tables {
		if strings.ToLower(tableName) == lowerName {
			actualName = tableName
			break
		}
	}

	if actualName == "" {
		return fmt.Errorf("table not found: %s", name)
	}

	// Remove all indexes for this table
	for indexName, idx := range s.Indexes {
		if strings.ToLower(idx.Table) == lowerName {
			delete(s.Indexes, indexName)
		}
	}

	// Remove the table
	delete(s.Tables, actualName)

	return nil
}

// DropIndex removes an index from the schema.
func (s *Schema) DropIndex(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	lowerName := strings.ToLower(name)

	// Find the actual index name (case-insensitive)
	for indexName := range s.Indexes {
		if strings.ToLower(indexName) == lowerName {
			delete(s.Indexes, indexName)
			return nil
		}
	}

	return fmt.Errorf("index not found: %s", name)
}

// GetColumn retrieves a column from a table by name.
func (t *Table) GetColumn(name string) (*Column, bool) {
	lowerName := strings.ToLower(name)
	for _, col := range t.Columns {
		if strings.ToLower(col.Name) == lowerName {
			return col, true
		}
	}
	return nil, false
}

// GetColumnIndex returns the index of a column by name, or -1 if not found.
func (t *Table) GetColumnIndex(name string) int {
	lowerName := strings.ToLower(name)
	for i, col := range t.Columns {
		if strings.ToLower(col.Name) == lowerName {
			return i
		}
	}
	return -1
}

// HasRowID returns true if the table has an implicit rowid column.
// Tables have a rowid unless they are declared WITHOUT ROWID.
func (t *Table) HasRowID() bool {
	return !t.WithoutRowID
}

// CreateTable creates a table from a CREATE TABLE statement.
func (s *Schema) CreateTable(stmt *parser.CreateTableStmt) (*Table, error) {
	if stmt == nil {
		return nil, fmt.Errorf("nil statement")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if table already exists
	lowerName := strings.ToLower(stmt.Name)
	for tableName := range s.Tables {
		if strings.ToLower(tableName) == lowerName {
			if stmt.IfNotExists {
				// Return existing table
				return s.Tables[tableName], nil
			}
			return nil, fmt.Errorf("table already exists: %s", stmt.Name)
		}
	}

	// Convert parser columns to schema columns
	columns := make([]*Column, len(stmt.Columns))
	var primaryKeyColumns []string

	for i, colDef := range stmt.Columns {
		col := &Column{
			Name:     colDef.Name,
			Type:     colDef.Type,
			Affinity: DetermineAffinity(colDef.Type),
		}

		// Process column constraints
		for _, constraint := range colDef.Constraints {
			switch constraint.Type {
			case parser.ConstraintPrimaryKey:
				col.PrimaryKey = true
				primaryKeyColumns = append(primaryKeyColumns, col.Name)
				if constraint.PrimaryKey != nil && constraint.PrimaryKey.Autoincrement {
					col.Autoincrement = true
				}
			case parser.ConstraintNotNull:
				col.NotNull = true
			case parser.ConstraintUnique:
				col.Unique = true
			case parser.ConstraintCollate:
				col.Collation = constraint.Collate
			case parser.ConstraintDefault:
				// Store default as expression string for now
				if constraint.Default != nil {
					col.Default = constraint.Default.String()
				}
			case parser.ConstraintCheck:
				if constraint.Check != nil {
					col.Check = constraint.Check.String()
				}
			case parser.ConstraintGenerated:
				if constraint.Generated != nil {
					col.Generated = true
					if constraint.Generated.Expr != nil {
						col.GeneratedExpr = constraint.Generated.Expr.String()
					}
					col.GeneratedStored = constraint.Generated.Stored
				}
			}
		}

		columns[i] = col
	}

	// Process table-level constraints
	var tableConstraints []TableConstraint
	for _, constraint := range stmt.Constraints {
		tc := TableConstraint{
			Name: constraint.Name,
		}

		switch constraint.Type {
		case parser.ConstraintPrimaryKey:
			tc.Type = ConstraintPrimaryKey
			if constraint.PrimaryKey != nil {
				for _, col := range constraint.PrimaryKey.Columns {
					tc.Columns = append(tc.Columns, col.Column)
					primaryKeyColumns = append(primaryKeyColumns, col.Column)
				}
			}
		case parser.ConstraintUnique:
			tc.Type = ConstraintUnique
			if constraint.Unique != nil {
				for _, col := range constraint.Unique.Columns {
					tc.Columns = append(tc.Columns, col.Column)
				}
			}
		case parser.ConstraintCheck:
			tc.Type = ConstraintCheck
			if constraint.Check != nil {
				tc.Expression = constraint.Check.String()
			}
		case parser.ConstraintForeignKey:
			tc.Type = ConstraintForeignKey
			if constraint.ForeignKey != nil {
				tc.Columns = constraint.ForeignKey.Columns
			}
		}

		tableConstraints = append(tableConstraints, tc)
	}

	// Create the table
	table := &Table{
		Name:         stmt.Name,
		RootPage:     0, // Will be assigned when written to disk
		SQL:          stmt.String(),
		Columns:      columns,
		PrimaryKey:   uniqueStrings(primaryKeyColumns),
		WithoutRowID: stmt.WithoutRowID,
		Strict:       stmt.Strict,
		Temp:         stmt.Temp,
		Constraints:  tableConstraints,
	}

	s.Tables[stmt.Name] = table
	return table, nil
}

// CreateIndex creates an index from a CREATE INDEX statement.
func (s *Schema) CreateIndex(stmt *parser.CreateIndexStmt) (*Index, error) {
	if stmt == nil {
		return nil, fmt.Errorf("nil statement")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if index already exists
	lowerName := strings.ToLower(stmt.Name)
	for indexName := range s.Indexes {
		if strings.ToLower(indexName) == lowerName {
			if stmt.IfNotExists {
				return s.Indexes[indexName], nil
			}
			return nil, fmt.Errorf("index already exists: %s", stmt.Name)
		}
	}

	// Verify table exists
	lowerTableName := strings.ToLower(stmt.Table)
	tableExists := false
	for tableName := range s.Tables {
		if strings.ToLower(tableName) == lowerTableName {
			tableExists = true
			break
		}
	}
	if !tableExists {
		return nil, fmt.Errorf("table not found: %s", stmt.Table)
	}

	// Extract column names
	columns := make([]string, len(stmt.Columns))
	for i, col := range stmt.Columns {
		columns[i] = col.Column
	}

	// Create the index
	index := &Index{
		Name:     stmt.Name,
		Table:    stmt.Table,
		RootPage: 0, // Will be assigned when written to disk
		SQL:      stmt.String(),
		Columns:  columns,
		Unique:   stmt.Unique,
		Partial:  stmt.Where != nil,
	}

	if stmt.Where != nil {
		index.Where = stmt.Where.String()
	}

	s.Indexes[stmt.Name] = index
	return index, nil
}

// uniqueStrings removes duplicates from a slice while preserving order.
func uniqueStrings(strs []string) []string {
	seen := make(map[string]bool)
	result := make([]string, 0, len(strs))

	for _, s := range strs {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}

	return result
}
