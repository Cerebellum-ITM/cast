package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestGroqProvider_AnnotateRoundTrip drives the full HTTP path against a canned
// OpenAI-compatible server: it asserts the request shape and that the response
// is parsed and validated into a Plan.
func TestGroqProvider_AnnotateRoundTrip(t *testing.T) {
	var gotAuth, gotModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		body, _ := io.ReadAll(r.Body)
		var req chatRequest
		_ = json.Unmarshal(body, &req)
		gotModel = req.Model
		if req.ResponseFormat == nil || req.ResponseFormat.Type != "json_object" {
			t.Errorf("expected response_format json_object, got %+v", req.ResponseFormat)
		}
		content := `{"annotations":[{"name":"build","desc":"Compile the binary","tags":["build","bogus"]},{"name":"ghost","desc":"x","tags":[]}],"skipped":[{"name":"noop","reason":"empty recipe"}]}`
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"role": "assistant", "content": content}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	p := &GroqProvider{
		APIKey:   "secret",
		Model:    "llama-test",
		Endpoint: srv.URL,
		HTTP:     srv.Client(),
	}
	req := Request{
		Targets:     []TargetView{{Name: "build", Recipe: []string{"@go build ./..."}}},
		AllowedTags: []string{"build"},
		Model:       "llama-test",
	}
	plan, err := p.Annotate(context.Background(), req)
	if err != nil {
		t.Fatalf("Annotate: %v", err)
	}
	if gotAuth != "Bearer secret" {
		t.Errorf("auth header = %q", gotAuth)
	}
	if gotModel != "llama-test" {
		t.Errorf("model = %q", gotModel)
	}
	if len(plan.Annotations) != 1 || plan.Annotations[0].Name != "build" {
		t.Fatalf("annotations = %#v", plan.Annotations)
	}
	if strings.Join(plan.Annotations[0].Tags, ",") != "build" {
		t.Errorf("tags not filtered to allowed: %v", plan.Annotations[0].Tags)
	}
	// "ghost" (unknown target) + the model's own "noop" skip = 2 skipped.
	if len(plan.Skipped) != 2 {
		t.Errorf("skipped = %#v", plan.Skipped)
	}
}

func TestGroqProvider_NonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":{"message":"invalid api key"}}`)
	}))
	defer srv.Close()

	p := &GroqProvider{APIKey: "x", Model: "m", Endpoint: srv.URL, HTTP: srv.Client()}
	_, err := p.Annotate(context.Background(), Request{Targets: []TargetView{{Name: "build"}}})
	if err == nil || !strings.Contains(err.Error(), "invalid api key") {
		t.Fatalf("expected auth error, got %v", err)
	}
}
