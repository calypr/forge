package empty

import (
	"fmt"

	"github.com/calypr/forge/publish"
	"github.com/spf13/cobra"
)

var EmptyCmd = &cobra.Command{
	Use:   "empty <profile> <project-id>",
	Short: "empty metadata for a project",
	Long:  `The 'empty' command is how metadata is removed in calypr.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Using profile: %s\n", args[0])

		resp, err := publish.RunEmpty(args[1], args[0])
		if err != nil {
			return err
		}
		fmt.Printf("Uid: %s\t Name: %s\t Status: %s\n", resp.Uid, resp.Name, resp.Status)
		return nil
	},
}
