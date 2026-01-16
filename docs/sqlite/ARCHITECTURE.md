# SQLite Implementation Architecture

This document describes the architecture of JuniperBible's pure Go SQLite database engine.

## System Overview

The SQLite implementation follows a layered architecture, closely mirroring the design of the reference SQLite implementation:

```
┌─────────────────────────────────────────────────────────┐
│                  Application Layer                      │
│              (database/sql interface)                   │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│                  SQL Interface Layer                    │
│           (Parser, Planner, Code Generator)             │
├─────────────────────────────────────────────────────────┤
│  Parser  │  AST  │  Planner  │  Optimizer  │  Codegen  │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│              Virtual Database Engine (VDBE)             │
│              (Bytecode Interpreter)                     │
├─────────────────────────────────────────────────────────┤
│  Opcodes  │  Memory  │  Functions  │  Expression Eval  │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│                    B-tree Layer                         │
│         (Table and Index Storage Engine)                │
├─────────────────────────────────────────────────────────┤
│  Cursor  │  Cell Parser  │  Varint Encoding  │  Pages  │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│                     Pager Layer                         │
│           (Page Cache, I/O, Transactions)               │
├─────────────────────────────────────────────────────────┤
│  Cache  │  Journal  │  Locks  │  Page I/O  │  Fsync    │
└─────────────────────────────────────────────────────────┘
                          ↓
┌─────────────────────────────────────────────────────────┐
│                  Operating System                       │
│                (File System, I/O)                       │
└─────────────────────────────────────────────────────────┘
```

## Component Diagram

### Package Dependencies

```
core/sqlite (public API)
    │
    ├── internal/driver (database/sql driver)
    │       │
    │       ├── internal/engine (SQL execution engine)
    │       │       │
    │       │       ├── internal/vdbe (Virtual Database Engine)
    │       │       │       │
    │       │       │       ├── internal/functions (SQL functions)
    │       │       │       ├── internal/expr (expression evaluation)
    │       │       │       └── internal/utf (text encoding/collation)
    │       │       │
    │       │       ├── internal/planner (query planning)
    │       │       │       └── internal/parser (SQL parser)
    │       │       │
    │       │       └── internal/sql (SQL compilation)
    │       │
    │       ├── internal/btree (B-tree storage)
    │       │       └── internal/format (file format)
    │       │
    │       ├── internal/pager (page cache and I/O)
    │       │
    │       └── internal/schema (schema management)
    │
    ├── driver_purego.go (pure Go driver selection)
    └── driver_cgo.go (CGO driver selection)
```

## Data Flow

### Query Execution Flow

```
1. Application calls db.Query("SELECT ...")
                ↓
2. Driver receives query string
                ↓
3. Parser converts SQL to AST
   - Lexer tokenizes input
   - Parser builds syntax tree
   - Semantic analysis validates references
                ↓
4. Planner optimizes query
   - Analyzes WHERE clauses
   - Chooses indexes
   - Determines join order
   - Estimates costs
                ↓
5. Code Generator emits VDBE bytecode
   - Translates AST to opcodes
   - Allocates registers
   - Generates jump labels
                ↓
6. VDBE executes bytecode
   - Opens B-tree cursors
   - Evaluates expressions
   - Calls functions
   - Returns result rows
                ↓
7. B-tree layer accesses data
   - Positions cursors
   - Reads cells from pages
   - Handles overflow pages
                ↓
8. Pager retrieves pages
   - Checks page cache
   - Reads from disk if needed
   - Manages dirty pages
                ↓
9. Results returned to application
   - VDBE memory cells converted to driver.Value
   - Rows iterator provides sequential access
```

### Write Transaction Flow

