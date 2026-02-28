// Package testdata provides test fixtures for juniper testing.
//
// This file contains embedded test data for unit and integration tests.
// Golden files for regression testing are stored in the golden/ subdirectory.
package testdata

// OSISFixtures contains sample OSIS markup for testing.
var OSISFixtures = map[string]struct {
	Input    string
	Expected string
	Desc     string
}{
	"plain_verse": {
		Input:    `In the beginning God created the heaven and the earth.`,
		Expected: `In the beginning God created the heaven and the earth.`,
		Desc:     "Plain text passthrough",
	},
	"strongs_single": {
		Input:    `In the beginning <w lemma="strong:H7225">beginning</w> God created`,
		Expected: `In the beginning beginning^H7225^ God created`,
		Desc:     "Single Strong's number",
	},
	"strongs_multiple": {
		Input:    `<w lemma="strong:H430">God</w> <w lemma="strong:H1254">created</w>`,
		Expected: `God^H430^ created^H1254^`,
		Desc:     "Multiple Strong's numbers",
	},
	"divine_name": {
		Input:    `And <divineName>LORD</divineName> God said`,
		Expected: `And <span class="divine-name">LORD</span> God said`,
		Desc:     "Divine name formatting",
	},
	"red_letter": {
		Input:    `Jesus said, <q who="Jesus" marker="">Verily I say unto you</q>`,
		Expected: `Jesus said, <span class="red-letter">Verily I say unto you</span>`,
		Desc:     "Red letter (words of Christ)",
	},
	"poetry_line": {
		Input:    `<l>The heavens declare the glory of God;</l>`,
		Expected: `The heavens declare the glory of God;`,
		Desc:     "Poetry line (no indentation in current impl)",
	},
	"poetry_selah": {
		Input:    `<l>Praise him with the timbrel and dance</l> <selah>Selah</selah>`,
		Expected: `Praise him with the timbrel and dance Selah`,
		Desc:     "Poetry with Selah",
	},
	"note_footnote": {
		Input:    `the earth was without form<note type="x-footnote">Or, waste</note>`,
		Expected: `the earth was without form`,
		Desc:     "Footnote stripped (notes collected separately)",
	},
	"title_heading": {
		Input:    `<title>The Creation</title>In the beginning`,
		Expected: "### The Creation In the beginning",
		Desc:     "Section heading (h3)",
	},
	"paragraph_break": {
		Input:    `first verse.</p><p>Second verse`,
		Expected: "first verse.Second verse",
		Desc:     "Paragraph tags stripped",
	},
	"nested_elements": {
		Input:    `<q who="Jesus"><w lemma="strong:G3004">said</w> unto them</q>`,
		Expected: `<span class="red-letter">said^G3004^ unto them</span>`,
		Desc:     "Nested Strong's in red letter",
	},
	"morphology": {
		Input:    `<w lemma="strong:H1254" morph="robinson:V-QAL-P-3MS">created</w>`,
		Expected: `created^H1254^`,
		Desc:     "Morphology stripped, Strong's preserved",
	},
	"transchange_added": {
		Input:    `God <transChange type="added">had</transChange> created`,
		Expected: `God *had* created`,
		Desc:     "Translator additions italicized",
	},
	"complex_verse": {
		Input:    `<title>Genesis 1</title><p>In <w lemma="strong:H7225">the beginning</w> <divineName>God</divineName> <w lemma="strong:H1254">created</w> the heaven and the earth.</p>`,
		Expected: "### Genesis 1 In the beginning^H7225^ <span class=\"divine-name\">God</span> created^H1254^ the heaven and the earth.",
		Desc:     "Complex verse with multiple elements",
	},
}

// ThMLFixtures contains sample ThML markup for testing.
var ThMLFixtures = map[string]struct {
	Input    string
	Expected string
	Desc     string
}{
	"scripture_ref": {
		Input:    `See <scripRef passage="John 3:16">John 3:16</scripRef> for more.`,
		Expected: `See [John 3:16](bible://John%203:16) for more.`,
		Desc:     "Scripture reference link",
	},
	"note_inline": {
		Input:    `the text<note>A footnote here</note> continues`,
		Expected: `the text[^1] continues`,
		Desc:     "Inline note to footnote",
	},
	"heading_div": {
		Input:    `<div3 title="Chapter 1"><p>Content here</p></div3>`,
		Expected: "### Chapter 1\n\nContent here",
		Desc:     "Division heading",
	},
	"emphasis_bold": {
		Input:    `This is <b>important</b> text`,
		Expected: `This is **important** text`,
		Desc:     "Bold emphasis",
	},
	"emphasis_italic": {
		Input:    `This is <i>emphasized</i> text`,
		Expected: `This is *emphasized* text`,
		Desc:     "Italic emphasis",
	},
	"foreign_text": {
		Input:    `The Greek word <foreign lang="grc">λόγος</foreign> means`,
		Expected: `The Greek word *λόγος* means`,
		Desc:     "Foreign language text",
	},
	"strongs_sync": {
		Input:    `<sync type="Strongs" value="G3056"/>word`,
		Expected: `word^G3056^`,
		Desc:     "Strong's number sync",
	},
	"added_text": {
		Input:    `God <added>had</added> created`,
		Expected: `God *had* created`,
		Desc:     "Added/supplied text",
	},
}

