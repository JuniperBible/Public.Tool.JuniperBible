package parser

import (
	"fmt"
	"strconv"
	"strings"
)

// Parser implements a recursive descent parser for SQL.
type Parser struct {
	lexer   *Lexer
	tokens  []Token
	current int
	errors  []string
}

// NewParser creates a new parser for the given SQL input.
func NewParser(input string) *Parser {
	return &Parser{
		lexer:  NewLexer(input),
		tokens: make([]Token, 0),
		errors: make([]string, 0),
	}
}

// Parse parses the SQL input and returns a list of statements.
func (p *Parser) Parse() ([]Statement, error) {
	// Tokenize entire input first
	for {
		tok := p.lexer.NextToken()
		if tok.Type != TK_SPACE && tok.Type != TK_COMMENT {
			p.tokens = append(p.tokens, tok)
		}
		if tok.Type == TK_EOF {
			break
		}
	}

	statements := make([]Statement, 0)

	for !p.isAtEnd() {
		if p.match(TK_SEMI) {
			continue // skip empty statements
		}
		stmt, err := p.parseStatement()
		if err != nil {
			return statements, err
		}
		statements = append(statements, stmt)

		// Consume optional semicolon
		p.match(TK_SEMI)
	}

	if len(p.errors) > 0 {
		return statements, fmt.Errorf("parse errors: %s", strings.Join(p.errors, "; "))
	}

	return statements, nil
}

// parseStatement parses a single SQL statement.
func (p *Parser) parseStatement() (Statement, error) {
	switch {
	case p.match(TK_SELECT):
		return p.parseSelect()
	case p.match(TK_INSERT):
		return p.parseInsert()
	case p.match(TK_UPDATE):
		return p.parseUpdate()
	case p.match(TK_DELETE):
		return p.parseDelete()
	case p.match(TK_CREATE):
		return p.parseCreate()
	case p.match(TK_DROP):
		return p.parseDrop()
	case p.match(TK_BEGIN):
		return p.parseBegin()
	case p.match(TK_COMMIT):
		return &CommitStmt{}, nil
	case p.match(TK_ROLLBACK):
		return p.parseRollback()
	case p.match(TK_EXPLAIN):
		// Skip EXPLAIN and parse the actual statement
		p.match(TK_QUERY)
		p.match(TK_PLAN)
		return p.parseStatement()
	default:
		return nil, p.error("expected statement, got %s", p.peek().Type)
	}
}

// =============================================================================
// SELECT
// =============================================================================

func (p *Parser) parseSelect() (*SelectStmt, error) {
	stmt := &SelectStmt{}

	// DISTINCT or ALL
	if p.match(TK_DISTINCT) {
		stmt.Distinct = true
	} else {
		p.match(TK_ALL)
	}

	// Result columns
	cols, err := p.parseResultColumns()
	if err != nil {
		return nil, err
	}
	stmt.Columns = cols

	// FROM clause
	if p.match(TK_FROM) {
		fromClause, err := p.parseFromClause()
		if err != nil {
			return nil, err
		}
		stmt.From = fromClause
	}

	// WHERE clause
	if p.match(TK_WHERE) {
		where, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		stmt.Where = where
	}

	// GROUP BY clause
	if p.match(TK_GROUP) {
		if !p.match(TK_BY) {
			return nil, p.error("expected BY after GROUP")
		}
		groupBy, err := p.parseExpressionList()
		if err != nil {
			return nil, err
		}
		stmt.GroupBy = groupBy

		// HAVING clause
		if p.match(TK_HAVING) {
			having, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			stmt.Having = having
		}
	}

	// ORDER BY clause
	if p.match(TK_ORDER) {
		if !p.match(TK_BY) {
			return nil, p.error("expected BY after ORDER")
		}
		orderBy, err := p.parseOrderByList()
		if err != nil {
			return nil, err
		}
		stmt.OrderBy = orderBy
	}

	// LIMIT clause
	if p.match(TK_LIMIT) {
		limit, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		stmt.Limit = limit

		// OFFSET clause
		if p.match(TK_OFFSET) || p.match(TK_COMMA) {
			offset, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			stmt.Offset = offset
		}
	}

	// Compound SELECT (UNION, EXCEPT, INTERSECT)
	if p.check(TK_UNION) || p.check(TK_EXCEPT) || p.check(TK_INTERSECT) {
		compound, err := p.parseCompoundSelect(stmt)
		if err != nil {
			return nil, err
		}
		return compound, nil
	}

	return stmt, nil
}

func (p *Parser) parseCompoundSelect(left *SelectStmt) (*SelectStmt, error) {
	var op CompoundOp
	if p.match(TK_UNION) {
		if p.match(TK_ALL) {
			op = CompoundUnionAll
		} else {
			op = CompoundUnion
		}
	} else if p.match(TK_EXCEPT) {
		op = CompoundExcept
	} else if p.match(TK_INTERSECT) {
		op = CompoundIntersect
	}

	right, err := p.parseSelect()
	if err != nil {
		return nil, err
	}

	result := &SelectStmt{
		Compound: &CompoundSelect{
			Op:    op,
			Left:  left,
			Right: right,
		},
	}

	return result, nil
}

