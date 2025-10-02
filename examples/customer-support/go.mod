module github.com/zoobzio/pipz/examples/customer-support

go 1.24.0

toolchain go1.24.5

require github.com/zoobzio/pipz v0.0.0

require (
	github.com/zoobzio/clockz v0.0.2 // indirect
	github.com/zoobzio/hookz v0.0.6 // indirect
	github.com/zoobzio/metricz v0.0.2 // indirect
	github.com/zoobzio/tracez v0.0.12 // indirect
)

replace github.com/zoobzio/pipz => ../..
