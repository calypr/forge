package commit

import (
	comm "github.com/calypr/forge/commit"
	"github.com/spf13/cobra"
)

var PreCommitCmd = &cobra.Command{
	Use:   "pre-commit",
	Short: "Prepare Metadata files for Commit",
	Long: `Validates META directory and creates metadata snapshot.
Designed to be a pre-commit hook that runs before git commit.`,
	Example: "forge commit",
	RunE: func(cmd *cobra.Command, args []string) error {

		cObj, err := comm.NewCommitObj()
		if err != nil {
			return err
		}
		return cObj.RunPreCommit()
	},
}
