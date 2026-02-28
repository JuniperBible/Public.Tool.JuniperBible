#!/usr/bin/env bash
# Fetch e-Sword modules for SQLite parsing tests
# Clean room approach: all modules stored in vendor_external/esword_modules/

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ESWORD_DIR="${SCRIPT_DIR}/esword_modules"
CACHE_DIR="${ESWORD_DIR}/.cache"

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }

# e-Sword module sources
# Note: e-Sword modules are typically from e-sword.net but require user accounts
# For testing, we create synthetic test databases

mkdir -p "${ESWORD_DIR}"/{bibles,commentaries,dictionaries}
mkdir -p "${CACHE_DIR}"

create_test_bible_db() {
    local db_file="$1"
    local name="$2"

    log_info "Creating test Bible database: ${name}"

    sqlite3 "${db_file}" <<'EOF'
-- e-Sword Bible format (.bblx)
CREATE TABLE IF NOT EXISTS Bible (
    Book INTEGER,
    Chapter INTEGER,
    Verse INTEGER,
    Scripture TEXT,
    PRIMARY KEY (Book, Chapter, Verse)
);

CREATE TABLE IF NOT EXISTS Details (
    Description TEXT,
    Abbreviation TEXT,
    Comments TEXT,
    Version INTEGER,
    Font TEXT,
    RightToLeft INTEGER DEFAULT 0,
    OT INTEGER DEFAULT 1,
    NT INTEGER DEFAULT 1,
    Apocrypha INTEGER DEFAULT 0
);

-- Insert metadata
INSERT INTO Details (Description, Abbreviation, Comments, Version, Font, RightToLeft, OT, NT, Apocrypha)
VALUES ('Test Bible for Unit Testing', 'TEST', 'Synthetic test data', 1, 'Arial', 0, 1, 1, 0);

-- Insert sample verses (Genesis 1:1-5)
INSERT INTO Bible (Book, Chapter, Verse, Scripture) VALUES
    (1, 1, 1, 'In the beginning God created the heaven and the earth.'),
    (1, 1, 2, 'And the earth was without form, and void; and darkness was upon the face of the deep.'),
    (1, 1, 3, 'And God said, Let there be light: and there was light.'),
    (1, 1, 4, 'And God saw the light, that it was good: and God divided the light from the darkness.'),
    (1, 1, 5, 'And God called the light Day, and the darkness he called Night.');

-- Insert sample NT verses (John 1:1-5)
INSERT INTO Bible (Book, Chapter, Verse, Scripture) VALUES
    (43, 1, 1, 'In the beginning was the Word, and the Word was with God, and the Word was God.'),
    (43, 1, 2, 'The same was in the beginning with God.'),
    (43, 1, 3, 'All things were made by him; and without him was not any thing made that was made.'),
    (43, 1, 4, 'In him was life; and the life was the light of men.'),
    (43, 1, 5, 'And the light shineth in darkness; and the darkness comprehended it not.');

-- Insert Psalm 23
INSERT INTO Bible (Book, Chapter, Verse, Scripture) VALUES
    (19, 23, 1, 'The LORD is my shepherd; I shall not want.'),
    (19, 23, 2, 'He maketh me to lie down in green pastures: he leadeth me beside the still waters.'),
    (19, 23, 3, 'He restoreth my soul: he leadeth me in the paths of righteousness for his name''s sake.'),
    (19, 23, 4, 'Yea, though I walk through the valley of the shadow of death, I will fear no evil.'),
    (19, 23, 5, 'Thou preparest a table before me in the presence of mine enemies.'),
    (19, 23, 6, 'Surely goodness and mercy shall follow me all the days of my life.');
EOF

    log_info "Created: ${db_file}"
}

create_test_commentary_db() {
    local db_file="$1"
    local name="$2"

    log_info "Creating test Commentary database: ${name}"

    sqlite3 "${db_file}" <<'EOF'
-- e-Sword Commentary format (.cmtx)
CREATE TABLE IF NOT EXISTS Commentary (
    Book INTEGER,
    ChapterBegin INTEGER,
    VerseBegin INTEGER,
    ChapterEnd INTEGER,
    VerseEnd INTEGER,
    Comments TEXT,
    PRIMARY KEY (Book, ChapterBegin, VerseBegin)
);

CREATE TABLE IF NOT EXISTS Details (
    Description TEXT,
    Abbreviation TEXT,
    Comments TEXT,
    Version INTEGER
);

INSERT INTO Details (Description, Abbreviation, Comments, Version)
VALUES ('Test Commentary for Unit Testing', 'TESTCMT', 'Synthetic test data', 1);

-- Sample commentary entries
INSERT INTO Commentary (Book, ChapterBegin, VerseBegin, ChapterEnd, VerseEnd, Comments) VALUES
    (1, 1, 1, 1, 1, '<p>The opening verse of the Bible declares God as Creator.</p>'),
    (1, 1, 2, 1, 2, '<p>The earth was formless and empty before God''s creative work.</p>'),
    (43, 1, 1, 1, 1, '<p>The Logos, the eternal Word, was with God from the beginning.</p>'),
    (43, 3, 16, 3, 16, '<p>The most famous verse in the Bible, summarizing the Gospel.</p>');
EOF

    log_info "Created: ${db_file}"
}

