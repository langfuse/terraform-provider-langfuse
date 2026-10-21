package langfuse

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func newEvaluatorsTestServer(t *testing.T, handler http.HandlerFunc) (EvaluatorsClient, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewEvaluatorsClient(srv.URL, "pk-test", "sk-test", srv.Client()), srv
}

func TestEvaluatorsClient_CreateEvaluator_SendsBasicAuthAndBody(t *testing.T) {
	t.Parallel()

	var gotPath, gotMethod string
	var gotBody map[string]any
	client, _ := newEvaluatorsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotMethod = r.URL.Path, r.Method
		user, pass, ok := r.BasicAuth()
		if !ok || user != "pk-test" || pass != "sk-test" {
			t.Errorf("expected project basic auth, got ok=%v user=%q", ok, user)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"ev-1","name":"n","type":"code","status":"active","version":1,"versionId":"v-1","sourceCode":"x","sourceCodeLanguage":"PYTHON"}`))
	})

	ev, err := client.CreateEvaluator(context.Background(), &EvaluatorRequest{
		Name: "n", Type: EvaluatorTypeCode, SourceCode: "x", SourceCodeLanguage: "PYTHON",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost || gotPath != "/api/public/v2/evaluators" {
		t.Errorf("unexpected request %s %s", gotMethod, gotPath)
	}
	if gotBody["description"] != nil {
		t.Errorf("description must be serialised as null when unset, got %v", gotBody["description"])
	}
	if _, ok := gotBody["prompt"]; ok {
		t.Errorf("prompt must be omitted for code evaluators")
	}
	if ev.ID != "ev-1" || ev.Version != 1 || ev.SourceCodeLanguage != "PYTHON" {
		t.Errorf("unexpected evaluator: %+v", ev)
	}
}

func TestEvaluatorsClient_GetEvaluator_NotFoundIsTyped(t *testing.T) {
	t.Parallel()

	client, _ := newEvaluatorsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"Evaluator not found","code":"resource_not_found"}`))
	})

	_, err := client.GetEvaluator(context.Background(), "missing")
	if err == nil {
		t.Fatalf("expected an error")
	}
	if !IsNotFound(err) {
		t.Errorf("expected IsNotFound to be true, got %v", err)
	}
	if want := "request failed with status code 404 (resource_not_found): Evaluator not found"; !contains(err.Error(), want) {
		t.Errorf("expected structured error message containing %q, got %q", want, err.Error())
	}
}

func TestEvaluatorsClient_UnstructuredErrorFallsBackToBody(t *testing.T) {
	t.Parallel()

	client, _ := newEvaluatorsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`upstream unavailable`))
	})

	_, err := client.GetEvaluationRule(context.Background(), "r")
	if err == nil || IsNotFound(err) {
		t.Fatalf("expected a non-404 error, got %v", err)
	}
	if !contains(err.Error(), "status code 502") || !contains(err.Error(), "upstream unavailable") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestEvaluatorsClient_Delete_TreatsNotFoundAsSuccess(t *testing.T) {
	t.Parallel()

	client, _ := newEvaluatorsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("expected DELETE, got %s", r.Method)
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"gone","code":"resource_not_found"}`))
	})

	if err := client.DeleteEvaluator(context.Background(), "ev-1"); err != nil {
		t.Errorf("DeleteEvaluator must ignore 404, got %v", err)
	}
	if err := client.DeleteEvaluationRule(context.Background(), "rule-1"); err != nil {
		t.Errorf("DeleteEvaluationRule must ignore 404, got %v", err)
	}
}

func TestEvaluatorsClient_UpdateEvaluationRule_SerialisesArraysAndNulls(t *testing.T) {
	t.Parallel()

	var gotBody map[string]any
	client, _ := newEvaluatorsTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch || r.URL.Path != "/api/public/v2/evaluation-rules/rule-1" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_, _ = w.Write([]byte(`{"id":"rule-1","name":"n","enabled":true,"sampling":1,"filter":[],"evaluatorAssignments":[{"evaluatorId":"ev-1","variableMapping":null}]}`))
	})

	rule, err := client.UpdateEvaluationRule(context.Background(), "rule-1", &EvaluationRuleRequest{
		Name:                 "n",
		Enabled:              true,
		Filter:               []EvaluationRuleFilter{},
		EvaluatorAssignments: []EvaluatorAssignment{{EvaluatorID: "ev-1"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if f, ok := gotBody["filter"].([]any); !ok || len(f) != 0 {
		t.Errorf("filter must be serialised as an empty array, got %v", gotBody["filter"])
	}
	if _, ok := gotBody["sampling"]; ok {
		t.Errorf("sampling must be omitted when nil")
	}
	assignments := gotBody["evaluatorAssignments"].([]any)
	if _, present := assignments[0].(map[string]any)["variableMapping"]; !present {
		t.Errorf("variableMapping must be present (as null) to inherit the default mapping")
	}
	if rule.EvaluatorAssignments[0].VariableMapping != nil {
		t.Errorf("null variableMapping must decode to nil")
	}
}

func contains(s, substr string) bool {
	return len(substr) == 0 || (len(s) >= len(substr) && indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
