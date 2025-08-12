package meta

import (
	"github.com/calypr/forge/metadata"
	"github.com/spf13/cobra"
)

var outPath string

var MetaCmd = &cobra.Command{
	Use:   "meta",
	Short: "Tools for managing metadata within Forge projects.",
	Long: `The 'meta' command group provides specialized operations for
initializing, checking the status, and interacting with metadata.`,
	Example: "forge meta",
	RunE: func(cmd *cobra.Command, args []string) error {
		err := metadata.RunMetaInit(outPath)
		if err != nil {
			return err
		}
		return nil
	},
}

func init() {
	MetaCmd.PersistentFlags().StringVarP(&outPath, "out", "o", metadata.META_DIR, "Directory path to output FHIR .ndjson files")
}
