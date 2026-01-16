package parser

import (
	"fmt"
	"strings"
	"unicode"
)

// Lexer tokenizes SQL input.
type Lexer struct {
	input   string
	pos     int  // current position in input
	readPos int  // current reading position (after current char)
	ch      byte // current char under examination
	line    int  // current line number
	col     int  // current column number
}

// NewLexer creates a new Lexer for the given SQL input.
func NewLexer(input string) *Lexer {
	l := &Lexer{
		input: input,
		line:  1,
		col:   0,
	}
	l.readChar()
	return l
}

// readChar reads the next character and advances position.
func (l *Lexer) readChar() {
	if l.readPos >= len(l.input) {
		l.ch = 0 // EOF
	} else {
		l.ch = l.input[l.readPos]
	}
	l.pos = l.readPos
	l.readPos++
	l.col++
}

// peekChar returns the next character without advancing position.
func (l *Lexer) peekChar() byte {
	if l.readPos >= len(l.input) {
		return 0
	}
	return l.input[l.readPos]
}

// peekAhead returns the character n positions ahead without advancing.
func (l *Lexer) peekAhead(n int) byte {
	pos := l.readPos + n - 1
	if pos >= len(l.input) {
		return 0
	}
	return l.input[pos]
}

// NextToken returns the next token from the input.
func (l *Lexer) NextToken() Token {
	var tok Token

	l.skipWhitespace()

	tok.Pos = l.pos
	tok.Line = l.line
	tok.Col = l.col

	switch l.ch {
	case 0:
		tok.Type = TK_EOF
		tok.Lexeme = ""
	case ';':
		tok.Type = TK_SEMI
		tok.Lexeme = string(l.ch)
		l.readChar()
	case '(':
		tok.Type = TK_LP
		tok.Lexeme = string(l.ch)
		l.readChar()
	case ')':
		tok.Type = TK_RP
		tok.Lexeme = string(l.ch)
		l.readChar()
	case ',':
		tok.Type = TK_COMMA
		tok.Lexeme = string(l.ch)
		l.readChar()
	case '.':
		if isDigit(l.peekChar()) {
			tok = l.readNumber()
		} else {
			tok.Type = TK_DOT
			tok.Lexeme = string(l.ch)
			l.readChar()
		}
	case '+':
		tok.Type = TK_PLUS
		tok.Lexeme = string(l.ch)
		l.readChar()
	case '*':
		tok.Type = TK_STAR
		tok.Lexeme = string(l.ch)
		l.readChar()
	case '%':
		tok.Type = TK_REM
		tok.Lexeme = string(l.ch)
		l.readChar()
	case '~':
		tok.Type = TK_BITNOT
		tok.Lexeme = string(l.ch)
		l.readChar()
	case '&':
		tok.Type = TK_BITAND
		tok.Lexeme = string(l.ch)
		l.readChar()
	case '-':
		if l.peekChar() == '-' {
			tok = l.readLineComment()
		} else if l.peekChar() == '>' {
			l.readChar()
			if l.peekChar() == '>' {
				tok.Type = TK_PTR
				tok.Lexeme = "->>"
				l.readChar()
				l.readChar()
			} else {
				tok.Type = TK_PTR
				tok.Lexeme = "->"
				l.readChar()
			}
		} else {
			tok.Type = TK_MINUS
			tok.Lexeme = string(l.ch)
			l.readChar()
		}
	case '/':
		if l.peekChar() == '*' {
			tok = l.readBlockComment()
		} else {
			tok.Type = TK_SLASH
			tok.Lexeme = string(l.ch)
			l.readChar()
		}
	case '|':
		if l.peekChar() == '|' {
			tok.Type = TK_CONCAT
			tok.Lexeme = "||"
			l.readChar()
			l.readChar()
		} else {
			tok.Type = TK_BITOR
			tok.Lexeme = string(l.ch)
			l.readChar()
		}
	case '=':
		if l.peekChar() == '=' {
			tok.Type = TK_EQ
			tok.Lexeme = "=="
			l.readChar()
			l.readChar()
		} else {
			tok.Type = TK_EQ
			tok.Lexeme = string(l.ch)
			l.readChar()
		}
	case '<':
		if l.peekChar() == '=' {
			tok.Type = TK_LE
			tok.Lexeme = "<="
			l.readChar()
			l.readChar()
		} else if l.peekChar() == '>' {
			tok.Type = TK_NE
			tok.Lexeme = "<>"
			l.readChar()
			l.readChar()
		} else if l.peekChar() == '<' {
			tok.Type = TK_LSHIFT
			tok.Lexeme = "<<"
			l.readChar()
			l.readChar()
		} else {
			tok.Type = TK_LT
			tok.Lexeme = string(l.ch)
			l.readChar()
		}
	case '>':
		if l.peekChar() == '=' {
			tok.Type = TK_GE
			tok.Lexeme = ">="
			l.readChar()
			l.readChar()
		} else if l.peekChar() == '>' {
			tok.Type = TK_RSHIFT
			tok.Lexeme = ">>"
			l.readChar()
			l.readChar()
		} else {
			tok.Type = TK_GT
			tok.Lexeme = string(l.ch)
			l.readChar()
		}
	case '!':
		if l.peekChar() == '=' {
			tok.Type = TK_NE
			tok.Lexeme = "!="
			l.readChar()
			l.readChar()
		} else {
			tok.Type = TK_ILLEGAL
			tok.Lexeme = string(l.ch)
			l.readChar()
		}
	case '\'':
		tok = l.readString('\'')
	case '"':
		tok = l.readQuotedIdentifier('"')
	case '`':
		tok = l.readQuotedIdentifier('`')
	case '[':
		tok = l.readBracketedIdentifier()
	case '?':
		tok = l.readVariable()
	case '@', '#', ':':
		tok = l.readNamedVariable()
	case '$':
		if isLetter(l.peekChar()) || l.peekChar() == '_' {
			tok = l.readNamedVariable()
		} else {
			tok.Type = TK_ILLEGAL
			tok.Lexeme = string(l.ch)
			l.readChar()
		}
	default:
		if isLetter(l.ch) || l.ch == '_' {
			tok = l.readIdentifierOrKeyword()
			return tok
		} else if isDigit(l.ch) {
			tok = l.readNumber()
			return tok
		} else {
			tok.Type = TK_ILLEGAL
			tok.Lexeme = string(l.ch)
			l.readChar()
		}
	}

	return tok
}

