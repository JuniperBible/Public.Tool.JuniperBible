# Anthony: Production-Ready Pure Go SQLite Clone

## Executive Summary

Complete implementation plan to bring Anthony to 100% production-ready status with 99% test coverage. Based on comprehensive analysis of the existing codebase (~34,000 lines, 177 files), this plan covers 11 major areas with detailed sub-tasks, estimated timelines, and testing strategies.

**Current State**: ~60% feature complete with solid foundation in parser, VDBE, pager, and B-tree.
**Target State**: Full SQLite compatibility, ACID compliance, concurrent access, 99% test coverage.

---

## Phase 1: Core ACID & Storage (Weeks 1-8)

### 1.1 Transaction & ACID Compliance
**Priority: CRITICAL | Complexity: XL | Estimate: 6-8 weeks**

#### Sub-tasks:

| Task | Files | Complexity | Days |
|------|-------|------------|------|
| WAL Mode Implementation | `internal/pager/wal.go` (new), `wal_index.go`, `wal_checkpoint.go` | XL | 14 |
| Concurrent Reader Support | `internal/pager/lock.go` (new), `lock_unix.go`, `lock_windows.go` | L | 10 |
| Hot Journal Recovery | `internal/pager/journal.go` (modify) | M | 5 |
| Savepoint Enhancement | `internal/pager/savepoint.go` (modify) | M | 5 |
| Lock Manager | `internal/pager/lock_manager.go` (new), `busy.go` | L | 7 |
| Snapshot Isolation | `internal/pager/snapshot.go` (new) | M | 5 |

**Key Data Structures:**
```go
// WAL Header (32 bytes)
type WALHeader struct {
    Magic, Version, PageSize, CheckpointSeq uint32
    Salt1, Salt2, Checksum1, Checksum2 uint32
}

// Lock Manager
type LockManager struct {
    fileLock FileLock
    level    LockLevel  // None, Shared, Reserved, Pending, Exclusive
    busyHandler BusyHandler
}
```

**Test Cases Required:** 53 tests across WAL, locking, recovery, savepoints

---

### 1.2 B-tree & Storage Layer
**Priority: CRITICAL | Complexity: XL | Estimate: 8-10 weeks**

#### Sub-tasks:

| Task | Files | Complexity | Days |
|------|-------|------------|------|
| Page Splits/Merges | `internal/btree/split.go`, `merge.go`, `balance.go` (new) | H | 20 |
| Free List Management | `internal/pager/freelist.go` (new) | M | 10 |
| Overflow Pages | `internal/btree/overflow.go` (new) | M | 10 |
| Index B-trees | `internal/btree/index_cursor.go` (new) | M | 15 |
| WITHOUT ROWID Tables | `internal/btree/clustered.go` (new) | M | 10 |
| Auto-Vacuum/Incremental | `internal/pager/autovacuum.go`, `ptrmap.go` (new) | M | 10 |
| Page Cache (LRU) | `internal/pager/cache.go` (new) | M | 10 |
| Integrity Check | `internal/btree/integrity.go` (new) | M | 10 |

**Critical Algorithm - Page Split:**
```
splitPage(key, payload):
  1. Allocate new sibling page
  2. Find divider cell (median key)
  3. Move cells >= divider to sibling
  4. Update parent with divider key
  5. Defragment both pages
  6. Insert new cell in appropriate page
```

---

## Phase 2: SQL Features (Weeks 9-16)

### 2.1 Parser Completion
**Priority: HIGH | Complexity: L-H | Estimate: 2 weeks**

#### Sub-tasks:

| Feature | AST Nodes | Parser Functions | Complexity | Hours |
|---------|-----------|------------------|------------|-------|
| ALTER TABLE | AlterTableStmt, AlterAction | parseAlter() | M | 6 |
| PRAGMA | PragmaStmt | parsePragma() | M | 4 |
| ATTACH/DETACH | AttachStmt, DetachStmt | parseAttach() | L | 3 |
| CREATE/DROP VIEW | CreateViewStmt, DropViewStmt | parseCreateView() | M | 4 |
| CREATE/DROP TRIGGER | CreateTriggerStmt | parseCreateTrigger() | H | 8 |
| WITH (CTEs) | WithClause, CTE | parseWithClause() | H | 8 |
| Window Functions | WindowSpec (exists) | parseWindowSpec() | H | 10 |
| UPSERT (ON CONFLICT) | UpsertClause, DoUpdateClause | parseUpsertClause() | M | 5 |
| Virtual Table Syntax | CreateVirtualTableStmt | parseCreateVirtualTable() | M | 4 |
| VACUUM/ANALYZE/REINDEX | VacuumStmt, AnalyzeStmt | parseVacuum() | L | 3 |
| EXPLAIN | ExplainStmt | parseExplain() | L | 2 |

