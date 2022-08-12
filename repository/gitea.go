package repository

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"

	"code.gitea.io/sdk/gitea"
	"github.com/devon-mar/regexupdater/utils/giteautil"
	"golang.org/x/exp/slices"
)

const (
	typeGitea = "gitea"

	giteaContentTypeFile = "file"
)

type Gitea struct {
	URL string `cfg:"url" validate:"required"`
	// Basic Auth
	Username string `cfg:"username"`
	Password string `cfg:"password" validate:"required_with=Username"`
	// Token Auth
	Token string `cfg:"token" validate:"required_without=Username"`

	CommitterName  string `cfg:"committer_name"`
	CommitterEmail string `cfg:"committer_email" validate:"required_with=CommitterName"`

	Labels []string `cfg:"labels"`

	Owner      string `cfg:"owner" validate:"required"`
	Repo       string `cfg:"repo" validate:"required"`
	BaseBranch string `cfg:"base_branch"`
	PageSize   int    `cfg:"page_size"`

	client     *gitea.Client
	myUsername string
	committer  gitea.Identity
	labelIDs   []int64
}

func (g *Gitea) init() error {
	var err error

	opts := []gitea.ClientOption{}
	if g.Token != "" {
		opts = append(opts, gitea.SetToken(g.Token))
	} else if g.Username != "" && g.Password != "" {
		opts = append(opts, gitea.SetBasicAuth(g.Username, g.Password))
	}
	g.client, err = gitea.NewClient(g.URL, opts...)
	if err != nil {
		return fmt.Errorf("error initializing Gitea client")
	}

	if g.BaseBranch == "" {
		g.BaseBranch, err = g.getDefaultBranch()
		if err != nil {
			return fmt.Errorf("error getting default branch: %w", err)
		}
	}

	g.labelIDs, err = g.getLabelIDs(g.Labels)
	if err != nil {
		return fmt.Errorf("error getting label IDs: %w", err)
	}

	// It seems to be fine if they are both empty.
	g.committer.Name = g.CommitterName
	g.committer.Email = g.CommitterEmail

	if g.PageSize == 0 {
		// https://docs.gitea.io/en-us/config-cheat-sheet/
		// MAX_RESPONSE_ITEMS
		g.PageSize = 50
	}

	return err
}

func (g *Gitea) getLabelIDs(labelNames []string) ([]int64, error) {
	if len(labelNames) == 0 {
		return nil, nil
	}

	ret := make([]int64, 0, len(labelNames))
	found := make([]string, 0, len(labelNames))

	opts := gitea.ListLabelsOptions{ListOptions: gitea.ListOptions{PageSize: g.PageSize}}
	for {
		labels, resp, err := g.client.ListRepoLabels(g.Owner, g.Repo, opts)
		if err != nil {
			return nil, err
		}

		for _, l := range labels {
			if slices.Contains(labelNames, l.Name) {
				ret = append(ret, l.ID)
				found = append(found, l.Name)
			}
		}
		nextPage := giteautil.NextPage(resp.Header.Get("Link"))
		if nextPage == 0 {
			break
		}
		opts.Page = nextPage
	}

	if len(ret) != len(g.Labels) {
		return nil, fmt.Errorf("could not find all labels: found=%#v", found)
	}

	return ret, nil
}

func (g *Gitea) getDefaultBranch() (string, error) {
	repo, _, err := g.client.GetRepo(g.Owner, g.Repo)
	return repo.DefaultBranch, err
}

// FindPR implements Repository
func (g *Gitea) FindPR(s string) (PullRequest, error) {
	issues, _, err := g.client.ListRepoIssues(g.Owner, g.Repo, gitea.ListIssueOption{
		KeyWord:   `"` + s + `"`,
		State:     "all",
		Type:      gitea.IssueTypePull,
		CreatedBy: g.myUsername,
		Labels:    g.Labels,
	})
	if err != nil {
		return nil, err
	}

	if len(issues) == 0 {
		return nil, nil
	}

	pr, err := g.getPR(issues[0].Index)
	if err != nil {
		return nil, fmt.Errorf("error getting PR #%d", issues[0].Index)
	}

	return &GiteaPR{pr: pr}, nil
}

func (g *Gitea) getPR(index int64) (*gitea.PullRequest, error) {
	pr, _, err := g.client.GetPullRequest(g.Owner, g.Repo, index)
	return pr, err
}

// GetFile implements Repository
func (g *Gitea) GetFile(path string) (File, error) {
	content, _, err := g.client.GetContents(
		g.Owner,
		g.Repo,
		g.BaseBranch,
		path,
	)
	if err != nil {
		return nil, err
	}
	if content.Type != giteaContentTypeFile {
		return nil, fmt.Errorf("content was not file, was %s", content.Type)
	}
	if *content.Encoding != "base64" {
		return nil, fmt.Errorf("unsupported encoding %q", *content.Encoding)
	}
	if content.Content == nil {
		return nil, errors.New("content was nil")
	}

	return &GiteaFile{cr: content}, nil
}