// skipWhitespace skips whitespace characters and updates line/col tracking.
func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
		if l.ch == '\n' {
			l.line++
			l.col = 0
		}
		l.readChar()
	}
}

// readIdentifierOrKeyword reads an identifier or keyword.
func (l *Lexer) readIdentifierOrKeyword() Token {
	startPos := l.pos
	startLine := l.line
	startCol := l.col

	// Handle X'...' blob literals
	if (l.ch == 'x' || l.ch == 'X') && l.peekChar() == '\'' {
		l.readChar() // consume 'x' or 'X'
		l.readChar() // consume '\''
		return l.readBlobLiteral(startPos, startLine, startCol)
	}

	for isLetter(l.ch) || isDigit(l.ch) || l.ch == '_' || l.ch == '$' {
		l.readChar()
	}

	lexeme := l.input[startPos:l.pos]
	tokType := lookupKeyword(lexeme)

	return Token{
		Type:   tokType,
		Lexeme: lexeme,
		Pos:    startPos,
		Line:   startLine,
		Col:    startCol,
	}
}

// readNumber reads a numeric literal (integer or float).
func (l *Lexer) readNumber() Token {
	startPos := l.pos
	startLine := l.line
	startCol := l.col
	tokType := TK_INTEGER

	// Handle hexadecimal: 0x...
	if l.ch == '0' && (l.peekChar() == 'x' || l.peekChar() == 'X') {
		l.readChar() // consume '0'
		l.readChar() // consume 'x' or 'X'
		for isHexDigit(l.ch) || l.ch == '_' {
			l.readChar()
		}
		return Token{
			Type:   TK_INTEGER,
			Lexeme: l.input[startPos:l.pos],
			Pos:    startPos,
			Line:   startLine,
			Col:    startCol,
		}
	}

	// Read integer part
	for isDigit(l.ch) || l.ch == '_' {
		l.readChar()
	}

	// Check for decimal point
	if l.ch == '.' && isDigit(l.peekChar()) {
		tokType = TK_FLOAT
		l.readChar() // consume '.'
		for isDigit(l.ch) || l.ch == '_' {
			l.readChar()
		}
	}

	// Check for scientific notation
	if l.ch == 'e' || l.ch == 'E' {
		tokType = TK_FLOAT
		l.readChar()
		if l.ch == '+' || l.ch == '-' {
			l.readChar()
		}
		for isDigit(l.ch) || l.ch == '_' {
			l.readChar()
		}
	}

	return Token{
		Type:   tokType,
		Lexeme: l.input[startPos:l.pos],
		Pos:    startPos,
		Line:   startLine,
		Col:    startCol,
	}
}

