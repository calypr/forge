package schema

import (
	"fmt"
	"sync"

	"github.com/bmeg/golib"
	"github.com/bmeg/jsonschema/v6"
	"github.com/bmeg/jsonschemagraph/compile"
	"github.com/bmeg/jsonschemagraph/graph"
	"github.com/bytedance/sonic"
	"github.com/calypr/forge/client"
	"github.com/hashicorp/go-multierror"
)

type Schema struct {
	Sch *graph.GraphSchema
	Cli *client.Gen3Client
}

func NewSchema() (*Schema, error) {
	cli, err := client.NewGen3Client()
	if err != nil {
		return nil, err
	}
	compiler := jsonschema.NewCompiler()
	compiler.AssertVocabs()
	vc, err := compile.GetHyperMediaVocab()
	if err != nil {
		return nil, err
	}
	compiler.RegisterVocabulary(vc)

	out := &graph.GraphSchema{Classes: map[string]*jsonschema.Schema{}, Compiler: compiler}

	var schemaMap map[string]any
	if err := sonic.ConfigFastest.Unmarshal([]byte(CalyprSchema), &schemaMap); err != nil {
		return nil, fmt.Errorf("error unmarshaling schema string: %w", err)
	}
	defs, ok := schemaMap["$defs"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("schema does not contain a valid '$defs' map")
	}
	for defName, defSchema := range defs {
		id, ok := defSchema.(map[string]any)["$id"].(string)
		if !ok {
			return nil, fmt.Errorf("error indexing $id in %s", defSchema)
		}
		err = compiler.AddResource(id, defSchema)
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
	return &Schema{
		Sch: out,
		Cli: cli,
	}, nil
}

func (sch *Schema) Validate(filePath string) (int, int, *multierror.Error, error) {
	var (
		count     int
		countMu   sync.Mutex
		allErrors = new(multierror.Error)
		errMu     sync.Mutex
		wg        sync.WaitGroup
	)

	reader, err := golib.ReadFileLines(filePath)
	if err != nil {
		return 0, 0, nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	procChan := make(chan map[string]any, 100)

	go func() {
		defer close(procChan)
		lineNum := 0
		for line := range reader {
			lineNum++
			if len(line) == 0 {
				continue
			}
			var o map[string]any
			if err := sonic.ConfigFastest.Unmarshal(line, &o); err != nil {
				errMu.Lock()
				allErrors = multierror.Append(allErrors,
					fmt.Errorf("failed to unmarshal JSON on line %d: %w", lineNum, err))
				errMu.Unlock()
				continue
			}
			procChan <- o
		}
	}()

	numWorkers := 10
	wg.Add(numWorkers)
	for range numWorkers {
		go func() {
			defer wg.Done()
			for row := range procChan {
				// increment count safely
				countMu.Lock()
				count++
				rowNum := count
				countMu.Unlock()

				resource, ok := row["resourceType"].(string)
				if !ok {
					errMu.Lock()
					allErrors = multierror.Append(allErrors,
						fmt.Errorf("err indexing resourceType on row %d", rowNum))
					errMu.Unlock()
					continue
				}
				if err := sch.Sch.Validate(resource, row); err != nil {
					errMu.Lock()
					allErrors = multierror.Append(allErrors,
						fmt.Errorf("file: %s, row: %d, validation failed: %w", filePath, rowNum, err))
					errMu.Unlock()
				}
			}
		}()
	}

	wg.Wait()

	errorsCount := 0
	if allErrors != nil {
		errorsCount = len(allErrors.Errors)
	}
	return count, errorsCount, allErrors, nil
}
