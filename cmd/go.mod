module github.com/zoobzio/pipz/cmd

go 1.23.0

toolchain go1.24.5

require (
	github.com/spf13/cobra v1.8.0
	github.com/zoobzio/pipz v0.5.1
	github.com/zoobzio/pipz/examples/ai v0.0.0-00010101000000-000000000000
	github.com/zoobzio/pipz/examples/etl v0.0.0-00010101000000-000000000000
	github.com/zoobzio/pipz/examples/events v0.0.0-00010101000000-000000000000
	github.com/zoobzio/pipz/examples/middleware v0.0.0-00010101000000-000000000000
	github.com/zoobzio/pipz/examples/moderation v0.0.0-00010101000000-000000000000
	github.com/zoobzio/pipz/examples/payment v0.0.0-00010101000000-000000000000
	github.com/zoobzio/pipz/examples/security v0.0.0-00010101000000-000000000000
	github.com/zoobzio/pipz/examples/validation v0.0.0-00010101000000-000000000000
	github.com/zoobzio/pipz/examples/webhook v0.0.0-00010101000000-000000000000
	github.com/zoobzio/pipz/examples/workflow v0.0.0-00010101000000-000000000000
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
)

replace github.com/zoobzio/pipz => ..

replace github.com/zoobzio/pipz/examples/etl => ../examples/etl

replace github.com/zoobzio/pipz/examples/payment => ../examples/payment

replace github.com/zoobzio/pipz/examples/security => ../examples/security

replace github.com/zoobzio/pipz/examples/validation => ../examples/validation

replace github.com/zoobzio/pipz/examples/webhook => ../examples/webhook

replace github.com/zoobzio/pipz/examples/ai => ../examples/ai

replace github.com/zoobzio/pipz/examples/events => ../examples/events

replace github.com/zoobzio/pipz/examples/middleware => ../examples/middleware

replace github.com/zoobzio/pipz/examples/moderation => ../examples/moderation

replace github.com/zoobzio/pipz/examples/workflow => ../examples/workflow
