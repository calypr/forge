package validate

import (
	"fmt"

	"github.com/bmeg/golib"
	"github.com/bytedance/sonic"
	"github.com/calypr/forge/commit"
	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
)

// Cmd is the declaration of the command line
// A bit of a question how the schema should be fetched.
// Submodule to iceberg with the default path pathing into it is one option.
// Defining it as a go string and loading from the package is another option.
// Curling it as part of a build script is another option.
var ValidateCmd = &cobra.Command{
	Use:   "validate [inputFile]",
	Short: "validate data files given a jsonschema and a ndjson data target file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		filePath := args[0]
		comm, err := commit.NewCommitObj()

		count := 0
		allErrors := new(multierror.Error)
		reader, err := golib.ReadFileLines(filePath)
		if err != nil {
			return err
		}
		procChan := make(chan map[string]any, 100)
		go func() {
			for line := range reader {
				if len(line) > 0 {
					var o map[string]any
					sonic.Unmarshal(line, &o)
					procChan <- o
				}
			}
			close(procChan)
		}()
		for row := range procChan {
			count++
			resource, ok := row["resourceType"].(string)
			if !ok {
				return fmt.Errorf("err indexing resourceType on row %s", row)
			}
			err = comm.Sch.Validate(resource, row)
			if err != nil {
				allErrors = multierror.Append(allErrors, fmt.Errorf("failed to validate JSON: (Error: %w)", err))
				continue
			}
		}

		fmt.Printf("Validated %d rows, %d errors\n", count, len(allErrors.Errors))
		return allErrors.ErrorOrNil()
	},
}
