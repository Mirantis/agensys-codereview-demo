package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	intm "orchestrator/internal"

	"github.com/rs/zerolog"
)

type SummarizerOut struct {
	Markdown string `json:"markdown"` // FIXED: was "final_markdown"
}

// CallSummarizer invokes the Summarizer Agent with description, review, semgrep.
func CallSummarizer(
	ctx context.Context,
	log zerolog.Logger,
	client *http.Client,
	baseURL string,
	meta intm.PRMetadata,
	descriptionMarkdown string,
	reviewMarkdown string,
	semgrepMarkdown string,
	semgrepSeverity intm.SemgrepSeveritySummary,
) (*SummarizerOut, error) {

	payload := struct {
		PR                  intm.PRMetadata             `json:"pr"`
		DescriptionMarkdown string                      `json:"description_markdown"`
		ReviewMarkdown      string                      `json:"review_markdown"`
		SemgrepMarkdown     string                      `json:"semgrep_markdown"`
		SemgrepSeverity     intm.SemgrepSeveritySummary `json:"semgrep_severity"`
	}{
		PR:                  meta,
		DescriptionMarkdown: descriptionMarkdown,
		ReviewMarkdown:      reviewMarkdown,
		SemgrepMarkdown:     semgrepMarkdown,
		SemgrepSeverity:     semgrepSeverity,
	}

	log.Debug().
		Str("url", baseURL).
		Interface("severity", semgrepSeverity).
		Msg("calling Summarizer Agent")

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(payload); err != nil {
		return nil, fmt.Errorf("encode summarizer payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL, &buf)
	if err != nil {
		return nil, fmt.Errorf("create summarizer request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("summarizer http error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("summarizer returned status %d", resp.StatusCode)
	}

	// Decode using the fixed SummarizerOut type
	var out SummarizerOut
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode summarizer response: %w", err)
	}

	log.Debug().
		Str("markdown_preview", preview(out.Markdown)).
		Msg("Summarizer response decoded")

	return &out, nil
}

func preview(s string) string {
	if len(s) > 200 {
		return s[:200] + "...(truncated)"
	}
	return s
}
