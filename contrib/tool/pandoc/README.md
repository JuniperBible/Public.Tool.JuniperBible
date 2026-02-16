# Pandoc

Universal document converter for markup formats.

## Formats Supported

Pandoc is the reference tool for these Juniper Bible format plugins:

- format-markdown (Markdown)
- format-html (HTML)
- format-epub (EPUB)
- format-rtf (RTF input)
- format-odf (ODT/ODF)
- format-txt (plain text)

## Installation

### NixOS

```nix
# Add to your configuration.nix
environment.systemPackages = with pkgs; [
  pandoc
];
```

Or use the provided derivation:
```sh
nix-build nixos/default.nix
```

### Other Systems

```sh
# Debian/Ubuntu
sudo apt install pandoc

# macOS
brew install pandoc

# Windows (chocolatey)
choco install pandoc
```

## Usage

```sh
# Convert Markdown to HTML
pandoc input.md -o output.html

# Convert to EPUB
pandoc input.md -o output.epub --epub-metadata=metadata.xml

# Convert to ODT
pandoc input.md -o output.odt

# Convert to plain text
pandoc input.html -o output.txt

# Bible text conversion (OSIS to other formats)
pandoc bible.xml -f osis -o bible.html
```

## Reference

- Homepage: https://pandoc.org/
- Repository: https://github.com/jgm/pandoc
- Documentation: https://pandoc.org/MANUAL.html
- License: GPL-2.0-or-later

## Version

Juniper Bible targets Pandoc 3.x for behavioral testing.
