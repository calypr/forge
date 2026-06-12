package config

import (
	"fmt"

	"github.com/calypr/forge/config"
	"github.com/spf13/cobra"
)

var (
	configGitRemote string
)

var ConfigCmd = &cobra.Command{
	Use:     "config <profile>",
	Short:   "Build skeleton template for CALYPR explorer page config.",
	Long:    `Used for creating a template CALYPR explorer config to build and customize your own config`,
	Example: "forge config init --remote dev",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Using profile: %s\n", args[0])

		err := config.RunConfigInit(configGitRemote)
		if err != nil {
			return err
		}
		return nil
	},
}

func init() {
	ConfigCmd.Flags().StringVarP(&configGitRemote, "remote", "r", "", "git remote name for git-drs project config lookup")
}
