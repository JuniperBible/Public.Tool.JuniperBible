module github.com/FocuswithJustin/juniper

go 1.24.13

require (
	github.com/alecthomas/kong v1.13.0
	github.com/alecthomas/participle/v2 v2.1.4
	github.com/jlaffaye/ftp v0.2.0
	github.com/mattn/go-sqlite3 v1.14.24
	github.com/spf13/cobra v1.10.2
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/hashicorp/errwrap v1.0.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
)

replace (
	github.com/alecthomas/kong => github.com/FocuswithJustin/kong v1.13.0
	github.com/alecthomas/participle/v2 => github.com/FocuswithJustin/participle/v2 v2.1.1
)
