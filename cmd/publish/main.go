package publish

import (
	"context"
	"fmt"

	"github.com/calypr/calypr-cli/g3client"
	"github.com/calypr/forge/client"
	"github.com/calypr/forge/publish"
	"github.com/spf13/cobra"
)

var (
	publishGitRemote        string
	publishForceLoomRefresh bool
)

var PublishCmd = &cobra.Command{
	Use:   "publish <profile> <github_personal_access_token>",
	Short: "create metadata upload job for FHIR ndjson files",
	Long:  `The 'publish' command is how metadata is handled in calypr.`,
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Using profile: %s\n", args[0])

		resp, err := publish.RunPublishWithOptions(args[1], args[0], publish.Options{
			GitRemoteName:    publishGitRemote,
			ForceLoomRefresh: publishForceLoomRefresh,
		})
		if err != nil {
			return err
		}
		fmt.Printf("Uid: %s \t Name: %s \t Status: %s\n", resp.Uid, resp.Name, resp.Status)
		return nil
	},
}

var (
	listGitRemote string
)

var ListCmd = &cobra.Command{
	Use:   "list <profile>",
	Short: "view all of the jobs currently catalogued in sower",
	Long:  `The 'list' command is how jobs are displayed to the user`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Using profile: %s\n", args[0])
		if listGitRemote != "" {
			fmt.Printf("Using git remote: %s\n", listGitRemote)
		}

		sc, closer, err := client.NewGen3Client(args[0], g3client.WithClients(g3client.SowerClient, g3client.FenceClient))
		if err != nil {
			return err
		}
		defer closer()
		vals, err := sc.Gen3.SowerClient().List(context.Background())
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
	statusGitRemote string
)

var StatusCmd = &cobra.Command{
	Use:   "status <profile> <UID>",
	Short: "view the status of a specific job on sower",
	Long: `The 'status' command is how sower job status is communicated to the user.
	A specific job's UID can be found from running the list command`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Using profile: %s\n", args[0])
		if statusGitRemote != "" {
			fmt.Printf("Using git remote: %s\n", statusGitRemote)
		}

		sc, closer, err := client.NewGen3Client(args[0], g3client.WithClients(g3client.SowerClient, g3client.FenceClient))
		if err != nil {
			return err
		}
		defer closer()

		status, err := sc.Gen3.SowerClient().Status(context.Background(), args[1])
		if err != nil {
			return err
		}
		fmt.Printf("Uid: %s \t Name: %s \t Status: %s\n", status.Uid, status.Name, status.Status)
		return nil
	},
}

var (
	outputGitRemote string
)

var OutputCmd = &cobra.Command{
	Use:   "output <profile> <UID>",
	Short: "view output logs of a specific job on sower",
	Long: `The 'output' command is how sower job output logs are communicated to the user.
	A specific job's UID can be found from running the list command`,
	Args: cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Using profile: %s\n", args[0])
		if outputGitRemote != "" {
			fmt.Printf("Using git remote: %s\n", outputGitRemote)
		}

		sc, closer, err := client.NewGen3Client(args[0], g3client.WithClients(g3client.SowerClient, g3client.FenceClient))
		if err != nil {
			return err
		}
		defer closer()

		output, err := sc.Gen3.SowerClient().Output(context.Background(), args[1])
		if err != nil {
			return err
		}
		fmt.Printf("Logs: %s\n", output.Output)
		return nil
	},
}

func init() {
	PublishCmd.Flags().StringVar(&publishGitRemote, "git-remote", "", "Git remote name for the repository URL (default: origin, or the only configured remote)")
	PublishCmd.Flags().BoolVar(&publishForceLoomRefresh, "force-loom-refresh", false, "Create a fresh Loom generation and rerun semantic profiling for the current Git commit")
	ListCmd.Flags().StringVarP(&listGitRemote, "remote", "r", "", "git remote name when you want to document repo context")
	StatusCmd.Flags().StringVarP(&statusGitRemote, "remote", "r", "", "git remote name when you want to document repo context")
	OutputCmd.Flags().StringVarP(&outputGitRemote, "remote", "r", "", "git remote name when you want to document repo context")
}