func (p *Parser) parseResultColumns() ([]ResultColumn, error) {
	columns := make([]ResultColumn, 0)

	for {
		if p.match(TK_STAR) {
			columns = append(columns, ResultColumn{Star: true})
		} else {
			// Check for table.*
			if p.check(TK_ID) && p.peekAhead(1).Type == TK_DOT && p.peekAhead(2).Type == TK_STAR {
				table := p.advance().Lexeme
				p.advance() // consume dot
				p.advance() // consume star
				columns = append(columns, ResultColumn{
					Table: table,
					Star:  true,
				})
			} else {
				expr, err := p.parseExpression()
				if err != nil {
					return nil, err
				}

				col := ResultColumn{Expr: expr}

				// Optional AS alias
				if p.match(TK_AS) {
					if !p.check(TK_ID) && !p.check(TK_STRING) {
						return nil, p.error("expected alias after AS")
					}
					col.Alias = Unquote(p.advance().Lexeme)
				} else if p.check(TK_ID) || p.check(TK_STRING) {
					// Implicit alias
					col.Alias = Unquote(p.advance().Lexeme)
				}

				columns = append(columns, col)
			}
		}

		if !p.match(TK_COMMA) {
			break
		}
	}

	return columns, nil
}

func (p *Parser) parseFromClause() (*FromClause, error) {
	clause := &FromClause{
		Tables: make([]TableOrSubquery, 0),
		Joins:  make([]JoinClause, 0),
	}

	// Parse first table or subquery
	table, err := p.parseTableOrSubquery()
	if err != nil {
		return nil, err
	}
	clause.Tables = append(clause.Tables, *table)

	// Parse joins
	for p.isJoinKeyword() {
		join, err := p.parseJoinClause()
		if err != nil {
			return nil, err
		}
		clause.Joins = append(clause.Joins, *join)
	}

	// Parse comma-separated tables (implicit cross join)
	for p.match(TK_COMMA) {
		table, err := p.parseTableOrSubquery()
		if err != nil {
			return nil, err
		}
		clause.Tables = append(clause.Tables, *table)
	}

	return clause, nil
}

func (p *Parser) parseTableOrSubquery() (*TableOrSubquery, error) {
	table := &TableOrSubquery{}

	if p.match(TK_LP) {
		// Subquery
		if !p.match(TK_SELECT) {
			return nil, p.error("expected SELECT in subquery")
		}
		subquery, err := p.parseSelect()
		if err != nil {
			return nil, err
		}
		if !p.match(TK_RP) {
			return nil, p.error("expected ) after subquery")
		}
		table.Subquery = subquery
	} else {
		// Table name
		if !p.check(TK_ID) {
			return nil, p.error("expected table name")
		}
		table.TableName = Unquote(p.advance().Lexeme)

		// INDEXED BY
		if p.match(TK_INDEXED) {
			if !p.match(TK_BY) {
				return nil, p.error("expected BY after INDEXED")
			}
			if !p.check(TK_ID) {
				return nil, p.error("expected index name")
			}
			table.Indexed = Unquote(p.advance().Lexeme)
		}
	}

	// Optional alias
	if p.match(TK_AS) {
		if !p.check(TK_ID) {
			return nil, p.error("expected alias after AS")
		}
		table.Alias = Unquote(p.advance().Lexeme)
	} else if p.check(TK_ID) && !p.isJoinKeyword() {
		// Implicit alias
		table.Alias = Unquote(p.advance().Lexeme)
	}

	return table, nil
}

func (p *Parser) parseJoinClause() (*JoinClause, error) {
	join := &JoinClause{}

	// Parse join type
	if p.match(TK_NATURAL) {
		// NATURAL join
	}

	if p.match(TK_LEFT) {
		join.Type = JoinLeft
		p.match(TK_OUTER)
	} else if p.match(TK_RIGHT) {
		join.Type = JoinRight
		p.match(TK_OUTER)
	} else if p.match(TK_INNER) {
		join.Type = JoinInner
	} else if p.match(TK_CROSS) {
		join.Type = JoinCross
	}

	if !p.match(TK_JOIN) {
		return nil, p.error("expected JOIN")
	}

	// Parse table
	table, err := p.parseTableOrSubquery()
	if err != nil {
		return nil, err
	}
	join.Table = *table

	// Parse join condition
	if p.match(TK_ON) {
		condition, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		join.Condition.On = condition
	} else if p.match(TK_USING) {
		if !p.match(TK_LP) {
			return nil, p.error("expected ( after USING")
		}
		columns := make([]string, 0)
		for {
			if !p.check(TK_ID) {
				return nil, p.error("expected column name")
			}
			columns = append(columns, Unquote(p.advance().Lexeme))
			if !p.match(TK_COMMA) {
				break
			}
		}
		if !p.match(TK_RP) {
			return nil, p.error("expected ) after USING columns")
		}
		join.Condition.Using = columns
	}

	return join, nil
}

// =============================================================================
// INSERT
// =============================================================================

