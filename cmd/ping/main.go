package ping

import (
	"fmt"
	"log"

	"github.com/calypr/forge/client/fence"
	"github.com/calypr/git-drs/config"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

var PingCmd = &cobra.Command{
	Use:   "ping [remote]",
	Short: "Ping Calypr instance and return user's project and user permissions",
	RunE: func(cmd *cobra.Command, args []string) error {
		var remote config.Remote
		if len(args) > 0 {
			remote = config.Remote(args[0])
		} else {
			remote = config.Remote("origin")
		}

		FenceClient, err := fence.NewFenceClient(remote)
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
