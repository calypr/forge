package ping

import (
	"context"
	"fmt"
	"log"

	"github.com/calypr/data-client/g3client"
	"github.com/calypr/forge/client"
	"github.com/calypr/forge/utils/remoteutil"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

var (
	pingRemote string
)

var PingCmd = &cobra.Command{
	Use:   "ping",
	Short: "Ping Calypr instance and return user's project and user permissions",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		remote, err := remoteutil.LoadRemoteOrDefault(pingRemote)
		if err != nil {
			return fmt.Errorf("could not locate remote: %w", err)
		}
		fmt.Printf("Using remote: %s\n", string(*remote))

		sc, closer, err := client.NewGen3Client(*remote, g3client.WithClients(g3client.FenceClient))
		if err != nil {
			return err
		}
		defer closer()

		resp, err := sc.GetGen3Interface().Fence().UserPing(context.Background())
		if err != nil {
			return err
		}

		yamlOutput, err := yaml.Marshal(resp)
		if err != nil {
			log.Fatalf("Error marshaling to YAML: %v", err)
		}
		fmt.Println(string(yamlOutput))

		return nil
	},
}

func init() {
	PingCmd.Flags().StringVarP(&pingRemote, "remote", "r", "", "target DRS server (default: default_remote)")
}
