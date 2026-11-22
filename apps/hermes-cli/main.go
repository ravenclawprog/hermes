package main

import (
	"fmt"
	"hermes-cli/cmd"
	"os"
)

func main() {
	if err := cmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error at execution: %v\n", err)
		os.Exit(1)
	}
}
