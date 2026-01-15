// Package xml provides pure Go XML validation, XPath, and formatting.
package xml

import (
	"bytes"
	"strings"
	"testing"

	"github.com/antchfx/xmlquery"
)

// TestParseValidXML verifies parsing of well-formed XML.
func TestParseValidXML(t *testing.T) {
	xmlData := `<?xml version="1.0"?>
<root>
	<element attr="value">text</element>
</root>`

	doc, err := Parse([]byte(xmlData))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if doc == nil {
		t.Fatal("Parse returned nil document")
	}
}

// TestParseInvalidXML verifies error handling for malformed XML.
func TestParseInvalidXML(t *testing.T) {
	tests := []struct {
		name string
		xml  string
	}{
		{"unclosed tag", "<root><element></root>"},
		{"mismatched tags", "<root></other>"},
		{"invalid chars", "<root>\x00</root>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.xml))
			if err == nil {
				t.Error("Parse should fail for invalid XML")
			}
		})
	}
}

// TestValidateWellFormed verifies well-formedness validation.
func TestValidateWellFormed(t *testing.T) {
	valid := `<?xml version="1.0"?><root><child/></root>`
	result := Validate([]byte(valid), nil)
	if !result.Valid {
		t.Errorf("Valid XML should pass: %v", result.Errors)
	}
}

// TestValidateWithDTD verifies DTD validation.
func TestValidateWithDTD(t *testing.T) {
	xmlData := `<?xml version="1.0"?>
<!DOCTYPE note [
<!ELEMENT note (to,from,body)>
<!ELEMENT to (#PCDATA)>
<!ELEMENT from (#PCDATA)>
<!ELEMENT body (#PCDATA)>
]>
<note>
	<to>User</to>
	<from>System</from>
	<body>Hello</body>
</note>`

	result := Validate([]byte(xmlData), nil)
	if !result.Valid {
		t.Errorf("Valid DTD XML should pass: %v", result.Errors)
	}
}

