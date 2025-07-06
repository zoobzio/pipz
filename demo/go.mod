module pipz/demo

go 1.23.1

require (
	github.com/spf13/cobra v1.8.0
	pipz v0.0.0
	pipz/examples v0.0.0
)

replace pipz => ../

replace pipz/examples => ../examples

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/vmihailenco/msgpack/v5 v5.4.1 // indirect
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
)
