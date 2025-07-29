package schema

import (
	"log"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

var HyperMediaSchema = `{"properties": {
	"anchor": {
		"type": "string",
		"format": "uri-template"
	},
	"anchorPointer": {
		"type": "string",
		"anyOf": [
			{ "format": "json-pointer" },
			{ "format": "relative-json-pointer" }
		]
	},
	"rel": {
		"anyOf": [
			{ "type": "string" },
			{
				"type": "array",
				"items": { "type": "string" },
				"minItems": 1
			}
		]
	},
	"href": {
		"type": "string",
		"format": "uri-template"
	},
	"templatePointers": {
		"type": "object",
		"additionalProperties": {
			"type": "string",
			"anyOf": [
				{ "format": "json-pointer" },
				{ "format": "relative-json-pointer" }
			]
		}
	},
	"templateRequired": {
		"type": "array",
		"items": {
			"type": "string"
		},
		"uniqueItems": true
	},
	"title": {
		"type": "string"
	},
	"description": {
		"type": "string"
	},
	"$comment": {
		"type": "string"
	}
}
}`

func GetHyperMediaVocab() *jsonschema.Vocabulary {
	schema, err := jsonschema.UnmarshalJSON(strings.NewReader(HyperMediaSchema))
	if err != nil {
		log.Fatal(err)
	}
	c := jsonschema.NewCompiler()
	if err := c.AddResource("graphExtMeta.json", schema); err != nil {
		log.Fatal(err)
	}
	sch, err := c.Compile("graphExtMeta.json")
	if err != nil {
		log.Fatal(err)
	}

	return &jsonschema.Vocabulary{
		URL:     "graphExtMeta.json",
		Schema:  sch,
		Compile: nil,
	}

}
