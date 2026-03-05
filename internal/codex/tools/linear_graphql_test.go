package tools

import (
	"context"
	"errors"
	"testing"
)

type mockLinear struct {
	result map[string]any
	err    error
}

func (m *mockLinear) GraphQL(_ context.Context, _ string, _ map[string]any) (map[string]any, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

func TestLinearGraphQLSuccess(t *testing.T) {
	mgr := NewDynamicToolManager(&mockLinear{result: map[string]any{"data": map[string]any{"viewer": map[string]any{"id": "u1"}}}})
	resp := mgr.Execute(context.Background(), "linear_graphql", map[string]any{"query": "query Viewer { viewer { id } }"})
	if success, _ := resp["success"].(bool); !success {
		t.Fatalf("expected success response, got %#v", resp)
	}
}

func TestLinearGraphQLRejectsMultipleOperations(t *testing.T) {
	mgr := NewDynamicToolManager(&mockLinear{result: map[string]any{}})
	resp := mgr.Execute(context.Background(), "linear_graphql", map[string]any{"query": "query A { viewer { id } } query B { viewer { id } }"})
	if success, _ := resp["success"].(bool); success {
		t.Fatalf("expected failure response, got %#v", resp)
	}
}

func TestLinearGraphQLRejectsInvalidArguments(t *testing.T) {
	mgr := NewDynamicToolManager(&mockLinear{result: map[string]any{}})
	resp := mgr.Execute(context.Background(), "linear_graphql", 123)
	if success, _ := resp["success"].(bool); success {
		t.Fatalf("expected invalid args failure, got %#v", resp)
	}
}

func TestLinearGraphQLMarksGraphQLErrorsAsFailure(t *testing.T) {
	mgr := NewDynamicToolManager(&mockLinear{result: map[string]any{"errors": []any{map[string]any{"message": "bad"}}}})
	resp := mgr.Execute(context.Background(), "linear_graphql", "query Viewer { viewer { id } }")
	if success, _ := resp["success"].(bool); success {
		t.Fatalf("expected graphql errors to produce success=false, got %#v", resp)
	}
}

func TestLinearGraphQLPropagatesTransportError(t *testing.T) {
	mgr := NewDynamicToolManager(&mockLinear{err: errors.New("boom")})
	resp := mgr.Execute(context.Background(), "linear_graphql", "query Viewer { viewer { id } }")
	if success, _ := resp["success"].(bool); success {
		t.Fatalf("expected transport error to produce success=false, got %#v", resp)
	}
}
