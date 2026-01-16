module github.com/FocuswithJustin/JuniperBible

go 1.25.4

require (
	github.com/alecthomas/kong v1.13.0
	github.com/ulikunitz/xz v0.5.15
	github.com/zeebo/blake3 v0.2.4
)

require (
	github.com/alecthomas/participle/v2 v2.0.0-00010101000000-000000000000
	github.com/antchfx/xmlquery v1.5.0
	github.com/antchfx/xpath v1.3.5
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.3
	github.com/mattn/go-sqlite3 v1.14.33 // optional: CGO SQLite in contrib/sqlite-external (build with -tags cgo_sqlite)
)

require (
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
)

// Use forked repositories for custom functionality
// SECURITY: These forks must be kept in sync with upstream to receive security updates
// See docs/FORKED_DEPENDENCIES.md for fork rationale and maintenance schedule
// Last upstream sync: 2026-01-09
replace github.com/alecthomas/kong => github.com/FocuswithJustin/kong v1.13.0

replace github.com/alecthomas/participle/v2 => github.com/FocuswithJustin/participle/v2 v2.1.4
