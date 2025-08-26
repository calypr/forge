package initialize

import (
	drsInit "github.com/calypr/git-drs/cmd/initialize"
	"github.com/spf13/cobra"
)

var (
	mode         string
	apiEndpoint  string
	bucket       string
	credFile     string
	fenceToken   string
	profile      string
	project      string
	terraProject string
)

var InitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize repo and server access for forge",
	Long: "Description:" +
		"\n  Initialize repo and server access for git-drs with a gen3 server" +
		"\n   ~ Provide a url, bucket, profile, project ID, and either a credentials file or token",
	Args: cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		err := drsInit.Init("gen3", apiEndpoint, bucket, credFile, fenceToken, profile, project, terraProject)
		if err != nil {
			return err
		}
		return nil
	},
}

func init() {
	InitCmd.Flags().StringVar(&mode, "mode", "gen3", "Use an AnVIL-hosted DRS server rather than a gen3 one. Defaults to false")
	InitCmd.Flags().StringVar(&apiEndpoint, "url", "", "[gen3] Specify the API endpoint of the data commons")
	InitCmd.Flags().StringVar(&bucket, "bucket", "", "[gen3] Specify the bucket name")
	InitCmd.Flags().StringVar(&credFile, "cred", "", "[gen3] Specify the gen3 credential file that you want to use")
	InitCmd.Flags().StringVar(&fenceToken, "token", "", "[gen3] Specify the token to be used as a replacement for a credential file for temporary access")
	InitCmd.Flags().StringVar(&profile, "profile", "", "[gen3] Specify the gen3 profile to use")
	InitCmd.Flags().StringVar(&project, "project", "", "[gen3] Specify the gen3 project ID in the format <program>-<project>")
	InitCmd.Flags().StringVar(&terraProject, "terraProject", "", "[AnVIL] Specify the Terra project ID")
}
