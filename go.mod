module github.com/zoobzio/pipz

go 1.24

toolchain go1.25.0

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

require github.com/zoobzio/clockz v1.0.0

require github.com/zoobzio/capitan v1.0.0

require github.com/google/uuid v1.6.0
