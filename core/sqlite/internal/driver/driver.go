package driver

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"sync"

	"github.com/FocuswithJustin/JuniperBible/core/sqlite/internal/btree"
	"github.com/FocuswithJustin/JuniperBible/core/sqlite/internal/pager"
	"github.com/FocuswithJustin/JuniperBible/core/sqlite/internal/schema"
)

// dbState represents shared state for a database file
type dbState struct {
	pager  *pager.Pager
	btree  *btree.Btree
	schema *schema.Schema
	refCnt int
}

// Driver implements database/sql/driver.Driver for SQLite.
type Driver struct {
	mu    sync.Mutex
	conns map[string]*Conn
	dbs   map[string]*dbState // Shared database state per file
}

// sqliteDriver is the singleton driver instance
var sqliteDriver = &Driver{
	conns: make(map[string]*Conn),
	dbs:   make(map[string]*dbState),
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

	d.mu.Lock()
	defer d.mu.Unlock()

	// Initialize maps if needed (for Driver instances not created via singleton)
	if d.conns == nil {
		d.conns = make(map[string]*Conn)
	}
	if d.dbs == nil {
		d.dbs = make(map[string]*dbState)
	}

	// Check for existing shared database state
	state, exists := d.dbs[filename]
	if !exists {
		// Open the pager for the first time
		pgr, err := pager.Open(filename, false)
		if err != nil {
			return nil, fmt.Errorf("failed to open database: %w", err)
		}

		// Create btree and connect it to the pager
		bt := btree.NewBtree(uint32(pgr.PageSize()))
		bt.Provider = newPagerProvider(pgr)

		// Create shared schema
		sch := schema.NewSchema()

		state = &dbState{
			pager:  pgr,
			btree:  bt,
			schema: sch,
			refCnt: 0,
		}
		d.dbs[filename] = state
	}

	// Increment reference count
	state.refCnt++

	// Create connection using shared state
	conn := &Conn{
		driver:   d,
		filename: filename,
		pager:    state.pager,
		btree:    state.btree,
		schema:   state.schema, // Use shared schema
		stmts:    make(map[*Stmt]struct{}),
	}

	// Initialize the database (load schema and register functions)
	if err := conn.openDatabase(exists); err != nil {
		state.refCnt--
		if state.refCnt == 0 {
			state.pager.Close()
			delete(d.dbs, filename)
		}
		return nil, fmt.Errorf("failed to initialize database: %w", err)
	}

	d.conns[filename] = conn

	return conn, nil
}

// GetDriver returns the singleton driver instance.
func GetDriver() *Driver {
	return sqliteDriver
}

// pagerProvider implements btree.PageProvider to bridge btree and pager
type pagerProvider struct {
	pager    *pager.Pager
	nextPage uint32
}

// newPagerProvider creates a new pager provider
func newPagerProvider(pgr *pager.Pager) *pagerProvider {
	return &pagerProvider{
		pager:    pgr,
		nextPage: uint32(pgr.PageCount()) + 1,
	}
}

// GetPageData retrieves page data from the pager
func (pp *pagerProvider) GetPageData(pgno uint32) ([]byte, error) {
	page, err := pp.pager.Get(pager.Pgno(pgno))
	if err != nil {
		return nil, err
	}
	return page.GetData(), nil
}

// AllocatePageData allocates a new page
func (pp *pagerProvider) AllocatePageData() (uint32, []byte, error) {
	pgno := pp.nextPage
	pp.nextPage++
	data := make([]byte, pp.pager.PageSize())
	return pgno, data, nil
}

// MarkDirty marks a page as dirty
func (pp *pagerProvider) MarkDirty(pgno uint32) error {
	page, err := pp.pager.Get(pager.Pgno(pgno))
	if err != nil {
		return err
	}
	page.MakeDirty()
	return nil
}
