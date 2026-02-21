// Package gobible provides pure Go GoBible JAR creation.
// This replaces external GoBible Creator (Java) dependency with native Go implementation.
//
// GoBible is a J2ME Bible application format. The JAR files contain:
// - META-INF/MANIFEST.MF
// - Index file with book/chapter metadata
// - Book data files with verse content
package gobible

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/JuniperBible/Public.Tool.JuniperBible/core/encoding"
)

// zipWriter interface for testing.
type zipWriter interface {
	Create(name string) (io.Writer, error)
	Close() error
}

// Collection represents a GoBible collection (Bible translation).
type Collection struct {
	Name  string
	Info  string
	Books []*Book
}

// Book represents a book in the collection.
type Book struct {
	Name     string
	ID       string
	Chapters []*Chapter
}

// Chapter represents a chapter in a book.
type Chapter struct {
	Number int
	Verses []string
}

// NewCollection creates a new GoBible collection.
func NewCollection() *Collection {
	return &Collection{
		Books: make([]*Book, 0),
	}
}

// SetName sets the collection name.
func (c *Collection) SetName(name string) {
	c.Name = name
}

// SetInfo sets the collection info/description.
func (c *Collection) SetInfo(info string) {
	c.Info = info
}

// AddBook adds a book to the collection.
func (c *Collection) AddBook(name, id string) {
	c.Books = append(c.Books, &Book{
		Name:     name,
		ID:       id,
		Chapters: make([]*Chapter, 0),
	})
}

// GetBookByID returns a book by its ID.
func (c *Collection) GetBookByID(id string) *Book {
	for _, book := range c.Books {
		if book.ID == id {
			return book
		}
	}
	return nil
}

// AddChapter adds a chapter to a book.
// If the book does not exist, this is a no-op (silently ignored).
// Use GetBookByID first to verify the book exists if error handling is needed.
func (c *Collection) AddChapter(bookID string, number int, verses []string) {
	book := c.GetBookByID(bookID)
	if book == nil {
		return
	}
	book.Chapters = append(book.Chapters, &Chapter{
		Number: number,
		Verses: verses,
	})
}

// GetChapter returns a chapter by number.
func (b *Book) GetChapter(number int) *Chapter {
	for _, ch := range b.Chapters {
		if ch.Number == number {
			return ch
		}
	}
	return nil
}

// Validate validates the collection.
func (c *Collection) Validate() error {
	if c.Name == "" {
		return fmt.Errorf("collection name is required")
	}
	if len(c.Books) == 0 {
		return fmt.Errorf("collection must have at least one book")
	}
	for _, book := range c.Books {
		if len(book.Chapters) == 0 {
			return fmt.Errorf("book %s must have at least one chapter", book.ID)
		}
	}
	return nil
}

// BuildJAR creates the GoBible JAR file.
func (c *Collection) BuildJAR() ([]byte, error) {
	return c.BuildJARInternal(nil)
}

// BuildJARInternal creates the GoBible JAR file, optionally using a provided zipWriter for testing.
// If zw is nil, creates a new zip.Writer internally.
func (c *Collection) BuildJARInternal(zw zipWriter) ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	var needClose bool

	if zw == nil {
		zw = zip.NewWriter(&buf)
		needClose = true
	}

	if err := c.buildJARWithWriter(zw); err != nil {
		return nil, err
	}

	if needClose {
		return buf.Bytes(), nil
	}
	return nil, nil
}

// buildJARWithWriter is a helper that builds the JAR using the provided zipWriter.
// This allows testing with mock writers.
func (c *Collection) buildJARWithWriter(zw zipWriter) error {
	// Add META-INF/MANIFEST.MF
	if err := c.addManifest(zw); err != nil {
		return err
	}

	// Add index file
	if err := c.addIndex(zw); err != nil {
		return err
	}

	// Add book data files
	for i, book := range c.Books {
		if err := c.addBookData(zw, i, book); err != nil {
			return err
		}
	}

	if err := zw.Close(); err != nil {
		return err
	}

	return nil
}

func (c *Collection) addManifest(zw zipWriter) error {
	w, err := zw.Create("META-INF/MANIFEST.MF")
	if err != nil {
		return err
	}

	manifest := fmt.Sprintf(`Manifest-Version: 1.0
MIDlet-Name: %s
MIDlet-Version: 1.0
MIDlet-Vendor: GoBible
MIDlet-1: %s, /icon.png, goBible.GoBible
MicroEdition-Profile: MIDP-2.0
MicroEdition-Configuration: CLDC-1.1
`, encoding.EscapeManifest(c.Name), encoding.EscapeManifest(c.Name))

	_, err = w.Write([]byte(manifest))
	return err
}

func (c *Collection) addIndex(zw zipWriter) error {
	w, err := zw.Create("GoBible/Index")
	if err != nil {
		return err
	}

	var buf bytes.Buffer

	// Write collection name (UTF-8 string)
	writeUTF8String(&buf, c.Name)

	// Write info
	writeUTF8String(&buf, c.Info)

	// Write number of books
	binary.Write(&buf, binary.BigEndian, int16(len(c.Books)))

	// Write book info
	for _, book := range c.Books {
		writeUTF8String(&buf, book.Name)
		writeUTF8String(&buf, book.ID)
		binary.Write(&buf, binary.BigEndian, int16(len(book.Chapters)))
	}

	_, err = w.Write(buf.Bytes())
	return err
}

func (c *Collection) addBookData(zw zipWriter, bookIndex int, book *Book) error {
	// Create book data file
	filename := fmt.Sprintf("GoBible/Book%d", bookIndex)
	w, err := zw.Create(filename)
	if err != nil {
		return err
	}

	var buf bytes.Buffer

	// Write book name
	writeUTF8String(&buf, book.Name)

	// Write number of chapters
	binary.Write(&buf, binary.BigEndian, int16(len(book.Chapters)))

	// Write each chapter
	for _, chapter := range book.Chapters {
		// Write chapter number
		binary.Write(&buf, binary.BigEndian, int16(chapter.Number))

		// Write number of verses
		binary.Write(&buf, binary.BigEndian, int16(len(chapter.Verses)))

		// Write each verse
		for _, verse := range chapter.Verses {
			writeUTF8String(&buf, verse)
		}
	}

	_, err = w.Write(buf.Bytes())
	return err
}

func writeUTF8String(buf *bytes.Buffer, s string) {
	data := []byte(s)
	length := utf8.RuneCountInString(s)
	binary.Write(buf, binary.BigEndian, int16(length))
	buf.Write(data)
}

// BuildJAD creates the JAD (Java Application Descriptor) file.
func (c *Collection) BuildJAD(jarSize int) string {
	return fmt.Sprintf(`MIDlet-Name: %s
MIDlet-Version: 1.0
MIDlet-Vendor: GoBible
MIDlet-1: %s, /icon.png, goBible.GoBible
MIDlet-Jar-URL: %s.jar
MIDlet-Jar-Size: %d
MicroEdition-Profile: MIDP-2.0
MicroEdition-Configuration: CLDC-1.1
MIDlet-Description: %s
`,
		encoding.EscapeManifest(c.Name),
		encoding.EscapeManifest(c.Name),
		encoding.EscapeManifest(c.Name),
		jarSize,
		encoding.EscapeManifest(c.Info),
	)
}
