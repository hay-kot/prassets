package config

import (
	"encoding/json"
	"reflect"

	"github.com/invopop/jsonschema"
)

// Schema URLs, as the yaml-language-server comment in each file names them.
const (
	SchemaURL     = "https://raw.githubusercontent.com/hay-kot/prassets/main/schema.json"
	RepoSchemaURL = "https://raw.githubusercontent.com/hay-kot/prassets/main/schema.repo.json"
)

// GlobalSchema returns the JSON Schema of the global config file.
func GlobalSchema() ([]byte, error) {
	return schemaFor(&Config{}, SchemaURL, "prassets global config")
}

// RepoSchema returns the JSON Schema of .prassets.yaml.
func RepoSchema() ([]byte, error) {
	return schemaFor(&RepoFile{}, RepoSchemaURL, "prassets repo config (.prassets.yaml)")
}

func schemaFor(v any, id, title string) ([]byte, error) {
	r := &jsonschema.Reflector{
		FieldNameTag:               "yaml",
		RequiredFromJSONSchemaTags: true,
		DoNotReference:             false,
		Namer: func(t reflect.Type) string {
			return t.Name()
		},
	}
	s := r.Reflect(v)
	s.ID = jsonschema.ID(id)
	s.Title = title
	out, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}
