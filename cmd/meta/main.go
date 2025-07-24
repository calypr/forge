package meta

import (
	"fmt"

	"github.com/calypr/forge/template"
	"github.com/spf13/cobra"
)

var dirPath string

var MetaCmd = &cobra.Command{
	Use:   "meta",
	Short: "Tools for managing metadata within Forge projects.",
	Long: `The 'meta' command group provides specialized operations for
initializing, checking the status, and interacting with metadata.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("Searching for .meta files in: %s\n", dirPath)
		err := template.RunMetaInit(dirPath)
		if err != nil {
			return err
		}
		return nil
	},
}

func init() {
	MetaCmd.PersistentFlags().StringVarP(&dirPath, "dir", "d", ".", "Directory path to traverse for .meta files")
}