// readString reads a string literal enclosed in single quotes.
func (l *Lexer) readString(quote byte) Token {
	startPos := l.pos
	startLine := l.line
	startCol := l.col

	l.readChar() // consume opening quote

	for l.ch != 0 {
		if l.ch == quote {
			// Check for escaped quote (doubled quote)
			if l.peekChar() == quote {
				l.readChar() // consume first quote
				l.readChar() // consume second quote
			} else {
				l.readChar() // consume closing quote
				break
			}
		} else {
			if l.ch == '\n' {
				l.line++
				l.col = 0
			}
			l.readChar()
		}
	}

	return Token{
		Type:   TK_STRING,
		Lexeme: l.input[startPos:l.pos],
		Pos:    startPos,
		Line:   startLine,
		Col:    startCol,
	}
}

// readQuotedIdentifier reads a quoted identifier (double-quoted or backticked).
func (l *Lexer) readQuotedIdentifier(quote byte) Token {
	startPos := l.pos
	startLine := l.line
	startCol := l.col

	l.readChar() // consume opening quote

	for l.ch != 0 && l.ch != quote {
		if l.ch == '\n' {
			l.line++
			l.col = 0
		}
		l.readChar()
	}

	if l.ch == quote {
		l.readChar() // consume closing quote
	}

	return Token{
		Type:   TK_ID,
		Lexeme: l.input[startPos:l.pos],
		Pos:    startPos,
		Line:   startLine,
		Col:    startCol,
	}
}

// readBracketedIdentifier reads a bracketed identifier [...].
func (l *Lexer) readBracketedIdentifier() Token {
	startPos := l.pos
	startLine := l.line
	startCol := l.col

	l.readChar() // consume '['

	for l.ch != 0 && l.ch != ']' {
		if l.ch == '\n' {
			l.line++
			l.col = 0
		}
		l.readChar()
	}

	if l.ch == ']' {
		l.readChar() // consume ']'
	}

	return Token{
		Type:   TK_ID,
		Lexeme: l.input[startPos:l.pos],
		Pos:    startPos,
		Line:   startLine,
		Col:    startCol,
	}
}

// readBlobLiteral reads a blob literal X'...'.
func (l *Lexer) readBlobLiteral(startPos, startLine, startCol int) Token {
	// We're already past X'
	for isHexDigit(l.ch) {
		l.readChar()
	}

	if l.ch == '\'' {
		l.readChar() // consume closing quote
	}

	return Token{
		Type:   TK_BLOB,
		Lexeme: l.input[startPos:l.pos],
		Pos:    startPos,
		Line:   startLine,
		Col:    startCol,
	}
}

// readVariable reads a positional parameter (?NNN).
func (l *Lexer) readVariable() Token {
	startPos := l.pos
	startLine := l.line
	startCol := l.col

	l.readChar() // consume '?'

	for isDigit(l.ch) {
		l.readChar()
	}

	return Token{
		Type:   TK_VARIABLE,
		Lexeme: l.input[startPos:l.pos],
		Pos:    startPos,
		Line:   startLine,
		Col:    startCol,
	}
}

// readNamedVariable reads a named parameter (@name, :name, #name, $name).
func (l *Lexer) readNamedVariable() Token {
	startPos := l.pos
	startLine := l.line
	startCol := l.col

	l.readChar() // consume prefix

	for isLetter(l.ch) || isDigit(l.ch) || l.ch == '_' {
		l.readChar()
	}

	return Token{
		Type:   TK_VARIABLE,
		Lexeme: l.input[startPos:l.pos],
		Pos:    startPos,
		Line:   startLine,
		Col:    startCol,
	}
}

// readLineComment reads a line comment (-- ...).
func (l *Lexer) readLineComment() Token {
	startPos := l.pos
	startLine := l.line
	startCol := l.col

	l.readChar() // consume first '-'
	l.readChar() // consume second '-'

	for l.ch != 0 && l.ch != '\n' {
		l.readChar()
	}

	return Token{
		Type:   TK_COMMENT,
		Lexeme: l.input[startPos:l.pos],
		Pos:    startPos,
		Line:   startLine,
		Col:    startCol,
	}
}

// readBlockComment reads a block comment (/* ... */).
func (l *Lexer) readBlockComment() Token {
	startPos := l.pos
	startLine := l.line
	startCol := l.col

	l.readChar() // consume '/'
	l.readChar() // consume '*'

	for l.ch != 0 {
		if l.ch == '\n' {
			l.line++
			l.col = 0
		}
		if l.ch == '*' && l.peekChar() == '/' {
			l.readChar() // consume '*'
			l.readChar() // consume '/'
			break
		}
		l.readChar()
	}

	return Token{
		Type:   TK_COMMENT,
		Lexeme: l.input[startPos:l.pos],
		Pos:    startPos,
		Line:   startLine,
		Col:    startCol,
	}
}

