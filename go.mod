module github.com/FocuswithJustin/JuniperBible

go 1.25.4

require (
	github.com/alecthomas/kong v1.13.0
	github.com/ulikunitz/xz v0.5.15
	github.com/zeebo/blake3 v0.2.4
	modernc.org/sqlite v1.44.1
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
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	golang.org/x/exp v0.0.0-20251023183803-a4bb9ffd2546 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	modernc.org/libc v1.67.6 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
)

// Use forked repositories for custom functionality
// SECURITY: These forks must be kept in sync with upstream to receive security updates
// See docs/FORKED_DEPENDENCIES.md for fork rationale and maintenance schedule
// Last upstream sync: 2026-01-09
replace github.com/alecthomas/kong => github.com/FocuswithJustin/kong v1.13.0

replace github.com/alecthomas/participle/v2 => github.com/FocuswithJustin/participle/v2 v2.1.4
