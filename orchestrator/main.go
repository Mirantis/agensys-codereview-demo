package main

import (
	"net"
	"net/http"
	"os"
	"strconv"
	"time"

	intm "orchestrator/internal"

	"github.com/rs/zerolog"
)

type Orchestrator struct {
	log                zerolog.Logger
	cfg                intm.Config
	httpClient         *http.Client
	HTTPTimeoutMinutes int
}

func main() {
	cfg := loadConfigFromEnv()
	logger := intm.NewLogger(cfg.LogLevel)

	// Parse HTTP timeout from config (in minutes)
	httpTimeout := time.Duration(cfg.HTTPTimeoutMinutes) * time.Minute

	httpClient := &http.Client{
		Timeout: httpTimeout, // Use configurable timeout
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 60 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	oa := &Orchestrator{
		log:        logger,
		cfg:        cfg,
		httpClient: httpClient,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", oa.healthHandler)
	mux.Handle("/webhook", oa.loggingMiddleware(http.HandlerFunc(oa.prWebhookHandler)))

	logger.Info().
		Str("addr", cfg.ListenAddr).
		Str("pr_agent", cfg.PRAgentURL).
		Str("semgrep_service", cfg.SemgrepServiceURL). // CHANGED: Now logs Semgrep service URL
		Str("summarizer", cfg.SummarizerURL).
		Str("github_mcp", cfg.GitHubMCPURL).
		Int("http_timeout_minutes", cfg.HTTPTimeoutMinutes).
		Msg("starting orchestrator")

	if err := http.ListenAndServe(cfg.ListenAddr, mux); err != nil {
		logger.Fatal().Err(err).Msg("server exited")
	}
}

func loadConfigFromEnv() intm.Config {
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8085"
	}

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "debug"
	}

	// HTTP timeout configuration (in minutes)
	httpTimeoutMinutes := 15 // Default 15 minutes
	if timeoutStr := os.Getenv("HTTP_TIMEOUT_MINUTES"); timeoutStr != "" {
		if timeout, err := strconv.Atoi(timeoutStr); err == nil && timeout > 0 {
			httpTimeoutMinutes = timeout
		}
	}

	prAgentURL := os.Getenv("PR_AGENT_URL")
	if prAgentURL == "" {
		prAgentURL = "http://pr-agent:80/post"
	}

	// CHANGED: Now reads SEMGREP_SERVICE_URL instead of SEMGREP_MCP_URL
	semgrepServiceURL := os.Getenv("SEMGREP_SERVICE_URL")
	if semgrepServiceURL == "" {
		// Default to Kubernetes/Docker service name
		semgrepServiceURL = "http://semgrep-service:8086"
	}

	summarizerURL := os.Getenv("SUMMARIZER_URL")
	if summarizerURL == "" {
		summarizerURL = "http://summarizer-agent:80/post"
	}

	githubMCPURL := os.Getenv("GITHUB_MCP_URL")
	if githubMCPURL == "" {
		githubMCPURL = "http://github-mcp-server:80/comment"
	}

	return intm.Config{
		ListenAddr:         addr,
		LogLevel:           logLevel,
		HTTPTimeoutMinutes: httpTimeoutMinutes,
		PRAgentURL:         prAgentURL,
		SemgrepServiceURL:  semgrepServiceURL, // CHANGED: New field name
		SummarizerURL:      summarizerURL,
		GitHubMCPURL:       githubMCPURL,
	}
}

func (o *Orchestrator) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (o *Orchestrator) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		o.log.Debug().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Msg("incoming request")
		next.ServeHTTP(w, r)
	})
}
