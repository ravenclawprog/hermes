module hermes-cli

go 1.27

require github.com/spf13/cobra v1.10.2

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.9 // indirect
	golang.org/x/sys v0.50.0 // indirect
	golang.org/x/term v0.48.0 // indirect
)

require hermes-messages v0.0.0

replace hermes-messages => ../../packages/hermes-messages
