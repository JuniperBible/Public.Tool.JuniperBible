// libxml2 tool integration tests (xmllint, xsltproc).
// These tests require libxml2-utils to be installed.
package integration

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestXMLLintAvailable checks if xmllint is installed.
func TestXMLLintAvailable(t *testing.T) {
	if !HasTool(ToolXMLLint) {
		t.Skip("xmllint not installed")
	}

	cmd := exec.Command("xmllint", "--version")
	output, _ := cmd.CombinedOutput() // May output to stderr

	if !strings.Contains(string(output), "xmllint") && !strings.Contains(string(output), "libxml") {
		t.Errorf("unexpected xmllint output: %s", output)
	}

	t.Logf("xmllint version info: %s", strings.Split(string(output), "\n")[0])
}

// TestXSLTProcAvailable checks if xsltproc is installed.
func TestXSLTProcAvailable(t *testing.T) {
	if !HasTool(ToolXSLTProc) {
		t.Skip("xsltproc not installed")
	}

	cmd := exec.Command("xsltproc", "--version")
	output, _ := cmd.CombinedOutput() // May output to stderr

	if !strings.Contains(string(output), "xsltproc") && !strings.Contains(string(output), "libxslt") {
		t.Errorf("unexpected xsltproc output: %s", output)
	}

	t.Logf("xsltproc version info: %s", strings.Split(string(output), "\n")[0])
}

// TestXMLLintValidate tests XML validation with xmllint.
func TestXMLLintValidate(t *testing.T) {
	RequireTool(t, ToolXMLLint)

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "xmllint-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Write valid XML
	validXML := `<?xml version="1.0" encoding="UTF-8"?>
<root>
    <item id="1">First item</item>
    <item id="2">Second item</item>
</root>`

	validPath := filepath.Join(tempDir, "valid.xml")
	if err := os.WriteFile(validPath, []byte(validXML), 0600); err != nil {
		t.Fatalf("failed to write XML: %v", err)
	}

	// Validate with xmllint
	cmd := exec.Command("xmllint", "--noout", validPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Errorf("valid XML failed validation: %v\nOutput: %s", err, output)
	}

	// Write invalid XML
	invalidXML := `<?xml version="1.0"?>
<root>
    <item>Unclosed tag
</root>`

	invalidPath := filepath.Join(tempDir, "invalid.xml")
	if err := os.WriteFile(invalidPath, []byte(invalidXML), 0600); err != nil {
		t.Fatalf("failed to write invalid XML: %v", err)
	}

	// Validate should fail
	cmd = exec.Command("xmllint", "--noout", invalidPath)
	output, err = cmd.CombinedOutput()
	if err == nil {
		t.Error("invalid XML passed validation - should have failed")
	} else {
		t.Log("correctly detected invalid XML")
	}
}

// TestXMLLintXPath tests XPath queries with xmllint.
func TestXMLLintXPath(t *testing.T) {
	RequireTool(t, ToolXMLLint)

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "xmllint-xpath-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Write XML file
	xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<library>
    <book id="1">
        <title>The Go Programming Language</title>
        <author>Alan Donovan</author>
    </book>
    <book id="2">
        <title>Programming in Go</title>
        <author>Mark Summerfield</author>
    </book>
</library>`

	xmlPath := filepath.Join(tempDir, "library.xml")
	if err := os.WriteFile(xmlPath, []byte(xmlContent), 0600); err != nil {
		t.Fatalf("failed to write XML: %v", err)
	}

	// Query titles
	cmd := exec.Command("xmllint", "--xpath", "//book/title/text()", xmlPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("XPath query failed: %v\nOutput: %s", err, output)
	}

	outputStr := string(output)
	if !strings.Contains(outputStr, "The Go Programming Language") {
		t.Errorf("expected title not found in output: %s", outputStr)
	}

	// Query by attribute
	cmd = exec.Command("xmllint", "--xpath", "//book[@id='2']/title/text()", xmlPath)
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("XPath attribute query failed: %v\nOutput: %s", err, output)
	}

	if !strings.Contains(string(output), "Programming in Go") {
		t.Errorf("expected book not found: %s", output)
	}

	t.Log("successfully executed XPath queries")
}

// TestXMLLintFormat tests XML formatting with xmllint.
func TestXMLLintFormat(t *testing.T) {
	RequireTool(t, ToolXMLLint)

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "xmllint-format-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Write unformatted XML
	uglyXML := `<?xml version="1.0"?><root><item>One</item><item>Two</item></root>`

	xmlPath := filepath.Join(tempDir, "ugly.xml")
	if err := os.WriteFile(xmlPath, []byte(uglyXML), 0600); err != nil {
		t.Fatalf("failed to write XML: %v", err)
	}

	// Format with xmllint
	cmd := exec.Command("xmllint", "--format", xmlPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("xmllint format failed: %v\nOutput: %s", err, output)
	}

	formatted := string(output)
	if !strings.Contains(formatted, "\n") {
		t.Error("formatted output should contain newlines")
	}
	if !strings.Contains(formatted, "  ") {
		t.Error("formatted output should contain indentation")
	}

	t.Log("successfully formatted XML")
}

// TestXSLTProc tests XSLT transformation.
func TestXSLTProc(t *testing.T) {
	RequireTool(t, ToolXSLTProc)

	// Create temp directory
	tempDir, err := os.MkdirTemp("", "xsltproc-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Write XML file
	xmlContent := `<?xml version="1.0" encoding="UTF-8"?>
<books>
    <book><title>Book One</title></book>
    <book><title>Book Two</title></book>
</books>`

	xmlPath := filepath.Join(tempDir, "books.xml")
	if err := os.WriteFile(xmlPath, []byte(xmlContent), 0600); err != nil {
		t.Fatalf("failed to write XML: %v", err)
	}

	// Write XSLT stylesheet
	xsltContent := `<?xml version="1.0" encoding="UTF-8"?>
<xsl:stylesheet version="1.0" xmlns:xsl="http://www.w3.org/1999/XSL/Transform">
    <xsl:output method="html" indent="yes"/>
    <xsl:template match="/">
        <html>
            <body>
                <h1>Book List</h1>
                <ul>
                    <xsl:for-each select="books/book">
                        <li><xsl:value-of select="title"/></li>
                    </xsl:for-each>
                </ul>
            </body>
        </html>
    </xsl:template>
</xsl:stylesheet>`

	xsltPath := filepath.Join(tempDir, "transform.xslt")
	if err := os.WriteFile(xsltPath, []byte(xsltContent), 0600); err != nil {
		t.Fatalf("failed to write XSLT: %v", err)
	}

	// Transform
	outputPath := filepath.Join(tempDir, "output.html")
	cmd := exec.Command("xsltproc", "-o", outputPath, xsltPath, xmlPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("xsltproc failed: %v\nOutput: %s", err, output)
	}

	// Verify output
	htmlContent, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	html := string(htmlContent)
	if !strings.Contains(html, "<h1>Book List</h1>") {
		t.Error("expected h1 not found in output")
	}
	if !strings.Contains(html, "<li>Book One</li>") {
		t.Error("expected list item not found in output")
	}

	t.Log("successfully performed XSLT transformation")
}
