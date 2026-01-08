package empty

import (
	"fmt"

	"github.com/calypr/forge/publish"
	"github.com/calypr/forge/utils/remoteutil"
	"github.com/spf13/cobra"
)

var (
	emptyRemote string
)

var EmptyCmd = &cobra.Command{
	Use:   "empty <project-id>",
	Short: "empty metadata for a project",
	Long:  `The 'empty' command is how metadata is removed in calypr.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectID := args[0]

		remote, err := remoteutil.LoadRemoteOrDefault(emptyRemote)
		if err != nil {
			return fmt.Errorf("could not locate remote: %w", err)
		}
		fmt.Printf("Using remote: %s\n", string(*remote))

		resp, err := publish.RunEmpty(projectID, *remote)
		if err != nil {
			return err
		}
		fmt.Printf("Uid: %s\t Name: %s\t Status: %s\n", resp.Uid, resp.Name, resp.Status)
		return nil
	},
}

func init() {
	EmptyCmd.Flags().StringVarP(&emptyRemote, "remote", "r", "", "target DRS server (default: default_remote)")
}
