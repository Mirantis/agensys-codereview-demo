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

func PostGitHubComment(
    ctx context.Context,
    log zerolog.Logger,
    client *http.Client,
    baseURL string,
    meta intm.PRMetadata,
    markdown string,
) error {
    payload := intm.GitHubCommentRequest{
        Action:     "comment_pr",
        PR:         meta,
        Body:       markdown,
        BodyFormat: "markdown",
    }

    log.Debug().
        Str("url", baseURL).
        Int("pr", meta.PRNumber).
        Msg("calling GitHub MCP to post comment")

    var buf bytes.Buffer
    if err := json.NewEncoder(&buf).Encode(payload); err != nil {
        return fmt.Errorf("encode github comment payload: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL, &buf)
    if err != nil {
        return fmt.Errorf("create github comment request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")

    resp, err := client.Do(req)
    if err != nil {
        return fmt.Errorf("github mcp http error: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode >= 300 {
        return fmt.Errorf("github mcp returned status %d", resp.StatusCode)
    }

    log.Debug().Msg("GitHub MCP comment posted successfully")
    return nil
}
