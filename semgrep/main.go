package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

// Server
type SemgrepServer struct {
	log zerolog.Logger
}

// Request/Response types
type ScanRequest struct {
	RepoPath string            `json:"repo_path"` // For reference/logging only
	RepoURL  string            `json:"repo_url,omitempty"`
	Branch   string            `json:"branch,omitempty"`
	Files    map[string]string `json:"files"` // filename -> content
}

type ScanResponse struct {
	Status           string                 `json:"status"`
	FindingsMarkdown string                 `json:"findings_markdown"`
	Severity         SemgrepSeveritySummary `json:"severity"`
	FindingsCount    int                    `json:"findings_count"`
	ScanDuration     string                 `json:"scan_duration"`
	Error            string                 `json:"error,omitempty"`
}

type SemgrepSeveritySummary struct {
	Blocker  int `json:"blocker"`
	Critical int `json:"critical"`
	Major    int `json:"major"`
	Minor    int `json:"minor"`
	Info     int `json:"info"`
}

// Semgrep JSON-RPC types
type semgrepRPCReq struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      string      `json:"id"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params"`
}

type semgrepRPCParams struct {
	Name      string      `json:"name"`
	Arguments interface{} `json:"arguments"`
}

type semgrepScanArgs struct {
	CodeFiles []map[string]string `json:"code_files"`
	Config    string              `json:"config,omitempty"`
}