// Helper functions

func isLetter(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

func isHexDigit(ch byte) bool {
	return isDigit(ch) || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')
}

// lookupKeyword returns the token type for a keyword, or TK_ID if not a keyword.
func lookupKeyword(ident string) TokenType {
	// Convert to uppercase for case-insensitive comparison
	upper := strings.ToUpper(ident)

	switch upper {
	case "SELECT":
		return TK_SELECT
	case "FROM":
		return TK_FROM
	case "WHERE":
		return TK_WHERE
	case "INSERT":
		return TK_INSERT
	case "INTO":
		return TK_INTO
	case "VALUES":
		return TK_VALUES
	case "UPDATE":
		return TK_UPDATE
	case "SET":
		return TK_SET
	case "DELETE":
		return TK_DELETE
	case "CREATE":
		return TK_CREATE
	case "TABLE":
		return TK_TABLE
	case "INDEX":
		return TK_INDEX
	case "VIEW":
		return TK_VIEW
	case "TRIGGER":
		return TK_TRIGGER
	case "DROP":
		return TK_DROP
	case "ALTER":
		return TK_ALTER
	case "RENAME":
		return TK_RENAME
	case "ADD":
		return TK_ADD
	case "COLUMN":
		return TK_COLUMN
	case "ORDER":
		return TK_ORDER
	case "BY":
		return TK_BY
	case "GROUP":
		return TK_GROUP
	case "HAVING":
		return TK_HAVING
	case "LIMIT":
		return TK_LIMIT
	case "OFFSET":
		return TK_OFFSET
	case "DISTINCT":
		return TK_DISTINCT
	case "ALL":
		return TK_ALL
	case "ASC":
		return TK_ASC
	case "DESC":
		return TK_DESC
	case "JOIN":
		return TK_JOIN
	case "LEFT":
		return TK_LEFT
	case "RIGHT":
		return TK_RIGHT
	case "INNER":
		return TK_INNER
	case "OUTER":
		return TK_OUTER
	case "CROSS":
		return TK_CROSS
	case "NATURAL":
		return TK_NATURAL
	case "ON":
		return TK_ON
	case "USING":
		return TK_USING
	case "AND":
		return TK_AND
	case "OR":
		return TK_OR
	case "NOT":
		return TK_NOT
	case "IS":
		return TK_IS
	case "IN":
		return TK_IN
	case "LIKE":
		return TK_LIKE
	case "GLOB":
		return TK_GLOB
	case "BETWEEN":
		return TK_BETWEEN
	case "CASE":
		return TK_CASE
	case "WHEN":
		return TK_WHEN
	case "THEN":
		return TK_THEN
	case "ELSE":
		return TK_ELSE
	case "END":
		return TK_END
	case "NULL":
		return TK_NULL
	case "INTEGER":
		return TK_INTEGER_TYPE
	case "REAL":
		return TK_REAL
	case "TEXT":
		return TK_TEXT
	case "BLOB":
		return TK_BLOB_TYPE
	case "NUMERIC":
		return TK_NUMERIC
	case "PRIMARY":
		return TK_PRIMARY
	case "KEY":
		return TK_KEY
	case "UNIQUE":
		return TK_UNIQUE
	case "CHECK":
		return TK_CHECK
	case "DEFAULT":
		return TK_DEFAULT
	case "CONSTRAINT":
		return TK_CONSTRAINT
	case "FOREIGN":
		return TK_FOREIGN
	case "REFERENCES":
		return TK_REFERENCES
	case "AUTOINCREMENT":
		return TK_AUTOINCREMENT
	case "COLLATE":
		return TK_COLLATE
	case "AS":
		return TK_AS
	case "IF":
		return TK_IF
	case "EXISTS":
		return TK_EXISTS
	case "TEMPORARY", "TEMP":
		return TK_TEMP
	case "VIRTUAL":
		return TK_VIRTUAL
	case "BEGIN":
		return TK_BEGIN
	case "COMMIT":
		return TK_COMMIT
	case "ROLLBACK":
		return TK_ROLLBACK
	case "TRANSACTION":
		return TK_TRANSACTION
	case "SAVEPOINT":
		return TK_SAVEPOINT
	case "RELEASE":
		return TK_RELEASE
	case "DEFERRED":
		return TK_DEFERRED
	case "IMMEDIATE":
		return TK_IMMEDIATE
	case "EXCLUSIVE":
		return TK_EXCLUSIVE
	case "EXPLAIN":
		return TK_EXPLAIN
	case "QUERY":
		return TK_QUERY
	case "PLAN":
		return TK_PLAN
	case "PRAGMA":
		return TK_PRAGMA
	case "ANALYZE":
		return TK_ANALYZE
	case "ATTACH":
		return TK_ATTACH
	case "DETACH":
		return TK_DETACH
	case "DATABASE":
		return TK_DATABASE
	case "VACUUM":
		return TK_VACUUM
	case "REINDEX":
		return TK_REINDEX
	case "ISNULL":
		return TK_ISNULL
	case "NOTNULL":
		return TK_NOTNULL
	case "OVER":
		return TK_OVER
	case "PARTITION":
		return TK_PARTITION
	case "ROWS":
		return TK_ROWS
	case "RANGE":
		return TK_RANGE
	case "UNBOUNDED":
		return TK_UNBOUNDED
	case "CURRENT":
		return TK_CURRENT
	case "FOLLOWING":
		return TK_FOLLOWING
	case "PRECEDING":
		return TK_PRECEDING
	case "FILTER":
		return TK_FILTER
	case "WINDOW":
		return TK_WINDOW
	case "GROUPS":
		return TK_GROUPS
	case "EXCLUDE":
		return TK_EXCLUDE
	case "TIES":
		return TK_TIES
	case "OTHERS":
		return TK_OTHERS
	case "UNION":
		return TK_UNION
	case "EXCEPT":
		return TK_EXCEPT
	case "INTERSECT":
		return TK_INTERSECT
	case "CAST":
		return TK_CAST
	case "ESCAPE":
		return TK_ESCAPE
	case "MATCH":
		return TK_MATCH
	case "REGEXP":
		return TK_REGEXP
	case "ABORT":
		return TK_ABORT
	case "ACTION":
		return TK_ACTION
	case "AFTER":
		return TK_AFTER
	case "BEFORE":
		return TK_BEFORE
	case "CASCADE":
		return TK_CASCADE
	case "CONFLICT":
		return TK_CONFLICT
	case "FAIL":
		return TK_FAIL
	case "IGNORE":
		return TK_IGNORE
	case "REPLACE":
		return TK_REPLACE
	case "RESTRICT":
		return TK_RESTRICT
	case "NO":
		return TK_NO
	case "EACH":
		return TK_EACH
	case "FOR":
		return TK_FOR
	case "ROW":
		return TK_ROW
	case "INITIALLY":
		return TK_INITIALLY
	case "DEFERRABLE":
		return TK_DEFERRABLE
	case "INDEXED":
		return TK_INDEXED
	case "WITHOUT":
		return TK_WITHOUT
	case "ROWID":
		return TK_ROWID
	case "STRICT":
		return TK_STRICT
	case "GENERATED":
		return TK_GENERATED
	case "ALWAYS":
		return TK_ALWAYS
	case "STORED":
		return TK_STORED
	default:
		return TK_ID
	}
}

// TokenizeAll tokenizes the entire input and returns all tokens (excluding whitespace).
func TokenizeAll(input string) ([]Token, error) {
	lexer := NewLexer(input)
	var tokens []Token

	for {
		tok := lexer.NextToken()
		if tok.Type == TK_SPACE || tok.Type == TK_COMMENT {
			continue
		}
		tokens = append(tokens, tok)
		if tok.Type == TK_EOF {
			break
		}
		if tok.Type == TK_ILLEGAL {
			return tokens, fmt.Errorf("illegal token at line %d, col %d: %q", tok.Line, tok.Col, tok.Lexeme)
		}
	}

	return tokens, nil
}

// Unquote removes quotes from a quoted identifier or string.
func Unquote(s string) string {
	if len(s) < 2 {
		return s
	}

	// Handle different quote types
	if (s[0] == '\'' && s[len(s)-1] == '\'') ||
		(s[0] == '"' && s[len(s)-1] == '"') ||
		(s[0] == '`' && s[len(s)-1] == '`') {
		inner := s[1 : len(s)-1]
		// Replace doubled quotes with single quotes
		quote := string(s[0])
		return strings.ReplaceAll(inner, quote+quote, quote)
	}

	// Handle bracketed identifiers
	if s[0] == '[' && s[len(s)-1] == ']' {
		return s[1 : len(s)-1]
	}

	return s
}

// IsIdentChar returns true if the rune can be part of an unquoted identifier.
func IsIdentChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '$'
}
