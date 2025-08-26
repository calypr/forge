package publish

import (
	"github.com/calypr/forge/publish"
	"github.com/spf13/cobra"
)

var PublishCmd = &cobra.Command{
	Use:   "publish <github_personal_access_token>",
	Short: "create metadata upload job for FHIR ndjson files",
	Long:  `The 'publish' command is how metadata is handled in calypr.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		err := publish.RunPublish(args[0])
		if err != nil {
			return err
		}
		return nil
	},
}
