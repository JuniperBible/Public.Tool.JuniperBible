# UnRTF

RTF to other formats converter.

## Formats Supported

UnRTF is a reference tool for:

- format-rtf (Rich Text Format)

## Installation

### NixOS

```nix
# Add to your configuration.nix
environment.systemPackages = with pkgs; [
  unrtf
];
```

Or use the provided derivation:
```sh
nix-build nixos/default.nix
```

### Other Systems

```sh
# Debian/Ubuntu
sudo apt install unrtf

# macOS
brew install unrtf

# From source
git clone https://github.com/nesbox/unrtf
cd unrtf
./configure && make && sudo make install
```

## Usage

```sh
# Convert RTF to HTML
unrtf --html input.rtf > output.html

# Convert RTF to plain text
unrtf --text input.rtf > output.txt

# Convert RTF to LaTeX
unrtf --latex input.rtf > output.tex

# Show RTF structure (for debugging)
unrtf --verbose input.rtf
```

## Output Formats

- `--html` - HTML output
- `--text` - Plain text output
- `--latex` - LaTeX output
- `--rtf` - RTF output (for debugging/testing)

## Reference

- Repository: https://github.com/nesbox/unrtf
- Original: GNU UnRTF (unmaintained)
- License: GPL-3.0

## Version

Juniper Bible targets UnRTF 0.21+ for behavioral testing.

## Note

For creating RTF files, LibreOffice or Pandoc are recommended.
UnRTF is primarily for reading/converting existing RTF files.
