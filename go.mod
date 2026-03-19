module github.com/JuniperBible/Public.Tool.JuniperBible

go 1.26.1

require (
	github.com/FocuswithJustin/Private.Lib.Veronica v0.0.0
	github.com/alecthomas/kong v1.13.0
	github.com/cyanitol/Public.Lib.Anthony v0.4.2
	github.com/ulikunitz/xz v0.5.15
	github.com/zeebo/blake3 v0.2.4
)

require (
	github.com/alecthomas/participle/v2 v2.0.0-00010101000000-000000000000
	github.com/antchfx/xmlquery v1.5.0
	github.com/antchfx/xpath v1.3.5
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.3
)

require (
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
	google.golang.org/grpc v1.79.2 // indirect
	google.golang.org/protobuf v1.36.10 // indirect
)

// Use forked repositories for custom functionality
// SECURITY: These forks must be kept in sync with upstream to receive security updates
// See docs/FORKED_DEPENDENCIES.md for fork rationale and maintenance schedule
// Last upstream sync: 2026-01-09
replace github.com/alecthomas/kong => github.com/FocuswithJustin/kong v1.13.0

replace github.com/alecthomas/participle/v2 => github.com/FocuswithJustin/participle/v2 v2.1.4

// Use local Private.Lib.Veronica
replace github.com/FocuswithJustin/Private.Lib.Veronica => ../Private.Lib.Veronica
