package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

/* =====================================================================================
   CONFIG
===================================================================================== */

type Config struct {
	ListenAddr     string
	LogLevel       string
	OpenAIKey      string
	Model          string
	Timeout        int // Timeout in seconds
	MaxTokens      int // Max tokens for OpenAI response
	PromptDescribe string
	PromptReview   string
}

func loadConfig() Config {
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":80"
	}

	// Default 10 minutes (600 seconds)
	timeout := 600
	if t := os.Getenv("OPENAI_TIMEOUT"); t != "" {
		if parsed, err := strconv.Atoi(t); err == nil {
			timeout = parsed
		}
	}

	// Default 1500 tokens (balance between quality and speed)
	maxTokens := 1500
	if mt := os.Getenv("MAX_TOKENS"); mt != "" {
		if parsed, err := strconv.Atoi(mt); err == nil {
			maxTokens = parsed
		}
	}

	model := os.Getenv("PR_AGENT_MODEL")
	if model == "" {
		model = "gpt-4o"
	}

	return Config{
		ListenAddr:     addr,
		LogLevel:       os.Getenv("LOG_LEVEL"),
		OpenAIKey:      os.Getenv("OPENAI_API_KEY"),
		Model:          model,
		Timeout:        timeout,
		MaxTokens:      maxTokens,
		PromptDescribe: os.Getenv("PR_AGENT_PROMPT_DESCRIBE"),
		PromptReview:   os.Getenv("PR_AGENT_PROMPT_REVIEW"),
	}
}

/* =====================================================================================
   HTTP CLIENT
===================================================================================== */

