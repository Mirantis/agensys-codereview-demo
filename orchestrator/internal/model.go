package internal

type Config struct {
	ListenAddr         string
	LogLevel           string
	PRAgentURL         string
	SemgrepServiceURL  string // CHANGED: Renamed from SemgrepMCPURL
	SummarizerURL      string
	GitHubMCPURL       string
	HTTPTimeoutMinutes int
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
	URL          string `json:"url,omitempty"`
	LocalPath    string `json:"local_path"`
}

type PRAgentDescribeRequest struct {
	Mode string     `json:"mode"`
	PR   PRMetadata `json:"pr"`
}

type PRAgentDescribeOut struct {
	DescriptionMarkdown string `json:"description_markdown"`
}

type PRAgentReviewRequest struct {
	Mode                string     `json:"mode"`
	PR                  PRMetadata `json:"pr"`
	DescriptionMarkdown string     `json:"description_markdown"`
}

type PRAgentReviewOut struct {
	ReviewMarkdown string `json:"review_markdown"`
}

type SemgrepSeveritySummary struct {
	Blocker  int `json:"blocker"` // ADDED: Blocker severity level
	Critical int `json:"critical"`
	Major    int `json:"major"`
	Minor    int `json:"minor"`
	Info     int `json:"info"`
}

type SemgrepOut struct {
	FindingsMarkdown string                 `json:"findings_markdown"`
	Severity         SemgrepSeveritySummary `json:"severity"`
}

type GitHubCommentRequest struct {
	Action     string     `json:"action"`
	PR         PRMetadata `json:"pr"`
	Body       string     `json:"body"`
	BodyFormat string     `json:"body_format"`
}
