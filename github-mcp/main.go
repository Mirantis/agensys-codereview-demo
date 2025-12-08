package main

import (
	"context"
	"encoding/json"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/google/go-github/v62/github"
	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
)

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

// Simple format (for backward compatibility)
type CommentReqSimple struct {
	PR   int    `json:"pr"`
	Body string `json:"body"`
}

// Orchestrator format - FIXED VERSION
type CommentReqOrchestrator struct {
	Action     string     `json:"action"`
	PR         PRMetadata `json:"pr"` // Now accepts full metadata
	Body       string     `json:"body"`
	BodyFormat string     `json:"body_format"` // Optional
}

func mustEnv(key string) string {
	val := os.Getenv(key)
	if val == "" {
		log.Fatalf("❌ Missing required env var: %s", key)
	}
	return val
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "...(truncated)"
}

func main() {
	_ = godotenv.Load()

	debug := os.Getenv("DEBUG") == "true"
	token := mustEnv("GITHUB_TOKEN")

	// These can now be empty - we extract from webhook
	defaultOwner := os.Getenv("REPO_OWNER")
	defaultRepo := os.Getenv("REPO_NAME")

	port := os.Getenv("PORT")
	if port == "" {
		port = "80"
	}

	// GitHub client
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	gh := github.NewClient(tc)

	app := fiber.New(fiber.Config{
		AppName: "github-mcp-server",
	})

	if debug {
		app.Use(func(c *fiber.Ctx) error {
			log.Printf("[DEBUG] Incoming request: %s %s", c.Method(), c.Path())
			return c.Next()
		})
	}

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "healthy"})
	})

	app.Post("/comment", func(c *fiber.Ctx) error {

		raw := c.Body()
		log.Printf("\n====== [MCP] RAW incoming JSON ======\n%s\n", string(raw))

		// Try ORCHESTRATOR model first (most common)
		var o CommentReqOrchestrator
		if err := json.Unmarshal(raw, &o); err == nil && o.PR.PRNumber != 0 {
			log.Println("[MCP] ✅ Parsed as ORCHESTRATOR format")

			owner := o.PR.RepoOwner
			repo := o.PR.RepoName
			pr := o.PR.PRNumber
			body := o.Body

			// Fallback to defaults if not provided
			if owner == "" {
				owner = defaultOwner
			}
			if repo == "" {
				repo = defaultRepo
			}

			if owner == "" || repo == "" {
				log.Println("[ERROR] Missing repo_owner or repo_name")
				return fiber.NewError(fiber.StatusBadRequest, "missing repo_owner or repo_name")
			}

			return postComment(ctx, gh, owner, repo, pr, body, c)
		}

		// Try SIMPLE model (backward compatibility)
		var s CommentReqSimple
		if err := json.Unmarshal(raw, &s); err == nil && s.PR != 0 {
			log.Println("[MCP] ✅ Parsed as SIMPLE format")

			if defaultOwner == "" || defaultRepo == "" {
				log.Println("[ERROR] SIMPLE format requires REPO_OWNER and REPO_NAME env vars")
				return fiber.NewError(fiber.StatusBadRequest, "missing REPO_OWNER or REPO_NAME")
			}

			return postComment(ctx, gh, defaultOwner, defaultRepo, s.PR, s.Body, c)
		}

		log.Println("[ERROR] ❌ Could not parse body as CommentReqSimple or CommentReqOrchestrator")
		return fiber.NewError(fiber.StatusBadRequest, "invalid JSON format")
	})

	log.Printf("🚀 github-mcp-server running on :%s", port)
	if defaultOwner != "" && defaultRepo != "" {
		log.Printf("📦 Default repo: %s/%s", defaultOwner, defaultRepo)
	}
	log.Printf("🔍 Debug mode: %v", debug)

	if err := app.Listen(":" + port); err != nil {
		log.Fatal(err)
	}
}

func postComment(ctx context.Context, gh *github.Client, owner, repo string, pr int, body string, c *fiber.Ctx) error {
	log.Printf("[MCP] → Posting to GitHub PR #%d in %s/%s", pr, owner, repo)
	log.Printf("[MCP] Body preview:\n%s\n", truncate(body, 400))

	comment := &github.IssueComment{Body: github.String(body)}

	created, resp, err := gh.Issues.CreateComment(ctx, owner, repo, pr, comment)

	if err != nil {
		if resp != nil {
			log.Printf("[ERROR] GitHub Status: %d", resp.StatusCode)
		}
		log.Printf("[ERROR] GitHub Error: %v", err)
		return fiber.NewError(fiber.StatusBadGateway, err.Error())
	}

	log.Printf("[MCP] ✅ GitHub OK: CommentID=%d URL=%s", created.GetID(), created.GetHTMLURL())

	return c.JSON(fiber.Map{
		"success":    true,
		"comment_id": created.GetID(),
		"url":        created.GetHTMLURL(),
	})
}
