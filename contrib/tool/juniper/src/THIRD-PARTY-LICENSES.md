# Third-Party Licenses

This project incorporates code and data from third-party sources.
This file provides proper attribution as required by their respective licenses.

---

## SWORD Project / libsword

**Project:** The SWORD Project
**Source:** https://www.crosswire.org/sword/
**License:** GPL-2.0-or-later

Juniper reads and parses SWORD module files. The SWORD file format and module
structure are defined by the SWORD Project.

### GPL-2.0 License Notice

```
Copyright (C) 1997-2023 The SWORD Project

This program is free software; you can redistribute it and/or modify
it under the terms of the GNU General Public License as published by
the Free Software Foundation; either version 2 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
GNU General Public License for more details.
```

Note: Juniper is a clean-room implementation that reads SWORD file formats.
It does not link against or include libsword code.

---

## Bible Module Data

Bible text data extracted using Juniper may be subject to additional licensing
based on the source module:

- **Public Domain Texts** (KJV, DRC, etc.) - No restrictions
- **CrossWire Modules** - See individual module licensing
- **Proprietary Texts** - Require separate licensing

Always verify the license of source SWORD modules before distribution.

---

## OSIS (Open Scripture Information Standard)

**Project:** OSIS
**Source:** https://crosswire.org/osis/
**License:** Public specification

OSIS book identifiers and versification schemes are used for standardized
Bible reference handling.

---

## e-Sword

**Application:** e-Sword
**Source:** https://www.e-sword.net/
**Format:** .bblx, .cmtx, .dctx SQLite databases

Juniper can read e-Sword format databases. e-Sword is a trademark of
Rick Meyers. Module content licensing varies by module.

---

## Versification Data

Versification mappings are derived from:

- CrossWire SWORD versification schemas
- OpenScriptures reference data
- Academic biblical scholarship resources

These mappings are factual data and not subject to copyright.
