package push

import (
	"fmt"

	"github.com/calypr/forge/push"
	"github.com/spf13/cobra"
)

var dirPath string

var PushCmd = &cobra.Command{
	Use:   "push",
	Short: "create metadata upload job for FHIR ndjson files",
	Long:  `The 'push' command is how metadata is handled in calypr.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Searching for FHIR metadata files in %s\n", dirPath)
		err := push.RunPush(dirPath)
		if err != nil {
			return err
		}
		return nil
	},
}

func init() {
	PushCmd.PersistentFlags().StringVarP(&dirPath, "dir", "d", ".drs/META", "Directory path to traverse for .meta files")
}
