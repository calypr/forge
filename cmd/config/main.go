package config

import (
	"fmt"

	"github.com/calypr/forge/config"
	"github.com/calypr/forge/utils/remoteutil"
	"github.com/spf13/cobra"
)

var (
	configRemote string
)

var ConfigCmd = &cobra.Command{
	Use:     "config",
	Short:   "Build skeleton template for CALYPR explorer page config.",
	Long:    `Used for creating a template CALYPR explorer config to build and customize your own config`,
	Example: "forge config --remote local",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		remote, err := remoteutil.LoadRemoteOrDefault(configRemote)
		if err != nil {
			return fmt.Errorf("could not locate remote: %w", err)
		}
		fmt.Printf("Using remote: %s\n", string(*remote))

		err = config.RunConfigInit(*remote)
		if err != nil {
			return err
		}
		return nil
	},
}

func init() {
	ConfigCmd.Flags().StringVarP(&configRemote, "remote", "r", "", "target DRS server (default: default_remote)")
}
