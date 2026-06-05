// file: cmd/burndown/decompose_cmd.go
// version: 1.0.0

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/falkcorp/overnight-burndown/internal/config"
	"github.com/falkcorp/overnight-burndown/internal/decompose"
)

// cmdDecompose reads [failed-batch-hard] items from the target repo's
// TODO.md, calls Claude Haiku to split each into 3-5 subtasks, and writes
// them back in-place. The caller (CI workflow) is responsible for committing
// the result and opening a PR.
func cmdDecompose(args []string) int {
	fs := flag.NewFlagSet("decompose", flag.ExitOnError)
	fromEnv := fs.Bool("from-env", false, "build config from environment variables")
	repoPath := fs.String("repo-path", "", "path to the target repo checkout (required)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	if *repoPath == "" && *fromEnv {
		// Fall back to config-derived path when running in CI.
		cfg, err := config.FromEnv()
		if err != nil {
			fmt.Fprintln(os.Stderr, "burndown decompose:", err)
			return 1
		}
		if len(cfg.Repos) > 0 {
			*repoPath = cfg.Repos[0].LocalPath
		}
	}
	if *repoPath == "" {
		fmt.Fprintln(os.Stderr, "burndown decompose: --repo-path is required (or use --from-env)")
		return 2
	}

	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("BURNDOWN_BOT_CLAUDE_API_KEY")
	}
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "burndown decompose: ANTHROPIC_API_KEY or BURNDOWN_BOT_CLAUDE_API_KEY required")
		return 1
	}

	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	results, err := decompose.Run(context.Background(), client, *repoPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "burndown decompose:", err)
		return 1
	}

	if len(results) == 0 {
		fmt.Fprintln(os.Stderr, "burndown decompose: no [failed-batch-hard] items found")
		return 0
	}

	var failures []string
	for _, r := range results {
		if r.Err != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", r.ParentTitle, r.Err))
			continue
		}
		fmt.Fprintf(os.Stderr, "burndown decompose: %q → %d subtasks\n",
			r.ParentTitle, len(r.Subtasks))
		for _, st := range r.Subtasks {
			fmt.Fprintf(os.Stderr, "  - %s: %s (%s)\n", st.ID, st.Title, st.Size)
		}
	}
	if len(failures) > 0 {
		fmt.Fprintln(os.Stderr, "burndown decompose: errors:")
		fmt.Fprintln(os.Stderr, "  "+strings.Join(failures, "\n  "))
		return 1
	}
	return 0
}
