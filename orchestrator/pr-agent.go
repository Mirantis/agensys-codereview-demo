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

func CallPRAgentDescribe(
    ctx context.Context,
    log zerolog.Logger,
    client *http.Client,
    baseURL string,
    meta intm.PRMetadata,
) (*intm.PRAgentDescribeOut, error) {
    payload := intm.PRAgentDescribeRequest{
        Mode: "describe",
        PR:   meta,
    }

    log.Debug().
        Str("url", baseURL).
        Str("mode", payload.Mode).
        Msg("calling PR-Agent describe")

    var buf bytes.Buffer
    if err := json.NewEncoder(&buf).Encode(payload); err != nil {
        return nil, fmt.Errorf("encode pr-agent describe payload: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL, &buf)
    if err != nil {
        return nil, fmt.Errorf("create pr-agent describe request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("pr-agent describe http error: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 300 {
        return nil, fmt.Errorf("pr-agent describe returned status %d", resp.StatusCode)
    }

    var out intm.PRAgentDescribeOut
    if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
        return nil, fmt.Errorf("decode pr-agent describe response: %w", err)
    }

    log.Debug().Msg("PR-Agent describe response decoded")
    return &out, nil
}

func CallPRAgentReview(
    ctx context.Context,
    log zerolog.Logger,
    client *http.Client,
    baseURL string,
    meta intm.PRMetadata,
    descriptionMarkdown string,
) (*intm.PRAgentReviewOut, error) {
    payload := intm.PRAgentReviewRequest{
        Mode:                "review",
        PR:                  meta,
        DescriptionMarkdown: descriptionMarkdown,
    }

    log.Debug().
        Str("url", baseURL).
        Str("mode", payload.Mode).
        Msg("calling PR-Agent review")

    var buf bytes.Buffer
    if err := json.NewEncoder(&buf).Encode(payload); err != nil {
        return nil, fmt.Errorf("encode pr-agent review payload: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL, &buf)
    if err != nil {
        return nil, fmt.Errorf("create pr-agent review request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("pr-agent review http error: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 300 {
        return nil, fmt.Errorf("pr-agent review returned status %d", resp.StatusCode)
    }

    var out intm.PRAgentReviewOut
    if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
        return nil, fmt.Errorf("decode pr-agent review response: %w", err)
    }

    log.Debug().Msg("PR-Agent review response decoded")
    return &out, nil
}