var httpClient = &http.Client{
	Timeout: 15 * time.Minute, // Very generous - actual timeout controlled by context
	Transport: &http.Transport{
		DialContext: (&net.Dialer{
			Timeout:   10 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	},
}

/* =====================================================================================
   MODELS
===================================================================================== */

type PRMetadata struct {
	RepoOwner    string `json:"repo_owner"`
	RepoName     string `json:"repo_name"`
	PRNumber     int    `json:"pr_number"`
	Title        string `json:"title"`
	Body         string `json:"body"`
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
	HeadSHA      string `json:"head_sha"`
	LocalPath    string `json:"local_path"`
}

type PRAgentRequest struct {
	Mode                string     `json:"mode"` // describe | review
	PR                  PRMetadata `json:"pr"`
	DescriptionMarkdown string     `json:"description_markdown,omitempty"`
}

type PRAgentDescribeOut struct {
	DescriptionMarkdown string `json:"description_markdown"`
}

type PRAgentReviewOut struct {
	ReviewMarkdown string `json:"review_markdown"`
}

/* =====================================================================================
   OPENAI MODELS
===================================================================================== */

type openAIChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIChatRequest struct {
	Model       string              `json:"model"`
	Temperature float32             `json:"temperature"`
	MaxTokens   int                 `json:"max_tokens"`
	Messages    []openAIChatMessage `json:"messages"`
}

type openAIChatResponse struct {
	Choices []struct {
		Message openAIChatMessage `json:"message"`
	} `json:"choices"`
}

/* =====================================================================================
   HELPER FUNCTIONS
===================================================================================== */

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func estimateTokens(text string) int {
	// Rough estimate: 4 characters per token
	return len(text) / 4
}

func calculateDynamicTimeout(userPrompt string, baseTimeout int) time.Duration {
	tokens := estimateTokens(userPrompt)

	timeout := time.Duration(baseTimeout) * time.Second

	// Add extra time for large prompts
	if tokens > 3000 {
		timeout = timeout + 3*time.Minute
		log.Printf("📊 Large prompt detected (%d tokens), extended timeout to %v", tokens, timeout)
	} else if tokens > 1500 {
		timeout = timeout + 1*time.Minute
		log.Printf("📊 Medium prompt detected (%d tokens), extended timeout to %v", tokens, timeout)
	} else {
		log.Printf("📊 Prompt size: %d tokens, timeout: %v", tokens, timeout)
	}

	return timeout
}

/* =====================================================================================
   LLM CALL
===================================================================================== */

func callLLM(ctx context.Context, cfg Config, mode string, sys string, user string) (string, error) {

	start := time.Now()
	defer func() {
		duration := time.Since(start)
		log.Printf("⏱️ OpenAI call completed in: %.2f seconds", duration.Seconds())
	}()

	log.Printf("🧪================ LLM CALL DEBUG =================")
	log.Printf("🧪 Mode: %s", mode)
	log.Printf("🧪 Model: %s", cfg.Model)
	log.Printf("🧪 Max Tokens: %d", cfg.MaxTokens)

	// -----------------------------------------------------
	// VALIDATE MODEL
	// -----------------------------------------------------
	validModels := map[string]bool{
		"gpt-4o":       true,
		"gpt-4o-mini":  true,
		"gpt-4.1":      true,
		"gpt-4.1-mini": true,
	}

	if !validModels[cfg.Model] {
		log.Printf("❌ INVALID MODEL NAME: %s", cfg.Model)
		log.Printf("✔️ Allowed: gpt-4o, gpt-4o-mini, gpt-4.1, gpt-4.1-mini")
		return "", fmt.Errorf("invalid model: %s", cfg.Model)
	}
	log.Printf("✔️ Model validated")

	// -----------------------------------------------------
	// CHECK OPENAI KEY
	// -----------------------------------------------------
	if cfg.OpenAIKey == "" {
		log.Printf("❌ Missing OPENAI_API_KEY")
		return "", fmt.Errorf("missing OPENAI_API_KEY")
	}
	log.Printf("✔️ OPENAI_API_KEY is present")

	// -----------------------------------------------------
	// DNS CHECK
	// -----------------------------------------------------
	log.Printf("🌐 DNS: resolving api.openai.com ...")
	addrs, dnsErr := net.LookupHost("api.openai.com")
	if dnsErr != nil {
		log.Printf("❌ DNS failed: %v", dnsErr)
		return "", dnsErr
	}
	log.Printf("✔️ DNS OK → %v", addrs)

	// -----------------------------------------------------
	// TCP CHECK
	// -----------------------------------------------------
	log.Printf("🌐 TCP: connecting to api.openai.com:443 ...")
	conn, tcpErr := net.DialTimeout("tcp", "api.openai.com:443", 3*time.Second)
	if tcpErr != nil {
		log.Printf("❌ TCP failed: %v", tcpErr)
		return "", tcpErr
	}
	conn.Close()
	log.Printf("✔️ TCP connectivity OK")

	// -----------------------------------------------------
	// BUILD REQUEST
	// -----------------------------------------------------
	log.Printf("🧪 Sys prompt size: %d chars (~%d tokens)", len(sys), estimateTokens(sys))
	log.Printf("🧪 User prompt size: %d chars (~%d tokens)", len(user), estimateTokens(user))

	reqObj := openAIChatRequest{
		Model:       cfg.Model,
		Temperature: 0.3,
		MaxTokens:   cfg.MaxTokens,
		Messages: []openAIChatMessage{
			{Role: "system", Content: sys},
			{Role: "user", Content: user},
		},
	}

	body, _ := json.Marshal(reqObj)

	log.Printf("📤 OpenAI Request JSON (first 400 chars):\n%s",
		string(body[:min(400, len(body))]))

	req, err := http.NewRequestWithContext(
		ctx,
		"POST",
		"https://api.openai.com/v1/chat/completions",
		strings.NewReader(string(body)),
	)
	if err != nil {
		log.Printf("❌ Failed creating request: %v", err)
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+cfg.OpenAIKey)
	req.Header.Set("Content-Type", "application/json")

	// -----------------------------------------------------
	// SEND REQUEST
	// -----------------------------------------------------
	log.Printf("🚀 Calling OpenAI model=%s mode=%s ...", cfg.Model, mode)

	resp, err := httpClient.Do(req)
	if err != nil {
		log.Printf("❌ Network error: %v", err)
		return "", err
	}
	defer resp.Body.Close()

	log.Printf("🌐 OpenAI returned status: %d", resp.StatusCode)

	if resp.StatusCode >= 300 {
		errBody, _ := io.ReadAll(resp.Body)
		log.Printf("❌ OpenAI Error Body:\n%s", string(errBody))
		return "", fmt.Errorf("openai error: %d", resp.StatusCode)
	}

	// -----------------------------------------------------
	// PARSE RESPONSE
	// -----------------------------------------------------
	var parsed openAIChatResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		log.Printf("❌ Decode error: %v", err)
		return "", err
	}

	if len(parsed.Choices) == 0 {
		log.Printf("❌ Empty choices from OpenAI")
		return "", fmt.Errorf("empty OpenAI response")
	}

	out := parsed.Choices[0].Message.Content

	log.Printf("📥 OpenAI Response: %d chars (~%d tokens)", len(out), estimateTokens(out))
	log.Printf("📥 Response Preview:\n%s", out[:min(300, len(out))])

	log.Printf("🧪============== END LLM CALL DEBUG ==============")

	return out, nil
}

/* =====================================================================================
   HTTP HANDLER
===================================================================================== */

func prAgentHandler(cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		raw, _ := io.ReadAll(r.Body)
		log.Printf("📥 PR-Agent incoming request:\n%s\n", string(raw))

		var req PRAgentRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			http.Error(w, "invalid JSON", 400)
			return
		}

		switch req.Mode {

		case "describe":
			sys := cfg.PromptDescribe
			if sys == "" {
				sys = "You are a Senior Engineer writing a detailed PR description."
			}

			userPrompt := fmt.Sprintf(`
Repository: %s/%s
PR #%d
Title: %s
Body: %s
Branch: %s → %s
LocalPath: %s
`,
				req.PR.RepoOwner, req.PR.RepoName, req.PR.PRNumber,
				req.PR.Title, req.PR.Body,
				req.PR.SourceBranch, req.PR.TargetBranch,
				req.PR.LocalPath,
			)

			// Calculate dynamic timeout based on prompt size
			timeout := calculateDynamicTimeout(userPrompt, cfg.Timeout)
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			log.Printf("⏱️ Starting describe with %v timeout", timeout)

			out, err := callLLM(ctx, cfg, "describe", sys, userPrompt)
			if err != nil {
				log.Printf("❌ describe failed: %v", err)
				http.Error(w, err.Error(), 500)
				return
			}

			log.Printf("✅ describe completed successfully")
			json.NewEncoder(w).Encode(PRAgentDescribeOut{DescriptionMarkdown: out})
			return

		case "review":
			sys := cfg.PromptReview
			if sys == "" {
				sys = "You are a Staff Engineer performing a deep code review."
			}

			userPrompt := fmt.Sprintf(`
Repository: %s/%s
PR #%d
Description:
%s

Local path: %s
`,
				req.PR.RepoOwner, req.PR.RepoName, req.PR.PRNumber,
				req.DescriptionMarkdown,
				req.PR.LocalPath,
			)

			// Calculate dynamic timeout based on prompt size
			timeout := calculateDynamicTimeout(userPrompt, cfg.Timeout)
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			log.Printf("⏱️ Starting review with %v timeout", timeout)

			out, err := callLLM(ctx, cfg, "review", sys, userPrompt)
			if err != nil {
				log.Printf("❌ review failed: %v", err)
				http.Error(w, err.Error(), 500)
				return
			}

			log.Printf("✅ review completed successfully")
			json.NewEncoder(w).Encode(PRAgentReviewOut{ReviewMarkdown: out})
			return

		default:
			http.Error(w, "unknown mode", 400)
		}
	}
}

/* =====================================================================================
   HEALTH CHECK
===================================================================================== */

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

/* =====================================================================================
   MAIN
===================================================================================== */

func main() {
	cfg := loadConfig()

	http.HandleFunc("/post", prAgentHandler(cfg))
	http.HandleFunc("/health", healthHandler)

	log.Printf("🚀 PR-Review Agent running on %s", cfg.ListenAddr)
	log.Printf("📋 Model: %s", cfg.Model)
	log.Printf("⏱️ Base Timeout: %d seconds", cfg.Timeout)
	log.Printf("🎯 Max Tokens: %d", cfg.MaxTokens)
	log.Fatal(http.ListenAndServe(cfg.ListenAddr, nil))
}
