# Calibre

E-book management and conversion toolkit.

## Formats Supported

Calibre is a reference tool for the Juniper Bible format plugin:

- format-epub (EPUB creation and validation)

## Installation

### NixOS

```nix
# Add to your configuration.nix
environment.systemPackages = with pkgs; [
  calibre
];
```

Or use the provided derivation:
```sh
nix-build nixos/default.nix
```

### Other Systems

```sh
# Debian/Ubuntu
sudo apt install calibre

# macOS
brew install calibre

# Windows
# Download installer from https://calibre-ebook.com/download
```

## Usage

```sh
# Convert to EPUB
ebook-convert input.html output.epub

# Convert from EPUB
ebook-convert input.epub output.txt

# Validate EPUB
calibre-debug --run-plugin "EpubCheck" -- input.epub

# Extract EPUB metadata
ebook-meta input.epub

# Modify EPUB metadata
ebook-meta input.epub --title "New Title" --authors "Author Name"
```

## Key Tools

- `ebook-convert` - Format conversion
- `ebook-meta` - Metadata manipulation
- `calibre-debug` - Plugin runner and debugging
- `ebook-viewer` - EPUB viewer

## Reference

- Homepage: https://calibre-ebook.com/
- Repository: https://github.com/kovidgoyal/calibre
- Documentation: https://manual.calibre-ebook.com/
- License: GPL-3.0

## Version

Juniper Bible targets Calibre 6.x/7.x for behavioral testing.
