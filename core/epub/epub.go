// Package epub provides pure Go EPUB creation and manipulation.
// This replaces external calibre dependency with native Go implementation.
package epub

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/FocuswithJustin/JuniperBible/core/encoding"
)

// zipWriter interface allows for testing error paths.
type zipWriter interface {
	Create(name string) (io.Writer, error)
	CreateHeader(fh *zip.FileHeader) (io.Writer, error)
	Close() error
}

// zipWriterImpl wraps zip.Writer to implement zipWriter interface.
type zipWriterImpl struct {
	*zip.Writer
}

func (z *zipWriterImpl) Create(name string) (io.Writer, error) {
	return z.Writer.Create(name)
}

func (z *zipWriterImpl) CreateHeader(fh *zip.FileHeader) (io.Writer, error) {
	return z.Writer.CreateHeader(fh)
}

func (z *zipWriterImpl) Close() error {
	return z.Writer.Close()
}

// EPUB represents an EPUB document.
type EPUB struct {
	Metadata  BookMetadata
	Chapters  []Chapter
	cover     []byte
	coverMime string
	css       string
}

// BookMetadata contains EPUB metadata.
type BookMetadata struct {
	Title       string
	Author      string
	Language    string
	Identifier  string
	Publisher   string
	Description string
	Date        string
	Rights      string
}

// Chapter represents a chapter in the EPUB.
type Chapter struct {
	Title   string
	Content string
}

// New creates a new EPUB.
func New() *EPUB {
	return &EPUB{
		Metadata: BookMetadata{
			Language:   "en",
			Identifier: fmt.Sprintf("urn:uuid:%d", time.Now().UnixNano()),
			Date:       time.Now().Format("2006-01-02"),
		},
		Chapters: make([]Chapter, 0),
	}
}

// SetTitle sets the book title.
func (e *EPUB) SetTitle(title string) {
	e.Metadata.Title = title
}

// SetAuthor sets the book author.
func (e *EPUB) SetAuthor(author string) {
	e.Metadata.Author = author
}

// SetLanguage sets the book language.
func (e *EPUB) SetLanguage(lang string) {
	e.Metadata.Language = lang
}

// SetIdentifier sets the book identifier.
func (e *EPUB) SetIdentifier(id string) {
	e.Metadata.Identifier = id
}

// SetPublisher sets the book publisher.
func (e *EPUB) SetPublisher(publisher string) {
	e.Metadata.Publisher = publisher
}

// SetDescription sets the book description.
func (e *EPUB) SetDescription(desc string) {
	e.Metadata.Description = desc
}

// SetCover sets the cover image.
func (e *EPUB) SetCover(data []byte, mimeType string) {
	e.cover = data
	e.coverMime = mimeType
}

// SetCSS sets custom CSS.
func (e *EPUB) SetCSS(css string) {
	e.css = css
}

// AddChapter adds a chapter to the EPUB.
func (e *EPUB) AddChapter(title, content string) {
	e.Chapters = append(e.Chapters, Chapter{
		Title:   title,
		Content: content,
	})
}

// GetMetadata returns the book metadata.
func (e *EPUB) GetMetadata() BookMetadata {
	return e.Metadata
}