**Tokens to Add:** `TK_WITH`, `TK_RECURSIVE`, `TK_DO`, `TK_NOTHING`, `TK_USING`

---

### 2.2 VDBE Opcode Completion
**Priority: HIGH | Complexity: H | Estimate: 4 weeks**

**Current:** 41 of 146 opcodes implemented (28%)
**Target:** 100% implementation

#### Missing Opcodes by Category:

| Category | Opcodes | Priority | Days |
|----------|---------|----------|------|
| Bitwise (5) | OpBitAnd, OpBitOr, OpBitNot, OpShiftLeft, OpShiftRight | H | 1 |
| Logical (3) | OpAnd, OpOr, OpNot | H | 1 |
| Type Conversion (5) | OpToText, OpToBlob, OpToNumeric, OpToInt, OpToReal | H | 1 |
| Index Operations (7) | OpIdxInsert, OpIdxDelete, OpIdxRowid, OpIdxLT/GT/LE/GE | Critical | 8 |
| Cursor (8) | OpOpenEphemeral, OpSeekGT/LT, OpNotExists, OpDeferredSeek | H | 5 |
| Trigger (5) | OpProgram, OpParam, OpInitCoroutine, OpEndCoroutine | H | 8 |
| Window (12) | OpAggStepWindow, OpWindowRowNum, OpWindowRank, etc. | M | 10 |
| Virtual Table (8) | OpVOpen, OpVFilter, OpVColumn, OpVNext, OpVUpdate | M | 12 |
| Transaction (5) | OpSavepoint, OpRelease, OpVerifyCookie, OpAutocommit | H | 5 |

**New Data Structures for Windows:**
```go
type WindowState struct {
    Frame      *WindowFrame
    CurrentRow int
    TotalRows  int
    OrderCols  []int
    PartCols   []int
}
```

---

### 2.3 Constraints & Referential Integrity
**Priority: HIGH | Complexity: H | Estimate: 6 weeks**

#### Sub-tasks:

| Constraint | Files | Complexity | Hours |
|------------|-------|------------|-------|
| PRIMARY KEY | `internal/constraint/primary_key.go` | M | 13 |
| UNIQUE | `internal/constraint/unique.go`, auto-index | M | 15 |
| NOT NULL | `internal/constraint/not_null.go` | L | 6 |
| CHECK | `internal/constraint/check.go` | M | 15 |
| DEFAULT | `internal/constraint/default.go` | M | 9 |
| FOREIGN KEY (full) | `internal/constraint/foreign_key.go`, `fk_cascade.go` | H | 41 |
| COLLATE | `internal/constraint/collation.go` | M | 10 |
| AUTOINCREMENT | `internal/schema/sqlite_sequence.go` | M | 15 |
| Conflict Resolution | `internal/constraint/conflict.go` | M | 23 |

**Foreign Key Actions:**
- ON DELETE: CASCADE, SET NULL, SET DEFAULT, RESTRICT, NO ACTION
- ON UPDATE: CASCADE, SET NULL, SET DEFAULT, RESTRICT
- Deferred vs Immediate checking
- PRAGMA foreign_keys support

---

### 2.4 Views & Triggers
**Priority: MEDIUM | Complexity: H | Estimate: 4 weeks**

#### Views:
| Task | Complexity | Days |
|------|------------|------|
| CREATE VIEW parsing/storage | M | 3 |
| View resolution in SELECT | M | 3 |
| Updatable views (simple) | H | 4 |
| View dependency tracking | M | 2 |
| DROP VIEW | L | 1 |

