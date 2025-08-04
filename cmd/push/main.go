package push

import (
	"github.com/calypr/forge/push"
	"github.com/spf13/cobra"
)

var PushCmd = &cobra.Command{
	Use:   "push",
	Short: "create metadata upload job for FHIR ndjson files",
	Long:  `The 'push' command is how metadata is handled in calypr.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := push.RunPush()
		if err != nil {
			return err
		}
		return nil
	},
}
