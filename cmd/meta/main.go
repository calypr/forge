package meta

import (
	"fmt"
	"os"

	"github.com/calypr/forge/metadata"
	"github.com/calypr/git-drs/config"
	"github.com/spf13/cobra"
)

var (
	outPath   string
	remote    string
	treeDepth int
)

var MetaCmd = &cobra.Command{
	Use:     "meta",
	Short:   "Autogenerate metadata based off of files that have been uploaded",
	Long:    `Not needed for expected user workflow. Useful for debugging server side operations only.`,
	Example: "forge meta",
	Args:    cobra.RangeArgs(0, 1),
	RunE: func(cmd *cobra.Command, args []string) error {

		cfg, err := config.LoadConfig()
		if err != nil {
			return fmt.Errorf("unable to load config: %w", err)
		}

		remoteName, err := cfg.GetRemoteOrDefault(remote)
		if err != nil {
			return fmt.Errorf("could not locate remote: %w", err)
		}

		err = metadata.CreateMeta(outPath, remoteName)
		if err != nil {
			return fmt.Errorf("could not create metadata: %w", err)
		}

		// Display the created tree structure (default depth)
		fmt.Printf("\nMetadata created successfully in %s\n", outPath)
		return metadata.VisualizeTree(os.Stdout, outPath, 2)
	},
}

var TreeCmd = &cobra.Command{
	Use:     "tree",
	Short:   "Visualize the metadata tree structure",
	Long:    `Reads the generated NDJSON files and displays a rough directory tree.`,
	Example: "forge meta tree --depth 2",
	RunE: func(cmd *cobra.Command, args []string) error {
		return metadata.VisualizeTree(os.Stdout, outPath, treeDepth)
	},
}

func init() {
	MetaCmd.AddCommand(TreeCmd)
	MetaCmd.PersistentFlags().StringVarP(&outPath, "out", "o", metadata.META_DIR, "Directory path to output FHIR .ndjson files")
	MetaCmd.PersistentFlags().IntVarP(&treeDepth, "depth", "d", -1, "Maximum depth of the tree to display (-1 for unlimited)")
	MetaCmd.Flags().StringVarP(&remote, "remote", "r", "", "target DRS server (default: default_remote)")
}