// GBFFixtures contains sample GBF (General Bible Format) markup for testing.
var GBFFixtures = map[string]struct {
	Input    string
	Expected string
	Desc     string
}{
	"red_letter": {
		Input:    `<FR>Verily I say unto you<Fr>`,
		Expected: `<span class="words-of-christ">Verily I say unto you</span>`,
		Desc:     "Red letter text",
	},
	"strongs_hebrew": {
		Input:    `God<WH430>`,
		Expected: `God^H430^`,
		Desc:     "Hebrew Strong's number",
	},
	"strongs_greek": {
		Input:    `word<WG3056>`,
		Expected: `word^G3056^`,
		Desc:     "Greek Strong's number",
	},
	"footnote": {
		Input:    `the earth<RF>Or, waste<Rf>`,
		Expected: `the earth[^1]`,
		Desc:     "Footnote",
	},
	"cross_reference": {
		Input:    `heaven<RX Gen.1.1>`,
		Expected: `heaven [cf. Gen.1.1]`,
		Desc:     "Cross-reference",
	},
	"italic_added": {
		Input:    `God <FI>had<Fi> created`,
		Expected: `God *had* created`,
		Desc:     "Italic/added text",
	},
	"bold": {
		Input:    `<FB>Important<Fb> word`,
		Expected: `**Important** word`,
		Desc:     "Bold text",
	},
	"paragraph": {
		Input:    `First verse.<CM>Second verse.`,
		Expected: "First verse.\n\nSecond verse.",
		Desc:     "Paragraph marker",
	},
	"poetry_line": {
		Input:    `<PI>Praise the LORD<PI>`,
		Expected: `  Praise the LORD`,
		Desc:     "Poetry indentation",
	},
	"combined": {
		Input:    `<FR>Verily<Fr> I <FI>say<Fi> unto<WG3004> you`,
		Expected: `<span class="words-of-christ">Verily</span> I *say* unto^G3004^ you`,
		Desc:     "Combined formatting",
	},
}

// TEIFixtures contains sample TEI (Text Encoding Initiative) markup for testing.
var TEIFixtures = map[string]struct {
	Input    string
	Expected string
	Desc     string
}{
	"entry_basic": {
		Input:    `<entry><form><orth>λόγος</orth></form><sense>word, speech</sense></entry>`,
		Expected: "**λόγος**\n\nword, speech",
		Desc:     "Basic dictionary entry",
	},
	"etymology": {
		Input:    `<entry><etym>from <mentioned>λέγω</mentioned></etym></entry>`,
		Expected: "\n*Etymology:* from λέγω",
		Desc:     "Etymology section",
	},
	"pronunciation": {
		Input:    `<entry><pron>lógos</pron></entry>`,
		Expected: " /lógos/",
		Desc:     "Pronunciation",
	},
	"part_of_speech": {
		Input:    `<entry><gramGrp><pos>noun</pos><gen>masculine</gen></gramGrp></entry>`,
		Expected: " (*noun, masculine*)",
		Desc:     "Part of speech and gender",
	},
	"multiple_senses": {
		Input:    `<entry><sense n="1">word</sense><sense n="2">reason</sense></entry>`,
		Expected: "\n1. word\n2. reason",
		Desc:     "Multiple sense definitions",
	},
	"cross_reference": {
		Input:    `<entry><xr>See also <ref target="G3004">λέγω</ref></xr></entry>`,
		Expected: "\n*See also:* λέγω (G3004)",
		Desc:     "Cross-reference",
	},
	"usage_note": {
		Input:    `<entry><note type="usage">Common in philosophical texts</note></entry>`,
		Expected: "\n*Usage:* Common in philosophical texts",
		Desc:     "Usage note",
	},
	"hebrew_entry": {
		Input:    `<entry><form><orth>אֱלֹהִים</orth></form><sense>God, gods</sense></entry>`,
		Expected: "**אֱלֹהִים**\n\nGod, gods",
		Desc:     "Hebrew dictionary entry",
	},
}

