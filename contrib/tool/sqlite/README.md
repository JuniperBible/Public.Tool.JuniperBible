# SQLite

Lightweight, serverless SQL database engine.

## Formats Supported

SQLite is the reference tool for these Juniper Bible format plugins:

- format-sqlite (Generic SQLite Bible databases)
- format-esword (e-Sword .bblx/.cmtx files use SQLite)

## Installation

### NixOS

```nix
# Add to your configuration.nix
environment.systemPackages = with pkgs; [
  sqlite
];
```

Or use the provided derivation:
```sh
nix-build nixos/default.nix
```

### Other Systems

```sh
# Debian/Ubuntu
sudo apt install sqlite3

# macOS
brew install sqlite

# Windows
# Download from https://sqlite.org/download.html
```

## Usage

```sh
# Open a database
sqlite3 bible.db

# Query verses
sqlite3 bible.db "SELECT * FROM verses WHERE book='Gen' AND chapter=1;"

# Export to SQL
sqlite3 bible.db .dump > backup.sql

# Import SQL
sqlite3 new.db < backup.sql

# Export to CSV
sqlite3 -header -csv bible.db "SELECT * FROM verses;" > verses.csv

# Inspect schema
sqlite3 bible.db ".schema"

# List tables
sqlite3 bible.db ".tables"
```

## Common Bible Database Schemas

### Simple verse table
```sql
CREATE TABLE verses (
    book TEXT,
    chapter INTEGER,
    verse INTEGER,
    text TEXT
);
```

### e-Sword compatible
```sql
CREATE TABLE Bible (
    Book INTEGER,
    Chapter INTEGER,
    Verse INTEGER,
    Scripture TEXT
);
```

## Reference

- Homepage: https://sqlite.org/
- Documentation: https://sqlite.org/docs.html
- License: Public Domain

## Version

Juniper Bible targets SQLite 3.x for behavioral testing.
