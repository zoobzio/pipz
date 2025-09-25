module github.com/zoobzio/pipz

go 1.23.2

toolchain go1.24.5

retract (
	v1.0.1 // Not part of the package
	v1.0.0 // Not part of the package
	v0.6.0 // Not part of the package
	v0.5.1 // Not part of the package
	v0.5.0 // Not part of the package
	v0.4.0 // Not part of the package
	v0.3.0 // Not part of the package
	v0.2.1 // Not part of the package
	v0.2.0 // Not part of the package
	v0.1.0 // Not part of the package
	v0.0.1 // Not part of the package
)

require (
	github.com/zoobzio/clockz v0.0.2
	github.com/zoobzio/metricz v0.0.2
	github.com/zoobzio/tracez v0.0.6
)

require github.com/zoobzio/hookz v0.0.3