#### Triggers:
| Task | Complexity | Days |
|------|------------|------|
| CREATE TRIGGER parsing | H | 4 |
| BEFORE/AFTER/INSTEAD OF | M | 2 |
| OLD/NEW pseudo-tables | H | 4 |
| Trigger program compilation | H | 5 |
| WHEN clause filtering | M | 2 |
| RAISE() function | M | 3 |
| Recursive trigger prevention | M | 2 |

---

## Phase 3: Functions & Query Optimization (Weeks 17-24)

### 3.1 SQL Functions
**Priority: MEDIUM | Complexity: M-H | Estimate: 6 weeks**

#### Missing Functions:

| Category | Functions | Complexity | Days |
|----------|-----------|------------|------|
| String | printf, glob, like, soundex | M | 5 |
| Type | likely, unlikely | L | 1 |
| Window (12) | ROW_NUMBER, RANK, DENSE_RANK, NTILE, LAG, LEAD, etc. | H | 20 |
| JSON (15+) | json, json_array, json_extract, json_set, etc. | H | 15 |
| User-Defined | Registration API | M | 7 |

**Window Function Architecture:**
```go
type WindowFunction interface {
    AggregateFunction
    Value() (Value, error)           // Current value without finalizing
    Inverse(args []Value) error      // Remove value from frame
    SupportsInverse() bool
}
```

---

### 3.2 Query Optimizer
**Priority: MEDIUM | Complexity: H | Estimate: 5 weeks**

#### Sub-tasks:

| Feature | Files | Complexity | Days |
|---------|-------|------------|------|
| Statistics Store (sqlite_stat1) | `internal/planner/statistics.go` | M | 5 |
| ANALYZE Command | `internal/sql/analyze.go` | M | 4 |
| Index Selection Enhancement | `internal/planner/index.go` (modify) | M | 3 |
| Join Ordering (DP) | `internal/planner/join.go` (new) | H | 4 |
| Hash/Merge Joins | `internal/planner/join_algorithm.go`, VDBE opcodes | H | 6 |
| Predicate Pushdown | `internal/planner/pushdown.go` | M | 3 |
| Subquery Optimization | `internal/planner/subquery.go` | H | 6 |
| Query Plan Caching | `internal/planner/cache.go` | M | 3 |
| EXPLAIN QUERY PLAN | `internal/planner/explain.go` | M | 3 |

---

### 3.3 Virtual Tables
**Priority: LOW-MEDIUM | Complexity: H | Estimate: 5 weeks**

#### Sub-tasks:

| Component | Files | Complexity | Days |
|-----------|-------|------------|------|
| Module API | `internal/vtab/module.go`, `index.go`, `registry.go` | M | 5 |
| VDBE Integration | `internal/vdbe/vtab_exec.go` | M | 4 |
| sqlite_master vtab | `internal/vtab/builtin/sqlite_master.go` | L | 2 |
| pragma_* vtabs | `internal/vtab/builtin/pragma.go` | M | 3 |
| json_each/json_tree | `internal/vtab/builtin/json.go` | M | 4 |
| FTS5 (basic) | `internal/vtab/fts/` (tokenizer, index, query) | H | 15 |
| Planner Integration | `internal/planner/vtab.go` | M | 4 |

---

## Phase 4: Concurrency & Performance (Weeks 25-28)

### 4.1 Concurrency
**Priority: HIGH | Complexity: H | Estimate: 3 weeks**

| Feature | Files | Complexity | Days |
|---------|-------|------------|------|
| Reader/Writer Locking | `internal/pager/lock.go` | H | 10 |
| Connection Pooling | `internal/driver/pool.go` | M | 7 |
| Shared Cache Mode | `internal/pager/shared.go` | M | 5 |

### 4.2 Performance Optimization
**Priority: MEDIUM | Complexity: M | Estimate: 2 weeks**

| Feature | Files | Complexity | Days |
|---------|-------|------------|------|
| LRU Page Cache | `internal/pager/cache.go` | M | 5 |
| Memory Pool | `internal/vdbe/mempool.go` | M | 3 |
| Read-Ahead | `internal/pager/readahead.go` | M | 3 |
| Write Batching | `internal/pager/writebatch.go` | M | 3 |
| Cursor Pooling | `internal/vdbe/cursorpool.go` | L | 2 |

