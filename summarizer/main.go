package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

type Config struct {
	ListenAddr string

	AnthropicKey string
	Model        string

	SummarizerPrompt string
}

func loadConfig() Config {
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":80"
	}

	model := os.Getenv("ANTHROPIC_MODEL")
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}

	prompt := os.Getenv("SUMMARIZER_PROMPT")
	if strings.TrimSpace(prompt) == "" {
		prompt = defaultSummarizerPrompt
	}

	return Config{
		ListenAddr:       addr,
		AnthropicKey:     os.Getenv("ANTHROPIC_API_KEY"),
		Model:            model,
		SummarizerPrompt: prompt,
	}
}

type PRMetadata struct {
	RepoOwner    string `json:"repo_owner"`
	RepoName     string `json:"repo_name"`
	PRNumber     int    `json:"pr_number"`
	HeadSHA      string `json:"head_sha"`
	Title        string `json:"title"`
	Body         string `json:"body"`
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
	URL          string `json:"url"`
	LocalPath    string `json:"local_path"`
}

type SemgrepSeverity struct {
	Blocker  int `json:"blocker"`
	Critical int `json:"critical"`
	Major    int `json:"major"`
	Minor    int `json:"minor"`
	Info     int `json:"info"`
}

type SummarizerRequest struct {
	PR                  PRMetadata      `json:"pr"`
	DescriptionMarkdown string          `json:"description_markdown"`
	ReviewMarkdown      string          `json:"review_markdown"`
	SemgrepMarkdown     string          `json:"semgrep_markdown"`
	SonarQubeMarkdown   string          `json:"sonarqube_markdown,omitempty"`
	SemgrepSeverity     SemgrepSeverity `json:"semgrep_severity"`
}

type SummarizerResponse struct {
	Markdown string `json:"markdown"` // MATCHES orchestrator's SummarizerOut
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

func doConnectivityCheck() {
	log.Printf("🧪 DNS: api.anthropic.com ...")
	addrs, err := net.LookupIP("api.anthropic.com")
	if err != nil {
		log.Printf("❌ DNS fail: %v", err)
	} else {
		log.Printf("✔️ DNS OK → %v", addrs)
	}

	log.Printf("🧪 TCP: api.anthropic.com:443 ...")
	conn, err := net.DialTimeout("tcp", "api.anthropic.com:443", 5*time.Second)
	if err != nil {
		log.Printf("❌ TCP fail: %v", err)
	} else {
		log.Printf("✔️ TCP OK")
		conn.Close()
	}
}

func callLLM(ctx context.Context, cfg Config, systemPrompt, userPrompt string) (string, error) {
	if cfg.AnthropicKey == "" {
		return "", fmt.Errorf("missing ANTHROPIC_API_KEY")
	}

	reqData := anthropicRequest{
		Model:     cfg.Model,
		MaxTokens: 4096,
		System:    systemPrompt,
		Messages: []anthropicMessage{
			{Role: "user", Content: userPrompt},
		},
	}

	body, _ := json.Marshal(reqData)

	log.Printf("🧪================ ANTHROPIC CALL DEBUG =================")
	log.Printf("🧪 Model: %s", cfg.Model)
	log.Printf("📤 Request JSON (first 300 chars):")
	out := string(body)
	if len(out) > 300 {
		out = out[:300] + "...(truncated)"
	}
	log.Printf("%s", out)

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://api.anthropic.com/v1/messages",
		bytes.NewReader(body),
	)
	if err != nil {
		return "", err
	}

	req.Header.Set("x-api-key", cfg.AnthropicKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	log.Printf("🌐 Returned: %d %s", resp.StatusCode, resp.Status)

	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("Anthropic error: %s", b)
	}

	var parsed anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return "", err
	}
	if len(parsed.Content) == 0 {
		return "", fmt.Errorf("Anthropic returned no content")
	}

	answer := parsed.Content[0].Text

	log.Printf("📊 Token Usage: input=%d, output=%d, total=%d",
		parsed.Usage.InputTokens,
		parsed.Usage.OutputTokens,
		parsed.Usage.InputTokens+parsed.Usage.OutputTokens)

	log.Printf("📥 Response Preview:")
	respPreview := answer
	if len(respPreview) > 300 {
		respPreview = respPreview[:300] + "...(truncated)"
	}
	log.Printf("%s", respPreview)

	log.Printf("🧪============== END ANTHROPIC CALL DEBUG ==============")

	return answer, nil
}

