package validate

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/bmeg/golib"
	"github.com/bmeg/grip/gripql"
	"github.com/bytedance/sonic"
	"github.com/calypr/gecko/gecko/config"

	"github.com/cockroachdb/errors"

	"github.com/calypr/forge/metadata"
	"github.com/calypr/forge/schema"
	"github.com/hashicorp/go-multierror"
	"github.com/spf13/cobra"
)

// Holds the value of the --out-dir flag
var outputDir string
var dataPath string
var edgePath string
var configPath string

var ValidateParentCmd = &cobra.Command{
	Use:   "validate",
	Short: "Contains subcommands for validating config, data, and edges",
}

func init() {
	ValidateDataCmd.Flags().StringVarP(&dataPath, "path", "p", META_PATH, "Path to metadata file(s) to validate")
	ValidateEdgeCmd.Flags().StringVarP(&edgePath, "path", "p", META_PATH, "Path to metadata files directory")
	ValidateEdgeCmd.Flags().StringVarP(&outputDir, "out-dir", "o", "", "Directory to save vertices and edges files")
	ValidateConfigCmd.Flags().StringVarP(&configPath, "path", "p", CONFIG_PATH, "Path to config file to validate")
}

const META_PATH = "META"
const CONFIG_PATH = "CONFIG"

// ValidateCmd remains unchanged
var ValidateDataCmd = &cobra.Command{
	Use:   "data",
	Short: "validate metadata files given a jsonschema and a ndjson data target file or directory",
	Args:  cobra.NoArgs,
	Long:  "Validates metadata files. Use --path to specify a file or directory (defaults to META if not provided)",
	RunE: func(cmd *cobra.Command, args []string) error {
		path := dataPath
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

var ValidateEdgeCmd = &cobra.Command{
	Use:   "edge",
	Short: "Check for orphaned edges in graph data from FHIR .ndjson files",
	Long:  "Generates graph elements from FHIR .ndjson files and checks for edges referencing non-existent vertices. Use --path to specify directory (defaults to META if not provided)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		path := edgePath
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

		if outputDir != "" {
			fmt.Printf("\n--- Writing output files to %s ---\n", outputDir)
			if err := os.MkdirAll(outputDir, 0755); err != nil {
				return errors.Wrapf(err, "failed to create output directory: %s", outputDir)
			}

			vertexPath := filepath.Join(outputDir, "vertices.ndjson")
			edgePath := filepath.Join(outputDir, "edges.ndjson")

			vertexFile, err := os.Create(vertexPath)
			if err != nil {
				return errors.Wrapf(err, "failed to create vertex file: %s", vertexPath)
			}
			defer vertexFile.Close()

			edgeFile, err := os.Create(edgePath)
			if err != nil {
				return errors.Wrapf(err, "failed to create edge file: %s", edgePath)
			}
			defer edgeFile.Close()

			vertexEncoder := json.NewEncoder(vertexFile)
			edgeEncoder := json.NewEncoder(edgeFile)

			for _, element := range allElements {
				if element.Vertex != nil {
					if err := vertexEncoder.Encode(element.Vertex); err != nil {
						allErrors = multierror.Append(allErrors, errors.Wrap(err, "failed to write vertex"))
					}
				}
				if element.Edge != nil {
					if err := edgeEncoder.Encode(element.Edge); err != nil {
						allErrors = multierror.Append(allErrors, errors.Wrap(err, "failed to write edge"))
					}
				}
			}
			fmt.Printf("✓ Vertices written to: %s\n", vertexPath)
			fmt.Printf("✓ Edges written to: %s\n", edgePath)
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

var ValidateConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "validate explorer config file",
	Long:  "Validates explorer config file. Use --path to specify config file (defaults to CONFIG if not provided)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		path := configPath

		info, err := os.Stat(path)
		if err != nil {
			return errors.Wrapf(err, "could not get file info for %s", path)
		}
		if info.IsDir() {
			return fmt.Errorf("%s is a directory, expected a file", path)
		}
		bytes, err := os.ReadFile(path)
		if err != nil {
			return errors.Wrapf(err, "could not read file for %s", path)
		}
		var explorerConf config.Config
		err = json.Unmarshal(bytes, &explorerConf)
		if err != nil {
			return errors.Wrapf(err, "could not unmarshal file into explorerConfig for %s", path)
		}

		if explorerConf.ExplorerConfig != nil && len(explorerConf.ExplorerConfig) == 0 {
			return fmt.Errorf("No explorer tabs are defined for explorer config: %s", path)
		}

		fmt.Printf("%s is a valid explorer config\n", path) // Added \n for cleaner output
		return nil
	},
}
