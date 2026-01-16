// Package parser implements SQL tokenization and parsing for the SQLite engine.
package parser

// TokenType represents the type of a SQL token.
type TokenType int

// Token type constants - based on SQLite's token definitions
const (
	// Special tokens
	TK_EOF TokenType = iota
	TK_ILLEGAL
	TK_SPACE
	TK_COMMENT

	// Literals
	TK_INTEGER
	TK_FLOAT
	TK_STRING
	TK_BLOB
	TK_NULL
	TK_ID
	TK_VARIABLE

	// Keywords - DDL
	TK_CREATE
	TK_TABLE
	TK_INDEX
	TK_VIEW
	TK_TRIGGER
	TK_DROP
	TK_ALTER
	TK_RENAME
	TK_ADD
	TK_COLUMN

	// Keywords - DML
	TK_SELECT
	TK_FROM
	TK_WHERE
	TK_INSERT
	TK_INTO
	TK_VALUES
	TK_UPDATE
	TK_SET
	TK_DELETE

	// Keywords - Query clauses
	TK_ORDER
	TK_BY
	TK_GROUP
	TK_HAVING
	TK_LIMIT
	TK_OFFSET
	TK_DISTINCT
	TK_ALL
	TK_ASC
	TK_DESC

	// Keywords - Joins
	TK_JOIN
	TK_LEFT
	TK_RIGHT
	TK_INNER
	TK_OUTER
	TK_CROSS
	TK_NATURAL
	TK_ON
	TK_USING

	// Keywords - Logical operators
	TK_AND
	TK_OR
	TK_NOT
	TK_IS
	TK_IN
	TK_LIKE
	TK_GLOB
	TK_BETWEEN
	TK_CASE
	TK_WHEN
	TK_THEN
	TK_ELSE
	TK_END

	// Keywords - Data types
	TK_INTEGER_TYPE
	TK_REAL
	TK_TEXT
	TK_BLOB_TYPE
	TK_NUMERIC

	// Keywords - Constraints
	TK_PRIMARY
	TK_KEY
	TK_UNIQUE
	TK_CHECK
	TK_DEFAULT
	TK_CONSTRAINT
	TK_FOREIGN
	TK_REFERENCES
	TK_AUTOINCREMENT
	TK_COLLATE

	// Keywords - Modifiers
	TK_AS
	TK_IF
	TK_EXISTS
	TK_TEMPORARY
	TK_TEMP
	TK_VIRTUAL

	// Keywords - Transactions
	TK_BEGIN
	TK_COMMIT
	TK_ROLLBACK
	TK_TRANSACTION
	TK_SAVEPOINT
	TK_RELEASE
	TK_DEFERRED
	TK_IMMEDIATE
	TK_EXCLUSIVE

	// Keywords - Other
	TK_EXPLAIN
	TK_QUERY
	TK_PLAN
	TK_PRAGMA
	TK_ANALYZE
	TK_ATTACH
	TK_DETACH
	TK_DATABASE
	TK_VACUUM
	TK_REINDEX

	// Operators - Comparison
	TK_EQ     // =, ==
	TK_NE     // <>, !=
	TK_LT     // <
	TK_LE     // <=
	TK_GT     // >
	TK_GE     // >=
	TK_ISNULL
	TK_NOTNULL

	// Operators - Arithmetic
	TK_PLUS   // +
	TK_MINUS  // -
	TK_STAR   // *
	TK_SLASH  // /
	TK_REM    // %

	// Operators - Bitwise
	TK_BITAND  // &
	TK_BITOR   // |
	TK_BITNOT  // ~
	TK_LSHIFT  // <<
	TK_RSHIFT  // >>

	// Operators - String
	TK_CONCAT  // ||

	// Punctuation
	TK_LP      // (
	TK_RP      // )
	TK_COMMA   // ,
	TK_SEMI    // ;
	TK_DOT     // .

	// Keywords - Window functions
	TK_OVER
	TK_PARTITION
	TK_ROWS
	TK_RANGE
	TK_UNBOUNDED
	TK_CURRENT
	TK_FOLLOWING
	TK_PRECEDING
	TK_FILTER
	TK_WINDOW
	TK_GROUPS
	TK_EXCLUDE
	TK_TIES
	TK_OTHERS

	// Keywords - Set operations
	TK_UNION
	TK_EXCEPT
	TK_INTERSECT

	// Additional keywords
	TK_CAST
	TK_ESCAPE
	TK_MATCH
	TK_REGEXP
	TK_ABORT
	TK_ACTION
	TK_AFTER
	TK_BEFORE
	TK_CASCADE
	TK_CONFLICT
	TK_FAIL
	TK_IGNORE
	TK_REPLACE
	TK_RESTRICT
	TK_NO
	TK_EACH
	TK_FOR
	TK_ROW
	TK_INITIALLY
	TK_DEFERRABLE
	TK_INDEXED
	TK_WITHOUT
	TK_ROWID
	TK_STRICT
	TK_GENERATED
	TK_ALWAYS
	TK_STORED

	// Special operator types
	TK_PTR     // ->
	TK_QNUMBER // Quoted number (with separators)
)