func (p *Parser) parseInsert() (*InsertStmt, error) {
	stmt := &InsertStmt{}

	// OR conflict clause
	if p.match(TK_OR) {
		stmt.OnConflict = p.parseOnConflict()
	}

	if !p.match(TK_INTO) {
		return nil, p.error("expected INTO after INSERT")
	}

	if !p.check(TK_ID) {
		return nil, p.error("expected table name")
	}
	stmt.Table = Unquote(p.advance().Lexeme)

	// Column list
	if p.match(TK_LP) {
		for {
			if !p.check(TK_ID) {
				return nil, p.error("expected column name")
			}
			stmt.Columns = append(stmt.Columns, Unquote(p.advance().Lexeme))
			if !p.match(TK_COMMA) {
				break
			}
		}
		if !p.match(TK_RP) {
			return nil, p.error("expected ) after column list")
		}
	}

	// VALUES or SELECT
	if p.match(TK_VALUES) {
		for {
			if !p.match(TK_LP) {
				return nil, p.error("expected ( before values")
			}
			values, err := p.parseExpressionList()
			if err != nil {
				return nil, err
			}
			stmt.Values = append(stmt.Values, values)
			if !p.match(TK_RP) {
				return nil, p.error("expected ) after values")
			}
			if !p.match(TK_COMMA) {
				break
			}
		}
	} else if p.match(TK_SELECT) {
		sel, err := p.parseSelect()
		if err != nil {
			return nil, err
		}
		stmt.Select = sel
	} else if p.match(TK_DEFAULT) {
		if !p.match(TK_VALUES) {
			return nil, p.error("expected VALUES after DEFAULT")
		}
		stmt.DefaultVals = true
	} else {
		return nil, p.error("expected VALUES, SELECT, or DEFAULT")
	}

	return stmt, nil
}

// =============================================================================
// UPDATE
// =============================================================================

func (p *Parser) parseUpdate() (*UpdateStmt, error) {
	stmt := &UpdateStmt{}

	// OR conflict clause
	if p.match(TK_OR) {
		stmt.OnConflict = p.parseOnConflict()
	}

	if !p.check(TK_ID) {
		return nil, p.error("expected table name")
	}
	stmt.Table = Unquote(p.advance().Lexeme)

	if !p.match(TK_SET) {
		return nil, p.error("expected SET")
	}

	// Parse assignments
	for {
		if !p.check(TK_ID) {
			return nil, p.error("expected column name")
		}
		column := Unquote(p.advance().Lexeme)

		if !p.match(TK_EQ) {
			return nil, p.error("expected = after column name")
		}

		value, err := p.parseExpression()
		if err != nil {
			return nil, err
		}

		stmt.Sets = append(stmt.Sets, Assignment{
			Column: column,
			Value:  value,
		})

		if !p.match(TK_COMMA) {
			break
		}
	}

	// WHERE clause
	if p.match(TK_WHERE) {
		where, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		stmt.Where = where
	}

	// ORDER BY clause
	if p.match(TK_ORDER) {
		if !p.match(TK_BY) {
			return nil, p.error("expected BY after ORDER")
		}
		orderBy, err := p.parseOrderByList()
		if err != nil {
			return nil, err
		}
		stmt.OrderBy = orderBy
	}

	// LIMIT clause
	if p.match(TK_LIMIT) {
		limit, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		stmt.Limit = limit
	}

	return stmt, nil
}

// =============================================================================
// DELETE
// =============================================================================

func (p *Parser) parseDelete() (*DeleteStmt, error) {
	stmt := &DeleteStmt{}

	if !p.match(TK_FROM) {
		return nil, p.error("expected FROM after DELETE")
	}

	if !p.check(TK_ID) {
		return nil, p.error("expected table name")
	}
	stmt.Table = Unquote(p.advance().Lexeme)

	// WHERE clause
	if p.match(TK_WHERE) {
		where, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		stmt.Where = where
	}

	// ORDER BY clause
	if p.match(TK_ORDER) {
		if !p.match(TK_BY) {
			return nil, p.error("expected BY after ORDER")
		}
		orderBy, err := p.parseOrderByList()
		if err != nil {
			return nil, err
		}
		stmt.OrderBy = orderBy
	}

	// LIMIT clause
	if p.match(TK_LIMIT) {
		limit, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		stmt.Limit = limit
	}

	return stmt, nil
}

// =============================================================================
// CREATE
// =============================================================================

func (p *Parser) parseCreate() (Statement, error) {
	// TEMP/TEMPORARY
	temp := p.match(TK_TEMP) || p.match(TK_TEMPORARY)

	if p.match(TK_TABLE) {
		return p.parseCreateTable(temp)
	} else if p.match(TK_INDEX) {
		return p.parseCreateIndex()
	} else {
		return nil, p.error("expected TABLE or INDEX after CREATE")
	}
}

func (p *Parser) parseCreateTable(temp bool) (*CreateTableStmt, error) {
	stmt := &CreateTableStmt{Temp: temp}

	if p.match(TK_IF) {
		if !p.match(TK_NOT) || !p.match(TK_EXISTS) {
			return nil, p.error("expected NOT EXISTS after IF")
		}
		stmt.IfNotExists = true
	}

	if !p.check(TK_ID) {
		return nil, p.error("expected table name")
	}
	stmt.Name = Unquote(p.advance().Lexeme)

	// AS SELECT or column definitions
	if p.match(TK_AS) {
		if !p.match(TK_SELECT) {
			return nil, p.error("expected SELECT after AS")
		}
		sel, err := p.parseSelect()
		if err != nil {
			return nil, err
		}
		stmt.Select = sel
		return stmt, nil
	}

	if !p.match(TK_LP) {
		return nil, p.error("expected ( after table name")
	}

	// Parse column definitions
	for {
		col, err := p.parseColumnDef()
		if err != nil {
			// Check if it's a table constraint
			constraint, err2 := p.parseTableConstraint()
			if err2 != nil {
				return nil, err // return original column error
			}
			stmt.Constraints = append(stmt.Constraints, *constraint)
		} else {
			stmt.Columns = append(stmt.Columns, *col)
		}

		if !p.match(TK_COMMA) {
			break
		}
	}

	if !p.match(TK_RP) {
		return nil, p.error("expected ) after column definitions")
	}

	// Table options
	for {
		if p.match(TK_WITHOUT) {
			if !p.match(TK_ROWID) {
				return nil, p.error("expected ROWID after WITHOUT")
			}
			stmt.WithoutRowID = true
		} else if p.match(TK_STRICT) {
			stmt.Strict = true
		} else {
			break
		}
		p.match(TK_COMMA)
	}

	return stmt, nil
}

