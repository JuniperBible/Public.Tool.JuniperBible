package schema

import (
	"fmt"

	"github.com/FocuswithJustin/JuniperBible/core/sqlite/internal/btree"
	"github.com/FocuswithJustin/JuniperBible/core/sqlite/internal/parser"
)

// sqlite_master table schema:
//
// CREATE TABLE sqlite_master (
//   type TEXT,      -- "table", "index", "trigger", "view"
//   name TEXT,      -- object name
//   tbl_name TEXT,  -- table name (for indexes/triggers)
//   rootpage INT,   -- root B-tree page
//   sql TEXT        -- CREATE statement
// );
//
// The sqlite_master table is always stored on page 1 of the database.

// MasterRow represents a row in the sqlite_master table.
type MasterRow struct {
	Type     string // "table", "index", "trigger", "view"
	Name     string // Object name
	TblName  string // Associated table name
	RootPage uint32 // Root page number
	SQL      string // CREATE statement
}

// LoadFromMaster loads the schema from the sqlite_master table.
// This reads all table and index definitions from page 1 of the database.
func (s *Schema) LoadFromMaster(bt *btree.Btree) error {
	if bt == nil {
		return fmt.Errorf("nil btree")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// sqlite_master is on page 1
	const masterPageNum = 1

	// Parse the master page to get all rows
	rows, err := s.parseMasterPage(bt, masterPageNum)
	if err != nil {
		return fmt.Errorf("failed to parse sqlite_master: %w", err)
	}

	// Process each row
	for _, row := range rows {
		switch row.Type {
		case "table":
			// Skip internal tables
			if row.Name == "sqlite_master" || row.Name == "sqlite_sequence" {
				continue
			}

			// Parse the CREATE TABLE statement
			table, err := s.parseTableSQL(row)
			if err != nil {
				return fmt.Errorf("failed to parse table %s: %w", row.Name, err)
			}
			s.Tables[table.Name] = table

		case "index":
			// Skip auto-indexes (sqlite_autoindex_*)
			if len(row.Name) > 16 && row.Name[:16] == "sqlite_autoindex" {
				continue
			}

			// Parse the CREATE INDEX statement
			index, err := s.parseIndexSQL(row)
			if err != nil {
				return fmt.Errorf("failed to parse index %s: %w", row.Name, err)
			}
			s.Indexes[index.Name] = index

		case "view":
			// Views are not fully implemented yet, skip for now
			continue

		case "trigger":
			// Triggers are not implemented yet, skip for now
			continue
		}
	}

	return nil
}

// SaveToMaster saves the current schema to the sqlite_master table.
// This writes all table and index definitions to page 1 of the database.
func (s *Schema) SaveToMaster(bt *btree.Btree) error {
	if bt == nil {
		return fmt.Errorf("nil btree")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// Build master rows from current schema
	var rows []MasterRow

	// Add tables
	for _, table := range s.Tables {
		// Skip sqlite_master itself
		if table.Name == "sqlite_master" {
			continue
		}

		rows = append(rows, MasterRow{
			Type:     "table",
			Name:     table.Name,
			TblName:  table.Name,
			RootPage: table.RootPage,
			SQL:      table.SQL,
		})
	}

	// Add indexes
	for _, index := range s.Indexes {
		rows = append(rows, MasterRow{
			Type:     "index",
			Name:     index.Name,
			TblName:  index.Table,
			RootPage: index.RootPage,
			SQL:      index.SQL,
		})
	}

	// Write rows to sqlite_master (page 1)
	if err := s.writeMasterPage(bt, 1, rows); err != nil {
		return fmt.Errorf("failed to write sqlite_master: %w", err)
	}

	return nil
}

// parseMasterPage reads and parses the sqlite_master page.
// This is a simplified implementation - a full version would use the btree
// cursor to iterate through all cells in the page.
func (s *Schema) parseMasterPage(bt *btree.Btree, pageNum uint32) ([]MasterRow, error) {
	// In a real implementation, this would:
	// 1. Create a cursor on page 1
	// 2. Iterate through all cells
	// 3. Parse each record as a MasterRow
	// 4. Return the list of rows

	// For now, return empty list - this is a placeholder
	// The actual implementation would require a full record parser
	return []MasterRow{}, nil
}

// writeMasterPage writes rows to the sqlite_master page.
func (s *Schema) writeMasterPage(bt *btree.Btree, pageNum uint32, rows []MasterRow) error {
	// In a real implementation, this would:
	// 1. Clear the existing page 1 contents
	// 2. For each row, encode it as a SQLite record
	// 3. Insert the record into the B-tree page
	// 4. Update the page header

	// For now, this is a placeholder
	return nil
}

// parseTableSQL parses a CREATE TABLE statement from a master row.
func (s *Schema) parseTableSQL(row MasterRow) (*Table, error) {
	if row.SQL == "" {
		// Some system tables don't have SQL
		return &Table{
			Name:     row.Name,
			RootPage: row.RootPage,
			SQL:      row.SQL,
			Columns:  []*Column{},
		}, nil
	}

	// Parse the SQL statement
	p := parser.NewParser(row.SQL)
	stmts, err := p.Parse()
	if err != nil {
		return nil, fmt.Errorf("failed to parse SQL: %w", err)
	}

	// Should have exactly one statement
	if len(stmts) != 1 {
		return nil, fmt.Errorf("expected 1 statement, got %d", len(stmts))
	}

	// Ensure it's a CREATE TABLE statement
	createTable, ok := stmts[0].(*parser.CreateTableStmt)
	if !ok {
		return nil, fmt.Errorf("expected CREATE TABLE, got %T", stmts[0])
	}

	// Convert parser columns to schema columns directly
	// We can't call CreateTable here because it would require mutex manipulation
	columns := make([]*Column, len(createTable.Columns))
	var primaryKeyColumns []string

	for i, colDef := range createTable.Columns {
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

	// Create the table
	table := &Table{
		Name:         createTable.Name,
		RootPage:     row.RootPage, // Use the one from sqlite_master
		SQL:          row.SQL,
		Columns:      columns,
		PrimaryKey:   uniqueStrings(primaryKeyColumns),
		WithoutRowID: createTable.WithoutRowID,
		Strict:       createTable.Strict,
		Temp:         createTable.Temp,
	}

	return table, nil
}

// parseIndexSQL parses a CREATE INDEX statement from a master row.
func (s *Schema) parseIndexSQL(row MasterRow) (*Index, error) {
	if row.SQL == "" {
		// Some auto-indexes don't have SQL
		return &Index{
			Name:     row.Name,
			Table:    row.TblName,
			RootPage: row.RootPage,
			SQL:      row.SQL,
			Columns:  []string{},
		}, nil
	}

	// Parse the SQL statement
	p := parser.NewParser(row.SQL)
	stmts, err := p.Parse()
	if err != nil {
		return nil, fmt.Errorf("failed to parse SQL: %w", err)
	}

	// Should have exactly one statement
	if len(stmts) != 1 {
		return nil, fmt.Errorf("expected 1 statement, got %d", len(stmts))
	}

	// Ensure it's a CREATE INDEX statement
	createIndex, ok := stmts[0].(*parser.CreateIndexStmt)
	if !ok {
		return nil, fmt.Errorf("expected CREATE INDEX, got %T", stmts[0])
	}

	// Extract column names
	columns := make([]string, len(createIndex.Columns))
	for i, col := range createIndex.Columns {
		columns[i] = col.Column
	}

	// Create the index
	index := &Index{
		Name:     createIndex.Name,
		Table:    createIndex.Table,
		RootPage: row.RootPage, // Use the one from sqlite_master
		SQL:      row.SQL,
		Columns:  columns,
		Unique:   createIndex.Unique,
		Partial:  createIndex.Where != nil,
	}

	if createIndex.Where != nil {
		index.Where = createIndex.Where.String()
	}

	return index, nil
}

// InitializeMaster creates the sqlite_master table in a new database.
// This should be called when creating a new database file.
func (s *Schema) InitializeMaster() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Create the sqlite_master table
	masterTable := &Table{
		Name:     "sqlite_master",
		RootPage: 1,
		SQL:      "CREATE TABLE sqlite_master(type text,name text,tbl_name text,rootpage integer,sql text)",
		Columns: []*Column{
			{
				Name:     "type",
				Type:     "text",
				Affinity: AffinityText,
			},
			{
				Name:     "name",
				Type:     "text",
				Affinity: AffinityText,
			},
			{
				Name:     "tbl_name",
				Type:     "text",
				Affinity: AffinityText,
			},
			{
				Name:     "rootpage",
				Type:     "integer",
				Affinity: AffinityInteger,
			},
			{
				Name:     "sql",
				Type:     "text",
				Affinity: AffinityText,
			},
		},
		PrimaryKey:   []string{},
		WithoutRowID: false,
		Strict:       false,
		Temp:         false,
	}

	s.Tables["sqlite_master"] = masterTable
	return nil
}