func summarizerHandler(cfg Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		raw, _ := io.ReadAll(r.Body)
		log.Printf("📥 Summarizer received:\n%s\n", string(raw))

		var req SummarizerRequest
		if err := json.Unmarshal(raw, &req); err != nil {
			http.Error(w, "bad JSON", http.StatusBadRequest)
			return
		}

		// Build security block
		securityBlock := ""
		if strings.TrimSpace(req.SemgrepMarkdown) != "" {
			securityBlock += "\n=== Semgrep Findings ===\n" + req.SemgrepMarkdown + "\n"
		}

		if strings.TrimSpace(req.SonarQubeMarkdown) != "" {
			securityBlock += "\n=== SonarQube Findings ===\n" + req.SonarQubeMarkdown + "\n"
		}

		if securityBlock == "" {
			securityBlock = "\n=== Security Analysis ===\n(No static analysis provided)\n"
		}

		// Add severity summary section
		severityBlock := fmt.Sprintf(`
=== Semgrep Severity Summary ===
- Blocker: %d
- Critical: %d
- Major: %d
- Minor: %d
- Info: %d
`,
			req.SemgrepSeverity.Blocker,
			req.SemgrepSeverity.Critical,
			req.SemgrepSeverity.Major,
			req.SemgrepSeverity.Minor,
			req.SemgrepSeverity.Info,
		)

		// Merge into final user prompt
		userPrompt := fmt.Sprintf(`
PR: #%d - %s
Repo: %s/%s
URL: %s

=== PR Description ===
%s

=== Review ===
%s

%s
%s

Task:
Integrate description, review, Semgrep, and severity into a single PR comment using the required markdown format.
`,
			req.PR.PRNumber,
			req.PR.Title,
			req.PR.RepoOwner,
			req.PR.RepoName,
			req.PR.URL,
			req.DescriptionMarkdown,
			req.ReviewMarkdown,
			securityBlock,
			severityBlock,
		)

		doConnectivityCheck()

		ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
		defer cancel()

		md, err := callLLM(ctx, cfg, cfg.SummarizerPrompt, userPrompt)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		resp := SummarizerResponse{Markdown: md}

		log.Printf("====================== FINAL MERGED MARKDOWN ======================")
		prev := md
		if len(prev) > 600 {
			prev = prev[:600] + "...(truncated)"
		}
		log.Printf("%s", prev)
		log.Printf("==================================================================")

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func main() {
	cfg := loadConfig()

	if cfg.AnthropicKey == "" {
		log.Printf("⚠️ Warning: ANTHROPIC_API_KEY is empty — summarizer will fail.")
	}

	http.HandleFunc("/post", summarizerHandler(cfg))
	http.HandleFunc("/health", healthHandler)

	log.Printf("🚀 Summarizer Agent running on %s (model=%s)", cfg.ListenAddr, cfg.Model)
	if err := http.ListenAndServe(cfg.ListenAddr, nil); err != nil {
		log.Fatalf("Summarizer failed: %v", err)
	}
}

const defaultSummarizerPrompt = `
You are a Senior Software Engineering Manager who has received input from your lead Software QA Engineer and your Cybersecurity QA Engineer on a specific Pull Request.
You must carefully integrate their feedback into a single unified comment on the Pull Request.

YOU MUST STRICTLY FOLLOW THIS MARKDOWN FORMAT:

# PR Review — <title> (#{number}) • by <author> on <date>
**Recommendation**: <Approve | Changes Requested | Blocked> • **Risk**: <Low|Med|High> • **Confidence**: <%>

## TL;DR
- <one-sentence intent/outcome>
- **Must-do before merge (top 2)**: 1) <…>  2) <…>

## Scope & Changes
- **Areas Touched**: <packages/modules/paths>
- **Key Changes**:
  - <change 1>
  - <change 2>
- **Out of Scope**: <if any>

## High Level Assessment
- **Top Findings**:
  - <finding 1>
  - <finding 2>
- **Recommended Changes**:
  - [ ] <change request 1>
  - [ ] <change request 2>
- **Nice-to-Have Improvements**:
  - <improvement 1>

## Detailed Assessment

### Semgrep Summary
- **Issue Counts**

  | Blocker | Critical | Major | Minor | Info |
  | --- | --- | --- | --- | --- |
  | <n> | <n> | <n> | <n> | <n> |

- **Hotspots**: <security hotspots or smells>
- **New Debt**: <time/estimate>
- **Quality Gate**: <Pass | Fail>

### Tests & Coverage
- **New/Updated Tests**: <list or count>
- **Coverage Delta**: <+/- %> overall; <files at risk>
- **Failing/Flaky**: <if any>

### Security & Compliance
- **Dependency Changes**: <adds/bumps>
- **Secrets/PII**: <scan result>
- **Auth/Perms**: <changes to roles/scopes>
- **Data Handling**: <schema/retention/exports>

### Performance & Reliability
- **Perf Risks**: <complexity, N+1, allocations>
- **Concurrency/Async**: <thread-safety, races>
- **Observability**: <logs, metrics, tracing>

### Merge Readiness Checklist
- [ ] CI green
- [ ] Quality gate passed or justified
- [ ] Tests updated and sufficient
- [ ] Security review items resolved
- [ ] Migrations tested and reversible
- [ ] Docs/Changelog updated

Your output MUST strictly follow this format.
`