// Build creates the EPUB as bytes.
func (e *EPUB) Build() ([]byte, error) {
	if len(e.Chapters) == 0 {
		return nil, fmt.Errorf("EPUB must have at least one chapter")
	}

	var buf bytes.Buffer
	zw := &zipWriterImpl{zip.NewWriter(&buf)}

	err := e.build(zw)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// build is the internal build method that accepts a zipWriter interface for testing.
func (e *EPUB) build(zw zipWriter) error {
	// Add mimetype (must be first, uncompressed)
	mimetypeWriter, err := zw.CreateHeader(&zip.FileHeader{
		Name:   "mimetype",
		Method: zip.Store,
	})
	if err != nil {
		return err
	}
	if _, err := mimetypeWriter.Write([]byte("application/epub+zip")); err != nil {
		return err
	}

	// Add META-INF/container.xml
	if err := e.addContainerXML(zw); err != nil {
		return err
	}

	// Add OEBPS/content.opf
	if err := e.addContentOPF(zw); err != nil {
		return err
	}

	// Add OEBPS/toc.ncx
	if err := e.addTocNCX(zw); err != nil {
		return err
	}

	// Add OEBPS/toc.xhtml (EPUB 3 nav)
	if err := e.addTocXHTML(zw); err != nil {
		return err
	}

	// Add CSS
	if err := e.addCSS(zw); err != nil {
		return err
	}

	// Add cover if present
	if len(e.cover) > 0 {
		if err := e.addCover(zw); err != nil {
			return err
		}
	}

	// Add chapters
	for i, chapter := range e.Chapters {
		if err := e.addChapter(zw, i, chapter); err != nil {
			return err
		}
	}

	if err := zw.Close(); err != nil {
		return err
	}

	return nil
}

func (e *EPUB) addContainerXML(zw zipWriter) error {
	w, err := zw.Create("META-INF/container.xml")
	if err != nil {
		return err
	}

	container := `<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>`

	_, err = w.Write([]byte(container))
	return err
}

func (e *EPUB) addContentOPF(zw zipWriter) error {
	w, err := zw.Create("OEBPS/content.opf")
	if err != nil {
		return err
	}

	var manifestItems strings.Builder
	var spineItems strings.Builder

	// TOC
	manifestItems.WriteString(`    <item id="toc" href="toc.xhtml" media-type="application/xhtml+xml" properties="nav"/>` + "\n")
	manifestItems.WriteString(`    <item id="ncx" href="toc.ncx" media-type="application/x-dtbncx+xml"/>` + "\n")

	// CSS
	manifestItems.WriteString(`    <item id="style" href="style.css" media-type="text/css"/>` + "\n")

	// Cover
	if len(e.cover) > 0 {
		ext := "jpg"
		if strings.Contains(e.coverMime, "png") {
			ext = "png"
		}
		manifestItems.WriteString(fmt.Sprintf(`    <item id="cover-image" href="images/cover.%s" media-type="%s" properties="cover-image"/>`, ext, e.coverMime) + "\n")
	}

	// Chapters
	for i := range e.Chapters {
		id := fmt.Sprintf("chapter%d", i+1)
		manifestItems.WriteString(fmt.Sprintf(`    <item id="%s" href="text/%s.xhtml" media-type="application/xhtml+xml"/>`, id, id) + "\n")
		spineItems.WriteString(fmt.Sprintf(`    <itemref idref="%s"/>`, id) + "\n")
	}

	opf := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="BookId">
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/">
    <dc:identifier id="BookId">%s</dc:identifier>
    <dc:title>%s</dc:title>
    <dc:creator>%s</dc:creator>
    <dc:language>%s</dc:language>
    <dc:date>%s</dc:date>
    <dc:publisher>%s</dc:publisher>
    <dc:description>%s</dc:description>
    <meta property="dcterms:modified">%s</meta>
  </metadata>
  <manifest>
%s  </manifest>
  <spine toc="ncx">
%s  </spine>
</package>`,
		encoding.EscapeXML(e.Metadata.Identifier),
		encoding.EscapeXML(e.Metadata.Title),
		encoding.EscapeXML(e.Metadata.Author),
		encoding.EscapeXML(e.Metadata.Language),
		encoding.EscapeXML(e.Metadata.Date),
		encoding.EscapeXML(e.Metadata.Publisher),
		encoding.EscapeXML(e.Metadata.Description),
		time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		manifestItems.String(),
		spineItems.String(),
	)

	_, err = w.Write([]byte(opf))
	return err
}

func (e *EPUB) addTocNCX(zw zipWriter) error {
	w, err := zw.Create("OEBPS/toc.ncx")
	if err != nil {
		return err
	}

	var navPoints strings.Builder
	for i, chapter := range e.Chapters {
		navPoints.WriteString(fmt.Sprintf(`    <navPoint id="navpoint%d" playOrder="%d">
      <navLabel><text>%s</text></navLabel>
      <content src="text/chapter%d.xhtml"/>
    </navPoint>
`, i+1, i+1, encoding.EscapeXML(chapter.Title), i+1))
	}

	ncx := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1">
  <head>
    <meta name="dtb:uid" content="%s"/>
    <meta name="dtb:depth" content="1"/>
    <meta name="dtb:totalPageCount" content="0"/>
    <meta name="dtb:maxPageNumber" content="0"/>
  </head>
  <docTitle><text>%s</text></docTitle>
  <navMap>
%s  </navMap>
</ncx>`,
		encoding.EscapeXML(e.Metadata.Identifier),
		encoding.EscapeXML(e.Metadata.Title),
		navPoints.String(),
	)

	_, err = w.Write([]byte(ncx))
	return err
}

