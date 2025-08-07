package initialize

import (
	"os"
	"path/filepath"

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
	Short: "create metadata upload job for FHIR ndjson files",
	Long:  `The 'push' command is how metadata is handled in calypr.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := drsInit.Init("gen3", apiEndpoint, bucket, credFile, fenceToken, profile, project, terraProject)
		if err != nil {
			return err
		}
		preCommitPath := filepath.Join(".git", "hooks", "pre-commit")
		file, err := os.OpenFile(preCommitPath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0755)
		if err != nil {
			return err
		}
		defer file.Close()
		if _, err := file.WriteString("forge pre-commit\n"); err != nil {
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
