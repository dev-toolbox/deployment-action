package githubapi

import (
	"context"
	"encoding/base64"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/dev-toolbox/deployment-action/internal/syncer"
	"github.com/dev-toolbox/deployment-action/internal/versioning"
	"github.com/google/go-github/v67/github"
	"golang.org/x/oauth2"
)

type RepositoryClient interface {
	EnsureRepository(ctx context.Context, owner string, repo string) error
	ListTagsForCommit(ctx context.Context, owner string, repo string, sha string) ([]versioning.Tag, error)
	ListFiles(ctx context.Context, owner string, repo string, branch string, prefix string) ([]syncer.File, error)
	ReadFile(ctx context.Context, owner string, repo string, branch string, filePath string) ([]byte, string, bool, error)
	CommitChanges(ctx context.Context, owner string, repo string, branch string, message string, changes []Change) error
}

type Change struct {
	Path    string
	Content []byte
	Delete  bool
}

type Client struct {
	api *github.Client
}

func NewClient(ctx context.Context, token string) *Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	httpClient := oauth2.NewClient(ctx, ts)

	return &Client{api: github.NewClient(httpClient)}
}

func (c *Client) EnsureRepository(ctx context.Context, owner string, repo string) error {
	_, resp, err := c.api.Repositories.Get(ctx, owner, repo)
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			return fmt.Errorf("deployment repository %s/%s does not exist", owner, repo)
		}
		return err
	}

	return nil
}

func (c *Client) ListTagsForCommit(ctx context.Context, owner string, repo string, sha string) ([]versioning.Tag, error) {
	options := &github.ListOptions{PerPage: 100}
	tags := make([]versioning.Tag, 0)

	for {
		result, resp, err := c.api.Repositories.ListTags(ctx, owner, repo, options)
		if err != nil {
			return nil, err
		}

		for _, tag := range result {
			if tag.GetCommit().GetSHA() == sha {
				tags = append(tags, versioning.Tag{Name: tag.GetName(), CommitSHA: sha})
			}
		}

		if resp.NextPage == 0 {
			break
		}
		options.Page = resp.NextPage
	}

	return tags, nil
}

func (c *Client) ListFiles(ctx context.Context, owner string, repo string, branch string, prefix string) ([]syncer.File, error) {
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix == "" {
		return nil, nil
	}

	_, contents, resp, err := c.api.Repositories.GetContents(ctx, owner, repo, prefix, &github.RepositoryContentGetOptions{Ref: branch})
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			return nil, nil
		}
		return nil, err
	}

	files := make([]syncer.File, 0)
	for _, item := range contents {
		childFiles, err := c.walkContent(ctx, owner, repo, branch, item)
		if err != nil {
			return nil, err
		}
		files = append(files, childFiles...)
	}

	return files, nil
}

func (c *Client) walkContent(ctx context.Context, owner string, repo string, branch string, item *github.RepositoryContent) ([]syncer.File, error) {
	if item == nil {
		return nil, nil
	}

	if item.GetType() == "file" {
		content, _, _, err := c.api.Repositories.GetContents(ctx, owner, repo, item.GetPath(), &github.RepositoryContentGetOptions{Ref: branch})
		if err != nil {
			return nil, err
		}

		decoded, err := content.GetContent()
		if err != nil {
			return nil, err
		}

		return []syncer.File{{Path: item.GetPath(), Content: []byte(decoded), SHA: item.GetSHA()}}, nil
	}

	if item.GetType() != "dir" {
		return nil, nil
	}

	_, contents, _, err := c.api.Repositories.GetContents(ctx, owner, repo, item.GetPath(), &github.RepositoryContentGetOptions{Ref: branch})
	if err != nil {
		return nil, err
	}

	files := make([]syncer.File, 0)
	for _, child := range contents {
		childFiles, err := c.walkContent(ctx, owner, repo, branch, child)
		if err != nil {
			return nil, err
		}
		files = append(files, childFiles...)
	}

	return files, nil
}

func (c *Client) ReadFile(ctx context.Context, owner string, repo string, branch string, filePath string) ([]byte, string, bool, error) {
	content, _, resp, err := c.api.Repositories.GetContents(ctx, owner, repo, filePath, &github.RepositoryContentGetOptions{Ref: branch})
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			return nil, "", false, nil
		}
		return nil, "", false, err
	}

	decoded, err := content.GetContent()
	if err != nil {
		return nil, "", false, err
	}

	return []byte(decoded), content.GetSHA(), true, nil
}

func (c *Client) CommitChanges(ctx context.Context, owner string, repo string, branch string, message string, changes []Change) error {
	if len(changes) == 0 {
		return nil
	}

	ref, _, err := c.api.Git.GetRef(ctx, owner, repo, "refs/heads/"+branch)
	if err != nil {
		return fmt.Errorf("get ref for branch %s: %w", branch, err)
	}

	parentCommit, _, err := c.api.Git.GetCommit(ctx, owner, repo, ref.GetObject().GetSHA())
	if err != nil {
		return fmt.Errorf("get parent commit for branch %s: %w", branch, err)
	}

	entries := make([]*github.TreeEntry, 0, len(changes))
	sortedChanges := append([]Change(nil), changes...)
	sort.Slice(sortedChanges, func(i int, j int) bool {
		return sortedChanges[i].Path < sortedChanges[j].Path
	})

	for _, change := range sortedChanges {
		pathValue := change.Path
		mode := "100644"
		entryType := "blob"
		entry := &github.TreeEntry{
			Path: github.String(pathValue),
			Mode: github.String(mode),
			Type: github.String(entryType),
		}

		if change.Delete {
			entries = append(entries, entry)
			continue
		}

		blob, _, err := c.api.Git.CreateBlob(ctx, owner, repo, &github.Blob{
			Content:  github.String(base64.StdEncoding.EncodeToString(change.Content)),
			Encoding: github.String("base64"),
		})
		if err != nil {
			return fmt.Errorf("create blob for %s: %w", change.Path, err)
		}

		entry.SHA = blob.SHA
		entries = append(entries, entry)
	}

	tree, _, err := c.api.Git.CreateTree(ctx, owner, repo, parentCommit.GetTree().GetSHA(), entries)
	if err != nil {
		return fmt.Errorf("create tree: %w", err)
	}

	commit, _, err := c.api.Git.CreateCommit(ctx, owner, repo, &github.Commit{
		Message: github.String(message),
		Tree:    tree,
		Parents: []*github.Commit{{SHA: parentCommit.SHA}},
	}, nil)
	if err != nil {
		return fmt.Errorf("create commit: %w", err)
	}

	_, _, err = c.api.Git.UpdateRef(ctx, owner, repo, &github.Reference{
		Ref: ref.Ref,
		Object: &github.GitObject{
			SHA:  commit.SHA,
			Type: github.String("commit"),
		},
	}, false)
	if err != nil {
		return fmt.Errorf("update branch ref: %w", err)
	}

	return nil
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}

	return strings.Contains(strings.ToLower(err.Error()), "404") || strings.Contains(strings.ToLower(err.Error()), "not found")
}

func DecodeContent(encoded string) ([]byte, error) {
	clean := strings.ReplaceAll(encoded, "\n", "")
	return base64.StdEncoding.DecodeString(clean)
}

func Join(parts ...string) string {
	joined := path.Join(parts...)
	return strings.TrimPrefix(joined, "/")
}
