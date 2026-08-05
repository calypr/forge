package meta

import (
	"fmt"
	"log/slog"

	"github.com/calypr/forge/metadata"
	"github.com/spf13/cobra"
)

var (
	outPath   string
	gitRemote string
)

var MetaCmd = &cobra.Command{
	Use:     "meta <profile>",
	Short:   "Autogenerate metadata based off of files that have been uploaded",
	Long:    `Not needed for expected user workflow. Useful for debugging server side operations only.`,
	Example: "forge meta init --remote dev",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		slog.Info("Forge metadata generation started", "profile", args[0], "git_ref", "HEAD", "output", outPath, "remote", gitRemote)
		err := metadata.CreateMeta(outPath, args[0], gitRemote)
		if err != nil {
			return fmt.Errorf("could not create metadata: %w", err)
		}

		// Display the created tree structure (default depth)
		fmt.Printf("\nMetadata created successfully in %s\n", outPath)
		return nil
	},
}

func init() {
	MetaCmd.PersistentFlags().StringVarP(&outPath, "out", "o", metadata.META_DIR, "Path to output FHIR .ndjson files")
	MetaCmd.Flags().StringVarP(&gitRemote, "remote", "r", "", "git remote name for repository URL lookup (default: dev/origin per git config)")
}
