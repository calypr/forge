package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bmeg/golib"
	"github.com/bmeg/grip/gripql"
	"github.com/bytedance/sonic"
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
	Use:   "validate <path_to_metadata_file(s)>",
	Short: "validate data files given a jsonschema and a ndjson data target file or directory",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]

		sch, err := schema.NewSchema()
		if err != nil {
			return errors.Wrap(err, "failed to create schema")
		}

		var allErrors = new(multierror.Error)
		var totalFilesValidated, totalRowsValidated, totalErrors int

		// A helper function to validate a single file and print results immediately
		validateFile := func(filePath string) {
			rows, errorsCount, fileErrors, fileErr := sch.Validate(filePath)
			if fileErr != nil {
				allErrors = multierror.Append(allErrors,
					errors.Wrapf(fileErr, "validation failed for file %s", filePath))
				return
			}

			// Update totals
			totalFilesValidated++
			totalRowsValidated += rows
			totalErrors += errorsCount

			// Print results for this file immediately
			fmt.Printf("\nFile: %s\n", filePath)
			fmt.Printf("  Rows validated: %d\n", rows)
			fmt.Printf("  Errors found: %d\n", errorsCount)
			if fileErrors != nil {
				for _, fe := range fileErrors.Errors {
					fmt.Printf("    - %v\n", fe)
				}
			}
			fmt.Printf("---")
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

		fmt.Printf("\n--- Overall Totals ---\n")
		fmt.Printf("  Files validated: %d\n", totalFilesValidated)
		fmt.Printf("  Rows validated: %d\n", totalRowsValidated)
		fmt.Printf("  Errors: %d\n", totalErrors)
		fmt.Printf("----------------------\n")

		return allErrors.ErrorOrNil()
	},
}

var CheckEdgeCmd = &cobra.Command{
	Use:   "check-edge <path_to_metadata_files>",
	Short: "Check for orphaned edges in graph data from FHIR .ndjson files",
	Long:  "Generates graph elements from FHIR .ndjson files and checks for edges referencing non-existent vertices",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		path := args[0]
		sch, err := schema.NewSchema()
		if err != nil {
			return errors.Wrap(err, "failed to create schema")
		}

		fileResults := make(map[string]struct {
			Rows       int
			Edges      int
			Vertices   int
			FileErrors *multierror.Error
		})
		var allErrors = new(multierror.Error)
		var allElements []*gripql.GraphElement
		var allElementsMu sync.Mutex

		validateFile := func(filePath string) error {
			rows := 0
			var fileErrors = new(multierror.Error)
			var fileElements []*gripql.GraphElement
			var fileMu sync.Mutex

			dataItems, err := golib.ReadFileLines(filePath)
			if err != nil {
				fileErrors = multierror.Append(fileErrors,
					errors.Wrapf(err, "failed to read NDJSON file %s", filePath))
				fileResults[filePath] = struct {
					Rows       int
					Edges      int
					Vertices   int
					FileErrors *multierror.Error
				}{Rows: rows, FileErrors: fileErrors}
				return err
			}

			// Detect class from first row
			var class string
			firstLine := true

			procChan := make(chan map[string]any, 100)
			var wg sync.WaitGroup
			numWorkers := 10
			wg.Add(numWorkers)

			// Workers
			for range numWorkers {
				go func() {
					defer wg.Done()
					for sfgData := range procChan {
						elements, err := sch.Sch.Generate(class, sfgData, map[string]any{})
						if err != nil {
							fileMu.Lock()
							fileErrors = multierror.Append(fileErrors,
								errors.Wrapf(err, "failed to generate graph elements for row in %s", filePath))
							fileMu.Unlock()
							continue
						}
						fileMu.Lock()
						fileElements = append(fileElements, elements...)
						fileMu.Unlock()
					}
				}()
			}

			// Producer
			lineNum := 0
			go func() {
				defer close(procChan)
				for data := range dataItems {
					if len(data) == 0 {
						continue
					}
					lineNum++
					var sfgData map[string]any
					if err := sonic.ConfigFastest.Unmarshal(data, &sfgData); err != nil {
						fileMu.Lock()
						fileErrors = multierror.Append(fileErrors,
							errors.Wrapf(err, "Sonic unmarshal error for row %d in %s", lineNum, filePath))
						fileMu.Unlock()
						continue
					}

					if firstLine {
						var ok bool
						class, ok = sfgData["resourceType"].(string)
						if !ok {
							fileMu.Lock()
							fileErrors = multierror.Append(fileErrors,
								fmt.Errorf("Expecting FHIR row to have resourceType field %s", sfgData))
							fileMu.Unlock()
						}
						firstLine = false
					}
					rows++
					procChan <- sfgData
				}
			}()

			wg.Wait()

			// Count edges and vertices
			edges := 0
			vertices := 0
			for _, el := range fileElements {
				if el.Vertex != nil {
					vertices++
				}
				if el.Edge != nil {
					edges++
				}
			}

			// Add file’s elements to global list
			allElementsMu.Lock()
			allElements = append(allElements, fileElements...)
			allElementsMu.Unlock()

			fileResults[filePath] = struct {
				Rows       int
				Edges      int
				Vertices   int
				FileErrors *multierror.Error
			}{Rows: rows, Edges: edges, Vertices: vertices, FileErrors: fileErrors}
			return nil
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
				if !d.IsDir() && strings.HasSuffix(d.Name(), metadata.NDJSON_EXT) {
					if err := validateFile(filePath); err != nil {
						return err
					}
				}
				return nil
			})
			if err != nil {
				allErrors = multierror.Append(allErrors, err)
			}
		} else {
			return fmt.Errorf("Expecting directory of .ndjson files where each file is a separate FHIR resource type. %s is not a directory", path)
		}

		// Check for orphaned edges across all elements
		orphanEdges := sch.FindOrphanEdges(allElements)

		// Print summary
		fmt.Printf("\n--- Edge Check Summary ---\n")
		var totalFilesValidated, totalRowsValidated, totalEdges, totalVertices int

		for file, result := range fileResults {
			fmt.Printf("File: %s\n", file)
			fmt.Printf("  Rows processed: %d\n", result.Rows)
			fmt.Printf("  Vertices generated: %d\n", result.Vertices)
			fmt.Printf("  Edges generated: %d\n", result.Edges)
			if result.FileErrors != nil && len(result.FileErrors.Errors) > 0 {
				fmt.Println("  File errors:")
				for _, fileError := range result.FileErrors.Errors {
					fmt.Printf("    - %v\n", fileError)
				}
			}
			fmt.Printf("---\n")
			totalFilesValidated++
			totalRowsValidated += result.Rows
			totalEdges += result.Edges
			totalVertices += result.Vertices
			if result.FileErrors != nil {
				allErrors = multierror.Append(allErrors, result.FileErrors.Errors...)
			}
		}

		fmt.Printf("Orphaned Edges: %d\n", len(orphanEdges))
		if len(orphanEdges) > 0 {
			fmt.Println("Orphaned edge details:")
			for _, orphan := range orphanEdges {
				fmt.Printf("  - %s\n", orphan)
			}
		}
		fmt.Printf("\nOverall Totals:\n")
		fmt.Printf("  Files processed: %d\n", totalFilesValidated)
		fmt.Printf("  Rows processed: %d\n", totalRowsValidated)
		fmt.Printf("  Vertices generated: %d\n", totalVertices)
		fmt.Printf("  Edges generated: %d\n", totalEdges)
		fmt.Printf("  Orphaned edges: %d\n", len(orphanEdges))
		fmt.Printf("--------------------------\n")

		return allErrors.ErrorOrNil()
	},
}
