package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "hermes",
	Short: "Hermes — CLI-platform for coop work with documents",
	Long:  "Console client for managing, creating and editoring remote docs at real time",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// var verbose bool
}