create_test_dictionary_db() {
    local db_file="$1"
    local name="$2"

    log_info "Creating test Dictionary database: ${name}"

    sqlite3 "${db_file}" <<'EOF'
-- e-Sword Dictionary format (.dctx)
CREATE TABLE IF NOT EXISTS Dictionary (
    Topic TEXT PRIMARY KEY,
    Definition TEXT
);

CREATE TABLE IF NOT EXISTS Details (
    Description TEXT,
    Abbreviation TEXT,
    Comments TEXT,
    Version INTEGER
);

INSERT INTO Details (Description, Abbreviation, Comments, Version)
VALUES ('Test Dictionary for Unit Testing', 'TESTDCT', 'Synthetic test data', 1);

-- Sample dictionary entries
INSERT INTO Dictionary (Topic, Definition) VALUES
    ('God', '<p><b>God</b> - The supreme being, creator of the universe.</p>'),
    ('Jesus', '<p><b>Jesus</b> - The Christ, Son of God, Savior of mankind.</p>'),
    ('Faith', '<p><b>Faith</b> - Trust and belief in God and His promises.</p>'),
    ('Love', '<p><b>Love</b> - The greatest commandment; God''s nature (1 John 4:8).</p>'),
    ('Grace', '<p><b>Grace</b> - Unmerited favor from God toward sinners.</p>');
EOF

    log_info "Created: ${db_file}"
}

create_hebrew_test_db() {
    local db_file="$1"

    log_info "Creating Hebrew test database with RTL text"

    sqlite3 "${db_file}" <<'EOF'
CREATE TABLE IF NOT EXISTS Bible (
    Book INTEGER,
    Chapter INTEGER,
    Verse INTEGER,
    Scripture TEXT,
    PRIMARY KEY (Book, Chapter, Verse)
);

CREATE TABLE IF NOT EXISTS Details (
    Description TEXT,
    Abbreviation TEXT,
    Comments TEXT,
    Version INTEGER,
    Font TEXT,
    RightToLeft INTEGER DEFAULT 1,
    OT INTEGER DEFAULT 1,
    NT INTEGER DEFAULT 0,
    Apocrypha INTEGER DEFAULT 0
);

INSERT INTO Details (Description, Abbreviation, Comments, Version, Font, RightToLeft, OT, NT, Apocrypha)
VALUES ('Hebrew Test Bible', 'HEB', 'Hebrew RTL test data', 1, 'SBL Hebrew', 1, 1, 0, 0);

-- Hebrew text samples (Genesis 1:1-3)
INSERT INTO Bible (Book, Chapter, Verse, Scripture) VALUES
    (1, 1, 1, 'בְּרֵאשִׁית בָּרָא אֱלֹהִים אֵת הַשָּׁמַיִם וְאֵת הָאָרֶץ'),
    (1, 1, 2, 'וְהָאָרֶץ הָיְתָה תֹהוּ וָבֹהוּ וְחֹשֶׁךְ עַל־פְּנֵי תְהוֹם'),
    (1, 1, 3, 'וַיֹּאמֶר אֱלֹהִים יְהִי אוֹר וַיְהִי־אוֹר');
EOF

    log_info "Created: ${db_file}"
}

create_greek_test_db() {
    local db_file="$1"

    log_info "Creating Greek test database"

    sqlite3 "${db_file}" <<'EOF'
CREATE TABLE IF NOT EXISTS Bible (
    Book INTEGER,
    Chapter INTEGER,
    Verse INTEGER,
    Scripture TEXT,
    PRIMARY KEY (Book, Chapter, Verse)
);

CREATE TABLE IF NOT EXISTS Details (
    Description TEXT,
    Abbreviation TEXT,
    Comments TEXT,
    Version INTEGER,
    Font TEXT,
    RightToLeft INTEGER DEFAULT 0,
    OT INTEGER DEFAULT 0,
    NT INTEGER DEFAULT 1,
    Apocrypha INTEGER DEFAULT 0
);

INSERT INTO Details (Description, Abbreviation, Comments, Version, Font, RightToLeft, OT, NT, Apocrypha)
VALUES ('Greek Test Bible', 'GRK', 'Greek NT test data', 1, 'SBL Greek', 0, 0, 1, 0);

-- Greek text samples (John 1:1-3)
INSERT INTO Bible (Book, Chapter, Verse, Scripture) VALUES
    (43, 1, 1, 'Ἐν ἀρχῇ ἦν ὁ λόγος, καὶ ὁ λόγος ἦν πρὸς τὸν θεόν, καὶ θεὸς ἦν ὁ λόγος.'),
    (43, 1, 2, 'οὗτος ἦν ἐν ἀρχῇ πρὸς τὸν θεόν.'),
    (43, 1, 3, 'πάντα δι᾽ αὐτοῦ ἐγένετο, καὶ χωρὶς αὐτοῦ ἐγένετο οὐδὲ ἕν ὃ γέγονεν.');
EOF

    log_info "Created: ${db_file}"
}

generate_checksums() {
    log_info "Generating checksums..."
    cd "${ESWORD_DIR}"
    find . -type f -name "*.bblx" -o -name "*.cmtx" -o -name "*.dctx" | \
        xargs sha256sum > checksums.sha256 2>/dev/null || true
    log_info "Checksums written to ${ESWORD_DIR}/checksums.sha256"
}

# Main execution
log_info "e-Sword Module Generator - Clean Room Setup"
log_info "Target directory: ${ESWORD_DIR}"

# Create test databases
create_test_bible_db "${ESWORD_DIR}/bibles/test.bblx" "Test Bible"
create_test_commentary_db "${ESWORD_DIR}/commentaries/test.cmtx" "Test Commentary"
create_test_dictionary_db "${ESWORD_DIR}/dictionaries/test.dctx" "Test Dictionary"
create_hebrew_test_db "${ESWORD_DIR}/bibles/hebrew.bblx"
create_greek_test_db "${ESWORD_DIR}/bibles/greek.bblx"

generate_checksums

log_info "Done! e-Sword test modules stored in ${ESWORD_DIR}"
log_info "To use in tests: export ESWORD_PATH=${ESWORD_DIR}"