type semgrepRPCResp struct {
	JSONRPC string `json:"jsonrpc"`
	ID      string `json:"id"`
	Result  struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"result"`
	Error interface{} `json:"error"`
}

type semgrepScanPayload struct {
	Results []semgrepResult `json:"results"`
	Errors  []interface{}   `json:"errors"`
}

type semgrepResult struct {
	CheckID string `json:"check_id"`
	Path    string `json:"path"`
	Start   struct {
		Line int `json:"line"`
	} `json:"start"`
	Extra struct {
		Severity string `json:"severity"`
		Message  string `json:"message"`
		Lines    string `json:"lines"`
	} `json:"extra"`
}

func main() {
	logger := newLogger()

	server := &SemgrepServer{
		log: logger,
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "80"
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", server.healthHandler)
	mux.HandleFunc("/scan", server.scanHandler)

	addr := ":" + port
	logger.Info().Str("port", port).Msg("Semgrep service starting")

	if err := http.ListenAndServe(addr, mux); err != nil {
		logger.Fatal().Err(err).Msg("server failed to start")
	}
}

func newLogger() zerolog.Logger {
	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}

	level := zerolog.InfoLevel
	switch strings.ToLower(logLevel) {
	case "debug":
		level = zerolog.DebugLevel
	case "warn":
		level = zerolog.WarnLevel
	case "error":
		level = zerolog.ErrorLevel
	}

	zerolog.TimeFieldFormat = time.RFC3339
	return zerolog.New(os.Stdout).With().
		Timestamp().
		Str("service", "semgrep").
		Logger().
		Level(level)
}

func (s *SemgrepServer) healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy", "service": "semgrep"})
}

func (s *SemgrepServer) scanHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.log.Error().Err(err).Msg("failed to decode request")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.RepoPath == "" {
		http.Error(w, "repo_path is required", http.StatusBadRequest)
		return
	}

	s.log.Info().Str("repo_path", req.RepoPath).Msg("starting semgrep scan")

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Minute)
	defer cancel()

	startTime := time.Now()
	result := s.performScan(ctx, req)
	result.ScanDuration = time.Since(startTime).String()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (s *SemgrepServer) performScan(ctx context.Context, req ScanRequest) ScanResponse {
	// Use files from request body
	if len(req.Files) == 0 {
		s.log.Warn().Msg("no code files provided")
		return ScanResponse{
			Status:           "success",
			FindingsMarkdown: "No code files found to scan. ✅",
			FindingsCount:    0,
			Severity:         SemgrepSeveritySummary{},
		}
	}

	s.log.Info().Int("file_count", len(req.Files)).Msg("processing files from request")

	// Convert files map to codeFiles format for Semgrep
	codeFiles := make([]map[string]string, 0, len(req.Files))
	for filename, content := range req.Files {
		codeFiles = append(codeFiles, map[string]string{
			"filename": filename, // Semgrep expects "filename", not "path"
			"content":  content,
		})
	}

	s.log.Info().Int("files", len(codeFiles)).Msg("collected code files")

	// Get Semgrep MCP URL
	semgrepMCPURL := os.Getenv("SEMGREP_MCP_URL")
	if semgrepMCPURL == "" {
		semgrepMCPURL = "https://mcp.semgrep.ai/mcp"
	}

	// Try multiple Semgrep configurations
	semgrepConfigs := []string{
		"p/default",
		"p/security-audit",
		"p/ci",
	}

	var parsed semgrepScanPayload
	var lastError error

	for _, cfg := range semgrepConfigs {
		s.log.Debug().Str("config", cfg).Msg("trying Semgrep config")

		result, err := s.callSemgrepMCP(ctx, semgrepMCPURL, codeFiles, cfg)
		if err != nil {
			s.log.Warn().Err(err).Str("config", cfg).Msg("semgrep config failed")
			lastError = err
			continue
		}

		if len(result.Results) > 0 {
			parsed = result
			s.log.Info().
				Str("config", cfg).
				Int("findings", len(result.Results)).
				Msg("semgrep scan successful")
			break
		}
	}

	// No findings found
	if len(parsed.Results) == 0 {
		if lastError != nil {
			s.log.Warn().Err(lastError).Msg("all semgrep configs failed")
			return ScanResponse{
				Status:           "error",
				FindingsMarkdown: s.generateFallbackMarkdown(),
				Error:            "Semgrep scan failed for all configurations",
			}
		}

		s.log.Info().Msg("semgrep found no issues")
		return ScanResponse{
			Status:           "success",
			FindingsMarkdown: "No security issues found by Semgrep. ✅",
			FindingsCount:    0,
			Severity:         SemgrepSeveritySummary{},
		}
	}

	// Compute severity summary
	severity := SemgrepSeveritySummary{}
	for _, r := range parsed.Results {
		switch strings.ToLower(r.Extra.Severity) {
		case "blocker":
			severity.Blocker++
		case "error", "critical":
			severity.Critical++
		case "warning", "major":
			severity.Major++
		case "note", "minor":
			severity.Minor++
		default:
			severity.Info++
		}
	}

	// Format findings as markdown
	markdown := s.formatSemgrepMarkdown(parsed.Results)

	s.log.Info().
		Int("total", len(parsed.Results)).
		Int("blocker", severity.Blocker).
		Int("critical", severity.Critical).
		Int("major", severity.Major).
		Int("minor", severity.Minor).
		Int("info", severity.Info).
		Msg("semgrep scan completed successfully")

	return ScanResponse{
		Status:           "success",
		FindingsMarkdown: markdown,
		Severity:         severity,
		FindingsCount:    len(parsed.Results),
	}
}

func (s *SemgrepServer) callSemgrepMCP(ctx context.Context, url string, codeFiles []map[string]string, config string) (semgrepScanPayload, error) {
	reqBody := semgrepRPCReq{
		JSONRPC: "2.0",
		ID:      "semgrep_scan",
		Method:  "tools/call",
		Params: semgrepRPCParams{
			Name: "semgrep_scan",
			Arguments: semgrepScanArgs{
				CodeFiles: codeFiles,
				Config:    config,
			},
		},
	}

	bodyJSON, err := json.Marshal(reqBody)
	if err != nil {
		return semgrepScanPayload{}, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyJSON))
	if err != nil {
		return semgrepScanPayload{}, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	// Add Semgrep API token if available
	if token := os.Getenv("SEMGREP_APP_TOKEN"); token != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
		s.log.Debug().Msg("using SEMGREP_APP_TOKEN")
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return semgrepScanPayload{}, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return semgrepScanPayload{}, fmt.Errorf("status %d", resp.StatusCode)
	}

	var rpc semgrepRPCResp
	if err := json.NewDecoder(resp.Body).Decode(&rpc); err != nil {
		return semgrepScanPayload{}, fmt.Errorf("decode response: %w", err)
	}

	if rpc.Error != nil {
		return semgrepScanPayload{}, fmt.Errorf("semgrep error: %v", rpc.Error)
	}

	if len(rpc.Result.Content) == 0 {
		return semgrepScanPayload{}, fmt.Errorf("no content in response")
	}

	rawText := rpc.Result.Content[0].Text

	// Debug: log the raw response
	s.log.Debug().
		Str("raw_response_preview", truncate(rawText, 200)).
		Msg("received Semgrep MCP response")

	var parsed semgrepScanPayload
	if err := json.Unmarshal([]byte(rawText), &parsed); err != nil {
		s.log.Error().
			Str("raw_text", truncate(rawText, 500)).
			Msg("failed to parse Semgrep response")
		return semgrepScanPayload{}, fmt.Errorf("parse results: %w", err)
	}

	return parsed, nil
}

func (s *SemgrepServer) collectCodeFiles(root string) ([]map[string]string, error) {
	var out []map[string]string

	allowed := map[string]bool{
		".go": true, ".py": true, ".js": true, ".ts": true,
		".jsx": true, ".tsx": true, ".java": true, ".rb": true,
		".php": true, ".cs": true, ".c": true, ".cpp": true,
	}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			name := info.Name()
			if name == ".git" || name == "node_modules" || name == "vendor" ||
				name == "venv" || name == "__pycache__" || strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}

		ext := filepath.Ext(path)
		if !allowed[ext] {
			return nil
		}

		if info.Size() > 1024*1024 {
			s.log.Debug().Str("file", path).Msg("skipping large file")
			return nil
		}

		data, readErr := os.ReadFile(path)
		if readErr != nil {
			s.log.Warn().Err(readErr).Str("file", path).Msg("failed to read file")
			return nil
		}

		rel, _ := filepath.Rel(root, path)

		out = append(out, map[string]string{
			"filename": rel,
			"content":  string(data),
		})

		return nil
	})

	return out, err
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...(truncated)"
}

func (s *SemgrepServer) formatSemgrepMarkdown(results []semgrepResult) string {
	if len(results) == 0 {
		return "No security issues found by Semgrep."
	}

	var sb strings.Builder

	// Count by severity
	var blocker, critical, major, minor, info int
	for _, r := range results {
		sev := strings.ToLower(r.Extra.Severity)
		switch sev {
		case "blocker":
			blocker++
		case "error", "critical":
			critical++
		case "warning", "major":
			major++
		case "note", "minor":
			minor++
		default:
			info++
		}
	}

	sb.WriteString("### Semgrep Summary\n\n")
	sb.WriteString("**Issue Counts:**\n\n")
	sb.WriteString("| 🚫 Blocker | 🔴 Critical | 🟠 Major | 🟡 Minor | ℹ️ Info |\n")
	sb.WriteString("|:----------:|:-----------:|:--------:|:--------:|:-------:|\n")

	blockerStr := formatCount(blocker, true)
	criticalStr := formatCount(critical, true)
	majorStr := formatCount(major, true)
	minorStr := formatCount(minor, false)
	infoStr := formatCount(info, false)

	sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n\n",
		blockerStr, criticalStr, majorStr, minorStr, infoStr))

	// Group results by severity
	blockerIssues := []semgrepResult{}
	criticalIssues := []semgrepResult{}
	majorIssues := []semgrepResult{}
	minorIssues := []semgrepResult{}
	infoIssues := []semgrepResult{}

	for _, r := range results {
		sev := strings.ToLower(r.Extra.Severity)
		switch sev {
		case "blocker":
			blockerIssues = append(blockerIssues, r)
		case "error", "critical":
			criticalIssues = append(criticalIssues, r)
		case "warning", "major":
			majorIssues = append(majorIssues, r)
		case "note", "minor":
			minorIssues = append(minorIssues, r)
		default:
			infoIssues = append(infoIssues, r)
		}
	}

	// Write blocker issues
	if len(blockerIssues) > 0 {
		sb.WriteString("### 🚫 Blocker Issues\n\n")
		for _, r := range blockerIssues {
			sb.WriteString(fmt.Sprintf("- **%s** in `%s:%d`\n", r.Extra.Message, r.Path, r.Start.Line))
			sb.WriteString(fmt.Sprintf("  - Rule: `%s`\n", r.CheckID))
			if r.Extra.Lines != "" {
				sb.WriteString(fmt.Sprintf("  - Code: `%s`\n", strings.TrimSpace(r.Extra.Lines)))
			}
			sb.WriteString("\n")
		}
	}

	// Write critical issues
	if len(criticalIssues) > 0 {
		sb.WriteString("### 🔴 Critical Issues\n\n")
		for _, r := range criticalIssues {
			sb.WriteString(fmt.Sprintf("- **%s** in `%s:%d`\n", r.Extra.Message, r.Path, r.Start.Line))
			sb.WriteString(fmt.Sprintf("  - Rule: `%s`\n", r.CheckID))
			if r.Extra.Lines != "" {
				sb.WriteString(fmt.Sprintf("  - Code: `%s`\n", strings.TrimSpace(r.Extra.Lines)))
			}
			sb.WriteString("\n")
		}
	}

	// Write major issues
	if len(majorIssues) > 0 {
		sb.WriteString("### 🟠 Major Issues\n\n")
		for _, r := range majorIssues {
			sb.WriteString(fmt.Sprintf("- **%s** in `%s:%d`\n", r.Extra.Message, r.Path, r.Start.Line))
			sb.WriteString(fmt.Sprintf("  - Rule: `%s`\n", r.CheckID))
			if r.Extra.Lines != "" {
				lines := strings.TrimSpace(r.Extra.Lines)
				if len(lines) > 0 {
					sb.WriteString(fmt.Sprintf("  - Code: `%s`\n", lines))
				}
			}
			sb.WriteString("\n")
		}
	}

	// Write minor issues (limited to first 5)
	if len(minorIssues) > 0 {
		sb.WriteString("### 🟡 Minor Issues\n\n")
		for i, r := range minorIssues {
			if i < 5 {
				sb.WriteString(fmt.Sprintf("- %s in `%s:%d`\n", r.Extra.Message, r.Path, r.Start.Line))
			}
		}
		if len(minorIssues) > 5 {
			sb.WriteString(fmt.Sprintf("\n*...and %d more minor issues*\n\n", len(minorIssues)-5))
		}
	}

	// Write info issues (limited to first 3)
	if len(infoIssues) > 0 {
		sb.WriteString("### ℹ️ Info\n\n")
		for i, r := range infoIssues {
			if i < 3 {
				sb.WriteString(fmt.Sprintf("- %s in `%s:%d`\n", r.Extra.Message, r.Path, r.Start.Line))
			}
		}
		if len(infoIssues) > 3 {
			sb.WriteString(fmt.Sprintf("\n*...and %d more info items*\n\n", len(infoIssues)-3))
		}
	}

	return sb.String()
}

func formatCount(count int, shouldBold bool) string {
	if shouldBold && count > 0 {
		return fmt.Sprintf("**%d**", count)
	}
	return fmt.Sprintf("%d", count)
}

func (s *SemgrepServer) generateFallbackMarkdown() string {
	return `### Security Analysis

**Note:** Automated security scanning encountered issues. Here are general recommendations:

#### 🔒 Security Best Practices

- Ensure all user input is properly validated and sanitized
- Review authentication and authorization logic
- Check for hardcoded secrets or credentials
- Verify error handling doesn't expose sensitive information
- Ensure all dependencies are up-to-date

#### 📋 Code Quality

- Add unit tests for critical functions
- Review error handling for edge cases
- Ensure proper logging without exposing sensitive data
- Check for unused code or expired TODOs

**Recommendation:** Run a manual security review or local Semgrep scan for comprehensive analysis.
`
}
