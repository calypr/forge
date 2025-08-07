package meta

import (
	"fmt"

	"github.com/calypr/forge/metadata"
	"github.com/spf13/cobra"
)

var dirPath string
var outPath string
var rebuild bool

var MetaCmd = &cobra.Command{
	Use:   "meta",
	Short: "Tools for managing metadata within Forge projects.",
	Long: `The 'meta' command group provides specialized operations for
initializing, checking the status, and interacting with metadata.`,
	Example: "forge meta ./TEST ./META",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Searching for %s files in: %s\n", metadata.FILE_META_EXT, dirPath)
		err := metadata.RunMetaInit(dirPath, outPath, rebuild)
		if err != nil {
			return err
		}
		return nil
	},
}

func init() {
	MetaCmd.PersistentFlags().StringVarP(&dirPath, "dir", "d", ".", "Directory path to traverse for .meta files")
	MetaCmd.PersistentFlags().StringVarP(&outPath, "out", "o", metadata.META_DIR, "Directory path to output FHIR .ndjson files")
	MetaCmd.PersistentFlags().BoolVarP(&rebuild, "rebuild", "r", false, "Force rebuild metadata files even if no new .meta files exist")

}
