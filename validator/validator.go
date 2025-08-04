package validator

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/bmeg/golib"
	"github.com/bmeg/jsonschema/v5"
	"github.com/bytedance/sonic"
	"github.com/calypr/forge/schema"
	"github.com/hashicorp/go-multierror"
)

type Validator interface {
	Validate() error
}

type JsonSchema struct {
	sch *jsonschema.Schema
}

func NewJsonSchema() (*JsonSchema, error) {
	compiler := jsonschema.NewCompiler()
	compiler.ExtractAnnotations = true
	compiler.AddResource("CalyprSchema", strings.NewReader(schema.CalyprSchema))
	sch, err := compiler.Compile("CalyprSchema")
	if err != nil {
		return nil, fmt.Errorf("Error compiling Calypr Schema %s", err)
	}
	return &JsonSchema{
		sch: sch,
	}, nil
}

func (js JsonSchema) Validate(filePath string) error {
	var reader <-chan []byte
	var err error

	if strings.HasSuffix(filePath, ".gz") {
		reader, err = golib.ReadGzipLines(filePath)
	} else {
		reader, err = golib.ReadFileLines(filePath)
	}
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", filePath, err)
	}

	var wg sync.WaitGroup
	wg.Add(1)

	procChan := make(chan map[string]any, 100)
	var allErrors *multierror.Error
	var errMutex sync.Mutex

	go func() {
		defer wg.Done()
		defer close(procChan)
		var lineCounter int
		for line := range reader {
			lineCounter++
			o := map[string]any{}
			if len(line) > 0 {
				err := sonic.ConfigFastest.Unmarshal(line, &o)
				if err != nil {
					errMutex.Lock()
					allErrors = multierror.Append(allErrors, fmt.Errorf("line %d: failed to unmarshal JSON: %w", lineCounter, err))
					errMutex.Unlock()
					continue
				}
				procChan <- o
			}
		}
	}()

	validCount := 0
	for row := range procChan {
		err = js.sch.Validate(row)
		if err != nil {
			errMutex.Lock()
			allErrors = multierror.Append(allErrors, fmt.Errorf("validation error: %w", err))
			errMutex.Unlock()
		} else {
			validCount++
		}
	}
	wg.Wait()
	var invalidCount int
	if allErrors != nil {
		invalidCount = len(allErrors.Errors)
	}
	log.Printf("%s results: %d valid records %d invalid records\n", filePath, validCount, invalidCount)
	return allErrors.ErrorOrNil()
}

/*compiler := jsonschema.NewCompiler()
loader.Register("file", schema.MyYAMLLoader{})
compiler.RegisterVocabulary(schema.GetHyperMediaVocab())

sch, err := compiler.Compile(js.SchemaPath)
if err != nil {
	log.Fatalf("Error compiling %s : %s\n", js.SchemaPath, err)
}

if !sch.Types.IsEmpty() && sch.Types.ToStrings()[0] == "object" {
	log.Printf("OK: %s (%s)\n", js.SchemaPath, sch.Title)
}
gfile, err := os.Open(js.FilePath)
if err != nil {
	return err
}
file, err := gzip.NewReader(gfile)
if err != nil {
	return err
}

validCount := 0
errorCount := 0
err = sch.Validate(file)
if err != nil {
	errorCount++
	log.Printf("Error: %s\n", err)
}
validCount++

log.Printf("%s results: %d valid records %d invalid records\n", js.FilePath, validCount, errorCount)
return nil*/
