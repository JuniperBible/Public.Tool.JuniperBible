// dictionary.go implements e-Sword Dictionary (.dctx) parser.
// Dictionary files are SQLite databases with Dictionary and Details tables.
//
// Table: Dictionary
// - Topic TEXT
// - Definition TEXT (may contain RTF formatting)
//
// Table: Details
// - Title TEXT
// - Abbreviation TEXT
// - Information TEXT
// - Version INTEGER
package esword

import (
	"database/sql"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/FocuswithJustin/JuniperBible/core/sqlite"
)

// DictionaryDetails contains metadata about a dictionary module.
type DictionaryDetails struct {
	Title        string
	Abbreviation string
	Information  string
	Version      int
}

// DictionaryModuleInfo contains summary information about a dictionary.
type DictionaryModuleInfo struct {
	Title      string
	EntryCount int
}

// DictionaryEntry represents a dictionary entry.
type DictionaryEntry struct {
	Topic      string `json:"topic"`
	Definition string `json:"definition"`
}

// DictionaryParser handles parsing of e-Sword dictionary files.
type DictionaryParser struct {
	db      *sql.DB
	dbPath  string
	details *DictionaryDetails
	entries map[string]*DictionaryEntry
}

// NewDictionaryParser creates a new parser for an e-Sword dictionary file.
func NewDictionaryParser(path string) (*DictionaryParser, error) {
	db, err := sqlite.OpenReadOnly(path)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	parser := &DictionaryParser{
		db:      db,
		dbPath:  path,
		entries: make(map[string]*DictionaryEntry),
	}

	// Load details
	if err := parser.loadDetails(); err != nil {
		db.Close()
		return nil, err
	}

	// Load entries into cache
	if err := parser.loadEntries(); err != nil {
		db.Close()
		return nil, err
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

// loadDetails loads the Details table.
func (p *DictionaryParser) loadDetails() error {
	row := p.db.QueryRow(`SELECT Title, Abbreviation, Information, Version FROM Details LIMIT 1`)

	var d DictionaryDetails
	var title, abbrev, info sql.NullString
	var version sql.NullInt64
	if err := row.Scan(&title, &abbrev, &info, &version); err != nil {
		if err == sql.ErrNoRows {
			// No details table or empty
			p.details = &DictionaryDetails{}
			return nil
		}
		return fmt.Errorf("reading details: %w", err)
	}

	d.Title = title.String
	d.Abbreviation = abbrev.String
	d.Information = info.String
	d.Version = int(version.Int64)
	p.details = &d
	return nil
}

// loadEntries loads all dictionary entries into the cache.
func (p *DictionaryParser) loadEntries() error {
	rows, err := p.db.Query(`SELECT Topic, Definition FROM Dictionary`)
	if err != nil {
		return fmt.Errorf("querying dictionary: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var topic, definition string
		if err := rows.Scan(&topic, &definition); err != nil {
			return fmt.Errorf("scanning row: %w", err)
		}

		p.entries[topic] = &DictionaryEntry{
			Topic:      topic,
			Definition: definition,
		}
	}

	return rows.Err()
}

// GetEntry retrieves a dictionary entry by topic.
func (p *DictionaryParser) GetEntry(topic string) (*DictionaryEntry, error) {
	entry, ok := p.entries[topic]
	if !ok {
		return nil, fmt.Errorf("entry not found: %s", topic)
	}
	return entry, nil
}

// ListTopics returns all dictionary topics.
func (p *DictionaryParser) ListTopics() []string {
	topics := make([]string, 0, len(p.entries))
	for topic := range p.entries {
		topics = append(topics, topic)
	}
	return topics
}

// ListTopicsSorted returns all dictionary topics sorted alphabetically.
func (p *DictionaryParser) ListTopicsSorted() []string {
	topics := p.ListTopics()
	sort.Strings(topics)
	return topics
}

// SearchTopics returns topics matching the given prefix (case-insensitive).
func (p *DictionaryParser) SearchTopics(prefix string) []string {
	prefixLower := strings.ToLower(prefix)
	var matches []string
	for topic := range p.entries {
		if strings.HasPrefix(strings.ToLower(topic), prefixLower) {
			matches = append(matches, topic)
		}
	}
	return matches
}

// SearchDefinitions returns topics whose definitions contain the query.
func (p *DictionaryParser) SearchDefinitions(query string) []string {
	queryLower := strings.ToLower(query)
	var matches []string
	for topic, entry := range p.entries {
		if strings.Contains(strings.ToLower(entry.Definition), queryLower) {
			matches = append(matches, topic)
		}
	}
	return matches
}

// ModuleInfo returns summary information about the dictionary.
func (p *DictionaryParser) ModuleInfo() DictionaryModuleInfo {
	title := ""
	if p.details != nil {
		title = p.details.Title
	}
	return DictionaryModuleInfo{
		Title:      title,
		EntryCount: len(p.entries),
	}
}

// EntryCount returns the number of entries in the dictionary.
func (p *DictionaryParser) EntryCount() int {
	return len(p.entries)
}

// IsDictionaryFile returns true if the filename is an e-Sword dictionary file.
func IsDictionaryFile(filename string) bool {
	ext := strings.ToLower(filename)
	return strings.HasSuffix(ext, ".dctx")
}

// cleanDictionaryText removes RTF formatting from dictionary text.
func cleanDictionaryText(text string) string {
	// Remove RTF control words like \rtf1, \b, \par, etc. but keep content
	rtfControlWord := regexp.MustCompile(`\\[a-z]+\d*\s?`)
	cleaned := rtfControlWord.ReplaceAllString(text, " ")

	// Remove braces
	cleaned = strings.ReplaceAll(cleaned, "{", "")
	cleaned = strings.ReplaceAll(cleaned, "}", "")

	// Normalize multiple spaces to single space
	multiSpace := regexp.MustCompile(`\s+`)
	cleaned = multiSpace.ReplaceAllString(cleaned, " ")

	return strings.TrimSpace(cleaned)
}
