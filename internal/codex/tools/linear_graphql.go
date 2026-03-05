package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/vektah/gqlparser/v2/ast"
	"github.com/vektah/gqlparser/v2/parser"
)

type LinearGraphQLExecutor interface {
	GraphQL(ctx context.Context, query string, variables map[string]any) (map[string]any, error)
}

type DynamicToolManager struct {
	linear LinearGraphQLExecutor
}

func NewDynamicToolManager(linear LinearGraphQLExecutor) *DynamicToolManager {
	return &DynamicToolManager{linear: linear}
}

func (m *DynamicToolManager) ToolSpecs() []map[string]any {
	return []map[string]any{
		{
			"name":        "linear_graphql",
			"description": "Execute a raw GraphQL query or mutation against Linear using Symphony's configured auth.",
			"inputSchema": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"required":             []string{"query"},
				"properties": map[string]any{
					"query":     map[string]any{"type": "string", "description": "GraphQL query or mutation document."},
					"variables": map[string]any{"type": []string{"object", "null"}, "additionalProperties": true},
				},
			},
		},
	}
}

func (m *DynamicToolManager) Execute(ctx context.Context, tool string, arguments any) map[string]any {
	if tool != "linear_graphql" {
		return failure(map[string]any{"error": map[string]any{"message": fmt.Sprintf("unsupported_tool_call: %s", tool)}})
	}
	query, vars, err := normalizeArgs(arguments)
	if err != nil {
		return failure(map[string]any{"error": map[string]any{"message": err.Error()}})
	}
	if err := validateSingleOperation(query); err != nil {
		return failure(map[string]any{"error": map[string]any{"message": err.Error()}})
	}
	if m.linear == nil {
		return failure(map[string]any{"error": map[string]any{"message": "missing_tracker_api_key"}})
	}
	body, err := m.linear.GraphQL(ctx, query, vars)
	if err != nil {
		return failure(map[string]any{"error": map[string]any{"message": err.Error()}})
	}
	success := true
	if errs, ok := body["errors"].([]any); ok && len(errs) > 0 {
		success = false
	}
	text := stringify(body)
	return map[string]any{
		"success":      success,
		"contentItems": []map[string]any{{"type": "inputText", "text": text}},
	}
}

func normalizeArgs(arguments any) (string, map[string]any, error) {
	switch v := arguments.(type) {
	case string:
		q := strings.TrimSpace(v)
		if q == "" {
			return "", nil, fmt.Errorf("`linear_graphql` requires a non-empty `query` string")
		}
		return q, map[string]any{}, nil
	case map[string]any:
		rawQuery, _ := v["query"].(string)
		q := strings.TrimSpace(rawQuery)
		if q == "" {
			return "", nil, fmt.Errorf("`linear_graphql` requires a non-empty `query` string")
		}
		if vars, ok := v["variables"]; ok && vars != nil {
			m, ok := vars.(map[string]any)
			if !ok {
				return "", nil, fmt.Errorf("`linear_graphql.variables` must be a JSON object")
			}
			return q, m, nil
		}
		return q, map[string]any{}, nil
	default:
		return "", nil, fmt.Errorf("`linear_graphql` expects raw query text or object with query/variables")
	}
}

func validateSingleOperation(query string) error {
	doc, err := parser.ParseQuery(&ast.Source{Input: query})
	if err != nil {
		return fmt.Errorf("invalid GraphQL query: %w", err)
	}
	if len(doc.Operations) != 1 {
		return fmt.Errorf("`linear_graphql` requires exactly one GraphQL operation")
	}
	return nil
}

func failure(payload map[string]any) map[string]any {
	return map[string]any{
		"success":      false,
		"contentItems": []map[string]any{{"type": "inputText", "text": stringify(payload)}},
	}
}

func stringify(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}
