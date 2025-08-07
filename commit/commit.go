package commit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmeg/golib"
	"github.com/bmeg/jsonschema/v5"
	"github.com/bmeg/jsonschemagraph/compile"
	"github.com/bmeg/jsonschemagraph/graph"
	"github.com/bytedance/sonic"
	"github.com/calypr/forge/client"
	"github.com/calypr/forge/metadata"
	"github.com/calypr/forge/schema"
	"github.com/hashicorp/go-multierror"
)

type Commit struct {
	Sch *graph.GraphSchema
	Cli *client.Gen3Client
}

func NewCommitObj() (*Commit, error) {
	cli, err := client.NewGen3Client()
	if err != nil {
		return nil, err
	}
	compiler := jsonschema.NewCompiler()
	out := &graph.GraphSchema{Classes: map[string]*jsonschema.Schema{}, Compiler: compiler}
	compiler.ExtractAnnotations = true
	compiler.RegisterExtension(compile.GraphExtensionTag, compile.GraphExtMeta, compile.GraphExtCompiler{})

	var schemaMap map[string]any
	if err := sonic.ConfigFastest.Unmarshal([]byte(schema.CalyprSchema), &schemaMap); err != nil {
		return nil, fmt.Errorf("error unmarshaling schema string: %w", err)
	}
	defs, ok := schemaMap["$defs"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("schema does not contain a valid '$defs' map")
	}
	for defName, defSchema := range defs {
		jsonData, err := sonic.Marshal(defSchema)
		if err != nil {
			return nil, fmt.Errorf("error marshaling definition '%s': %w", defName, err)
		}
		reader := strings.NewReader(string(jsonData))
		id, ok := defSchema.(map[string]any)["$id"].(string)
		if !ok {
			return nil, fmt.Errorf("error indexing $id in %s", defSchema)
		}
		err = compiler.AddResource(id, reader)
		if err != nil {
			return nil, fmt.Errorf("error adding resource for '%s': %w", defName, err)
		}
	}
	for defName, defSchema := range defs {
		schLabel, ok := defSchema.(map[string]any)["$id"].(string)
		if !ok {
			return nil, fmt.Errorf("Json schema %s doesn't contain $id field", defSchema)
		}
		sch, err := compiler.Compile(schLabel)
		if err != nil {
			return nil, fmt.Errorf("error compiling schema for '%s': %w", defName, err)
		}
		out.Classes[defName] = sch
	}
	return &Commit{
		Sch: out,
		Cli: cli,
	}, nil
}

func (c *Commit) RunPreCommit() error {
	err := metadata.RunMetaInit(".", metadata.META_DIR, false)
	if err != nil {
		return err
	}
	files, err := findNdjsonFiles(metadata.META_DIR)
	if err != nil {
		return err
	}
	count := 0
	allErrors := new(multierror.Error)
	for _, filePath := range files {
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
			err = c.Sch.Validate(resource, row)
			if err != nil {
				allErrors = multierror.Append(allErrors, fmt.Errorf("failed to validate JSON: (Error: %w)", err))
				continue
			}
		}
	}
	fmt.Printf("Validated %d rows, %d errors\n", count, len(allErrors.Errors))
	return allErrors.ErrorOrNil()
}

func findNdjsonFiles(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// Check if it's a regular file and has the correct suffix.
		if !info.IsDir() && strings.HasSuffix(info.Name(), metadata.NDJSON_EXT) {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}
