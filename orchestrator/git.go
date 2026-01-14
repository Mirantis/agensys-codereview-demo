package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	intm "orchestrator/internal"

	"github.com/rs/zerolog"
)

func PrepareRepository(
	ctx context.Context,
	log zerolog.Logger,
	meta *intm.PRMetadata,
) (string, error) {

	if meta == nil {
		return "", fmt.Errorf("nil PRMetadata passed to PrepareRepository")
	}

	baseName := fmt.Sprintf("%s-%s-pr%d",
		meta.RepoOwner,
		meta.RepoName,
		meta.PRNumber,
	)
	dest := filepath.Join(os.TempDir(), baseName)

	_ = os.RemoveAll(dest)

	// Use head repo (fork) for cloning - this is where the source branch exists
	cloneURL := fmt.Sprintf(
		"https://github.com/%s/%s.git",
		meta.HeadRepoOwner,
		meta.HeadRepoName,
	)

	log.Debug().
		Str("clone_url", cloneURL).
		Str("dest", dest).
		Msg("cloning repository")

	cmd := exec.CommandContext(
		ctx,
		"git", "clone",
		"--depth=1",
		"--branch", meta.SourceBranch,
		cloneURL,
		dest,
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Error().
			Err(err).
			Str("output", string(out)).
			Msg("git clone failed")
		return "", fmt.Errorf("git clone failed: %w", err)
	}

	log.Info().Str("path", dest).Msg("repository cloned")

	meta.LocalPath = dest
	return dest, nil
}
