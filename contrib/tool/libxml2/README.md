# libxml2 (xmllint, xsltproc)

XML parsing and transformation toolkit.

## Formats Supported

libxml2 tools are references for these Juniper Bible format plugins:

- format-osis (OSIS XML)
- format-usx (USX XML)
- format-zefania (Zefania XML)
- format-tei (TEI XML)
- format-xml (Generic XML Bible)
- format-dbl (Digital Bible Library metadata)

## Installation

### NixOS

```nix
# Add to your configuration.nix
environment.systemPackages = with pkgs; [
  libxml2
  libxslt
];
```

Or use the provided derivation:
```sh
nix-build nixos/default.nix
```

### Other Systems

```sh
# Debian/Ubuntu
sudo apt install libxml2-utils xsltproc

# macOS
brew install libxml2 libxslt

# Windows
# Use WSL or download from http://xmlsoft.org/
```

## Usage

### xmllint (XML validation and formatting)

```sh
# Validate XML against schema
xmllint --schema osis.xsd bible.xml --noout

# Validate against RelaxNG
xmllint --relaxng tei.rng bible.xml --noout

# Format/pretty-print XML
xmllint --format bible.xml > formatted.xml

# Extract with XPath
xmllint --xpath "//verse[@osisID='Gen.1.1']" bible.xml

# Check well-formedness
xmllint --noout bible.xml
```

### xsltproc (XSLT transformations)

```sh
# Apply XSLT transformation
xsltproc osis-to-html.xsl bible.xml > bible.html

# With parameters
xsltproc --stringparam lang "en" transform.xsl input.xml

# Output to file
xsltproc -o output.xml transform.xsl input.xml
```

## OSIS Schema Validation

```sh
# Download OSIS schema
wget https://www.crosswire.org/osis/osisCore.2.1.1.xsd

# Validate OSIS file
xmllint --schema osisCore.2.1.1.xsd bible.osis.xml --noout
```

## Reference

- Homepage: http://xmlsoft.org/
- Repository: https://gitlab.gnome.org/GNOME/libxml2
- Documentation: http://xmlsoft.org/html/index.html
- License: MIT

## Version

Juniper Bible targets libxml2 2.9+ for behavioral testing.
