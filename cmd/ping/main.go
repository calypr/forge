package ping

import (
	"fmt"
	"log"

	"github.com/calypr/forge/client/fence"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

var PingCmd = &cobra.Command{
	Use:   "ping",
	Short: "Ping Calypr instance and return user's project and user permissions",
	RunE: func(cmd *cobra.Command, args []string) error {
		FenceClient, err := fence.NewFenceClient()
		if err != nil {
			return err
		}
		resp, err := FenceClient.UserPing()
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
