package commit

import (
	"github.com/calypr/forge/client"
	InternalCommit "github.com/calypr/forge/commit"
	"github.com/spf13/cobra"
)

const metaDir = "META/"
const dotDrsCommitsDir = ".drs/commits"

var CommitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Commit metadata files",
	Long: `Validates META directory and creates metadata snapshot.
Designed to be a pre-commit hook that runs before git commit.`,
	Example: "forge commit",
	RunE: func(cmd *cobra.Command, args []string) error {
		cli, err := client.NewGen3Client()
		if err != nil {
			return err
		}
		cfg := InternalCommit.Config{
			MetaDir:       "META/",
			DrsCommitsDir: ".drs/commits",
			FileExtension: ".ndjson",
			ProjectId:     cli.ProjectId,
		}

		return InternalCommit.RunCommit(cfg)
	},
}
