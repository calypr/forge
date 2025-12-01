package publish

import (
	"fmt"

	"github.com/calypr/forge/client/sower"
	"github.com/calypr/forge/publish"
	"github.com/calypr/git-drs/config"
	"github.com/spf13/cobra"
)

var PublishCmd = &cobra.Command{
	Use:   "publish <github_personal_access_token> [remote]",
	Short: "create metadata upload job for FHIR ndjson files",
	Long:  `The 'publish' command is how metadata is handled in calypr.`,
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		var remote config.Remote
		if len(args) == 2 {
			remote = config.Remote(args[1])
			fmt.Printf("Using remote: %s\n", remote)
		} else {
			remote = config.Remote("")
			fmt.Printf("Using default remote: %s\n", remote)
		}
		resp, err := publish.RunPublish(args[0], remote)
		if err != nil {
			return err
		}
		fmt.Printf("Uid: %s \t Name: %s \t Status: %s\n", resp.Uid, resp.Name, resp.Status)
		return nil
	},
}

var ListCmd = &cobra.Command{
	Use:   "list [remote]",
	Short: "view all of the jobs currently catalogued in sower",
	Long:  `The 'list' command is how jobs are displayed to the user`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sc, err := sower.NewSowerClient(config.Remote(args[0]))
		if err != nil {
			return err
		}
		vals, err := sc.List()
		if len(vals) == 0 {
			fmt.Printf("There are no jobs to list: %s\n", vals)
		} else {
			for _, val := range vals {
				fmt.Printf("Uid: %s \t Name: %s \t Status: %s\n", val.Uid, val.Name, val.Status)
			}
		}
		return nil
	},
}

var StatusCmd = &cobra.Command{
	Use:   "status <UID> [remote]",
	Short: "view the status of a specific job on sower",
	Long: `The 'status' command is how sower job status is communicated to the user.
	A specific job's UID can be found from running the list command`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		var remote config.Remote
		if len(args) == 2 {
			remote = config.Remote(args[1])
			fmt.Printf("Using remote: %s\n", remote)
		} else {
			remote = config.Remote("")
			fmt.Printf("Using default remote: %s\n", remote)
		}
		sc, err := sower.NewSowerClient(remote)
		if err != nil {
			return err
		}
		status, err := sc.Status(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("Uid: %s \t Name: %s \t Status: %s\n", status.Uid, status.Name, status.Status)
		return nil
	},
}

var OutputCmd = &cobra.Command{
	Use:   "output <UID> [remote]",
	Short: "view output logs of a specific job on sower",
	Long: `The 'output' command is how sower job output logs are communicated to the user.
	A specific job's UID can be found from running the list command`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var remote config.Remote
		if len(args) == 2 {
			remote = config.Remote(args[1])
			fmt.Printf("Using remote: %s\n", remote)
		} else {
			remote = config.Remote("")
			fmt.Printf("Using default remote: %s\n", remote)
		}
		sc, err := sower.NewSowerClient(remote)
		if err != nil {
			return err
		}
		output, err := sc.Output(args[0])
		if err != nil {
			return err
		}
		fmt.Printf("Logs: %s\n", output.Output)
		return nil
	},
}