```
1. Application calls db.Begin()
                ↓
2. Pager begins transaction
   - Acquires locks
   - Opens rollback journal
                ↓
3. Application executes INSERT/UPDATE/DELETE
                ↓
4. Parser → Planner → Code Generator
                ↓
5. VDBE executes write operations
   - Opens B-tree cursors for writing
   - Modifies pages in memory
                ↓
6. Pager journals original pages
   - Before modifying, writes original page to journal
   - Marks pages as dirty in cache
                ↓
7. Application calls tx.Commit()
                ↓
8. Pager commits transaction
   - Writes all dirty pages to database file
   - Syncs database file to disk
   - Deletes journal file
   - Releases locks
                ↓
9. OR Application calls tx.Rollback()
                ↓
10. Pager rolls back transaction
    - Restores pages from journal
    - Discards dirty pages
    - Deletes journal file
    - Releases locks
```

## Layer Details

### 1. SQL Interface Layer

**Parser** (`internal/parser`)
- **Lexer**: Tokenizes SQL text into tokens (keywords, identifiers, operators, literals)
- **Parser**: Builds Abstract Syntax Tree (AST) from token stream
- **AST**: Represents SQL statements as structured data
- **Validation**: Performs semantic analysis and type checking

**Planner** (`internal/planner`)
- **Query Analysis**: Analyzes query structure and predicates
- **Index Selection**: Chooses optimal indexes for table access
- **Cost Estimation**: Estimates query execution cost
- **Optimization**: Applies query rewrite rules

**Code Generator** (`internal/sql`, `internal/engine`)
- **Register Allocation**: Assigns VDBE memory cells
- **Opcode Emission**: Generates bytecode instructions
- **Label Resolution**: Resolves jump targets
- **Compilation**: Converts AST to executable VDBE program

### 2. Virtual Database Engine (VDBE)

**Bytecode Interpreter** (`internal/vdbe`)
- **Opcodes**: ~100 bytecode instructions
- **Memory**: Array of typed memory cells (NULL, INTEGER, FLOAT, TEXT, BLOB)
- **Program Counter**: Tracks current instruction
- **Stack**: Manages nested operations
- **Cursors**: Array of B-tree cursors for table/index access

**Expression Evaluator** (`internal/expr`)
- **Arithmetic**: +, -, *, /, %
- **Comparison**: =, !=, <, >, <=, >=, IS, IS NOT
- **Logical**: AND, OR, NOT
- **Type Affinity**: SQLite's dynamic type system
- **Collation**: Text comparison rules

**Functions** (`internal/functions`)
- **Scalar Functions**: 75+ built-in functions (upper, lower, substr, abs, round, etc.)
- **Aggregate Functions**: COUNT, SUM, AVG, MIN, MAX, GROUP_CONCAT
- **Date/Time Functions**: date, time, datetime, julianday, strftime
- **Function Registry**: Extensible function registration

### 3. B-tree Layer

**Storage Engine** (`internal/btree`)
- **B-tree Structure**: Balanced tree for sorted data
- **Page Types**: Interior/Leaf for Tables/Indexes (4 types)
- **Cells**: Key-value pairs stored in B-tree pages
- **Varint Encoding**: Variable-length integer encoding
- **Overflow Pages**: Handles large payloads

**Cursors** (`internal/btree/cursor.go`)
- **Positioning**: MoveToFirst, MoveToLast, Seek
- **Navigation**: Next, Previous
- **State Management**: Valid, Invalid, SkipNext, RequireSeek, Fault
- **Stack-based Navigation**: Efficient tree traversal

### 4. Pager Layer

**Page Cache** (`internal/pager`)
- **Cache Management**: LRU eviction for clean pages
- **Hash Map**: O(1) page lookup by page number
- **Reference Counting**: Prevents premature eviction
- **Dirty Tracking**: Maintains list of modified pages

**Transaction Management** (`internal/pager/transaction.go`, `internal/pager/journal.go`)
- **Rollback Journal**: Atomic commit/rollback
- **State Machine**: Tracks transaction state
- **File Syncing**: Ensures durability
- **Lock Management**: Prevents concurrent writers