func (p *Parser) parseColumnDef() (*ColumnDef, error) {
	if !p.check(TK_ID) {
		return nil, p.error("expected column name")
	}

	col := &ColumnDef{
		Name: Unquote(p.advance().Lexeme),
	}

	// Type name (optional)
	if p.check(TK_ID) || p.check(TK_INTEGER_TYPE) || p.check(TK_TEXT) ||
		p.check(TK_REAL) || p.check(TK_BLOB_TYPE) || p.check(TK_NUMERIC) {
		col.Type = p.parseTypeName()
	}

	// Column constraints
	for p.isColumnConstraint() {
		constraint, err := p.parseColumnConstraint()
		if err != nil {
			return nil, err
		}
		col.Constraints = append(col.Constraints, *constraint)
	}

	return col, nil
}

func (p *Parser) parseTypeName() string {
	parts := make([]string, 0)
	parts = append(parts, p.advance().Lexeme)

	// Handle type modifiers like INTEGER(10) or NUMERIC(10, 2)
	if p.match(TK_LP) {
		parts = append(parts, "(")
		parts = append(parts, p.advance().Lexeme)
		if p.match(TK_COMMA) {
			parts = append(parts, ",")
			parts = append(parts, p.advance().Lexeme)
		}
		if p.match(TK_RP) {
			parts = append(parts, ")")
		}
	}

	return strings.Join(parts, "")
}

func (p *Parser) parseColumnConstraint() (*ColumnConstraint, error) {
	constraint := &ColumnConstraint{}

	// Optional constraint name
	if p.match(TK_CONSTRAINT) {
		if !p.check(TK_ID) {
			return nil, p.error("expected constraint name")
		}
		constraint.Name = Unquote(p.advance().Lexeme)
	}

	if p.match(TK_PRIMARY) {
		if !p.match(TK_KEY) {
			return nil, p.error("expected KEY after PRIMARY")
		}
		constraint.Type = ConstraintPrimaryKey
		constraint.PrimaryKey = &PrimaryKeyConstraint{}

		if p.match(TK_ASC) {
			constraint.PrimaryKey.Order = SortAsc
		} else if p.match(TK_DESC) {
			constraint.PrimaryKey.Order = SortDesc
		}

		if p.match(TK_AUTOINCREMENT) {
			constraint.PrimaryKey.Autoincrement = true
		}
	} else if p.match(TK_NOT) {
		if !p.match(TK_NULL) {
			return nil, p.error("expected NULL after NOT")
		}
		constraint.Type = ConstraintNotNull
		constraint.NotNull = true
	} else if p.match(TK_UNIQUE) {
		constraint.Type = ConstraintUnique
		constraint.Unique = true
	} else if p.match(TK_CHECK) {
		if !p.match(TK_LP) {
			return nil, p.error("expected ( after CHECK")
		}
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if !p.match(TK_RP) {
			return nil, p.error("expected ) after CHECK expression")
		}
		constraint.Type = ConstraintCheck
		constraint.Check = expr
	} else if p.match(TK_DEFAULT) {
		expr, err := p.parsePrimaryExpression()
		if err != nil {
			return nil, err
		}
		constraint.Type = ConstraintDefault
		constraint.Default = expr
	} else if p.match(TK_COLLATE) {
		if !p.check(TK_ID) {
			return nil, p.error("expected collation name")
		}
		constraint.Type = ConstraintCollate
		constraint.Collate = Unquote(p.advance().Lexeme)
	} else {
		return nil, p.error("expected column constraint")
	}

	return constraint, nil
}

func (p *Parser) parseTableConstraint() (*TableConstraint, error) {
	constraint := &TableConstraint{}

	// Optional constraint name
	if p.match(TK_CONSTRAINT) {
		if !p.check(TK_ID) {
			return nil, p.error("expected constraint name")
		}
		constraint.Name = Unquote(p.advance().Lexeme)
	}

	if p.match(TK_PRIMARY) {
		if !p.match(TK_KEY) {
			return nil, p.error("expected KEY after PRIMARY")
		}
		constraint.Type = ConstraintPrimaryKey
		constraint.PrimaryKey = &PrimaryKeyTableConstraint{}

		if !p.match(TK_LP) {
			return nil, p.error("expected ( after PRIMARY KEY")
		}
		cols, err := p.parseIndexedColumns()
		if err != nil {
			return nil, err
		}
		constraint.PrimaryKey.Columns = cols
		if !p.match(TK_RP) {
			return nil, p.error("expected ) after PRIMARY KEY columns")
		}
	} else if p.match(TK_UNIQUE) {
		constraint.Type = ConstraintUnique
		constraint.Unique = &UniqueTableConstraint{}

		if !p.match(TK_LP) {
			return nil, p.error("expected ( after UNIQUE")
		}
		cols, err := p.parseIndexedColumns()
		if err != nil {
			return nil, err
		}
		constraint.Unique.Columns = cols
		if !p.match(TK_RP) {
			return nil, p.error("expected ) after UNIQUE columns")
		}
	} else if p.match(TK_CHECK) {
		if !p.match(TK_LP) {
			return nil, p.error("expected ( after CHECK")
		}
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if !p.match(TK_RP) {
			return nil, p.error("expected ) after CHECK expression")
		}
		constraint.Type = ConstraintCheck
		constraint.Check = expr
	} else {
		return nil, p.error("expected table constraint")
	}

	return constraint, nil
}

