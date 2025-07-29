package validator

import (
	"compress/gzip"
	"log"
	"os"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/loader"
	"github.com/calypr/forge/schema"
)

type Validator interface {
	Validate() error
}

type JsonSchema struct {
	FilePath   string
	SchemaPath string
}

func NewJsonSchema(filePath, schemaPath string) *JsonSchema {
	return &JsonSchema{
		FilePath:   filePath,
		SchemaPath: schemaPath,
	}
}

func (js JsonSchema) Validate() error {
	compiler := jsonschema.NewCompiler()
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
	return nil
}
