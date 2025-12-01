package meta

import (
	"github.com/calypr/forge/metadata"
	"github.com/calypr/git-drs/config"
	"github.com/spf13/cobra"
)

var outPath string

var MetaCmd = &cobra.Command{
	Use:     "meta",
	Short:   "Autogenerate metadata based off of files that have been uploaded",
	Long:    `Not needed for expected user workflow. Useful for debugging server side operations only.`,
	Example: "forge meta [remote]",
	Args:    cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {
		err := metadata.RunMetaInit(outPath, config.Remote(args[0]))
		if err != nil {
			return err
		}
		return nil
	},
}

func init() {
	MetaCmd.PersistentFlags().StringVarP(&outPath, "out", "o", metadata.META_DIR, "Directory path to output FHIR .ndjson files")
}
