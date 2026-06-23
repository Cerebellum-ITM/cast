package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// GroqProvider talks to Groq's OpenAI-compatible /chat/completions endpoint.
// Groq exposes the same wire format as OpenAI, so the client is hand-written
// over net/http with zero SDK dependency.
type GroqProvider struct {
	APIKey   string
	Model    string
	Endpoint string
	HTTP     *http.Client
}

// ── wire types ───────────────────────────────────────────────────────────────

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	Temperature    float64         `json:"temperature"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// rawPlan mirrors the JSON the model is asked to return.
type rawPlan struct {
	Annotations []struct {
		Name string   `json:"name"`
		Desc string   `json:"desc"`
		Tags []string `json:"tags"`
	} `json:"annotations"`
	Skipped []struct {
		Name   string `json:"name"`
		Reason string `json:"reason"`
	} `json:"skipped"`
}

// Annotate satisfies Provider. When the request carries more targets than
// req.MaxTargets it splits them into sequential batches and merges the plans.
func (g *GroqProvider) Annotate(ctx context.Context, req Request) (Plan, error) {
	batchSize := req.MaxTargets
	if batchSize <= 0 || batchSize >= len(req.Targets) {
		return g.annotateBatch(ctx, req)
	}

	var merged Plan
	for start := 0; start < len(req.Targets); start += batchSize {
		end := start + batchSize
		if end > len(req.Targets) {
			end = len(req.Targets)
		}
		batch := req
		batch.Targets = req.Targets[start:end]
		p, err := g.annotateBatch(ctx, batch)
		if err != nil {
			return Plan{}, err
		}
		merged.Annotations = append(merged.Annotations, p.Annotations...)
		merged.Skipped = append(merged.Skipped, p.Skipped...)
	}
	return merged, nil
}

func (g *GroqProvider) annotateBatch(ctx context.Context, req Request) (Plan, error) {
	if len(req.Targets) == 0 {
		return Plan{}, nil
	}
	model := req.Model
	if model == "" {
		model = g.Model
	}

	payload := chatRequest{
		Model:       model,
		Temperature: 0.2,
		Messages: []chatMessage{
			{Role: "system", Content: SystemPrompt(req.AllowedTags)},
			{Role: "user", Content: UserPayload(req)},
		},
		ResponseFormat: &responseFormat{Type: "json_object"},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Plan{}, fmt.Errorf("groq: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, g.Endpoint, bytes.NewReader(body))
	if err != nil {
		return Plan{}, fmt.Errorf("groq: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+g.APIKey)

	client := g.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return Plan{}, fmt.Errorf("groq: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return Plan{}, fmt.Errorf("groq: read response: %w", err)
	}

	var cr chatResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return Plan{}, fmt.Errorf("groq: decode response (status %d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK {
		if cr.Error != nil {
			return Plan{}, fmt.Errorf("groq: status %d: %s", resp.StatusCode, cr.Error.Message)
		}
		return Plan{}, fmt.Errorf("groq: status %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if len(cr.Choices) == 0 {
		return Plan{}, fmt.Errorf("groq: empty choices in response")
	}

	var rp rawPlan
	content := cr.Choices[0].Message.Content
	if err := json.Unmarshal([]byte(content), &rp); err != nil {
		return Plan{}, fmt.Errorf("groq: model did not return valid JSON: %w", err)
	}

	return validatePlan(rp, req), nil
}

// validatePlan filters the model's raw output against the request: unknown
// target names are demoted to Skipped, descriptions are trimmed/capped at 80
// chars, and (when an allowed list is configured) tags outside it are dropped.
func validatePlan(rp rawPlan, req Request) Plan {
	valid := make(map[string]bool, len(req.Targets))
	for _, t := range req.Targets {
		valid[t.Name] = true
	}
	allowed := make(map[string]bool, len(req.AllowedTags))
	for _, t := range req.AllowedTags {
		allowed[t] = true
	}

	var plan Plan
	for _, a := range rp.Annotations {
		if !valid[a.Name] {
			plan.Skipped = append(plan.Skipped, SkipReason{
				Name:   a.Name,
				Reason: "model returned an unknown target name",
			})
			continue
		}
		desc := strings.TrimSpace(a.Desc)
		if len(desc) > 80 {
			desc = strings.TrimSpace(desc[:80])
		}
		tags := a.Tags
		if len(allowed) > 0 {
			kept := tags[:0:0]
			for _, tg := range a.Tags {
				if allowed[tg] {
					kept = append(kept, tg)
				}
			}
			tags = kept
		}
		plan.Annotations = append(plan.Annotations, Annotation{
			Name: a.Name,
			Desc: desc,
			Tags: tags,
		})
	}
	for _, s := range rp.Skipped {
		plan.Skipped = append(plan.Skipped, SkipReason{Name: s.Name, Reason: s.Reason})
	}
	return plan
}
