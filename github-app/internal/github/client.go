package github

import (
	"context"
	"fmt"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v57/github"
)

// Client wraps a GitHub API client
type Client struct {
	*github.Client
	installationID int64
}

// NewClient creates a new GitHub client using GitHub App authentication
func NewClient(appID, installationID int64, privateKeyPath string) (*Client, error) {
	itr, err := ghinstallation.NewKeyFromFile(http.DefaultTransport, appID, installationID, privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub App transport: %w", err)
	}

	client := github.NewClient(&http.Client{Transport: itr})

	return &Client{
		Client:         client,
		installationID: installationID,
	}, nil
}

// PRRequest represents a request to create a pull request
type PRRequest struct {
	Owner         string
	Repo          string
	Title         string
	Body          string
	BaseBranch    string
	HeadBranch    string
	FilePath      string
	FileContent   string
	CommitMessage string
}

// CreatePR creates a pull request with a single file change
func (c *Client) CreatePR(ctx context.Context, req *PRRequest) (int, string, error) {
	// Step 1: Get base branch reference
	baseRef, _, err := c.Git.GetRef(ctx, req.Owner, req.Repo, fmt.Sprintf("refs/heads/%s", req.BaseBranch))
	if err != nil {
		return 0, "", fmt.Errorf("failed to get base branch: %w", err)
	}

	// Step 2: Create new branch
	newRef := &github.Reference{
		Ref: github.String(fmt.Sprintf("refs/heads/%s", req.HeadBranch)),
		Object: &github.GitObject{
			SHA: baseRef.Object.SHA,
		},
	}

	_, _, err = c.Git.CreateRef(ctx, req.Owner, req.Repo, newRef)
	if err != nil {
		// Branch might already exist - try to update it instead
		_, _, err = c.Git.UpdateRef(ctx, req.Owner, req.Repo, newRef, false)
		if err != nil {
			return 0, "", fmt.Errorf("failed to create/update branch: %w", err)
		}
	}

	// Step 3: Get current file (if exists)
	var currentSHA *string
	fileContent, _, resp, err := c.Repositories.GetContents(ctx, req.Owner, req.Repo, req.FilePath, &github.RepositoryContentGetOptions{
		Ref: req.HeadBranch,
	})
	if err != nil && resp.StatusCode != 404 {
		return 0, "", fmt.Errorf("failed to get current file: %w", err)
	}
	if fileContent != nil {
		currentSHA = fileContent.SHA
	}

	// Step 4: Create or update file
	opts := &github.RepositoryContentFileOptions{
		Message: github.String(req.CommitMessage),
		Content: []byte(req.FileContent),
		Branch:  github.String(req.HeadBranch),
		SHA:     currentSHA,
	}

	_, _, err = c.Repositories.CreateFile(ctx, req.Owner, req.Repo, req.FilePath, opts)
	if err != nil {
		// If file exists, try update
		_, _, err = c.Repositories.UpdateFile(ctx, req.Owner, req.Repo, req.FilePath, opts)
		if err != nil {
			return 0, "", fmt.Errorf("failed to create/update file: %w", err)
		}
	}

	// Step 5: Create pull request
	pr, _, err := c.PullRequests.Create(ctx, req.Owner, req.Repo, &github.NewPullRequest{
		Title: github.String(req.Title),
		Body:  github.String(req.Body),
		Head:  github.String(req.HeadBranch),
		Base:  github.String(req.BaseBranch),
	})
	if err != nil {
		return 0, "", fmt.Errorf("failed to create pull request: %w", err)
	}

	return pr.GetNumber(), pr.GetHTMLURL(), nil
}
