module github.com/zoobzio/pipz/examples/order-processing

go 1.24.0

toolchain go1.24.5

require github.com/zoobzio/pipz v0.0.0

require (
	github.com/zoobzio/clockz v0.0.2 // indirect
	github.com/zoobzio/hookz v0.0.5 // indirect
	github.com/zoobzio/metricz v0.0.2 // indirect
	github.com/zoobzio/tracez v0.0.11 // indirect
)

replace github.com/zoobzio/pipz => ../..