func (p *Parser) parseCreateIndex() (*CreateIndexStmt, error) {
	stmt := &CreateIndexStmt{}

	if p.match(TK_UNIQUE) {
		stmt.Unique = true
	}

	if p.match(TK_IF) {
		if !p.match(TK_NOT) || !p.match(TK_EXISTS) {
			return nil, p.error("expected NOT EXISTS after IF")
		}
		stmt.IfNotExists = true
	}

	if !p.check(TK_ID) {
		return nil, p.error("expected index name")
	}
	stmt.Name = Unquote(p.advance().Lexeme)

	if !p.match(TK_ON) {
		return nil, p.error("expected ON after index name")
	}

	if !p.check(TK_ID) {
		return nil, p.error("expected table name")
	}
	stmt.Table = Unquote(p.advance().Lexeme)

	if !p.match(TK_LP) {
		return nil, p.error("expected ( after table name")
	}

	cols, err := p.parseIndexedColumns()
	if err != nil {
		return nil, err
	}
	stmt.Columns = cols

	if !p.match(TK_RP) {
		return nil, p.error("expected ) after columns")
	}

	// WHERE clause
	if p.match(TK_WHERE) {
		where, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		stmt.Where = where
	}

	return stmt, nil
}

func (p *Parser) parseIndexedColumns() ([]IndexedColumn, error) {
	columns := make([]IndexedColumn, 0)

	for {
		if !p.check(TK_ID) {
			return nil, p.error("expected column name")
		}
		col := IndexedColumn{
			Column: Unquote(p.advance().Lexeme),
		}

		if p.match(TK_ASC) {
			col.Order = SortAsc
		} else if p.match(TK_DESC) {
			col.Order = SortDesc
		}

		columns = append(columns, col)

		if !p.match(TK_COMMA) {
			break
		}
	}

	return columns, nil
}

// =============================================================================
// DROP
// =============================================================================

func (p *Parser) parseDrop() (Statement, error) {
	if p.match(TK_TABLE) {
		return p.parseDropTable()
	} else if p.match(TK_INDEX) {
		return p.parseDropIndex()
	} else {
		return nil, p.error("expected TABLE or INDEX after DROP")
	}
}

func (p *Parser) parseDropTable() (*DropTableStmt, error) {
	stmt := &DropTableStmt{}

	if p.match(TK_IF) {
		if !p.match(TK_EXISTS) {
			return nil, p.error("expected EXISTS after IF")
		}
		stmt.IfExists = true
	}

	if !p.check(TK_ID) {
		return nil, p.error("expected table name")
	}
	stmt.Name = Unquote(p.advance().Lexeme)

	return stmt, nil
}

func (p *Parser) parseDropIndex() (*DropIndexStmt, error) {
	stmt := &DropIndexStmt{}

	if p.match(TK_IF) {
		if !p.match(TK_EXISTS) {
			return nil, p.error("expected EXISTS after IF")
		}
		stmt.IfExists = true
	}

	if !p.check(TK_ID) {
		return nil, p.error("expected index name")
	}
	stmt.Name = Unquote(p.advance().Lexeme)

	return stmt, nil
}

// =============================================================================
// Transactions
// =============================================================================

func (p *Parser) parseBegin() (*BeginStmt, error) {
	stmt := &BeginStmt{Mode: TransactionDeferred}

	p.match(TK_TRANSACTION)

	if p.match(TK_DEFERRED) {
		stmt.Mode = TransactionDeferred
	} else if p.match(TK_IMMEDIATE) {
		stmt.Mode = TransactionImmediate
	} else if p.match(TK_EXCLUSIVE) {
		stmt.Mode = TransactionExclusive
	}

	return stmt, nil
}

func (p *Parser) parseRollback() (*RollbackStmt, error) {
	stmt := &RollbackStmt{}

	p.match(TK_TRANSACTION)

	return stmt, nil
}

// =============================================================================
// Expressions
// =============================================================================

func (p *Parser) parseExpression() (Expression, error) {
	return p.parseOrExpression()
}

func (p *Parser) parseOrExpression() (Expression, error) {
	left, err := p.parseAndExpression()
	if err != nil {
		return nil, err
	}

	for p.match(TK_OR) {
		right, err := p.parseAndExpression()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{
			Left:  left,
			Op:    OpOr,
			Right: right,
		}
	}

	return left, nil
}

func (p *Parser) parseAndExpression() (Expression, error) {
	left, err := p.parseNotExpression()
	if err != nil {
		return nil, err
	}

	for p.match(TK_AND) {
		right, err := p.parseNotExpression()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{
			Left:  left,
			Op:    OpAnd,
			Right: right,
		}
	}

	return left, nil
}

func (p *Parser) parseNotExpression() (Expression, error) {
	if p.match(TK_NOT) {
		expr, err := p.parseNotExpression()
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{
			Op:   OpNot,
			Expr: expr,
		}, nil
	}

	return p.parseComparisonExpression()
}

