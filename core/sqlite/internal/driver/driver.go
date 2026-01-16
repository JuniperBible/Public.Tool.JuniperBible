package driver

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"sync"

	"github.com/FocuswithJustin/JuniperBible/core/sqlite/internal/btree"
	"github.com/FocuswithJustin/JuniperBible/core/sqlite/internal/pager"
)

// Driver implements database/sql/driver.Driver for SQLite.
type Driver struct {
	mu    sync.Mutex
	conns map[string]*Conn
}

// sqliteDriver is the singleton driver instance
var sqliteDriver = &Driver{
	conns: make(map[string]*Conn),
}

// init registers the driver with database/sql
func init() {
	sql.Register("sqlite", sqliteDriver)
}

// Open opens a connection to the database.
// The name is the database file path, optionally with query parameters.
func (d *Driver) Open(name string) (driver.Conn, error) {
	return d.OpenConnector(name)
}

// OpenConnector returns a connector for the database.
func (d *Driver) OpenConnector(name string) (driver.Conn, error) {
	// Parse connection string
	// For simplicity, treat name as just the filename
	// In a full implementation, this would parse DSN parameters
	filename := name
	if filename == "" || filename == ":memory:" {
		return nil, fmt.Errorf("in-memory databases not yet supported")
	}

	// Open the pager
	pgr, err := pager.Open(filename, false)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Create btree
	bt := btree.NewBtree(uint32(pgr.PageSize()))

	// Create connection
	conn := &Conn{
		driver:   d,
		filename: filename,
		pager:    pgr,
		btree:    bt,
		stmts:    make(map[*Stmt]struct{}),
	}

	// Initialize the database (load schema and register functions)
	if err := conn.openDatabase(); err != nil {
		pgr.Close()
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	d.mu.Lock()
	d.conns[filename] = conn
	d.mu.Unlock()

	return conn, nil
}

// GetDriver returns the singleton driver instance.
func GetDriver() *Driver {
	return sqliteDriver
}
