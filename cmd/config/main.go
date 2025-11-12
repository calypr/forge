package config

import (
	"github.com/calypr/forge/config"
	"github.com/spf13/cobra"
)

var ConfigCmd = &cobra.Command{
	Use:     "config",
	Short:   "Autogenerate metadata based off of files that have been uploaded",
	Long:    `Not needed for expected user workflow. Useful for debugging server side operations only.`,
	Example: "forge meta",
	RunE: func(cmd *cobra.Command, args []string) error {
		err := config.RunConfigInit()
		if err != nil {
			return err
		}
		return nil
	},
}
