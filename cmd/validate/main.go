package validate

import (
	"fmt"

	"github.com/calypr/forge/validator"
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
		v, err := validator.NewJsonSchema()
		if err != nil {
			return err
		}

		if err := v.Validate(filePath); err != nil {
			return err
		}

		fmt.Printf("Validation successful for %s\n", filePath)
		return nil
	},
}
