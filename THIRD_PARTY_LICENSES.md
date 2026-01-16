# Third-Party Licenses

This file contains license information for all third-party dependencies used by
Juniper Bible. These dependencies are distributed under their
respective licenses as noted below.

Last updated: 2026-01-15

---

## Internal SQLite Implementation (Public Domain)

The `core/sqlite/internal/` directory contains a pure Go implementation of SQLite
based on the SQLite 3.51.2 source code. SQLite is in the public domain.

- **License**: Public Domain
- **URL**: https://sqlite.org/copyright.html
- **Location**: core/sqlite/internal/*
- **Note**: This is a clean-room Go implementation based on the public domain
  SQLite C source code. The original SQLite source code was authored by
  D. Richard Hipp and is released into the public domain.

The SQLite blessing applies:

    May you do good and not evil.
    May you find forgiveness for yourself and forgive others.
    May you share freely, never taking more than you give.

---

## Direct Dependencies

### github.com/alecthomas/kong (v1.13.0)
- **License**: MIT
- **URL**: https://github.com/alecthomas/kong
- **Note**: Used via fork at github.com/FocuswithJustin/kong

### github.com/alecthomas/participle/v2 (v2.1.4)
- **License**: MIT
- **URL**: https://github.com/alecthomas/participle
- **Note**: Used via fork at github.com/FocuswithJustin/participle

### github.com/antchfx/xmlquery (v1.5.0)
- **License**: MIT
- **URL**: https://github.com/antchfx/xmlquery

### github.com/antchfx/xpath (v1.3.5)
- **License**: MIT
- **URL**: https://github.com/antchfx/xpath

### github.com/google/uuid (v1.6.0)
- **License**: BSD-3-Clause
- **URL**: https://github.com/google/uuid

### github.com/gorilla/websocket (v1.5.3)
- **License**: BSD-2-Clause
- **URL**: https://github.com/gorilla/websocket

### github.com/mattn/go-sqlite3 (v1.14.33)
- **License**: MIT
- **URL**: https://github.com/mattn/go-sqlite3
- **Location**: contrib/sqlite-external/
- **Note**: Optional CGO SQLite driver (build with -tags cgo_sqlite)

### github.com/ulikunitz/xz (v0.5.15)
- **License**: BSD-3-Clause
- **URL**: https://github.com/ulikunitz/xz

### github.com/zeebo/blake3 (v0.2.4)
- **License**: CC0-1.0 (Public Domain)
- **URL**: https://github.com/zeebo/blake3

### modernc.org/sqlite (v1.42.2)
- **License**: BSD-3-Clause
- **URL**: https://gitlab.com/cznic/sqlite
- **Status**: OPTIONAL - No longer used by default
- **Note**: Previously used as the pure Go SQLite driver. Replaced by internal
  implementation in core/sqlite/internal/. May still be referenced in go.mod
  for backwards compatibility testing.

---

## Indirect Dependencies

### github.com/dustin/go-humanize (v1.0.1)
- **License**: MIT
- **URL**: https://github.com/dustin/go-humanize

### github.com/golang/groupcache (v0.0.0-20210331224755-41bb18bfe9da)
- **License**: Apache-2.0
- **URL**: https://github.com/golang/groupcache

### github.com/klauspost/cpuid/v2 (v2.3.0)
- **License**: MIT
- **URL**: https://github.com/klauspost/cpuid

### github.com/mattn/go-isatty (v0.0.20)
- **License**: MIT
- **URL**: https://github.com/mattn/go-isatty

### github.com/ncruces/go-strftime (v0.1.9)
- **License**: MIT
- **URL**: https://github.com/ncruces/go-strftime

### github.com/remyoudompheng/bigfft (v0.0.0-20230129092748-24d4a6f8daec)
- **License**: BSD-3-Clause
- **URL**: https://github.com/remyoudompheng/bigfft

### modernc.org/libc (v1.66.10)
- **License**: BSD-3-Clause
- **URL**: https://gitlab.com/cznic/libc

### modernc.org/mathutil (v1.7.1)
- **License**: BSD-3-Clause
- **URL**: https://gitlab.com/cznic/mathutil

### modernc.org/memory (v1.11.0)
- **License**: BSD-3-Clause
- **URL**: https://gitlab.com/cznic/memory

### modernc.org/cc/v4, ccgo/v4, fileutil, gc/v2, goabi0, opt, sortutil, strutil, token
- **License**: BSD-3-Clause
- **URL**: https://gitlab.com/cznic

---

## Go Standard Library Extensions

### golang.org/x/crypto
- **License**: BSD-3-Clause
- **URL**: https://golang.org/x/crypto

### golang.org/x/exp
- **License**: BSD-3-Clause
- **URL**: https://golang.org/x/exp

### golang.org/x/mod
- **License**: BSD-3-Clause
- **URL**: https://golang.org/x/mod

### golang.org/x/net
- **License**: BSD-3-Clause
- **URL**: https://golang.org/x/net

### golang.org/x/sync
- **License**: BSD-3-Clause
- **URL**: https://golang.org/x/sync

### golang.org/x/sys
- **License**: BSD-3-Clause
- **URL**: https://golang.org/x/sys

### golang.org/x/text
- **License**: BSD-3-Clause
- **URL**: https://golang.org/x/text

### golang.org/x/tools
- **License**: BSD-3-Clause
- **URL**: https://golang.org/x/tools

---

## Test Dependencies (not included in binary)

### github.com/alecthomas/assert/v2 (v2.11.0)
- **License**: MIT
- **URL**: https://github.com/alecthomas/assert

### github.com/alecthomas/repr (v0.5.2)
- **License**: MIT
- **URL**: https://github.com/alecthomas/repr

### github.com/google/go-cmp (v0.6.0)
- **License**: BSD-3-Clause
- **URL**: https://github.com/google/go-cmp

### github.com/google/pprof
- **License**: Apache-2.0
- **URL**: https://github.com/google/pprof

### github.com/hexops/gotextdiff (v1.0.3)
- **License**: BSD-3-Clause and Apache-2.0
- **URL**: https://github.com/hexops/gotextdiff

### github.com/yuin/goldmark (v1.4.13)
- **License**: MIT
- **URL**: https://github.com/yuin/goldmark

### github.com/zeebo/assert (v1.1.0)
- **License**: CC0-1.0 (Public Domain)
- **URL**: https://github.com/zeebo/assert

### github.com/zeebo/pcg (v1.0.1)
- **License**: CC0-1.0 (Public Domain)
- **URL**: https://github.com/zeebo/pcg

---

## License Texts

The full license texts for the above dependencies can be found in the Go module
cache or at the respective repository URLs. The most common licenses used are:

- **MIT License**: Permissive license allowing commercial use, modification, and distribution
- **BSD-3-Clause**: Permissive license with attribution requirement
- **BSD-2-Clause**: Simplified BSD license
- **Apache-2.0**: Permissive license with patent grant
- **CC0-1.0**: Public domain dedication

All dependencies are compatible with proprietary use of Juniper Bible.
