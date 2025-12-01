package empty

import (
	"fmt"

	"github.com/calypr/forge/publish"
	"github.com/calypr/git-drs/config"
	"github.com/spf13/cobra"
)

var EmptyCmd = &cobra.Command{
	Use:   "empty <project-id> [remote]",
	Short: "empty metadata for a project",
	Long:  `The 'empty' command is how metadata is removed in calypr.`,
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectID := args[0]
		var remote config.Remote
		if len(args) == 2 {
			remote = config.Remote(args[1])
			fmt.Printf("Using remote: %s\n", remote)
		} else {
			remote = config.Remote("")
			fmt.Printf("Using default remote: %s\n", remote)
		}
		resp, err := publish.RunEmpty(projectID, remote)
		if err != nil {
			return err
		}
		fmt.Printf("Uid: %s\t Name: %s\t Status: %s\n", resp.Uid, resp.Name, resp.Status)
		return nil
	},
}