**File Format** (`internal/format`)
- **Database Header**: 100-byte header with metadata
- **Page Header**: 8-12 bytes per page
- **Page Size**: 512 to 65536 bytes (power of 2)
- **Text Encoding**: UTF-8, UTF-16LE, UTF-16BE

### 5. Schema Management

**Schema Layer** (`internal/schema`)
- **sqlite_master Table**: Stores table/index definitions
- **Schema Parsing**: Extracts schema information
- **Type Affinity**: Column type rules
- **Metadata**: Tracks tables, indexes, views, triggers

## Execution Examples

### Example 1: Simple SELECT Query

SQL: `SELECT name FROM users WHERE id = 42`

**Bytecode:**
```
0   Init         0    10   0              0   Initialize program
1   OpenRead     0    2    0              0   Open table 'users' (root=2)
2   Integer      42   1    0              0   r[1] = 42
3   SeekGE       0    9    1              0   Seek cursor 0 to id >= 42
4   IdxGE        0    9    1              0   If id > 42, goto 9
5   Column       0    1    2              0   r[2] = users.name
6   ResultRow    2    1    0              0   Output r[2]
7   Next         0    4    0              0   Move to next row, goto 4
8   Close        0    0    0              0   Close cursor 0
9   Halt         0    0    0              0   End program
```

### Example 2: INSERT Statement

SQL: `INSERT INTO users (id, name) VALUES (1, 'Alice')`

**Bytecode:**
```
0   Init         0    7    0              0   Initialize program
1   OpenWrite    0    2    0              0   Open table 'users' for writing
2   NewRowid     0    1    0              0   Generate new rowid in r[1]
3   Integer      1    2    0              0   r[2] = 1 (id)
4   String       0    3    0    'Alice'   0   r[3] = 'Alice' (name)
5   MakeRecord   2    2    4              0   r[4] = record(r[2], r[3])
6   Insert       0    4    1              0   Insert record at rowid r[1]
7   Close        0    0    0              0   Close cursor 0
8   Halt         0    0    0              0   End program
```

## Thread Safety

**Concurrency Model:**

- **Pager**: Thread-safe with RWMutex
- **B-tree**: Thread-safe operations
- **VDBE**: NOT thread-safe (one instance per query)
- **Driver**: Connection pool managed by database/sql

**Locking Strategy:**

- **Read Lock**: Multiple readers allowed
- **Write Lock**: Exclusive lock for writers
- **Transaction Isolation**: Serializable (one writer at a time)

## Performance Characteristics

**Time Complexity:**

- **Point Query**: O(log N) with index
- **Range Scan**: O(log N + M) where M = result size
- **Table Scan**: O(N)
- **Index Lookup**: O(log N)
- **Sort**: O(N log N)

**Space Complexity:**

- **Page Cache**: Configurable (default: 2000 pages)
- **VDBE Memory**: O(K) where K = registers needed
- **Transaction**: O(M) where M = modified pages

## Configuration

The pager and cache can be configured:

```go
// Page cache size (number of pages)
db.SetMaxOpenConns(100)  // Limits concurrent connections

// Each connection has its own page cache
// Default: 2000 pages × page_size (typically 4KB = 8MB)
```

## Error Handling

Each layer has specific error handling:

1. **Parser**: Syntax errors, semantic errors
2. **Planner**: Reference errors, type errors
3. **VDBE**: Runtime errors, constraint violations
4. **B-tree**: Corruption errors, page errors
5. **Pager**: I/O errors, disk full, lock conflicts

Errors propagate up the stack with context.

## References

- [SQLite Architecture](https://www.sqlite.org/arch.html)
- [SQLite File Format](https://www.sqlite.org/fileformat.html)
- [SQLite VDBE](https://www.sqlite.org/opcode.html)
- [B-tree Data Structure](https://en.wikipedia.org/wiki/B-tree)
