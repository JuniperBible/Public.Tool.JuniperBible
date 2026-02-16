# GoBible Creator

Tool for creating GoBible applications for J2ME mobile phones.

## Formats Supported

GoBible Creator is the reference tool for:

- format-gobible (GoBible .jar/.jad files)

## About GoBible

GoBible is a Java ME (J2ME) application for reading the Bible on older mobile
phones that support Java. The format packages Bible text into a JAR file that
can be installed on feature phones.

## Installation

GoBible Creator is a Java application. It requires Java Runtime Environment.

### Prerequisites

```sh
# Install Java (NixOS)
nix-shell -p jdk

# Install Java (Debian/Ubuntu)
sudo apt install default-jdk

# Install Java (macOS)
brew install openjdk
```

### Download GoBible Creator

```sh
# Download from SourceForge
wget https://sourceforge.net/projects/gobible/files/GoBibleCreator/GoBibleCreator2.4.0/GoBibleCreator2.4.0.zip

# Extract
unzip GoBibleCreator2.4.0.zip
```

## Usage

```sh
# Create a GoBible application
java -jar GoBibleCreator.jar collections.txt

# The collections.txt file specifies:
# - Source format (OSIS, ThML, USFM, etc.)
# - Books to include
# - Output settings
```

### Example collections.txt

```
# Source file
Source-Text: bible.osis.xml
Source-Format: OSIS

# Output settings
Phone-Icon: icon.png
Application-Name: KJV Bible

# Books to include (use standard abbreviations)
Book: Gen
Book: Exod
Book: Lev
# ... etc
```

## Reference

- SourceForge: https://sourceforge.net/projects/gobible/
- Documentation: https://gobible.jolon.org/
- License: GPL-2.0

## Version

Juniper Bible targets GoBible Creator 2.4.x for behavioral testing.

## Note

GoBible is primarily of historical interest for legacy J2ME devices.
Modern smartphones use other Bible app formats.
