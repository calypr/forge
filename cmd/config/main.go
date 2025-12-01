package config

import (
	"github.com/calypr/forge/config"
	conf "github.com/calypr/git-drs/config"
	"github.com/spf13/cobra"
)

var ConfigCmd = &cobra.Command{
	Use:     "config [remote]",
	Short:   "Build skeleton template for CALYPR explorer page config.",
	Long:    `Used for creating a template CALYPR explorer config to build and customize your own config`,
	Example: "forge config local",
	RunE: func(cmd *cobra.Command, args []string) error {
		var remote string
		if len(args) > 0 {
			remote = args[0]
		} else {
			remote = ""
		}
		err := config.RunConfigInit(conf.Remote(remote))
		if err != nil {
			return err
		}
		return nil
	},
}
