package publish

import (
	"context"
	"fmt"

	"github.com/calypr/forge/client"
	"github.com/calypr/forge/publish"
	"github.com/calypr/forge/utils/remoteutil"
	"github.com/spf13/cobra"
)

var (
	publishRemote string
)

var PublishCmd = &cobra.Command{
	Use:   "publish <github_personal_access_token>",
	Short: "create metadata upload job for FHIR ndjson files",
	Long:  `The 'publish' command is how metadata is handled in calypr.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		remote, err := remoteutil.LoadRemoteOrDefault(publishRemote)
		if err != nil {
			return fmt.Errorf("could not locate remote: %w", err)
		}
		fmt.Printf("Using remote: %s\n", string(*remote))

		resp, err := publish.RunPublish(args[0], *remote)
		if err != nil {
			return err
		}
		fmt.Printf("Uid: %s \t Name: %s \t Status: %s\n", resp.Uid, resp.Name, resp.Status)
		return nil
	},
}

var (
	listRemote string
)

var ListCmd = &cobra.Command{
	Use:   "list",
	Short: "view all of the jobs currently catalogued in sower",
	Long:  `The 'list' command is how jobs are displayed to the user`,
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		remote, err := remoteutil.LoadRemoteOrDefault(listRemote)
		if err != nil {
			return fmt.Errorf("could not locate remote: %w", err)
		}
		fmt.Printf("Using remote: %s\n", string(*remote))

		sc, closer, err := client.NewGen3Client(*remote, g3client.WithClients(g3client.SowerClient, g3client.FenceClient))
		if err != nil {
			return err
		}
		defer closer()
		vals, err := sc.GetGen3Interface().Sower().List(context.Background())
		if err != nil {
			return fmt.Errorf("unable to list jobs: %w", err)
		}

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

var (
	statusRemote string
)

var StatusCmd = &cobra.Command{
	Use:   "status <UID>",
	Short: "view the status of a specific job on sower",
	Long: `The 'status' command is how sower job status is communicated to the user.
	A specific job's UID can be found from running the list command`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		remote, err := remoteutil.LoadRemoteOrDefault(statusRemote)
		if err != nil {
			return fmt.Errorf("could not locate remote: %w", err)
		}
		fmt.Printf("Using remote: %s\n", string(*remote))

		sc, closer, err := client.NewGen3Client(*remote, g3client.WithClients(g3client.SowerClient, g3client.FenceClient))
		if err != nil {
			return err
		}
		defer closer()

		status, err := sc.GetGen3Interface().Sower().Status(context.Background(), args[0])
		if err != nil {
			return err
		}
		fmt.Printf("Uid: %s \t Name: %s \t Status: %s\n", status.Uid, status.Name, status.Status)
		return nil
	},
}

var (
	outputRemote string
)

var OutputCmd = &cobra.Command{
	Use:   "output <UID>",
	Short: "view output logs of a specific job on sower",
	Long: `The 'output' command is how sower job output logs are communicated to the user.
	A specific job's UID can be found from running the list command`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		remote, err := remoteutil.LoadRemoteOrDefault(outputRemote)
		if err != nil {
			return fmt.Errorf("could not locate remote: %w", err)
		}
		fmt.Printf("Using remote: %s\n", string(*remote))

		sc, closer, err := client.NewGen3Client(*remote, g3client.WithClients(g3client.SowerClient, g3client.FenceClient))
		if err != nil {
			return err
		}
		defer closer()

		output, err := sc.GetGen3Interface().Sower().Output(context.Background(), args[0])
		if err != nil {
			return err
		}
		fmt.Printf("Logs: %s\n", output.Output)
		return nil
	},
}

func init() {
	PublishCmd.Flags().StringVarP(&publishRemote, "remote", "r", "", "target DRS server (default: default_remote)")
	ListCmd.Flags().StringVarP(&listRemote, "remote", "r", "", "target DRS server (default: default_remote)")
	StatusCmd.Flags().StringVarP(&statusRemote, "remote", "r", "", "target DRS server (default: default_remote)")
	OutputCmd.Flags().StringVarP(&outputRemote, "remote", "r", "", "target DRS server (default: default_remote)")
}