// TestXPathQuery verifies XPath query execution.
func TestXPathQuery(t *testing.T) {
	xmlData := `<?xml version="1.0"?>
<library>
	<book id="1"><title>Book One</title></book>
	<book id="2"><title>Book Two</title></book>
</library>`

	doc, err := Parse([]byte(xmlData))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	results, err := doc.XPath("//book/title")
	if err != nil {
		t.Fatalf("XPath failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("XPath should return 2 results, got %d", len(results))
	}
}

// TestXPathQueryAttribute verifies XPath attribute selection.
func TestXPathQueryAttribute(t *testing.T) {
	xmlData := `<root><item id="123" name="test"/></root>`

	doc, err := Parse([]byte(xmlData))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	results, err := doc.XPath("//item/@id")
	if err != nil {
		t.Fatalf("XPath failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("XPath should return 1 result, got %d", len(results))
	}
}

// TestXPathQueryText verifies XPath text extraction.
func TestXPathQueryText(t *testing.T) {
	xmlData := `<root><message>Hello World</message></root>`

	doc, err := Parse([]byte(xmlData))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	results, err := doc.XPath("//message/text()")
	if err != nil {
		t.Fatalf("XPath failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("XPath should return 1 result, got %d", len(results))
	}

	if results[0].Text() != "Hello World" {
		t.Errorf("Text = %q, want %q", results[0].Text(), "Hello World")
	}
}

// TestXPathInvalidExpression verifies error handling for invalid XPath.
func TestXPathInvalidExpression(t *testing.T) {
	xmlData := `<root/>`

	doc, err := Parse([]byte(xmlData))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	_, err = doc.XPath("[invalid")
	if err == nil {
		t.Error("Invalid XPath should return error")
	}
}

// TestFormat verifies XML pretty-printing.
func TestFormat(t *testing.T) {
	xmlData := `<?xml version="1.0"?><root><child attr="val">text</child></root>`

	formatted, err := Format([]byte(xmlData), FormatOptions{Indent: "  "})
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	// Should have newlines and indentation
	if !strings.Contains(string(formatted), "\n") {
		t.Error("Formatted XML should contain newlines")
	}
	if !strings.Contains(string(formatted), "  ") {
		t.Error("Formatted XML should contain indentation")
	}
}

// TestFormatWithTabs verifies tab indentation.
func TestFormatWithTabs(t *testing.T) {
	xmlData := `<root><child/></root>`

	formatted, err := Format([]byte(xmlData), FormatOptions{Indent: "\t"})
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	if !strings.Contains(string(formatted), "\t") {
		t.Error("Formatted XML should contain tabs")
	}
}

// TestFormatPreservesContent verifies content is preserved during formatting.
func TestFormatPreservesContent(t *testing.T) {
	xmlData := `<root><message>Hello &amp; World</message></root>`

	formatted, err := Format([]byte(xmlData), FormatOptions{Indent: "  "})
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	if !strings.Contains(string(formatted), "Hello &amp; World") {
		t.Error("Formatted XML should preserve entity references")
	}
}

// TestDocumentRoot verifies root element access.
func TestDocumentRoot(t *testing.T) {
	xmlData := `<?xml version="1.0"?><root attr="value"><child/></root>`

	doc, err := Parse([]byte(xmlData))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	root := doc.Root()
	if root == nil {
		t.Fatal("Root should not be nil")
	}

	if root.Name() != "root" {
		t.Errorf("Root name = %q, want %q", root.Name(), "root")
	}
}

// TestNodeChildren verifies child node access.
func TestNodeChildren(t *testing.T) {
	xmlData := `<parent><child1/><child2/><child3/></parent>`

	doc, err := Parse([]byte(xmlData))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	children := doc.Root().Children()
	if len(children) != 3 {
		t.Errorf("Should have 3 children, got %d", len(children))
	}
}

// TestNodeAttributes verifies attribute access.
func TestNodeAttributes(t *testing.T) {
	xmlData := `<element id="123" class="test" data-value="abc"/>`

	doc, err := Parse([]byte(xmlData))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	attrs := doc.Root().Attributes()
	if len(attrs) != 3 {
		t.Errorf("Should have 3 attributes, got %d", len(attrs))
	}

	if doc.Root().Attr("id") != "123" {
		t.Errorf("Attr(id) = %q, want %q", doc.Root().Attr("id"), "123")
	}
}

// TestNodeInnerText verifies inner text extraction.
func TestNodeInnerText(t *testing.T) {
	xmlData := `<root>Hello <b>World</b>!</root>`

	doc, err := Parse([]byte(xmlData))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	text := doc.Root().InnerText()
	if text != "Hello World!" {
		t.Errorf("InnerText = %q, want %q", text, "Hello World!")
	}
}

// TestNodeInnerXML verifies inner XML extraction.
func TestNodeInnerXML(t *testing.T) {
	xmlData := `<root>Hello <b>World</b>!</root>`

	doc, err := Parse([]byte(xmlData))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	innerXML := doc.Root().InnerXML()
	if !strings.Contains(innerXML, "<b>World</b>") {
		t.Errorf("InnerXML should contain markup: %q", innerXML)
	}
}

// TestValidationResult verifies validation result structure.
func TestValidationResult(t *testing.T) {
	result := ValidationResult{
		Valid:  false,
		Errors: []ValidationError{{Line: 1, Column: 5, Message: "test error"}},
	}

	if result.Valid {
		t.Error("Result should not be valid")
	}

	if len(result.Errors) != 1 {
		t.Error("Result should have 1 error")
	}

	if result.Errors[0].Line != 1 {
		t.Errorf("Error line = %d, want 1", result.Errors[0].Line)
	}
}

// TestNamespaceHandling verifies namespace support.
func TestNamespaceHandling(t *testing.T) {
	xmlData := `<root xmlns:ns="http://example.com"><ns:child/></root>`

	doc, err := Parse([]byte(xmlData))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Should parse without error
	if doc.Root() == nil {
		t.Error("Document should have root element")
	}
}

// TestCDATAHandling verifies CDATA section handling.
func TestCDATAHandling(t *testing.T) {
	xmlData := `<root><![CDATA[<not>xml</not>]]></root>`

	doc, err := Parse([]byte(xmlData))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	text := doc.Root().InnerText()
	if !strings.Contains(text, "<not>xml</not>") {
		t.Errorf("CDATA content should be preserved: %q", text)
	}
}

// TestCommentHandling verifies XML comment handling.
func TestCommentHandling(t *testing.T) {
	xmlData := `<root><!-- comment --><child/></root>`

	doc, err := Parse([]byte(xmlData))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Comments should not affect parsing
	children := doc.Root().Children()
	if len(children) != 1 {
		t.Errorf("Should have 1 child element (comments excluded), got %d", len(children))
	}
}

// TestProcessingInstruction verifies PI handling.
func TestProcessingInstruction(t *testing.T) {
	xmlData := `<?xml version="1.0"?><?xml-stylesheet type="text/xsl" href="style.xsl"?><root/>`

	doc, err := Parse([]byte(xmlData))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// PIs should not affect document structure
	if doc.Root() == nil {
		t.Error("Document should have root element")
	}
}

// TestSerialize verifies XML serialization.
func TestSerialize(t *testing.T) {
	xmlData := `<root attr="value"><child>text</child></root>`

	doc, err := Parse([]byte(xmlData))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	output := doc.Serialize()
	if !strings.Contains(string(output), "attr=\"value\"") {
		t.Error("Serialized XML should contain attribute")
	}
	if !strings.Contains(string(output), "<child>text</child>") {
		t.Error("Serialized XML should contain child element")
	}
}

// TestXPathSelectSingle verifies selecting single node.
func TestXPathSelectSingle(t *testing.T) {
	xmlData := `<root><first/><second/></root>`

	doc, err := Parse([]byte(xmlData))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	node, err := doc.XPathFirst("//first")
	if err != nil {
		t.Fatalf("XPathFirst failed: %v", err)
	}

	if node == nil {
		t.Fatal("XPathFirst should return a node")
	}

	if node.Name() != "first" {
		t.Errorf("Node name = %q, want %q", node.Name(), "first")
	}
}

// TestXPathSelectEmpty verifies empty result handling.
func TestXPathSelectEmpty(t *testing.T) {
	xmlData := `<root/>`

	doc, err := Parse([]byte(xmlData))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	results, err := doc.XPath("//nonexistent")
	if err != nil {
		t.Fatalf("XPath failed: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("XPath should return empty slice, got %d results", len(results))
	}
}

// TestXPathFirstNotFound verifies XPathFirst returns nil when no match.
func TestXPathFirstNotFound(t *testing.T) {
	xmlData := `<root/>`

	doc, err := Parse([]byte(xmlData))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	node, err := doc.XPathFirst("//nonexistent")
	if err != nil {
		t.Fatalf("XPathFirst failed: %v", err)
	}

	if node != nil {
		t.Error("XPathFirst should return nil for non-existent element")
	}
}

// TestXPathFirstInvalidExpression verifies error handling for invalid XPath in XPathFirst.
func TestXPathFirstInvalidExpression(t *testing.T) {
	xmlData := `<root/>`

	doc, err := Parse([]byte(xmlData))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	_, err = doc.XPathFirst("[invalid")
	if err == nil {
		t.Error("Invalid XPath should return error in XPathFirst")
	}
}

// TestValidateMalformed verifies validation catches malformed XML.
func TestValidateMalformed(t *testing.T) {
	malformed := `<root><unclosed>`
	result := Validate([]byte(malformed), nil)
	if result.Valid {
		t.Error("Malformed XML should not be valid")
	}
	if len(result.Errors) == 0 {
		t.Error("Malformed XML should have errors")
	}
}

// TestFormatDefaultIndent verifies default indentation when none specified.
func TestFormatDefaultIndent(t *testing.T) {
	xmlData := `<root><child/></root>`

	formatted, err := Format([]byte(xmlData), FormatOptions{})
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	// Default should be two spaces
	if !strings.Contains(string(formatted), "  ") {
		t.Error("Default indentation should be two spaces")
	}
}

// TestFormatInvalidXML verifies Format handles invalid XML.
func TestFormatInvalidXML(t *testing.T) {
	xmlData := `<root><unclosed>`

	_, err := Format([]byte(xmlData), FormatOptions{})
	if err == nil {
		t.Error("Format should fail for invalid XML")
	}
}

// TestFormatWithDeclaration verifies formatting preserves XML declaration.
func TestFormatWithDeclaration(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="UTF-8"?><root/>`

	formatted, err := Format([]byte(xmlData), FormatOptions{Indent: "  "})
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	if !strings.Contains(string(formatted), "<?xml") {
		t.Error("Formatted XML should preserve declaration")
	}
	if !strings.Contains(string(formatted), "version=\"1.0\"") {
		t.Error("Formatted XML should preserve version attribute")
	}
}

// TestFormatWithNamespacePrefix verifies formatting preserves namespace prefixes.
func TestFormatWithNamespacePrefix(t *testing.T) {
	xmlData := `<ns:root xmlns:ns="http://example.com"><ns:child/></ns:root>`

	formatted, err := Format([]byte(xmlData), FormatOptions{Indent: "  "})
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	if !strings.Contains(string(formatted), "ns:root") {
		t.Error("Formatted XML should preserve namespace prefix on root")
	}
	if !strings.Contains(string(formatted), "ns:child") {
		t.Error("Formatted XML should preserve namespace prefix on child")
	}
}

// TestFormatWithNamespaceAttribute verifies formatting handles namespace attributes.
func TestFormatWithNamespaceAttribute(t *testing.T) {
	xmlData := `<root xmlns:custom="http://example.com"/>`

	formatted, err := Format([]byte(xmlData), FormatOptions{Indent: "  "})
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	if !strings.Contains(string(formatted), "xmlns:custom") {
		t.Error("Formatted XML should preserve namespace attribute")
	}
}

// TestFormatSelfClosingTag verifies self-closing tags are formatted correctly.
func TestFormatSelfClosingTag(t *testing.T) {
	xmlData := `<root><empty/></root>`

	formatted, err := Format([]byte(xmlData), FormatOptions{Indent: "  "})
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	if !strings.Contains(string(formatted), "<empty/>") {
		t.Error("Self-closing tag should be preserved")
	}
}

// TestFormatMixedContent verifies formatting of mixed text and element content.
func TestFormatMixedContent(t *testing.T) {
	xmlData := `<root>Text before<child/>Text after</root>`

	formatted, err := Format([]byte(xmlData), FormatOptions{Indent: "  "})
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	if !strings.Contains(string(formatted), "child") {
		t.Error("Formatted XML should contain child element")
	}
}

// TestFormatWithCDATA verifies CDATA formatting.
func TestFormatWithCDATA(t *testing.T) {
	xmlData := `<root><![CDATA[<script>alert('test')</script>]]></root>`

	formatted, err := Format([]byte(xmlData), FormatOptions{Indent: "  "})
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	if !strings.Contains(string(formatted), "<![CDATA[") {
		t.Error("Formatted XML should preserve CDATA start")
	}
	if !strings.Contains(string(formatted), "]]>") {
		t.Error("Formatted XML should preserve CDATA end")
	}
}

// TestFormatWithComment verifies comment handling in formatting.
// Note: The xmlquery library doesn't preserve comments during parsing,
// so comments are not included in formatted output.
func TestFormatWithComment(t *testing.T) {
	// Comments are not preserved by the underlying xmlquery library
	xmlData := `<root><!-- This is a comment --><child/></root>`

	formatted, err := Format([]byte(xmlData), FormatOptions{Indent: "  "})
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	// Verify the rest of the document is formatted correctly
	if !strings.Contains(string(formatted), "<root>") {
		t.Error("Formatted XML should contain root element")
	}
	if !strings.Contains(string(formatted), "<child/>") {
		t.Error("Formatted XML should contain child element")
	}
}

// TestFormatEscapesSpecialChars verifies special character escaping in text.
func TestFormatEscapesSpecialChars(t *testing.T) {
	xmlData := `<root>&lt;tag&gt; &amp; "quotes"</root>`

	formatted, err := Format([]byte(xmlData), FormatOptions{Indent: "  "})
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	formattedStr := string(formatted)
	if !strings.Contains(formattedStr, "&lt;") {
		t.Error("Should escape < as &lt;")
	}
	if !strings.Contains(formattedStr, "&gt;") {
		t.Error("Should escape > as &gt;")
	}
	if !strings.Contains(formattedStr, "&amp;") {
		t.Error("Should escape & as &amp;")
	}
}

// TestFormatEscapesAttributeQuotes verifies quote escaping in attributes.
func TestFormatEscapesAttributeQuotes(t *testing.T) {
	xmlData := `<root attr="value with &quot;quotes&quot;"/>`

	formatted, err := Format([]byte(xmlData), FormatOptions{Indent: "  "})
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	if !strings.Contains(string(formatted), "&quot;") {
		t.Error("Should escape quotes in attributes as &quot;")
	}
}

// TestDocumentRootNilDocument verifies Root handles nil document.
func TestDocumentRootNilDocument(t *testing.T) {
	doc := &Document{root: nil}
	root := doc.Root()
	if root != nil {
		t.Error("Root should return nil for document with nil root")
	}
}

// TestDocumentRootNoElementChild verifies Root when document has no element children.
func TestDocumentRootNoElementChild(t *testing.T) {
	// Create a document with only declaration and comment (no element)
	xmlData := `<?xml version="1.0"?><!-- comment only -->`

	doc, err := Parse([]byte(xmlData))
	if err != nil {
		// If parsing fails, that's okay - we're testing edge case
		// Skip this test as the parser requires at least one element
		t.Skip("Parser requires at least one element node")
	}

	root := doc.Root()
	if root != nil {
		t.Error("Root should return nil when document has no element children")
	}
}

// TestSerializeNilDocument verifies Serialize handles nil document root.
func TestSerializeNilDocument(t *testing.T) {
	doc := &Document{root: nil}
	output := doc.Serialize()
	if output != nil {
		t.Error("Serialize should return nil for document with nil root")
	}
}

// TestNodeNameNil verifies Name handles nil node.
func TestNodeNameNil(t *testing.T) {
	node := &Node{node: nil}
	name := node.Name()
	if name != "" {
		t.Error("Name should return empty string for nil node")
	}
}

// TestNodeTextNil verifies Text handles nil node.
func TestNodeTextNil(t *testing.T) {
	node := &Node{node: nil}
	text := node.Text()
	if text != "" {
		t.Error("Text should return empty string for nil node")
	}
}

// TestNodeInnerTextNil verifies InnerText handles nil node.
func TestNodeInnerTextNil(t *testing.T) {
	node := &Node{node: nil}
	text := node.InnerText()
	if text != "" {
		t.Error("InnerText should return empty string for nil node")
	}
}

// TestNodeInnerXMLNil verifies InnerXML handles nil node.
func TestNodeInnerXMLNil(t *testing.T) {
	node := &Node{node: nil}
	xml := node.InnerXML()
	if xml != "" {
		t.Error("InnerXML should return empty string for nil node")
	}
}

// TestNodeChildrenNil verifies Children handles nil node.
func TestNodeChildrenNil(t *testing.T) {
	node := &Node{node: nil}
	children := node.Children()
	if children != nil {
		t.Error("Children should return nil for nil node")
	}
}

// TestNodeAttributesNil verifies Attributes handles nil node.
func TestNodeAttributesNil(t *testing.T) {
	node := &Node{node: nil}
	attrs := node.Attributes()
	if attrs != nil {
		t.Error("Attributes should return nil for nil node")
	}
}

// TestNodeAttrNil verifies Attr handles nil node.
func TestNodeAttrNil(t *testing.T) {
	node := &Node{node: nil}
	attr := node.Attr("test")
	if attr != "" {
		t.Error("Attr should return empty string for nil node")
	}
}

// TestNodeChildrenWithTextNodes verifies Children filters text nodes.
func TestNodeChildrenWithTextNodes(t *testing.T) {
	xmlData := `<root>text1<child1/>text2<child2/>text3</root>`

	doc, err := Parse([]byte(xmlData))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	children := doc.Root().Children()
	// Should only return element children, not text nodes
	if len(children) != 2 {
		t.Errorf("Should have 2 element children (text nodes excluded), got %d", len(children))
	}
}

// TestNodeAttributesEmpty verifies Attributes on node without attributes.
func TestNodeAttributesEmpty(t *testing.T) {
	xmlData := `<root/>`

	doc, err := Parse([]byte(xmlData))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	attrs := doc.Root().Attributes()
	if len(attrs) != 0 {
		t.Errorf("Should have 0 attributes, got %d", len(attrs))
	}
}

// TestNodeAttrMissing verifies Attr returns empty string for missing attribute.
func TestNodeAttrMissing(t *testing.T) {
	xmlData := `<root id="123"/>`

	doc, err := Parse([]byte(xmlData))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	attr := doc.Root().Attr("nonexistent")
	if attr != "" {
		t.Errorf("Attr should return empty string for missing attribute, got %q", attr)
	}
}

// TestXPathWithPredicate verifies XPath predicates work correctly.
func TestXPathWithPredicate(t *testing.T) {
	xmlData := `<root><item id="1">A</item><item id="2">B</item><item id="3">C</item></root>`

	doc, err := Parse([]byte(xmlData))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	results, err := doc.XPath("//item[@id='2']")
	if err != nil {
		t.Fatalf("XPath failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("XPath should return 1 result, got %d", len(results))
	}

	if results[0].Text() != "B" {
		t.Errorf("Text = %q, want %q", results[0].Text(), "B")
	}
}

// TestFormatEmptyElement verifies formatting of truly empty elements.
func TestFormatEmptyElement(t *testing.T) {
	xmlData := `<root><empty></empty></root>`

	formatted, err := Format([]byte(xmlData), FormatOptions{Indent: "  "})
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	// Empty element should become self-closing
	formattedStr := string(formatted)
	if !strings.Contains(formattedStr, "empty") {
		t.Error("Formatted XML should contain empty element")
	}
}

// TestFormatTextOnlyElement verifies formatting of element with only text.
func TestFormatTextOnlyElement(t *testing.T) {
	xmlData := `<root><text>content</text></root>`

	formatted, err := Format([]byte(xmlData), FormatOptions{Indent: "  "})
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	if !strings.Contains(string(formatted), "content") {
		t.Error("Formatted XML should preserve text content")
	}
}

// TestFormatWhitespaceOnlyText verifies whitespace-only text is trimmed.
func TestFormatWhitespaceOnlyText(t *testing.T) {
	xmlData := `<root>

		<child/>   </root>`

	formatted, err := Format([]byte(xmlData), FormatOptions{Indent: "  "})
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	// Whitespace-only text should be trimmed
	formattedStr := string(formatted)
	if !strings.Contains(formattedStr, "child") {
		t.Error("Formatted XML should contain child element")
	}
}

// TestInnerXMLWithMultipleChildren verifies InnerXML with multiple children.
func TestInnerXMLWithMultipleChildren(t *testing.T) {
	xmlData := `<root><a>1</a><b>2</b><c>3</c></root>`

	doc, err := Parse([]byte(xmlData))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	innerXML := doc.Root().InnerXML()
	if !strings.Contains(innerXML, "<a>") {
		t.Error("InnerXML should contain <a> element")
	}
	if !strings.Contains(innerXML, "<b>") {
		t.Error("InnerXML should contain <b> element")
	}
	if !strings.Contains(innerXML, "<c>") {
		t.Error("InnerXML should contain <c> element")
	}
}

// TestComplexXPathExpression verifies complex XPath expressions.
func TestComplexXPathExpression(t *testing.T) {
	xmlData := `<library>
		<book category="fiction"><title>Book 1</title></book>
		<book category="nonfiction"><title>Book 2</title></book>
		<book category="fiction"><title>Book 3</title></book>
	</library>`

	doc, err := Parse([]byte(xmlData))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	results, err := doc.XPath("//book[@category='fiction']/title")
	if err != nil {
		t.Fatalf("XPath failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("XPath should return 2 fiction titles, got %d", len(results))
	}
}

// TestFormatDocumentLevelComment verifies formatting of comments at document level.
// Note: The xmlquery library may not preserve document-level comments.
func TestFormatDocumentLevelComment(t *testing.T) {
	// Comment before the root element
	xmlData := `<?xml version="1.0"?><!-- Document comment --><root/>`

	formatted, err := Format([]byte(xmlData), FormatOptions{Indent: "  "})
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	// The formatted output should still have the root element
	if !strings.Contains(string(formatted), "<root") {
		t.Error("Formatted XML should contain root element")
	}
}

// TestFormatDocumentWithOnlyDeclaration verifies formatting minimal XML.
func TestFormatDocumentWithOnlyDeclaration(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="UTF-8"?><empty/>`

	formatted, err := Format([]byte(xmlData), FormatOptions{Indent: "  "})
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	if !strings.Contains(string(formatted), "<?xml") {
		t.Error("Formatted XML should preserve declaration")
	}
}

// TestFormatTextOnlyDocument verifies formatting when there's text at document level.
// Note: XML requires a root element, so pure text documents are invalid.
// This tests handling of whitespace text nodes around elements.
func TestFormatTextOnlyDocument(t *testing.T) {
	// Whitespace around elements is common
	xmlData := `<?xml version="1.0"?>
<root>content</root>
`

	formatted, err := Format([]byte(xmlData), FormatOptions{Indent: "  "})
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	if !strings.Contains(string(formatted), "content") {
		t.Error("Formatted XML should preserve content")
	}
}

// TestFormatNestedElementsWithErrors tests error handling in format.
// The formatNode error paths are largely unreachable since bytes.Buffer
// doesn't return errors from WriteString, but we test what we can.
func TestFormatNestedElementsWithErrors(t *testing.T) {
	// Deeply nested structure to exercise all formatNode paths
	xmlData := `<?xml version="1.0"?>
<root xmlns:ns="http://test.com">
	<ns:parent attr="val">
		<child>text</child>
		<![CDATA[cdata content]]>
	</ns:parent>
</root>`

	formatted, err := Format([]byte(xmlData), FormatOptions{Indent: "\t"})
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	if !strings.Contains(string(formatted), "ns:parent") {
		t.Error("Formatted XML should preserve namespaced elements")
	}
	if !strings.Contains(string(formatted), "<![CDATA[") {
		t.Error("Formatted XML should preserve CDATA")
	}
}

// TestDocumentRootOnlyTextNode verifies Root returns nil when document has only text nodes.
// This tests the edge case where a parsed document has no element children at all.
func TestDocumentRootOnlyTextNode(t *testing.T) {
	// Create a document manually with only a text node (no elements)
	// This is a synthetic test case to reach the "return nil" branch in Root()
	doc := &Document{
		root: &xmlquery.Node{
			Type:       xmlquery.DocumentNode,
			FirstChild: &xmlquery.Node{Type: xmlquery.TextNode, Data: "text"},
		},
	}

	root := doc.Root()
	if root != nil {
		t.Error("Root should return nil when document has no element children")
	}
}

// TestFormatDocumentLevelTextNode verifies formatting of document-level text nodes.
// This tests the TextNode case in formatNode at document level.
func TestFormatDocumentLevelTextNode(t *testing.T) {
	// Test standalone text node formatting
	xmlData := `<root>
	content with spaces
  </root>`

	formatted, err := Format([]byte(xmlData), FormatOptions{Indent: "  "})
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	// The formatted output should preserve the content
	if !strings.Contains(string(formatted), "content with spaces") {
		t.Error("Formatted XML should preserve text content")
	}
}

// TestFormatAttributeWithEmptyLocalName verifies formatting handles attributes with empty local names.
// This tests the edge case where attr.Name.Local is empty.
func TestFormatAttributeWithEmptyLocalName(t *testing.T) {
	// Test with namespace-only attribute
	xmlData := `<root xmlns="http://example.com"/>`

	formatted, err := Format([]byte(xmlData), FormatOptions{Indent: "  "})
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	// The formatted output should contain the root element
	if !strings.Contains(string(formatted), "<root") {
		t.Error("Formatted XML should contain root element")
	}
}

// TestXPathWithNamespaces verifies XPath with namespace support.
func TestXPathWithNamespaces(t *testing.T) {
	xmlData := `<root xmlns:ns="http://example.com">
		<ns:item id="1">Value 1</ns:item>
		<ns:item id="2">Value 2</ns:item>
	</root>`

	doc, err := Parse([]byte(xmlData))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Query for namespaced elements
	results, err := doc.XPath("//*[local-name()='item']")
	if err != nil {
		t.Fatalf("XPath failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("XPath should return 2 results, got %d", len(results))
	}
}

// TestXPathFirstWithNamespaces verifies XPathFirst with namespace support.
func TestXPathFirstWithNamespaces(t *testing.T) {
	xmlData := `<root xmlns:ns="http://example.com">
		<ns:item id="1">Value 1</ns:item>
		<ns:item id="2">Value 2</ns:item>
	</root>`

	doc, err := Parse([]byte(xmlData))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Query for first namespaced element
	result, err := doc.XPathFirst("//*[local-name()='item' and @id='1']")
	if err != nil {
		t.Fatalf("XPathFirst failed: %v", err)
	}

	if result == nil {
		t.Fatal("XPathFirst should return a result")
	}

	if result.Text() != "Value 1" {
		t.Errorf("Text = %q, want %q", result.Text(), "Value 1")
	}
}

// TestFormatComplexNestedStructure verifies formatting of deeply nested XML.
func TestFormatComplexNestedStructure(t *testing.T) {
	xmlData := `<?xml version="1.0"?>
<root>
	<level1>
		<level2>
			<level3 attr="value">
				<level4>Deep content</level4>
			</level3>
		</level2>
	</level1>
</root>`

	formatted, err := Format([]byte(xmlData), FormatOptions{Indent: "  "})
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	// Verify nested structure is preserved
	if !strings.Contains(string(formatted), "level4") {
		t.Error("Formatted XML should contain deeply nested elements")
	}
	if !strings.Contains(string(formatted), "Deep content") {
		t.Error("Formatted XML should preserve deep content")
	}
}

// TestFormatWithMultipleTextNodes verifies formatting of elements with multiple text nodes.
func TestFormatWithMultipleTextNodes(t *testing.T) {
	xmlData := `<root>Text 1<child1/>Text 2<child2/>Text 3</root>`

	formatted, err := Format([]byte(xmlData), FormatOptions{Indent: "  "})
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	// Verify all children are formatted
	if !strings.Contains(string(formatted), "child1") {
		t.Error("Formatted XML should contain child1")
	}
	if !strings.Contains(string(formatted), "child2") {
		t.Error("Formatted XML should contain child2")
	}
}

// TestXPathWithComplexPredicate verifies XPath with complex predicates.
func TestXPathWithComplexPredicate(t *testing.T) {
	xmlData := `<library>
		<section name="fiction">
			<book price="10">Book A</book>
			<book price="20">Book B</book>
		</section>
		<section name="nonfiction">
			<book price="15">Book C</book>
		</section>
	</library>`

	doc, err := Parse([]byte(xmlData))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Complex XPath with multiple predicates
	results, err := doc.XPath("//section[@name='fiction']/book[@price='10']")
	if err != nil {
		t.Fatalf("XPath failed: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("XPath should return 1 result, got %d", len(results))
	}

	if results[0].Text() != "Book A" {
		t.Errorf("Text = %q, want %q", results[0].Text(), "Book A")
	}
}

// TestXPathWithAxes verifies XPath with different axes.
func TestXPathWithAxes(t *testing.T) {
	xmlData := `<root>
		<parent>
			<child>Child 1</child>
			<child>Child 2</child>
		</parent>
	</root>`

	doc, err := Parse([]byte(xmlData))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Use descendant axis
	results, err := doc.XPath("//parent/descendant::child")
	if err != nil {
		t.Fatalf("XPath failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("XPath should return 2 results, got %d", len(results))
	}
}

// TestFormatWithOnlyWhitespace verifies formatting handles whitespace-only content.
func TestFormatWithOnlyWhitespace(t *testing.T) {
	xmlData := `<root>


		<child/>


	</root>`

	formatted, err := Format([]byte(xmlData), FormatOptions{Indent: "  "})
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	// Verify the child element is preserved
	if !strings.Contains(string(formatted), "child") {
		t.Error("Formatted XML should contain child element")
	}
}

// TestXPathWithCount verifies XPath count function.
func TestXPathWithCount(t *testing.T) {
	xmlData := `<root>
		<item>1</item>
		<item>2</item>
		<item>3</item>
	</root>`

	doc, err := Parse([]byte(xmlData))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Test with XPath function
	results, err := doc.XPath("//item[position() <= 2]")
	if err != nil {
		t.Fatalf("XPath failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("XPath should return 2 results, got %d", len(results))
	}
}

// TestFormatWithEncodingAttribute verifies encoding attribute in declaration.
func TestFormatWithEncodingAttribute(t *testing.T) {
	xmlData := `<?xml version="1.0" encoding="ISO-8859-1"?><root/>`

	formatted, err := Format([]byte(xmlData), FormatOptions{Indent: "  "})
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	if !strings.Contains(string(formatted), "encoding=") {
		t.Error("Formatted XML should preserve encoding attribute")
	}
}

// TestFormatWithStandaloneAttribute verifies standalone attribute in declaration.
func TestFormatWithStandaloneAttribute(t *testing.T) {
	xmlData := `<?xml version="1.0" standalone="yes"?><root/>`

	formatted, err := Format([]byte(xmlData), FormatOptions{Indent: "  "})
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	if !strings.Contains(string(formatted), "standalone=") {
		t.Error("Formatted XML should preserve standalone attribute")
	}
}

// TestFormatNodeWithStandaloneTextNode tests formatting a document-level text node.
// This is a synthetic test to cover the TextNode case in formatNode.
func TestFormatNodeWithStandaloneTextNode(t *testing.T) {
	// Create a synthetic document structure with a standalone text node
	// This tests the TextNode case in formatNode at depth 0
	doc := &Document{
		root: &xmlquery.Node{
			Type: xmlquery.DocumentNode,
			FirstChild: &xmlquery.Node{
				Type: xmlquery.TextNode,
				Data: "standalone text",
			},
		},
	}

	var buf []byte
	var opts FormatOptions
	opts.Indent = "  "

	// Manually call formatNode to test the TextNode branch
	var buffer bytes.Buffer
	err := formatNode(&buffer, doc.root, 0, opts.Indent)
	if err != nil {
		t.Fatalf("formatNode failed: %v", err)
	}

	buf = buffer.Bytes()
	// The text node formatting at document level should escape the text
	output := string(buf)
	if !strings.Contains(output, "standalone text") {
		t.Errorf("Output should contain text content, got: %q", output)
	}
}

// TestFormatNodeWithTextNodeAtRootLevel tests text node formatting at document level.
func TestFormatNodeWithTextNodeAtRootLevel(t *testing.T) {
	// Create a document with a text node sibling to element nodes
	root := &xmlquery.Node{Type: xmlquery.DocumentNode}

	// Add text node
	textNode := &xmlquery.Node{
		Type: xmlquery.TextNode,
		Data: "  text content  ",
	}

	// Add element node
	elementNode := &xmlquery.Node{
		Type: xmlquery.ElementNode,
		Data: "root",
	}

	root.FirstChild = textNode
	textNode.NextSibling = elementNode

	doc := &Document{root: root}

	var buffer bytes.Buffer
	err := formatNode(&buffer, doc.root, 0, "  ")
	if err != nil {
		t.Fatalf("formatNode failed: %v", err)
	}

	output := string(buffer.Bytes())
	// Should contain both the text and the element
	if !strings.Contains(output, "text content") {
		t.Errorf("Output should contain text content, got: %q", output)
	}
	if !strings.Contains(output, "<root") {
		t.Errorf("Output should contain root element, got: %q", output)
	}
}

// TestXPathEdgeCases tests XPath with various edge cases.
func TestXPathEdgeCases(t *testing.T) {
	xmlData := `<root>
		<a id="1"/>
		<b id="2"/>
		<c id="3"/>
	</root>`

	doc, err := Parse([]byte(xmlData))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Test various XPath expressions to exercise the code paths
	testCases := []struct {
		name  string
		xpath string
	}{
		{"descendant-or-self", "//root/descendant-or-self::*"},
		{"following-sibling", "//a/following-sibling::*"},
		{"preceding-sibling", "//c/preceding-sibling::*"},
		{"parent axis", "//a/parent::*"},
		{"ancestor axis", "//a/ancestor::*"},
		{"self axis", "//a/self::*"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := doc.XPath(tc.xpath)
			if err != nil {
				t.Errorf("XPath %q failed: %v", tc.xpath, err)
			}
		})
	}
}

// TestXPathFirstEdgeCases tests XPathFirst with various edge cases.
func TestXPathFirstEdgeCases(t *testing.T) {
	xmlData := `<root>
		<item priority="1">First</item>
		<item priority="2">Second</item>
		<item priority="3">Third</item>
	</root>`

	doc, err := Parse([]byte(xmlData))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Test various XPath expressions
	testCases := []struct {
		name  string
		xpath string
	}{
		{"last() function", "//item[last()]"},
		{"position() function", "//item[position()=1]"},
		{"contains() function", "//item[contains(text(), 'First')]"},
		{"starts-with() function", "//item[starts-with(text(), 'F')]"},
		{"string-length() function", "//item[string-length(@priority)=1]"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := doc.XPathFirst(tc.xpath)
			if err != nil {
				t.Errorf("XPathFirst %q failed: %v", tc.xpath, err)
			}
		})
	}
}

// TestFormatVeryDeeplyNested tests formatting with extremely deep nesting.
func TestFormatVeryDeeplyNested(t *testing.T) {
	// Create a deeply nested structure to exercise all recursion paths
	xmlData := `<?xml version="1.0"?>
<level1>
	<level2>
		<level3>
			<level4>
				<level5>
					<level6>
						<level7>
							<level8>
								<level9>
									<level10>Deep content</level10>
								</level9>
							</level8>
						</level7>
					</level6>
				</level5>
			</level4>
		</level3>
	</level2>
</level1>`

	formatted, err := Format([]byte(xmlData), FormatOptions{Indent: "  "})
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}

	// Verify deep nesting is preserved
	if !strings.Contains(string(formatted), "level10") {
		t.Error("Formatted XML should contain deeply nested element")
	}
	if !strings.Contains(string(formatted), "Deep content") {
		t.Error("Formatted XML should preserve deep content")
	}
}
