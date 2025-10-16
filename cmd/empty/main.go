package empty

import (
	"github.com/calypr/forge/publish"
	"github.com/spf13/cobra"
)

var EmptyCmd = &cobra.Command{
	Use:   "empty <project-id>",
	Short: "empty metadata for a project",
	Long:  `The 'empty' command is how metadata is removed in calypr.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		err := publish.RunEmpty(args[0])
		if err != nil {
			return err
		}
		return nil
	},
}