// Token represents a SQL token with its type, text, and position.
type Token struct {
	Type   TokenType // Token type
	Lexeme string    // Raw text of the token
	Pos    int       // Starting position in source
	Line   int       // Line number (1-based)
	Col    int       // Column number (1-based)
}

// String returns a string representation of the token type.
func (t TokenType) String() string {
	switch t {
	case TK_EOF:
		return "EOF"
	case TK_ILLEGAL:
		return "ILLEGAL"
	case TK_SPACE:
		return "SPACE"
	case TK_COMMENT:
		return "COMMENT"
	case TK_INTEGER:
		return "INTEGER"
	case TK_FLOAT:
		return "FLOAT"
	case TK_STRING:
		return "STRING"
	case TK_BLOB:
		return "BLOB"
	case TK_NULL:
		return "NULL"
	case TK_ID:
		return "ID"
	case TK_VARIABLE:
		return "VARIABLE"
	case TK_SELECT:
		return "SELECT"
	case TK_FROM:
		return "FROM"
	case TK_WHERE:
		return "WHERE"
	case TK_INSERT:
		return "INSERT"
	case TK_INTO:
		return "INTO"
	case TK_VALUES:
		return "VALUES"
	case TK_UPDATE:
		return "UPDATE"
	case TK_SET:
		return "SET"
	case TK_DELETE:
		return "DELETE"
	case TK_CREATE:
		return "CREATE"
	case TK_TABLE:
		return "TABLE"
	case TK_INDEX:
		return "INDEX"
	case TK_DROP:
		return "DROP"
	case TK_ORDER:
		return "ORDER"
	case TK_BY:
		return "BY"
	case TK_GROUP:
		return "GROUP"
	case TK_HAVING:
		return "HAVING"
	case TK_LIMIT:
		return "LIMIT"
	case TK_OFFSET:
		return "OFFSET"
	case TK_AND:
		return "AND"
	case TK_OR:
		return "OR"
	case TK_NOT:
		return "NOT"
	case TK_IS:
		return "IS"
	case TK_IN:
		return "IN"
	case TK_LIKE:
		return "LIKE"
	case TK_BETWEEN:
		return "BETWEEN"
	case TK_EQ:
		return "EQ"
	case TK_NE:
		return "NE"
	case TK_LT:
		return "LT"
	case TK_LE:
		return "LE"
	case TK_GT:
		return "GT"
	case TK_GE:
		return "GE"
	case TK_PLUS:
		return "PLUS"
	case TK_MINUS:
		return "MINUS"
	case TK_STAR:
		return "STAR"
	case TK_SLASH:
		return "SLASH"
	case TK_REM:
		return "REM"
	case TK_CONCAT:
		return "CONCAT"
	case TK_LP:
		return "LP"
	case TK_RP:
		return "RP"
	case TK_COMMA:
		return "COMMA"
	case TK_SEMI:
		return "SEMI"
	case TK_DOT:
		return "DOT"
	case TK_PRIMARY:
		return "PRIMARY"
	case TK_KEY:
		return "KEY"
	case TK_UNIQUE:
		return "UNIQUE"
	case TK_AS:
		return "AS"
	case TK_DISTINCT:
		return "DISTINCT"
	case TK_ASC:
		return "ASC"
	case TK_DESC:
		return "DESC"
	case TK_JOIN:
		return "JOIN"
	case TK_LEFT:
		return "LEFT"
	case TK_INNER:
		return "INNER"
	case TK_ON:
		return "ON"
	default:
		return "UNKNOWN"
	}
}

// IsKeyword returns true if the token is a SQL keyword.
func (t TokenType) IsKeyword() bool {
	return t >= TK_CREATE && t <= TK_STORED
}

// IsOperator returns true if the token is an operator.
func (t TokenType) IsOperator() bool {
	return (t >= TK_EQ && t <= TK_NOTNULL) ||
		(t >= TK_PLUS && t <= TK_REM) ||
		(t >= TK_BITAND && t <= TK_RSHIFT) ||
		t == TK_CONCAT
}

// IsLiteral returns true if the token is a literal value.
func (t TokenType) IsLiteral() bool {
	return t >= TK_INTEGER && t <= TK_NULL
}

// IsPunctuation returns true if the token is punctuation.
func (t TokenType) IsPunctuation() bool {
	return t >= TK_LP && t <= TK_DOT
}
