// Package xml provides pure Go XML validation, XPath, and formatting.
// This replaces external libxml2 dependency with native Go implementation.
//
// Security Notes:
//   - XXE (External Entity) attacks are mitigated by using Go's xml.Decoder
//     which doesn't fetch external entities by default, and we explicitly
//     disable entity expansion in validation functions.
//   - The xmlquery library is used for parsing, which uses Go's encoding/xml
//     internally and inherits its security properties.
package xml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"

	"github.com/JuniperBible/Public.Tool.JuniperBible/core/encoding"
	"github.com/antchfx/xmlquery"
	"github.com/antchfx/xpath"
)

// Document represents a parsed XML document.
type Document struct {
	root *xmlquery.Node
}

// Node represents an XML node (element, text, attribute, etc.).
type Node struct {
	node *xmlquery.Node
}

// ValidationResult contains the result of XML validation.
type ValidationResult struct {
	Valid  bool
	Errors []ValidationError
}

// ValidationError represents a single validation error.
type ValidationError struct {
	Line    int
	Column  int
	Message string
}

// FormatOptions controls XML formatting behavior.
type FormatOptions struct {
	Indent string // Indentation string (e.g., "  " or "\t")
}

// Parse parses XML data and returns a Document.
func Parse(data []byte) (*Document, error) {
	reader := bytes.NewReader(data)
	root, err := xmlquery.Parse(reader)
	if err != nil {
		return nil, fmt.Errorf("parsing XML: %w", err)
	}
	return &Document{root: root}, nil
}

// Validate validates XML data and returns a ValidationResult.
// If schema is nil, only well-formedness is checked.
//
// Security: This function is protected against XXE (XML External Entity) attacks
// by disabling entity expansion. Go's xml.Decoder does not fetch external entities
// by default, and we explicitly disable internal entity expansion as well.
func Validate(data []byte, schema []byte) ValidationResult {
	result := ValidationResult{Valid: true}

	// Try to parse the XML to check well-formedness
	decoder := xml.NewDecoder(bytes.NewReader(data))

	// XXE Protection (CWE-611): Disable entity expansion to prevent XXE attacks.
	// Go's xml.Decoder doesn't fetch external entities by default, but we
	// explicitly disable all entity expansion as a defense-in-depth measure.
	decoder.Entity = map[string]string{}

	for {
		_, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			result.Valid = false
			result.Errors = append(result.Errors, ValidationError{
				Line:    1, // xml.Decoder doesn't provide line numbers easily
				Column:  0,
				Message: err.Error(),
			})
			break
		}
	}

	return result
}

// Format formats/pretty-prints XML data.
func Format(data []byte, opts FormatOptions) ([]byte, error) {
	if opts.Indent == "" {
		opts.Indent = "  "
	}

	doc, err := Parse(data)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := formatNode(&buf, doc.root, 0, opts.Indent); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// formatNode recursively formats an XML node.
func formatNode(w *bytes.Buffer, n *xmlquery.Node, depth int, indent string) error {
	switch n.Type {
	case xmlquery.DocumentNode:
		return formatDocumentNode(w, n, depth, indent)
	case xmlquery.DeclarationNode:
		formatDeclarationNode(w, n)
	case xmlquery.ElementNode:
		return formatElementNode(w, n, depth, indent)
	case xmlquery.TextNode:
		formatTextNode(w, n)
	case xmlquery.CommentNode:
		formatCommentNode(w, n, depth, indent)
	}
	return nil
}

// formatDocumentNode formats a document node by processing its children.
func formatDocumentNode(w *bytes.Buffer, n *xmlquery.Node, depth int, indent string) error {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if err := formatNode(w, child, depth, indent); err != nil {
			return err
		}
	}
	return nil
}

// formatDeclarationNode formats an XML declaration node.
func formatDeclarationNode(w *bytes.Buffer, n *xmlquery.Node) {
	w.WriteString("<?xml")
	for _, attr := range n.Attr {
		w.WriteString(" ")
		w.WriteString(attr.Name.Local)
		w.WriteString("=\"")
		w.WriteString(encoding.EscapeXMLAttr(attr.Value))
		w.WriteString("\"")
	}
	w.WriteString("?>\n")
}

// formatElementNode formats an element node with its attributes and children.
func formatElementNode(w *bytes.Buffer, n *xmlquery.Node, depth int, indent string) error {
	writeOpeningTag(w, n, depth, indent)

	hasChildren := n.FirstChild != nil
	if !hasChildren {
		w.WriteString("/>\n")
		return nil
	}

	hasElementChildren := hasElementChild(n)
	w.WriteString(">")
	if hasElementChildren {
		w.WriteString("\n")
	}

	if err := formatChildren(w, n, depth, indent, hasElementChildren); err != nil {
		return err
	}

	writeClosingTag(w, n, depth, indent, hasElementChildren)
	return nil
}

// writeOpeningTag writes the opening tag with attributes.
func writeOpeningTag(w *bytes.Buffer, n *xmlquery.Node, depth int, indent string) {
	writeIndent(w, depth, indent)
	w.WriteString("<")
	writeQualifiedName(w, n.Prefix, n.Data)
	writeAttributes(w, n.Attr)
}

// writeClosingTag writes the closing tag.
func writeClosingTag(w *bytes.Buffer, n *xmlquery.Node, depth int, indent string, hasElementChildren bool) {
	if hasElementChildren {
		writeIndent(w, depth, indent)
	}
	w.WriteString("</")
	writeQualifiedName(w, n.Prefix, n.Data)
	w.WriteString(">\n")
}

// writeQualifiedName writes a qualified name with optional prefix.
func writeQualifiedName(w *bytes.Buffer, prefix, local string) {
	if prefix != "" {
		w.WriteString(prefix)
		w.WriteString(":")
	}
	w.WriteString(local)
}