func (p *Parser) parseComparisonExpression() (Expression, error) {
	left, err := p.parseBitwiseExpression()
	if err != nil {
		return nil, err
	}

	// IS NULL, IS NOT NULL
	if p.match(TK_IS) {
		if p.match(TK_NOT) {
			if p.match(TK_NULL) {
				return &UnaryExpr{Op: OpNotNull, Expr: left}, nil
			}
			return nil, p.error("expected NULL after IS NOT")
		} else if p.match(TK_NULL) {
			return &UnaryExpr{Op: OpIsNull, Expr: left}, nil
		}
		// IS comparison
		right, err := p.parseBitwiseExpression()
		if err != nil {
			return nil, err
		}
		return &BinaryExpr{Left: left, Op: OpEq, Right: right}, nil
	}

	// IN
	if p.check(TK_IN) || (p.check(TK_NOT) && p.peekAhead(1).Type == TK_IN) {
		not := p.match(TK_NOT)
		p.match(TK_IN)

		if !p.match(TK_LP) {
			return nil, p.error("expected ( after IN")
		}

		inExpr := &InExpr{Expr: left, Not: not}

		if p.match(TK_SELECT) {
			sel, err := p.parseSelect()
			if err != nil {
				return nil, err
			}
			inExpr.Select = sel
		} else {
			values, err := p.parseExpressionList()
			if err != nil {
				return nil, err
			}
			inExpr.Values = values
		}

		if !p.match(TK_RP) {
			return nil, p.error("expected ) after IN values")
		}

		return inExpr, nil
	}

	// BETWEEN
	if p.check(TK_BETWEEN) || (p.check(TK_NOT) && p.peekAhead(1).Type == TK_BETWEEN) {
		not := p.match(TK_NOT)
		p.match(TK_BETWEEN)

		lower, err := p.parseBitwiseExpression()
		if err != nil {
			return nil, err
		}

		if !p.match(TK_AND) {
			return nil, p.error("expected AND in BETWEEN")
		}

		upper, err := p.parseBitwiseExpression()
		if err != nil {
			return nil, err
		}

		return &BetweenExpr{
			Expr:  left,
			Lower: lower,
			Upper: upper,
			Not:   not,
		}, nil
	}

	// LIKE, GLOB, REGEXP, MATCH
	if p.match(TK_LIKE) {
		right, err := p.parseBitwiseExpression()
		if err != nil {
			return nil, err
		}
		return &BinaryExpr{Left: left, Op: OpLike, Right: right}, nil
	} else if p.match(TK_GLOB) {
		right, err := p.parseBitwiseExpression()
		if err != nil {
			return nil, err
		}
		return &BinaryExpr{Left: left, Op: OpGlob, Right: right}, nil
	} else if p.match(TK_REGEXP) {
		right, err := p.parseBitwiseExpression()
		if err != nil {
			return nil, err
		}
		return &BinaryExpr{Left: left, Op: OpRegexp, Right: right}, nil
	} else if p.match(TK_MATCH) {
		right, err := p.parseBitwiseExpression()
		if err != nil {
			return nil, err
		}
		return &BinaryExpr{Left: left, Op: OpMatch, Right: right}, nil
	}

	// Comparison operators
	var op BinaryOp
	matched := false
	if p.match(TK_EQ) {
		op = OpEq
		matched = true
	} else if p.match(TK_NE) {
		op = OpNe
		matched = true
	} else if p.match(TK_LT) {
		op = OpLt
		matched = true
	} else if p.match(TK_LE) {
		op = OpLe
		matched = true
	} else if p.match(TK_GT) {
		op = OpGt
		matched = true
	} else if p.match(TK_GE) {
		op = OpGe
		matched = true
	}

	if matched {
		right, err := p.parseBitwiseExpression()
		if err != nil {
			return nil, err
		}
		return &BinaryExpr{
			Left:  left,
			Op:    op,
			Right: right,
		}, nil
	}

	return left, nil
}

func (p *Parser) parseBitwiseExpression() (Expression, error) {
	left, err := p.parseAdditiveExpression()
	if err != nil {
		return nil, err
	}

	for {
		var op BinaryOp
		matched := false
		if p.match(TK_BITAND) {
			op = OpBitAnd
			matched = true
		} else if p.match(TK_BITOR) {
			op = OpBitOr
			matched = true
		} else if p.match(TK_LSHIFT) {
			op = OpLShift
			matched = true
		} else if p.match(TK_RSHIFT) {
			op = OpRShift
			matched = true
		}

		if !matched {
			break
		}

		right, err := p.parseAdditiveExpression()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{
			Left:  left,
			Op:    op,
			Right: right,
		}
	}

	return left, nil
}

func (p *Parser) parseAdditiveExpression() (Expression, error) {
	left, err := p.parseMultiplicativeExpression()
	if err != nil {
		return nil, err
	}

	for {
		var op BinaryOp
		matched := false
		if p.match(TK_PLUS) {
			op = OpPlus
			matched = true
		} else if p.match(TK_MINUS) {
			op = OpMinus
			matched = true
		} else if p.match(TK_CONCAT) {
			op = OpConcat
			matched = true
		}

		if !matched {
			break
		}

		right, err := p.parseMultiplicativeExpression()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{
			Left:  left,
			Op:    op,
			Right: right,
		}
	}

	return left, nil
}

