// Package esword provides parsers for e-Sword Bible module format.
package esword

import (
	"database/sql"
	"fmt"
	"sort"
	"strings"
)

// DictionaryParser parses e-Sword dictionary files (.dctx).
//
// The .dctx format is a SQLite database with a Dictionary table:
//   - Topic: TEXT (dictionary key/headword)
//   - Definition: TEXT (definition content)
type DictionaryParser struct {
	db       *sql.DB
	filePath string
	metadata *DictionaryMetadata
}

// DictionaryMetadata contains information about an e-Sword dictionary.
type DictionaryMetadata struct {
	Title        string
	Abbreviation string
	Information  string
	Version      string
}

// DictionaryEntry represents a dictionary entry from e-Sword.
type DictionaryEntry struct {
	Topic      string
	Definition string
}

// NewDictionaryParser creates a new e-Sword dictionary parser.
func NewDictionaryParser(filePath string) (*DictionaryParser, error) {
	db, err := sql.Open("sqlite3", filePath+"?mode=ro")
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	parser := &DictionaryParser{
		db:       db,
		filePath: filePath,
	}

	if err := parser.loadMetadata(); err != nil {
		db.Close()
		return nil, fmt.Errorf("loading metadata: %w", err)
	}

	return parser, nil
}

// Close closes the database connection.
func (p *DictionaryParser) Close() error {
	if p.db != nil {
		return p.db.Close()
	}
	return nil
}

// loadMetadata loads dictionary metadata from the Details table.
func (p *DictionaryParser) loadMetadata() error {
	p.metadata = &DictionaryMetadata{}

	var count int
	err := p.db.QueryRow("SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='Details'").Scan(&count)
	if err != nil || count == 0 {
		return nil
	}

	rows, err := p.db.Query("SELECT Description, Abbreviation, Information, Version FROM Details LIMIT 1")
	if err != nil {
		return nil
	}
	defer rows.Close()

	if rows.Next() {
		var title, abbrev, info, version sql.NullString
		if err := rows.Scan(&title, &abbrev, &info, &version); err != nil {
			return nil
		}
		p.metadata.Title = title.String
		p.metadata.Abbreviation = abbrev.String
		p.metadata.Information = info.String
		p.metadata.Version = version.String
	}

	return nil
}

// GetMetadata returns the dictionary metadata.
func (p *DictionaryParser) GetMetadata() *DictionaryMetadata {
	return p.metadata
}

// GetEntry retrieves a dictionary entry by topic.
func (p *DictionaryParser) GetEntry(topic string) (*DictionaryEntry, error) {
	row := p.db.QueryRow(
		"SELECT Topic, Definition FROM Dictionary WHERE Topic = ? COLLATE NOCASE LIMIT 1",
		topic,
	)

	entry := &DictionaryEntry{}
	var definition sql.NullString
	if err := row.Scan(&entry.Topic, &definition); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("entry not found: %s", topic)
		}
		return nil, err
	}

	entry.Definition = cleanESwordText(definition.String)
	return entry, nil
}

// GetAllTopics returns all topics in the dictionary.
func (p *DictionaryParser) GetAllTopics() ([]string, error) {
	rows, err := p.db.Query("SELECT Topic FROM Dictionary ORDER BY Topic")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var topics []string
	for rows.Next() {
		var topic string
		if err := rows.Scan(&topic); err != nil {
			return nil, err
		}
		topics = append(topics, topic)
	}

	return topics, rows.Err()
}

// GetAllEntries retrieves all dictionary entries.
func (p *DictionaryParser) GetAllEntries() ([]*DictionaryEntry, error) {
	rows, err := p.db.Query("SELECT Topic, Definition FROM Dictionary ORDER BY Topic")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*DictionaryEntry
	for rows.Next() {
		entry := &DictionaryEntry{}
		var definition sql.NullString
		if err := rows.Scan(&entry.Topic, &definition); err != nil {
			return nil, err
		}
		entry.Definition = cleanESwordText(definition.String)
		entries = append(entries, entry)
	}

	return entries, rows.Err()
}

// SearchTopics searches for topics matching a pattern.
func (p *DictionaryParser) SearchTopics(pattern string) ([]string, error) {
	rows, err := p.db.Query(
		"SELECT Topic FROM Dictionary WHERE Topic LIKE ? ORDER BY Topic",
		"%"+pattern+"%",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var topics []string
	for rows.Next() {
		var topic string
		if err := rows.Scan(&topic); err != nil {
			return nil, err
		}
		topics = append(topics, topic)
	}

	return topics, rows.Err()
}

// GetTopicsByLetter returns all topics starting with a specific letter.
func (p *DictionaryParser) GetTopicsByLetter(letter string) ([]string, error) {
	rows, err := p.db.Query(
		"SELECT Topic FROM Dictionary WHERE Topic LIKE ? ORDER BY Topic",
		letter+"%",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var topics []string
	for rows.Next() {
		var topic string
		if err := rows.Scan(&topic); err != nil {
			return nil, err
		}
		topics = append(topics, topic)
	}

	return topics, rows.Err()
}

// GetLetterIndex returns a list of letters that have entries.
func (p *DictionaryParser) GetLetterIndex() ([]string, error) {
	rows, err := p.db.Query("SELECT DISTINCT UPPER(SUBSTR(Topic, 1, 1)) as Letter FROM Dictionary ORDER BY Letter")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var letters []string
	for rows.Next() {
		var letter string
		if err := rows.Scan(&letter); err != nil {
			return nil, err
		}
		letters = append(letters, letter)
	}

	// Sort letters properly
	sort.Strings(letters)

	return letters, rows.Err()
}

// GetEntryCount returns the total number of entries.
func (p *DictionaryParser) GetEntryCount() (int, error) {
	var count int
	err := p.db.QueryRow("SELECT COUNT(*) FROM Dictionary").Scan(&count)
	return count, err
}

// IsStrongsLexicon checks if this is a Strong's lexicon.
func (p *DictionaryParser) IsStrongsLexicon() bool {
	title := strings.ToLower(p.metadata.Title)
	return strings.Contains(title, "strong") ||
		strings.Contains(title, "strongs")
}

// GetStrongsEntry retrieves a Strong's lexicon entry by number.
func (p *DictionaryParser) GetStrongsEntry(strongsNum string) (*DictionaryEntry, error) {
	// Normalize Strong's number (e.g., "H430" -> various formats)
	strongsNum = strings.ToUpper(strings.TrimSpace(strongsNum))

	// Try exact match first
	entry, err := p.GetEntry(strongsNum)
	if err == nil {
		return entry, nil
	}

	// Try without prefix
	if len(strongsNum) > 1 && (strongsNum[0] == 'H' || strongsNum[0] == 'G') {
		entry, err = p.GetEntry(strongsNum[1:])
		if err == nil {
			return entry, nil
		}
	}

	// Try with leading zeros
	if len(strongsNum) > 1 {
		prefix := ""
		num := strongsNum
		if strongsNum[0] == 'H' || strongsNum[0] == 'G' {
			prefix = string(strongsNum[0])
			num = strongsNum[1:]
		}
		for len(num) < 5 {
			num = "0" + num
		}
		entry, err = p.GetEntry(prefix + num)
		if err == nil {
			return entry, nil
		}
	}

	return nil, fmt.Errorf("Strong's entry not found: %s", strongsNum)
}
