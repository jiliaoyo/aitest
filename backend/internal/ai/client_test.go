package ai

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRunPromptForGenerationUsesResponsesJSONSchema(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/responses" {
			http.Error(w, "unexpected request path", http.StatusBadRequest)
			return
		}
		var request struct {
			Text struct {
				Format struct {
					Type   string         `json:"type"`
					Name   string         `json:"name"`
					Schema map[string]any `json:"schema"`
				} `json:"format"`
			} `json:"text"`
			Reasoning struct {
				Effort string `json:"effort"`
			} `json:"reasoning"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		if request.Text.Format.Type != "json_schema" || request.Text.Format.Name != "generation" || request.Text.Format.Schema["type"] != "object" || request.Reasoning.Effort != "none" {
			http.Error(w, "unexpected structured request", http.StatusBadRequest)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": "completed",
			"output": []map[string]any{{"type": "message", "content": []map[string]string{{"type": "output_text", "text": `{"questions":[]}`}}}},
			"usage":  map[string]int{"input_tokens": 10, "output_tokens": 4},
		})
	}))
	defer server.Close()

	client := NewClient(Config{BaseURL: server.URL, APIKey: "test-key", Model: "test-model", APIStyle: aiAPIStyleResponses, Timeout: time.Second}, nil, nil)
	out, _, err := client.RunPromptForGenerationAndAudit(t.Context(), "user", "generation", "v1", "ref", "system", "input", map[string]any{"type": "object"}, 0.4)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"questions":[]}` {
		t.Fatalf("output = %s", out)
	}
}