// UpdateFilePR implements Repository
func (g *Gitea) UpdateFilePR(path string, oldSHA string, newContent []byte, commitMsg string, newBranch string, prTitle string, prBody string) (prID string, err error) {
	_, _, err = g.client.UpdateFile(
		g.Owner,
		g.Repo,
		path,
		gitea.UpdateFileOptions{
			FileOptions: gitea.FileOptions{
				Message:       commitMsg,
				BranchName:    g.BaseBranch,
				NewBranchName: newBranch,
				Author:        g.committer,
			},
			SHA:     oldSHA,
			Content: base64.StdEncoding.EncodeToString(newContent),
		},
	)
	if err != nil {
		return "", fmt.Errorf("error updating file: %w", err)
	}

	pr, _, err := g.client.CreatePullRequest(
		g.Owner,
		g.Repo,
		gitea.CreatePullRequestOption{
			Head:   newBranch,
			Base:   g.BaseBranch,
			Title:  prTitle,
			Body:   prBody,
			Labels: g.labelIDs,
		},
	)
	if err != nil {
		return "", fmt.Errorf("error creating PR: %w", err)
	}
	p := GiteaPR{pr: pr}
	return p.ID(), nil
}

// AddPRComment implements Repository
func (g *Gitea) AddPRComment(pr PullRequest, body string) error {
	_, _, err := g.client.CreateIssueComment(
		g.Owner,
		g.Repo,
		pr.(*GiteaPR).pr.Index,
		gitea.CreateIssueCommentOption{Body: body},
	)
	return err
}

// ClosePR implements Repository
func (g *Gitea) ClosePR(pr PullRequest) error {
	state := gitea.StateClosed
	gpr := pr.(*GiteaPR)
	_, _, err := g.client.EditPullRequest(
		g.Owner,
		g.Repo,
		gpr.pr.Index,
		gitea.EditPullRequestOption{State: &state},
	)
	if err != nil {
		gpr.wasClosed = true
	}
	return err
}

// RebasePR implements Repository
//
// Updates the file by creating a new PR based on this one since Gitea
// doesn't have an API for git references.
func (g *Gitea) RebasePR(pr PullRequest, path string, oldSHA string, newContent []byte, commitMsg string) error {
	gpr := pr.(*GiteaPR)

	file, err := g.GetFile(path)
	if err != nil {
		return fmt.Errorf("error getting file %s: %w", path, err)
	}

	// First delete this PR (and it's branch).
	if err := g.ClosePR(pr); err != nil {
		return fmt.Errorf("error closing PR %s: %w", pr.ID(), err)
	}

	// Create a new PR
	_, err = g.UpdateFilePR(
		path,
		file.SHA(),
		newContent,
		commitMsg,
		gpr.pr.Head.Ref,
		gpr.pr.Title,
		gpr.pr.Body,
	)
	return err
}

// DeletePRBranch implements Repository
func (g *Gitea) DeletePRBranch(prID string) (string, error) {
	index, err := strconv.ParseInt(prID, 10, 64)
	if err != nil || index < 1 {
		return "", fmt.Errorf("%s is not a valid PR index.", prID)
	}

	pr, _, err := g.client.GetPullRequest(g.Owner, g.Repo, index)
	if err != nil {
		return "", err
	}

	ok, _, err := g.client.DeleteRepoBranch(g.Owner, g.Repo, pr.Head.Name)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", errors.New("gitea did not return ok")
	}
	return pr.Head.Name, nil
}

type GiteaFile struct {
	cr *gitea.ContentsResponse
}

// SHA implements File
func (f *GiteaFile) SHA() string {
	return f.cr.SHA
}

// Content implements File
func (f *GiteaFile) Content() []byte {
	// In GetFile we check that content is not nil.
	b, _ := base64.StdEncoding.DecodeString(*f.cr.Content)
	return b
}

// Path implements File
func (f *GiteaFile) Path() string {
	return f.cr.Path
}

type GiteaPR struct {
	pr        *gitea.PullRequest
	wasClosed bool
}

// Body implements PullRequest
func (pr *GiteaPR) Body() string {
	return pr.pr.Body
}

// ID implements PullRequest
func (pr *GiteaPR) ID() string {
	return fmt.Sprintf("!%d", pr.pr.Index)
}

// IsMergeable implements PullRequest
func (pr *GiteaPR) IsMergeable() bool {
	return pr.pr.Mergeable
}

// IsOpen implements PullRequest
func (pr *GiteaPR) IsOpen() bool {
	if pr.wasClosed {
		return false
	}
	return pr.pr.State == gitea.StateOpen
}
