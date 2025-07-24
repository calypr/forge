package cmd

import (
	"github.com/calypr/forge/cmd/meta"
	"github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
	Use:   "forge",
	Short: "A powerful command-line tool for project management.",
	Long: `Forge is a versatile CLI application designed to streamline various
development and project management tasks.`,
}

func init() {

	RootCmd.AddCommand(meta.MetaCmd)
}
