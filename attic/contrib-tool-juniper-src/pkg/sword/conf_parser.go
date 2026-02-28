package sword

import (
	"strings"

	"github.com/alecthomas/participle/v2"
	"github.com/alecthomas/participle/v2/lexer"
)

// ConfFile represents a parsed SWORD .conf file.
type ConfFile struct {
	Lines []ConfLine `@@*`
}

// ConfLine represents a single meaningful line in a conf file.
type ConfLine struct {
	Section  string `  @Section`
	Property string `| @Property`
}

// confLexer defines tokens for SWORD .conf files using line-based patterns.
// Order matters: more specific patterns should come first.
var confLexer = lexer.MustSimple([]lexer.SimpleRule{
	// Comment lines (full line starting with #)
	{Name: "Comment", Pattern: `#[^\r\n]*`},
	// Section header line: [SectionName]
	{Name: "Section", Pattern: `\[[^\]\r\n]+\]`},
	// Property line: Key=Value (keys can contain letters, digits, underscores, dots)
	// Examples: Description=, ModDrv=, History_1.0=, MinimumVersion=
	{Name: "Property", Pattern: `[a-zA-Z][a-zA-Z0-9_.]*=[^\r\n]*`},
	// Continuation lines (RTF escapes like \par, or any line not starting with [, #, or a property)
	// These are multi-line value continuations or garbage we ignore
	{Name: "Continuation", Pattern: `\\[^\r\n]*`},
	// Whitespace (spaces/tabs)
	{Name: "Whitespace", Pattern: `[ \t]+`},
	// Newlines
	{Name: "Newline", Pattern: `[\r\n]+`},
})

// confParser is the Participle parser for SWORD .conf files.
var confParser = participle.MustBuild[ConfFile](
	participle.Lexer(confLexer),
	participle.Elide("Comment", "Whitespace", "Newline", "Continuation"),
)

// parseConfString parses a SWORD .conf file from a string.
func parseConfString(input string) (*ConfFile, error) {
	return confParser.ParseString("", input)
}

// parseConfBytes parses a SWORD .conf file from bytes.
func parseConfBytes(input []byte) (*ConfFile, error) {
	return confParser.ParseBytes("", input)
}

// ToModule converts a parsed ConfFile to a Module struct.
func (cf *ConfFile) ToModule(confPath string) *Module {
	module := &Module{
		ConfPath:            confPath,
		Features:            []string{},
		GlobalOptionFilters: []string{},
	}

	for _, line := range cf.Lines {
		// Handle section header: [SectionName]
		if line.Section != "" {
			// Strip brackets: [Name] -> Name
			name := strings.TrimPrefix(line.Section, "[")
			name = strings.TrimSuffix(name, "]")
			module.ID = strings.ToLower(name)
			continue
		}

		// Handle property: Key=Value
		if line.Property != "" {
			// Split on first "="
			idx := strings.Index(line.Property, "=")
			if idx < 0 {
				continue
			}
			key := line.Property[:idx]
			value := strings.TrimSpace(line.Property[idx+1:])

			switch key {
			case "Description":
				module.Title = value
			case "About":
				module.About = parseAboutText(value)
			case "ModDrv":
				module.Driver = ModuleDriver(value)
				module.ModuleType = driverToModuleType(module.Driver)
			case "SourceType":
				module.SourceType = SourceType(value)
			case "Lang":
				module.Language = value
			case "Versification":
				module.Versification = value
			case "DataPath":
				module.DataPath = value
			case "CompressType":
				module.CompressType = value
			case "BlockType":
				module.BlockType = value
			case "Encoding":
				module.Encoding = value
			case "Version":
				module.Version = value
			case "SwordVersionDate":
				module.SwordVersionDate = value
			case "Copyright":
				module.Copyright = value
			case "DistributionLicense":
				module.DistributionLicense = value
			case "Category":
				module.Category = value
			case "LCSH":
				module.LCSH = value
			case "MinimumVersion":
				module.MinimumVersion = value
			case "Feature":
				module.Features = append(module.Features, value)
			case "GlobalOptionFilter":
				module.GlobalOptionFilters = append(module.GlobalOptionFilters, value)
			}
		}
	}

	// Generate description from About if not set
	if module.Description == "" && module.About != "" {
		module.Description = truncateDescription(module.About, 200)
	}

	return module
}
