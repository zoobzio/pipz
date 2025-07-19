module github.com/zoobzio/pipz/cmd

go 1.23

toolchain go1.24.5

require (
	github.com/spf13/cobra v1.8.0
	github.com/zoobzio/pipz v0.5.1
	github.com/zoobzio/pipz/examples/security v0.0.0-00010101000000-000000000000
	github.com/zoobzio/pipz/examples/validation v0.0.0-00010101000000-000000000000
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
)

replace github.com/zoobzio/pipz => ..

replace github.com/zoobzio/pipz/examples/validation => ../examples/validation

replace github.com/zoobzio/pipz/examples/security => ../examples/security
