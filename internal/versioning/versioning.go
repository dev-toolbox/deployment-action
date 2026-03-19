package versioning

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

var versionTagPattern = regexp.MustCompile(`^v\d+\.\d+\.\d+$`)

type Stage string

const (
	StageDev  Stage = "dev"
	StageProd Stage = "prod"
)

type Tag struct {
	Name      string
	CommitSHA string
}

type TagService interface {
	ListTagsForCommit(ctx context.Context, owner string, repo string, sha string) ([]Tag, error)
}

type Result struct {
	Stage   Stage
	Version string
	TagName string
}

func Resolve(ctx context.Context, service TagService, owner string, repo string, branch string, sha string) (Result, error) {
	switch branch {
	case "develop":
		return Result{Stage: StageDev, Version: shortSHA(sha)}, nil
	case "main":
		tags, err := service.ListTagsForCommit(ctx, owner, repo, sha)
		if err != nil {
			return Result{}, fmt.Errorf("list tags for commit: %w", err)
		}

		matching := make([]string, 0, len(tags))
		for _, tag := range tags {
			if versionTagPattern.MatchString(tag.Name) {
				matching = append(matching, tag.Name)
			}
		}

		if len(matching) == 0 {
			return Result{Stage: StageDev, Version: shortSHA(sha)}, nil
		}

		sort.Strings(matching)
		selected := matching[len(matching)-1]
		return Result{Stage: StageProd, Version: strings.TrimPrefix(selected, "v"), TagName: selected}, nil
	default:
		return Result{}, fmt.Errorf("unsupported branch %q: only main and develop are supported", branch)
	}
}

func shortSHA(sha string) string {
	if len(sha) <= 6 {
		return sha
	}

	return sha[:6]
}