func (p *Parser) parseMultiplicativeExpression() (Expression, error) {
	left, err := p.parseUnaryExpression()
	if err != nil {
		return nil, err
	}

	for {
		var op BinaryOp
		matched := false
		if p.match(TK_STAR) {
			op = OpMul
			matched = true
		} else if p.match(TK_SLASH) {
			op = OpDiv
			matched = true
		} else if p.match(TK_REM) {
			op = OpRem
			matched = true
		}

		if !matched {
			break
		}

		right, err := p.parseUnaryExpression()
		if err != nil {
			return nil, err
		}
		left = &BinaryExpr{
			Left:  left,
			Op:    op,
			Right: right,
		}
	}

	return left, nil
}

func (p *Parser) parseUnaryExpression() (Expression, error) {
	if p.match(TK_MINUS) {
		expr, err := p.parseUnaryExpression()
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{
			Op:   OpNeg,
			Expr: expr,
		}, nil
	} else if p.match(TK_PLUS) {
		return p.parseUnaryExpression()
	} else if p.match(TK_BITNOT) {
		expr, err := p.parseUnaryExpression()
		if err != nil {
			return nil, err
		}
		return &UnaryExpr{
			Op:   OpBitNot,
			Expr: expr,
		}, nil
	}

	return p.parsePostfixExpression()
}

func (p *Parser) parsePostfixExpression() (Expression, error) {
	expr, err := p.parsePrimaryExpression()
	if err != nil {
		return nil, err
	}

	// COLLATE
	if p.match(TK_COLLATE) {
		if !p.check(TK_ID) {
			return nil, p.error("expected collation name")
		}
		return &CollateExpr{
			Expr:      expr,
			Collation: Unquote(p.advance().Lexeme),
		}, nil
	}

	return expr, nil
}

func (p *Parser) parsePrimaryExpression() (Expression, error) {
	// Literals
	if p.check(TK_INTEGER) {
		tok := p.advance()
		return &LiteralExpr{
			Type:  LiteralInteger,
			Value: tok.Lexeme,
		}, nil
	} else if p.check(TK_FLOAT) {
		tok := p.advance()
		return &LiteralExpr{
			Type:  LiteralFloat,
			Value: tok.Lexeme,
		}, nil
	} else if p.check(TK_STRING) {
		tok := p.advance()
		return &LiteralExpr{
			Type:  LiteralString,
			Value: Unquote(tok.Lexeme),
		}, nil
	} else if p.check(TK_BLOB) {
		tok := p.advance()
		return &LiteralExpr{
			Type:  LiteralBlob,
			Value: tok.Lexeme,
		}, nil
	} else if p.match(TK_NULL) {
		return &LiteralExpr{
			Type:  LiteralNull,
			Value: "NULL",
		}, nil
	}

	// Variable
	if p.check(TK_VARIABLE) {
		tok := p.advance()
		return &VariableExpr{
			Name: tok.Lexeme,
		}, nil
	}

	// Identifier or function call
	if p.check(TK_ID) {
		name := Unquote(p.advance().Lexeme)

		// Function call
		if p.match(TK_LP) {
			fn := &FunctionExpr{Name: name}

			// Check for DISTINCT
			if p.match(TK_DISTINCT) {
				fn.Distinct = true
			}

			// Check for *
			if p.match(TK_STAR) {
				fn.Star = true
			} else if !p.check(TK_RP) {
				args, err := p.parseExpressionList()
				if err != nil {
					return nil, err
				}
				fn.Args = args
			}

			if !p.match(TK_RP) {
				return nil, p.error("expected ) after function arguments")
			}

			// FILTER clause
			if p.match(TK_FILTER) {
				if !p.match(TK_LP) {
					return nil, p.error("expected ( after FILTER")
				}
				if !p.match(TK_WHERE) {
					return nil, p.error("expected WHERE in FILTER")
				}
				filter, err := p.parseExpression()
				if err != nil {
					return nil, err
				}
				fn.Filter = filter
				if !p.match(TK_RP) {
					return nil, p.error("expected ) after FILTER")
				}
			}

			return fn, nil
		}

		// Column reference with optional table qualifier
		if p.match(TK_DOT) {
			if !p.check(TK_ID) {
				return nil, p.error("expected column name after .")
			}
			column := Unquote(p.advance().Lexeme)
			return &IdentExpr{
				Table: name,
				Name:  column,
			}, nil
		}

		return &IdentExpr{Name: name}, nil
	}

	// CASE expression
	if p.match(TK_CASE) {
		caseExpr := &CaseExpr{}

		// Optional case expression
		if !p.check(TK_WHEN) {
			expr, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			caseExpr.Expr = expr
		}

		// WHEN clauses
		for p.match(TK_WHEN) {
			condition, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			if !p.match(TK_THEN) {
				return nil, p.error("expected THEN after WHEN condition")
			}
			result, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			caseExpr.WhenClauses = append(caseExpr.WhenClauses, WhenClause{
				Condition: condition,
				Result:    result,
			})
		}

		// ELSE clause
		if p.match(TK_ELSE) {
			elseExpr, err := p.parseExpression()
			if err != nil {
				return nil, err
			}
			caseExpr.ElseClause = elseExpr
		}

		if !p.match(TK_END) {
			return nil, p.error("expected END after CASE")
		}

		return caseExpr, nil
	}

	// CAST expression
	if p.match(TK_CAST) {
		if !p.match(TK_LP) {
			return nil, p.error("expected ( after CAST")
		}
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if !p.match(TK_AS) {
			return nil, p.error("expected AS in CAST")
		}
		if !p.check(TK_ID) {
			return nil, p.error("expected type name")
		}
		typeName := p.parseTypeName()
		if !p.match(TK_RP) {
			return nil, p.error("expected ) after CAST")
		}
		return &CastExpr{
			Expr: expr,
			Type: typeName,
		}, nil
	}

	// Parenthesized expression or subquery
	if p.match(TK_LP) {
		if p.match(TK_SELECT) {
			sel, err := p.parseSelect()
			if err != nil {
				return nil, err
			}
			if !p.match(TK_RP) {
				return nil, p.error("expected ) after subquery")
			}
			return &SubqueryExpr{Select: sel}, nil
		}

		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		if !p.match(TK_RP) {
			return nil, p.error("expected ) after expression")
		}
		return &ParenExpr{Expr: expr}, nil
	}

	return nil, p.error("expected expression, got %s", p.peek().Type)
}

