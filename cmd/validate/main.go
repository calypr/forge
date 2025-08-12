package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cockroachdb/errors"

	"github.com/calypr/forge/metadata"
	"github.com/calypr/forge/schema"
	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
)

// Cmd is the declaration of the command line
// A bit of a question how the schema should be fetched.
// Submodule to iceberg with the default path pathing into it is one option.
// Defining it as a go string and loading from the package is another option.
// Curling it as part of a build script is another option.
var ValidateCmd = &cobra.Command{
	Use:   "validate [path]",
	Short: "validate data files given a jsonschema and a ndjson data target file or directory",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]

		sch, err := schema.NewSchema()
		if err != nil {
			return errors.Wrap(err, "failed to create schema")
		}

		fileResults := make(map[string]struct {
			Rows       int
			Errors     int
			FileErrors *multierror.Error // Store the specific errors for this file
		})
		var allErrors = new(multierror.Error)

		// A helper function to validate a single file and update the results
		validateFile := func(filePath string) {
			rows, errorsCount, fileErrors, fileErr := sch.Validate(filePath)
			if fileErr != nil {
				allErrors = multierror.Append(allErrors, errors.Wrapf(fileErr, "validation failed for file %s", filePath))
				return
			}
			fileResults[filePath] = struct {
				Rows       int
				Errors     int
				FileErrors *multierror.Error
			}{Rows: rows, Errors: errorsCount, FileErrors: fileErrors}
		}

		info, err := os.Stat(path)
		if err != nil {
			return errors.Wrapf(err, "could not get file info for %s", path)
		}

		if info.IsDir() {
			err = filepath.WalkDir(path, func(filePath string, d os.DirEntry, err error) error {
				if err != nil {
					return errors.Wrapf(err, "walk error at %s", filePath)
				}
				// Only process regular files with a .ndjson extension
				if !d.IsDir() && strings.HasSuffix(d.Name(), metadata.NDJSON_EXT) {
					validateFile(filePath)
				}
				return nil
			})
			if err != nil {
				allErrors = multierror.Append(allErrors, err)
			}
		} else {
			if !strings.HasSuffix(info.Name(), metadata.NDJSON_EXT) {
				return fmt.Errorf("file %s is not an .ndjson file", path)
			}
			validateFile(path)
		}

		fmt.Printf("\n--- Validation Summary ---\n")
		var totalFilesValidated int
		var totalRowsValidated int
		var totalErrors int

		for file, result := range fileResults {
			fmt.Printf("File: %s\n", file)
			fmt.Printf("  Rows validated: %d\n", result.Rows)
			fmt.Printf("  Errors found: %d\n", result.Errors)
			// Print individual errors for each file
			if result.FileErrors != nil {
				for _, fileError := range result.FileErrors.Errors {
					fmt.Printf("    - %v\n", fileError)
				}
			}
			fmt.Printf("---\n")
			totalFilesValidated++
			totalRowsValidated += result.Rows
			totalErrors += result.Errors
		}

		fmt.Printf("Overall Totals:\n")
		fmt.Printf("  Files validated: %d\n", totalFilesValidated)
		fmt.Printf("  Rows validated: %d\n", totalRowsValidated)
		fmt.Printf("  Errors: %d\n", totalErrors)
		fmt.Printf("--------------------------\n")

		return allErrors.ErrorOrNil()
	},
}

var CheckEdgeCmd = &cobra.Command{
	Use:   "check [path]",
	Short: "given a path to a metadata directory, checks for missing vertices in graph",
	Long:  "generates edges from FHIR .ndjson files then checks wether there exists any edges that reference vertices that do not exist",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]

		sch, err := schema.NewSchema()
		if err != nil {
			return errors.Wrap(err, "failed to create schema")
		}

		info, err := os.Stat(path)
		if err != nil {
			return errors.Wrapf(err, "could not get file info for %s", path)
		}
		if info.IsDir() {
			err = filepath.WalkDir(path, func(filePath string, d os.DirEntry, err error) error {
				if err != nil {
					return errors.Wrapf(err, "walk error at %s", filePath)
				}
				// Only process regular files with a .ndjson extension
				if !d.IsDir() && strings.HasSuffix(d.Name(), metadata.NDJSON_EXT) {
					validateFile(filePath)
				}
				return nil
			})
			if err != nil {
				allErrors = multierror.Append(allErrors, err)
			}
		} else {
			return fmt.Errorf("expecting directory of ndjson files but got %s instead", info.Name())
		}

		return nil
	},
}
