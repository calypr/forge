package cmd

import (
	"github.com/calypr/forge/cmd/config"
	"github.com/calypr/forge/cmd/empty"
	"github.com/calypr/forge/cmd/meta"
	"github.com/calypr/forge/cmd/ping"
	"github.com/calypr/forge/cmd/publish"
	"github.com/calypr/forge/cmd/validate"
	"github.com/spf13/cobra"
)

var RootCmd = &cobra.Command{
	Use:   "forge",
	Short: "A powerful command-line tool for project management.",
	Long: `Forge is a versatile CLI application designed to streamline various
development and project management tasks.`,
}

func init() {
	// RootCmd.AddCommand(initialize.InitCmd) comment this out for now
	RootCmd.AddCommand(ping.PingCmd)
	RootCmd.AddCommand(meta.MetaCmd)
	RootCmd.AddCommand(publish.PublishCmd)
	RootCmd.AddCommand(empty.EmptyCmd)
	RootCmd.AddCommand(config.ConfigCmd)
	RootCmd.AddCommand(publish.ListCmd)
	RootCmd.AddCommand(publish.StatusCmd)
	RootCmd.AddCommand(publish.OutputCmd)

	validate.ValidateParentCmd.AddCommand(validate.ValidateConfigCmd)
	validate.ValidateParentCmd.AddCommand(validate.ValidateDataCmd)
	validate.ValidateParentCmd.AddCommand(validate.ValidateEdgeCmd)

	RootCmd.AddCommand(validate.ValidateParentCmd)

	RootCmd.SilenceUsage = true // Hide usage on error
}