// =============================================================================
// Helper methods
// =============================================================================

func (p *Parser) parseExpressionList() ([]Expression, error) {
	exprs := make([]Expression, 0)

	for {
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}
		exprs = append(exprs, expr)

		if !p.match(TK_COMMA) {
			break
		}
	}

	return exprs, nil
}

func (p *Parser) parseOrderByList() ([]OrderingTerm, error) {
	terms := make([]OrderingTerm, 0)

	for {
		expr, err := p.parseExpression()
		if err != nil {
			return nil, err
		}

		term := OrderingTerm{
			Expr: expr,
			Asc:  true,
		}

		if p.match(TK_DESC) {
			term.Asc = false
		} else {
			p.match(TK_ASC)
		}

		terms = append(terms, term)

		if !p.match(TK_COMMA) {
			break
		}
	}

	return terms, nil
}

func (p *Parser) parseOnConflict() OnConflictClause {
	if p.match(TK_ROLLBACK) {
		return OnConflictRollback
	} else if p.match(TK_ABORT) {
		return OnConflictAbort
	} else if p.match(TK_FAIL) {
		return OnConflictFail
	} else if p.match(TK_IGNORE) {
		return OnConflictIgnore
	} else if p.match(TK_REPLACE) {
		return OnConflictReplace
	}
	return OnConflictNone
}

func (p *Parser) isJoinKeyword() bool {
	return p.check(TK_JOIN) || p.check(TK_LEFT) || p.check(TK_RIGHT) ||
		p.check(TK_INNER) || p.check(TK_OUTER) || p.check(TK_CROSS) ||
		p.check(TK_NATURAL)
}

func (p *Parser) isColumnConstraint() bool {
	return p.check(TK_CONSTRAINT) || p.check(TK_PRIMARY) || p.check(TK_NOT) ||
		p.check(TK_UNIQUE) || p.check(TK_CHECK) || p.check(TK_DEFAULT) ||
		p.check(TK_COLLATE)
}

func (p *Parser) peek() Token {
	if p.current >= len(p.tokens) {
		return Token{Type: TK_EOF}
	}
	return p.tokens[p.current]
}

func (p *Parser) peekAhead(n int) Token {
	pos := p.current + n
	if pos >= len(p.tokens) {
		return Token{Type: TK_EOF}
	}
	return p.tokens[pos]
}

func (p *Parser) advance() Token {
	if !p.isAtEnd() {
		p.current++
	}
	return p.tokens[p.current-1]
}

func (p *Parser) check(t TokenType) bool {
	if p.isAtEnd() {
		return false
	}
	return p.peek().Type == t
}

func (p *Parser) match(types ...TokenType) bool {
	for _, t := range types {
		if p.check(t) {
			p.advance()
			return true
		}
	}
	return false
}

func (p *Parser) isAtEnd() bool {
	return p.current >= len(p.tokens) || p.peek().Type == TK_EOF
}

func (p *Parser) error(format string, args ...interface{}) error {
	tok := p.peek()
	msg := fmt.Sprintf(format, args...)
	fullMsg := fmt.Sprintf("parse error at line %d, col %d: %s", tok.Line, tok.Col, msg)
	p.errors = append(p.errors, fullMsg)
	return fmt.Errorf("%s", fullMsg)
}

// ParseString is a convenience function to parse a SQL string.
func ParseString(sql string) ([]Statement, error) {
	parser := NewParser(sql)
	return parser.Parse()
}

// IntValue returns the integer value of a literal expression.
func IntValue(expr Expression) (int64, error) {
	if lit, ok := expr.(*LiteralExpr); ok && lit.Type == LiteralInteger {
		return strconv.ParseInt(lit.Value, 10, 64)
	}
	return 0, fmt.Errorf("not an integer literal")
}

// FloatValue returns the float value of a literal expression.
func FloatValue(expr Expression) (float64, error) {
	if lit, ok := expr.(*LiteralExpr); ok && (lit.Type == LiteralFloat || lit.Type == LiteralInteger) {
		return strconv.ParseFloat(lit.Value, 64)
	}
	return 0, fmt.Errorf("not a numeric literal")
}

// StringValue returns the string value of a literal expression.
func StringValue(expr Expression) (string, error) {
	if lit, ok := expr.(*LiteralExpr); ok && lit.Type == LiteralString {
		return lit.Value, nil
	}
	return "", fmt.Errorf("not a string literal")
}
