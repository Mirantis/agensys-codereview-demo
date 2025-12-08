package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	intm "orchestrator/internal"

	"github.com/rs/zerolog"
)

// SemgrepServiceRequest represents the request to the Semgrep service
type SemgrepServiceRequest struct {
	RepoPath string            `json:"repo_path"` // For reference only
	RepoURL  string            `json:"repo_url,omitempty"`
	Branch   string            `json:"branch,omitempty"`
	Files    map[string]string `json:"files"` // filename -> content
}

// SemgrepServiceResponse represents the response from Semgrep service
type SemgrepServiceResponse struct {
	Status           string                      `json:"status"`
	FindingsMarkdown string                      `json:"findings_markdown"`
	Severity         intm.SemgrepSeveritySummary `json:"severity"`
	FindingsCount    int                         `json:"findings_count"`
	ScanDuration     string                      `json:"scan_duration"`
	Error            string                      `json:"error,omitempty"`
}

// CallSemgrep now collects files and sends them via JSON
func CallSemgrep(
	ctx context.Context,
	log zerolog.Logger,
	client *http.Client,
	semgrepURL string,
	meta intm.PRMetadata,
) (*intm.SemgrepOut, error) {

	if meta.LocalPath == "" {
		return nil, fmt.Errorf("CallSemgrep: meta.LocalPath empty")
	}

	log.Info().
		Str("path", meta.LocalPath).
		Str("semgrep_url", semgrepURL).
		Msg("collecting files for Semgrep scan")

	// Collect code files
	files, err := collectCodeFiles(meta.LocalPath, log)
	if err != nil {
		log.Error().Err(err).Msg("failed to collect code files")
		return generateHeuristicOutput(), nil // Fallback on error
	}

	if len(files) == 0 {
		log.Warn().Msg("no code files found")
		return generateHeuristicOutput(), nil
	}

	log.Info().
		Int("file_count", len(files)).
		Msg("files collected, sending to Semgrep service")

	// Build request
	reqPayload := SemgrepServiceRequest{
		RepoPath: meta.LocalPath,
		RepoURL:  fmt.Sprintf("https://github.com/%s/%s", meta.RepoOwner, meta.RepoName),
		Branch:   meta.SourceBranch,
		Files:    files,
	}

	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(reqPayload); err != nil {
		log.Error().Err(err).Msg("failed to encode semgrep request")
		return generateHeuristicOutput(), nil // Fallback on error
	}

	// Create HTTP request with timeout
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, semgrepURL+"/scan", &buf)
	if err != nil {
		log.Error().Err(err).Msg("failed to create semgrep request")
		return generateHeuristicOutput(), nil
	}

	req.Header.Set("Content-Type", "application/json")

	// Call Semgrep service
	startTime := time.Now()
	resp, err := client.Do(req)
	if err != nil {
		log.Error().Err(err).Msg("semgrep service http error")
		return generateHeuristicOutput(), nil // Fallback on error
	}
	defer resp.Body.Close()

	duration := time.Since(startTime)

	// Handle non-2xx responses
	if resp.StatusCode >= 300 {
		log.Warn().
			Int("status", resp.StatusCode).
			Msg("semgrep service returned error status")
		return generateHeuristicOutput(), nil
	}

	// Parse response
	var semgrepResp SemgrepServiceResponse
	if err := json.NewDecoder(resp.Body).Decode(&semgrepResp); err != nil {
		log.Error().Err(err).Msg("failed to decode semgrep response")
		return generateHeuristicOutput(), nil
	}

	// Check for service-level errors
	if semgrepResp.Error != "" {
		log.Warn().Str("error", semgrepResp.Error).Msg("semgrep service reported error")
		// Continue with results if available, otherwise fallback
		if semgrepResp.FindingsMarkdown == "" {
			return generateHeuristicOutput(), nil
		}
	}

	log.Info().
		Int("findings", semgrepResp.FindingsCount).
		Dur("duration", duration).
		Int("blocker", semgrepResp.Severity.Blocker).
		Int("critical", semgrepResp.Severity.Critical).
		Int("major", semgrepResp.Severity.Major).
		Int("minor", semgrepResp.Severity.Minor).
		Int("info", semgrepResp.Severity.Info).
		Msg("semgrep scan completed")

	return &intm.SemgrepOut{
		FindingsMarkdown: semgrepResp.FindingsMarkdown,
		Severity:         semgrepResp.Severity,
	}, nil
}

// collectCodeFiles walks the directory and collects code files with their content
func collectCodeFiles(repoPath string, log zerolog.Logger) (map[string]string, error) {
	files := make(map[string]string)

	// Supported extensions
	supportedExts := map[string]bool{
		".go":   true,
		".py":   true,
		".js":   true,
		".ts":   true,
		".jsx":  true,
		".tsx":  true,
		".java": true,
		".rb":   true,
		".php":  true,
		".cs":   true,
		".c":    true,
		".cpp":  true,
		".cc":   true,
		".h":    true,
		".hpp":  true,
	}

	err := filepath.WalkDir(repoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			name := d.Name()
			// Skip common non-code directories
			if name == ".git" || name == "node_modules" || name == "vendor" ||
				name == "__pycache__" || name == ".venv" || name == "venv" ||
				name == "dist" || name == "build" {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file extension is supported
		ext := strings.ToLower(filepath.Ext(path))
		if !supportedExts[ext] {
			return nil
		}

		// Read file content
		content, err := os.ReadFile(path)
		if err != nil {
			log.Warn().Err(err).Str("file", path).Msg("failed to read file")
			return nil // Skip this file but continue
		}

		// Get relative path from repo root
		relPath, err := filepath.Rel(repoPath, path)
		if err != nil {
			relPath = path // Use absolute path if relative fails
		}

		// Store with relative path as key
		files[relPath] = string(content)

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk directory: %w", err)
	}

	return files, nil
}

// generateHeuristicOutput returns generic security advice when Semgrep service fails
func generateHeuristicOutput() *intm.SemgrepOut {
	markdown := `### Security Analysis

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

	return &intm.SemgrepOut{
		FindingsMarkdown: markdown,
		Severity:         intm.SemgrepSeveritySummary{}, // all zeros
	}
}