func (e *EPUB) addTocXHTML(zw zipWriter) error {
	w, err := zw.Create("OEBPS/toc.xhtml")
	if err != nil {
		return err
	}

	var tocItems strings.Builder
	for i, chapter := range e.Chapters {
		tocItems.WriteString(fmt.Sprintf(`      <li><a href="text/chapter%d.xhtml">%s</a></li>
`, i+1, encoding.EscapeXML(chapter.Title)))
	}

	toc := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops">
<head>
  <title>Table of Contents</title>
  <link rel="stylesheet" type="text/css" href="style.css"/>
</head>
<body>
  <nav epub:type="toc" id="toc">
    <h1>Table of Contents</h1>
    <ol>
%s    </ol>
  </nav>
</body>
</html>`, tocItems.String())

	_, err = w.Write([]byte(toc))
	return err
}

func (e *EPUB) addCSS(zw zipWriter) error {
	w, err := zw.Create("OEBPS/style.css")
	if err != nil {
		return err
	}

	css := e.css
	if css == "" {
		css = `body {
  font-family: serif;
  margin: 1em;
  line-height: 1.6;
}
h1, h2, h3 {
  font-family: sans-serif;
}
p {
  text-indent: 1.5em;
  margin: 0.5em 0;
}
`
	}

	_, err = w.Write([]byte(css))
	return err
}

func (e *EPUB) addCover(zw zipWriter) error {
	ext := "jpg"
	if strings.Contains(e.coverMime, "png") {
		ext = "png"
	}

	w, err := zw.Create(fmt.Sprintf("OEBPS/images/cover.%s", ext))
	if err != nil {
		return err
	}

	_, err = w.Write(e.cover)
	return err
}

func (e *EPUB) addChapter(zw zipWriter, index int, chapter Chapter) error {
	w, err := zw.Create(fmt.Sprintf("OEBPS/text/chapter%d.xhtml", index+1))
	if err != nil {
		return err
	}

	xhtml := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml">
<head>
  <title>%s</title>
  <link rel="stylesheet" type="text/css" href="../style.css"/>
</head>
<body>
  <h1>%s</h1>
  %s
</body>
</html>`,
		encoding.EscapeXML(chapter.Title),
		encoding.EscapeXML(chapter.Title),
		chapter.Content,
	)

	_, err = w.Write([]byte(xhtml))
	return err
}

// Parse parses an existing EPUB from bytes.
func Parse(data []byte) (*EPUB, error) {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("invalid EPUB archive: %w", err)
	}

	epub := New()

	// Find and parse content.opf
	for _, f := range r.File {
		if strings.HasSuffix(f.Name, "content.opf") {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}

			content, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return nil, fmt.Errorf("failed to read content.opf: %w", err)
			}

			epub.parseOPF(string(content))
			break
		}
	}

	return epub, nil
}

func (e *EPUB) parseOPF(content string) {
	// Simple metadata extraction
	if title := extractTag(content, "dc:title"); title != "" {
		e.Metadata.Title = title
	}
	if author := extractTag(content, "dc:creator"); author != "" {
		e.Metadata.Author = author
	}
	if lang := extractTag(content, "dc:language"); lang != "" {
		e.Metadata.Language = lang
	}
	if id := extractTag(content, "dc:identifier"); id != "" {
		e.Metadata.Identifier = id
	}
	if pub := extractTag(content, "dc:publisher"); pub != "" {
		e.Metadata.Publisher = pub
	}
	if desc := extractTag(content, "dc:description"); desc != "" {
		e.Metadata.Description = desc
	}
}

func extractTag(content, tag string) string {
	startTag := "<" + tag
	endTag := "</" + tag + ">"

	start := strings.Index(content, startTag)
	if start == -1 {
		return ""
	}

	// Find the closing > of the start tag
	tagEnd := strings.Index(content[start:], ">")
	if tagEnd == -1 {
		return ""
	}
	contentStart := start + tagEnd + 1

	end := strings.Index(content[contentStart:], endTag)
	if end == -1 {
		return ""
	}

	return strings.TrimSpace(content[contentStart : contentStart+end])
}
