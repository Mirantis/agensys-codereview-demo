package main

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	intm "orchestrator/internal"
)

type githubPREvent struct {
	Action      string `json:"action"`
	Number      int    `json:"number"`
	PullRequest struct {
		Title   string `json:"title"`
		Body    string `json:"body"`
		HTMLURL string `json:"html_url"`
		Head    struct {
			Ref  string `json:"ref"`
			SHA  string `json:"sha"`
			Repo struct {
				Name  string `json:"name"`
				Owner struct {
					Login string `json:"login"`
				} `json:"owner"`
			} `json:"repo"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
		} `json:"base"`
	} `json:"pull_request"`
	Repository struct {
		Name  string `json:"name"`
		Owner struct {
			Login string `json:"login"`
		} `json:"owner"`
	} `json:"repository"`
}

func (o *Orchestrator) prWebhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var event githubPREvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		o.log.Error().Err(err).Msg("failed to decode webhook payload")
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}

	o.log.Debug().
		Int("pr_number", event.Number).
		Str("action", event.Action).
		Msg("received GitHub PR webhook")

	if event.Action != "opened" && event.Action != "reopened" && event.Action != "synchronize" {
		o.log.Debug().
			Str("action", event.Action).
			Msg("ignoring PR event action")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ignored"))
		return
	}

	meta := intm.PRMetadata{
		RepoOwner:    event.Repository.Owner.Login,
		RepoName:     event.Repository.Name,
		PRNumber:     event.Number,
		HeadSHA:      event.PullRequest.Head.SHA,
		Title:        event.PullRequest.Title,
		Body:         event.PullRequest.Body,
		SourceBranch: event.PullRequest.Head.Ref,
		TargetBranch: event.PullRequest.Base.Ref,
		URL:          event.PullRequest.HTMLURL,
	}

	// Use context.Background() instead of r.Context()
	// This prevents cancellation if webhook client disconnects
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	o.log.Info().
		Str("repo", meta.RepoOwner+"/"+meta.RepoName).
		Int("pr", meta.PRNumber).
		Dur("timeout", 20*time.Minute).
		Msg("starting PR processing with background context")

	// Optional: Monitor request context separately
	go func() {
		<-r.Context().Done()
		if r.Context().Err() != nil {
			o.log.Warn().
				Err(r.Context().Err()).
				Msg("webhook client disconnected, but processing continues")
		}
	}()

	if err := o.processPR(ctx, meta); err != nil {
		o.log.Error().Err(err).Msg("failed to process PR")
		http.Error(w, "processing error", http.StatusInternalServerError)
		return
	}

	o.log.Info().
		Str("repo", meta.RepoOwner+"/"+meta.RepoName).
		Int("pr", meta.PRNumber).
		Msg("PR processing completed successfully")

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("processed"))
}

func (o *Orchestrator) processPR(ctx context.Context, meta intm.PRMetadata) error {
	o.log.Info().
		Str("repo", meta.RepoOwner+"/"+meta.RepoName).
		Int("pr", meta.PRNumber).
		Msg("starting PR processing pipeline")

	_, err := PrepareRepository(ctx, o.log, &meta)
	if err != nil {
		return err
	}

	o.log.Debug().
		Str("path", meta.LocalPath).
		Msg("repository prepared")

	desc, err := CallPRAgentDescribe(ctx, o.log, o.httpClient, o.cfg.PRAgentURL, meta)
	if err != nil {
		return err
	}
	o.log.Debug().Msg("PR-Agent describe completed")

	review, err := CallPRAgentReview(ctx, o.log, o.httpClient, o.cfg.PRAgentURL, meta, desc.DescriptionMarkdown)
	if err != nil {
		return err
	}
	o.log.Debug().Msg("PR-Agent review completed")

	// CHANGED: Now calls cfg.SemgrepServiceURL instead of cfg.SemgrepMCPURL
	semgrep, err := CallSemgrep(ctx, o.log, o.httpClient, o.cfg.SemgrepServiceURL, meta)
	if err != nil {
		return err
	}
	o.log.Debug().Msg("Semgrep scan completed")

	summary, err := CallSummarizer(
		ctx,
		o.log,
		o.httpClient,
		o.cfg.SummarizerURL,
		meta,
		desc.DescriptionMarkdown,
		review.ReviewMarkdown,
		semgrep.FindingsMarkdown,
		semgrep.Severity,
	)
	if err != nil {
		return err
	}
	o.log.Debug().Msg("Summarizer Agent completed")

	if err := PostGitHubComment(ctx, o.log, o.httpClient, o.cfg.GitHubMCPURL, meta, summary.Markdown); err != nil {
		return err
	}

	o.log.Info().Msg("GitHub PR comment posted successfully")
	return nil
}