// VerseFixtures contains sample verses for parser testing.
var VerseFixtures = map[string]struct {
	Reference string
	Text      string
	Book      int
	Chapter   int
	Verse     int
}{
	"genesis_1_1": {
		Reference: "Gen.1.1",
		Text:      "In the beginning God created the heaven and the earth.",
		Book:      1,
		Chapter:   1,
		Verse:     1,
	},
	"john_3_16": {
		Reference: "John.3.16",
		Text:      "For God so loved the world, that he gave his only begotten Son, that whosoever believeth in him should not perish, but have everlasting life.",
		Book:      43,
		Chapter:   3,
		Verse:     16,
	},
	"psalm_23_1": {
		Reference: "Ps.23.1",
		Text:      "The LORD is my shepherd; I shall not want.",
		Book:      19,
		Chapter:   23,
		Verse:     1,
	},
	"proverbs_3_5": {
		Reference: "Prov.3.5",
		Text:      "Trust in the LORD with all thine heart; and lean not unto thine own understanding.",
		Book:      20,
		Chapter:   3,
		Verse:     5,
	},
	"revelation_22_21": {
		Reference: "Rev.22.21",
		Text:      "The grace of our Lord Jesus Christ be with you all. Amen.",
		Book:      66,
		Chapter:   22,
		Verse:     21,
	},
}

// BookFixtures contains book metadata for testing.
var BookFixtures = []struct {
	OSIS     string
	Name     string
	Chapters int
	Index    int
}{
	{"Gen", "Genesis", 50, 1},
	{"Exod", "Exodus", 40, 2},
	{"Lev", "Leviticus", 27, 3},
	{"Ps", "Psalms", 150, 19},
	{"Prov", "Proverbs", 31, 20},
	{"Matt", "Matthew", 28, 40},
	{"John", "John", 21, 43},
	{"Rev", "Revelation", 22, 66},
}

// ModuleConfFixtures contains sample module configuration data.
var ModuleConfFixtures = map[string]string{
	"kjv": `[KJV]
DataPath=./modules/texts/ztext/kjv/
ModDrv=zText
SourceType=OSIS
Encoding=UTF-8
Lang=en
Description=King James Version (1769) with Strongs Numbers and Morphology
Version=2.9
MinimumVersion=1.5.9
SwordVersionDate=2014-07-21
TextSource=Textus Receptus (Online Bible)
LCSH=Bible. English.
DistributionLicense=Public Domain
Category=Biblical Texts
Feature=StrongsNumbers
GlobalOptionFilter=OSISStrongs
GlobalOptionFilter=OSISMorph
GlobalOptionFilter=OSISFootnotes
GlobalOptionFilter=OSISHeadings
GlobalOptionFilter=OSISRedLetterWords
`,
	"nave": `[Nave]
DataPath=./modules/lexdict/zld/nave/
ModDrv=zLD
SourceType=ThML
Encoding=UTF-8
Lang=en
Description=Nave's Topical Bible
About=Nave's Topical Bible\par Originally compiled by Orville J. Nave
Version=1.1
LCSH=Bible--Encyclopedias
DistributionLicense=Public Domain
Category=Lexicons / Dictionaries
`,
	"mhc": `[MHC]
DataPath=./modules/comments/zcom/mhc/
ModDrv=zCom
SourceType=ThML
Encoding=UTF-8
Lang=en
Description=Matthew Henry's Complete Commentary on the Whole Bible
Version=1.0
LCSH=Bible--Commentaries
DistributionLicense=Public Domain
Category=Commentaries
`,
}

// EdgeCaseFixtures contains edge cases for robustness testing.
var EdgeCaseFixtures = map[string]struct {
	Input    string
	Expected string
	Desc     string
}{
	"empty_string": {
		Input:    "",
		Expected: "",
		Desc:     "Empty input",
	},
	"whitespace_only": {
		Input:    "   \t\n  ",
		Expected: "",
		Desc:     "Whitespace-only input",
	},
	"unclosed_tag": {
		Input:    "<w lemma=\"strong:H430\">God",
		Expected: "God^H430^",
		Desc:     "Unclosed XML tag",
	},
	"nested_unclosed": {
		Input:    "<q><w lemma=\"strong:G3004\">said",
		Expected: "said^G3004^",
		Desc:     "Nested unclosed tags",
	},
	"malformed_strongs": {
		Input:    `<w lemma="strong:">word</w>`,
		Expected: "word",
		Desc:     "Empty Strong's number",
	},
	"unicode_hebrew": {
		Input:    `בְּרֵאשִׁית בָּרָא אֱלֹהִים`,
		Expected: `בְּרֵאשִׁית בָּרָא אֱלֹהִים`,
		Desc:     "Hebrew Unicode passthrough",
	},
	"unicode_greek": {
		Input:    `Ἐν ἀρχῇ ἦν ὁ λόγος`,
		Expected: `Ἐν ἀρχῇ ἦν ὁ λόγος`,
		Desc:     "Greek Unicode passthrough",
	},
	"special_chars": {
		Input:    `&amp; &lt; &gt; &quot;`,
		Expected: `& < > "`,
		Desc:     "HTML entity decoding",
	},
	"very_long_verse": {
		Input:    "word " + string(make([]byte, 10000)),
		Expected: "word " + string(make([]byte, 10000)),
		Desc:     "Very long text handling",
	},
	"multiple_spaces": {
		Input:    "In   the    beginning",
		Expected: "In the beginning",
		Desc:     "Multiple space normalization",
	},
}
