package ping

import (
	"context"
	"fmt"
	"log"

	"github.com/calypr/calypr-cli/g3client"
	"github.com/calypr/forge/client"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

var PingCmd = &cobra.Command{
	Use:   "ping <profile>",
	Short: "Ping Calypr instance and return user's project and user permissions",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Using profile: %s\n", args[0])

		sc, closer, err := client.NewGen3Client(args[0], g3client.WithClients(g3client.FenceClient))
		if err != nil {
			return err
		}
		defer closer()

		resp, err := sc.Gen3.FenceClient().UserPing(context.Background())
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