---

## Phase 5: Testing (Weeks 29-32)

### 5.1 Unit Tests (Target: 99% Coverage)

| Package | Current Est. | Target | New Tests | Hours |
|---------|--------------|--------|-----------|-------|
| vdbe/exec.go | 40% | 99% | ~200 | 40 |
| parser/parser.go | 60% | 99% | ~150 | 35 |
| driver/stmt.go | 50% | 99% | ~100 | 25 |
| btree/cursor.go | 45% | 99% | ~80 | 30 |
| pager/pager.go | 55% | 99% | ~70 | 20 |

### 5.2 Integration Tests

| Category | Test Cases | Hours |
|----------|------------|-------|
| Full SQL Execution | 150 | 45 |
| Transaction Boundaries | 50 | 25 |
| Concurrent Access | 40 | 30 |

### 5.3 Compatibility Tests

| Category | Hours |
|----------|-------|
| SQLite TCL Suite Adaptation | 80 |
| Result Comparison vs Real SQLite | 40 |
| Edge Cases | 20 |

### 5.4 Fuzz Testing

| Target | Hours |
|--------|-------|
| SQL Parser | 15 |
| Malformed Input | 20 |
| File Format | 25 |

### 5.5 Benchmarks

| Category | Hours |
|----------|-------|
| Throughput (Insert/Select) | 15 |
| Index Performance | 12 |
| Transaction Overhead | 8 |
| Memory Profiling | 10 |

### 5.6 Stress Tests

| Category | Hours |
|----------|-------|
| Large Datasets (1M+ rows) | 20 |
| Concurrent Connections (1000+) | 15 |
| Long-Running Transactions | 12 |

---

## Critical Files Summary

### Highest Priority Modifications:
1. `internal/pager/pager.go` - WAL, locking, cache integration
2. `internal/btree/cursor.go` - Page splits, index operations
3. `internal/vdbe/exec.go` - Missing opcodes, window functions
4. `internal/driver/stmt.go` - Constraint enforcement, triggers
5. `internal/parser/parser.go` - Missing SQL features

### New Files to Create:
| File | Purpose |
|------|---------|
| `internal/pager/wal.go` | Write-ahead logging |
| `internal/pager/lock.go` | Concurrent reader/writer locking |
| `internal/pager/cache.go` | LRU page cache |
| `internal/pager/freelist.go` | Free page management |
| `internal/btree/split.go` | Page split algorithm |
| `internal/btree/overflow.go` | Large payload handling |
| `internal/constraint/*.go` | All constraint types |
| `internal/vtab/*.go` | Virtual table system |
| `internal/planner/statistics.go` | Query statistics |
| `test/integration/*.go` | Integration tests |
| `test/fuzz/*.go` | Fuzz tests |
| `test/benchmark/*.go` | Performance benchmarks |

---

## Timeline Summary

| Phase | Weeks | Focus |
|-------|-------|-------|
| 1 | 1-8 | ACID, Storage Layer |
| 2 | 9-16 | SQL Features, Parser, VDBE |
| 3 | 17-24 | Functions, Optimizer, Virtual Tables |
| 4 | 25-28 | Concurrency, Performance |
| 5 | 29-32 | Testing (99% coverage) |

**Total Estimated Time: 32 weeks (~8 months) for full production readiness**

---

## Implementation Order (Critical Path)

1. **Week 1-2**: Page Cache LRU + Free List (foundation for everything)
2. **Week 3-4**: Page Splits/Merges (required for real inserts)
3. **Week 5-6**: Index B-trees (required for constraints)
4. **Week 7-8**: Reader/Writer Locking (ACID compliance)
5. **Week 9-10**: WAL Mode (concurrent readers)
6. **Week 11-12**: Constraint System (PK, UNIQUE, FK)
7. **Week 13-14**: Missing VDBE Opcodes
8. **Week 15-16**: Parser Completion
9. **Week 17-20**: Query Optimizer + Functions
10. **Week 21-24**: Views, Triggers, Virtual Tables
11. **Week 25-28**: Performance Tuning
12. **Week 29-32**: Test Coverage to 99%
