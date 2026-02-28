// Package sword provides parsers for SWORD Bible module format.
package sword

// ModuleType represents the type of SWORD module.
type ModuleType string

const (
	ModuleTypeBible      ModuleType = "Bible"
	ModuleTypeCommentary ModuleType = "Commentary"
	ModuleTypeDictionary ModuleType = "Dictionary"
	ModuleTypeGenBook    ModuleType = "GenBook"
)

// ModuleDriver represents the SWORD module driver type.
type ModuleDriver string

const (
	DriverZText      ModuleDriver = "zText"
	DriverZText4     ModuleDriver = "zText4"
	DriverRawText    ModuleDriver = "RawText"
	DriverRawText4   ModuleDriver = "RawText4"
	DriverZCom       ModuleDriver = "zCom"
	DriverZCom4      ModuleDriver = "zCom4"
	DriverRawCom     ModuleDriver = "RawCom"
	DriverRawCom4    ModuleDriver = "RawCom4"
	DriverZLD        ModuleDriver = "zLD"
	DriverRawLD      ModuleDriver = "RawLD"
	DriverRawLD4     ModuleDriver = "RawLD4"
	DriverRawGenBook ModuleDriver = "RawGenBook"
)

// SourceType represents the markup format of the module content.
type SourceType string

const (
	SourceOSIS  SourceType = "OSIS"
	SourceThML  SourceType = "ThML"
	SourceGBF   SourceType = "GBF"
	SourceTEI   SourceType = "TEI"
	SourcePlain SourceType = "Plain"
)

// Module represents a SWORD module's metadata.
type Module struct {
	// Identifier is the module's unique ID (from .conf filename and [Section])
	ID string

	// Basic metadata
	Title       string
	Description string
	About       string

	// Classification
	ModuleType ModuleType
	Driver     ModuleDriver
	SourceType SourceType
	Language   string

	// Versification system (e.g., KJV, NRSV, Vulgate)
	Versification string

	// Features (e.g., StrongsNumbers, Morphology, Headings)
	Features []string

	// Global option filters
	GlobalOptionFilters []string

	// File paths
	DataPath string // Relative path to module data
	ConfPath string // Path to .conf file

	// Compression
	CompressType string
	BlockType    string

	// Encoding
	Encoding string

	// Version info
	Version          string
	SwordVersionDate string

	// Copyright and distribution
	Copyright           string
	DistributionLicense string

	// Additional metadata
	Category       string
	LCSH           string // Library of Congress Subject Heading
	MinimumVersion string
}

// Reference represents a scripture reference.
type Reference struct {
	Book    string
	Chapter int
	Verse   int // 0 = chapter-level, -1 = book-level
}

// Verse represents a single verse with its text and annotations.
type Verse struct {
	Reference  Reference
	Text       string
	Strongs    []string // Strong's numbers (e.g., H430, G2316)
	Morphology []string // Morphology codes
}

// Chapter represents a chapter containing verses.
type Chapter struct {
	Number int
	Verses []Verse
}

// Book represents a book containing chapters.
type Book struct {
	ID        string // e.g., "Gen", "Matt"
	Name      string // e.g., "Genesis", "Matthew"
	Abbrev    string
	Testament string // "OT" or "NT"
	Chapters  []Chapter
}

// Parser is the interface for SWORD module parsers.
type Parser interface {
	// ParseConfig parses a .conf file and returns module metadata.
	ParseConfig(confPath string) (*Module, error)

	// GetVerse returns a single verse's text.
	GetVerse(ref Reference) (*Verse, error)

	// GetChapter returns all verses in a chapter.
	GetChapter(book string, chapter int) (*Chapter, error)

	// GetBook returns all chapters in a book.
	GetBook(book string) (*Book, error)

	// GetAllBooks returns all books in the module.
	GetAllBooks() ([]*Book, error)
}
