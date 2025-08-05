package cmd

import (
	"github.com/calypr/forge/cmd/commit"
	"github.com/calypr/forge/cmd/meta"
	"github.com/calypr/forge/cmd/ping"
	"github.com/calypr/forge/cmd/push"
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
	RootCmd.AddCommand(meta.MetaCmd)
	RootCmd.AddCommand(validate.ValidateCmd)
	RootCmd.AddCommand(push.PushCmd)
	RootCmd.AddCommand(ping.PingCmd)
	RootCmd.AddCommand(commit.PreCommitCmd)
	RootCmd.AddCommand(commit.PostCommitCmd)

	// Don't show the help menu for that command every time there is an error
	RootCmd.SilenceUsage = true

}
