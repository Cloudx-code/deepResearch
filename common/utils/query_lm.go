package utils

import (
	"deepResearch/entity"
	"encoding/json"
)

// BuildTool 把 []*FieldSchema → http.Tool  (DeepSeek 可用)
func BuildTool(fields []*entity.FieldSchema, fnName, desc string) (*entity.Tool, error) {
	props := make(map[string]map[string]interface{})
	required := make([]string, 0, len(fields))

	for _, f := range fields {
		props[f.Name] = map[string]interface{}{
			"type":        f.Type,
			"description": f.Description,
			"maxLength":   f.MaxLength,
		}
		if f.Required {
			required = append(required, f.Name)
		}
	}

	schemaJSON, err := json.Marshal(map[string]interface{}{
		"type":       "object",
		"properties": props,
		"required":   required,
	})
	if err != nil {
		return &entity.Tool{}, err
	}

	return &entity.Tool{
		Type: "function",
		Function: entity.Function{
			Name:        fnName,
			Description: desc,
			Parameters:  json.RawMessage(schemaJSON),
		},
	}, nil
}

// BuildJSONSchema 把 []FieldSchema → json.RawMessage，DeepSeek 用不了，其他ai或许可以
func BuildJSONSchema(fields []*entity.FieldSchema) (json.RawMessage, error) {
	props := make(map[string]map[string]interface{})
	required := make([]string, 0, len(fields))

	for _, f := range fields {
		props[f.Name] = map[string]interface{}{
			"type":        f.Type,
			"description": f.Description,
			"maxLength":   f.MaxLength,
		}
		if f.Required {
			required = append(required, f.Name)
		}
	}

	schema := map[string]interface{}{
		"type":       "object",
		"properties": props,
		"required":   required,
	}
	return json.Marshal(schema)
}