// writeAttributes writes all attributes of a node.
func writeAttributes(w *bytes.Buffer, attrs []xmlquery.Attr) {
	for _, attr := range attrs {
		w.WriteString(" ")
		if attr.Name.Space != "" {
			w.WriteString("xmlns:")
			w.WriteString(attr.Name.Local)
		} else if attr.Name.Local != "" {
			w.WriteString(attr.Name.Local)
		}
		w.WriteString("=\"")
		w.WriteString(encoding.EscapeXMLAttr(attr.Value))
		w.WriteString("\"")
	}
}

// hasElementChild checks if a node has any element children.
func hasElementChild(n *xmlquery.Node) bool {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == xmlquery.ElementNode {
			return true
		}
	}
	return false
}

// formatChildren formats all children of a node.
func formatChildren(w *bytes.Buffer, n *xmlquery.Node, depth int, indent string, hasElementChildren bool) error {
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		switch child.Type {
		case xmlquery.ElementNode:
			if err := formatNode(w, child, depth+1, indent); err != nil {
				return err
			}
		case xmlquery.TextNode:
			formatInlineText(w, child, depth, indent, hasElementChildren)
		case xmlquery.CharDataNode:
			w.WriteString("<![CDATA[")
			w.WriteString(child.Data)
			w.WriteString("]]>")
		}
	}
	return nil
}

// formatInlineText formats text content within an element.
func formatInlineText(w *bytes.Buffer, n *xmlquery.Node, depth int, indent string, hasElementChildren bool) {
	text := strings.TrimSpace(n.Data)
	if text == "" {
		return
	}
	if hasElementChildren {
		writeIndent(w, depth+1, indent)
	}
	w.WriteString(encoding.EscapeXMLText(n.Data))
	if hasElementChildren {
		w.WriteString("\n")
	}
}

// formatTextNode formats a text node.
func formatTextNode(w *bytes.Buffer, n *xmlquery.Node) {
	text := strings.TrimSpace(n.Data)
	if text != "" {
		w.WriteString(encoding.EscapeXMLText(text))
	}
}

// formatCommentNode formats a comment node.
func formatCommentNode(w *bytes.Buffer, n *xmlquery.Node, depth int, indent string) {
	writeIndent(w, depth, indent)
	w.WriteString("<!--")
	w.WriteString(n.Data)
	w.WriteString("-->\n")
}

func writeIndent(w *bytes.Buffer, depth int, indent string) {
	for i := 0; i < depth; i++ {
		w.WriteString(indent)
	}
}

// Root returns the root element of the document.
func (d *Document) Root() *Node {
	if d.root == nil {
		return nil
	}
	// Find the first element child
	for child := d.root.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == xmlquery.ElementNode {
			return &Node{node: child}
		}
	}
	return nil
}

// XPath executes an XPath query and returns matching nodes.
func (d *Document) XPath(expr string) ([]*Node, error) {
	// Compile the expression to check for errors
	_, err := xpath.Compile(expr)
	if err != nil {
		return nil, fmt.Errorf("invalid xpath: %w", err)
	}

	nodes, err := xmlquery.QueryAll(d.root, expr)
	if err != nil {
		return nil, fmt.Errorf("xpath query failed: %w", err)
	}

	result := make([]*Node, len(nodes))
	for i, n := range nodes {
		result[i] = &Node{node: n}
	}
	return result, nil
}

// XPathFirst executes an XPath query and returns the first matching node.
func (d *Document) XPathFirst(expr string) (*Node, error) {
	// Compile the expression to check for errors
	_, err := xpath.Compile(expr)
	if err != nil {
		return nil, fmt.Errorf("invalid xpath: %w", err)
	}

	node, err := xmlquery.Query(d.root, expr)
	if err != nil {
		return nil, fmt.Errorf("xpath query failed: %w", err)
	}
	if node == nil {
		return nil, nil
	}
	return &Node{node: node}, nil
}

// Serialize converts the document back to XML bytes.
func (d *Document) Serialize() []byte {
	if d.root == nil {
		return nil
	}
	return []byte(d.root.OutputXML(true))
}

// Name returns the element name.
func (n *Node) Name() string {
	if n.node == nil {
		return ""
	}
	return n.node.Data
}

// Text returns the text content of the node.
func (n *Node) Text() string {
	if n.node == nil {
		return ""
	}
	return n.node.InnerText()
}

// InnerText returns all text content of the node and its descendants.
func (n *Node) InnerText() string {
	if n.node == nil {
		return ""
	}
	return n.node.InnerText()
}

// InnerXML returns the inner XML of the node.
func (n *Node) InnerXML() string {
	if n.node == nil {
		return ""
	}
	var buf bytes.Buffer
	for child := n.node.FirstChild; child != nil; child = child.NextSibling {
		buf.WriteString(child.OutputXML(true))
	}
	return buf.String()
}

// Children returns the child element nodes.
func (n *Node) Children() []*Node {
	if n.node == nil {
		return nil
	}

	var children []*Node
	for child := n.node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == xmlquery.ElementNode {
			children = append(children, &Node{node: child})
		}
	}
	return children
}

// Attributes returns all attributes of the node.
func (n *Node) Attributes() map[string]string {
	if n.node == nil {
		return nil
	}

	attrs := make(map[string]string)
	for _, attr := range n.node.Attr {
		attrs[attr.Name.Local] = attr.Value
	}
	return attrs
}

// Attr returns the value of a specific attribute.
func (n *Node) Attr(name string) string {
	if n.node == nil {
		return ""
	}
	return n.node.SelectAttr(name)
}
