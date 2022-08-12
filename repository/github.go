package repository

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/devon-mar/regexupdater/utils/githubutil"
	"github.com/google/go-github/v45/github"
	"golang.org/x/exp/slices"
)

const (
	typeGitHub = "github"

	githubPRStateOpen   = "open"
	githubPRStateClosed = "closed"
)

type GitHub struct {
	githubutil.GitHubOptions `cfg:",squash"`

	Owner      string `cfg:"owner" validate:"required"`
	Repo       string `cfg:"repo" validate:"required"`
	BaseBranch string `cfg:"base_branch"`

	Labels []string `cfg:"labels"`
	labels []*github.Label

	client  *github.Client
	appSlug string

	author  *github.CommitAuthor
	headSHA string
}

// Gets the current username and login.
func (gh *GitHub) init() error {
	var err error

	gh.GitHubOptions.Owner = gh.Owner
	gh.GitHubOptions.Repo = gh.Repo
	appClient, installClient, appSlug, err := githubutil.NewGitHub(&gh.GitHubOptions)
	if err != nil {
		return fmt.Errorf("error initializing client: %v", err)
	}
	gh.appSlug = appSlug
	if installClient != nil {
		gh.client = installClient
	} else {
		gh.client = appClient
	}

	gh.labels, err = gh.getLabels()
	if err != nil {
		return fmt.Errorf("error getting labels: %v", err)
	}

	if gh.BaseBranch == "" {
		repo, _, err := gh.client.Repositories.Get(context.Background(), gh.Owner, gh.Repo)
		if err != nil {
			return fmt.Errorf("error getting repository %s/%s: %v", gh.Owner, gh.Repo, err)
		}
		gh.BaseBranch = repo.GetDefaultBranch()
	}
	return nil
}

func (gh *GitHub) getLabels() ([]*github.Label, error) {
	if len(gh.Labels) == 0 {
		return nil, nil
	}
	ret := make([]*github.Label, 0, len(gh.Labels))
	opts := &github.ListOptions{PerPage: 100}

	found := make([]string, 0, len(gh.Labels))
	for {
		labels, resp, err := gh.client.Issues.ListLabels(context.Background(), gh.Owner, gh.Repo, opts)
		if err != nil {
			return nil, err
		}
		for _, l := range labels {
			if slices.Contains(gh.Labels, l.GetName()) {
				ret = append(ret, l)
				found = append(found, l.GetName())
			}
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	if len(ret) != len(gh.Labels) {
		return nil, fmt.Errorf("could not find all labels: found %#v", found)
	}

	return ret, nil
}

func (gh *GitHub) getAuthor() string {
	if gh.appSlug != "" {
		return "app/" + gh.appSlug
	}
	return "@me"
}

// FindPR implements Repository
func (gh *GitHub) FindPR(s string) (PullRequest, error) {
	query := fmt.Sprintf(`"%s" in:body is:pr repo:%s/%s author:%s`, s, gh.Owner, gh.Repo, gh.getAuthor())
	for _, l := range gh.labels {
		query += fmt.Sprintf(` label:"%s"`, l.GetName())
	}
	prs, _, err := gh.client.Search.Issues(
		context.Background(),
		query,
		&github.SearchOptions{
			Sort:  "created",
			Order: "desc",
		},
	)
	if err != nil {
		return nil, err
	}
	if len(prs.Issues) == 0 {
		return nil, nil
	}

	prNumber := prs.Issues[0].GetNumber()
	if prNumber == 0 {
		return nil, errors.New("PR number is 0")
	}
	pr, _, err := gh.client.PullRequests.Get(
		context.Background(),
		gh.Owner,
		gh.Repo,
		prNumber,
	)

	return &GitHubPR{pr: pr}, err
}

// GetFile implements Repository
func (gh *GitHub) GetFile(path string) (File, error) {
	file, _, _, err := gh.client.Repositories.GetContents(
		context.Background(),
		gh.Owner,
		gh.Repo,
		path,
		&github.RepositoryContentGetOptions{},
	)
	if err != nil {
		return nil, err
	}
	return &GitHubFile{rc: file}, nil
}

func (gh *GitHub) getHeadSHA() (string, error) {
	if gh.headSHA != "" {
		return gh.headSHA, nil
	}

	ref, _, err := gh.client.Git.GetRef(
		context.Background(),
		gh.Owner,
		gh.Repo,
		"heads/"+gh.BaseBranch,
	)
	if err != nil {
		return "", err
	}
	if ref.Object == nil {
		return "", errors.New("ref object was nil")
	} else if ref.Object.SHA == nil {
		return "", errors.New("ref object SHA was nil")
	}
	return *ref.Object.SHA, nil
}

func (gh *GitHub) createBranch(name string) error {
	headSHA, err := gh.getHeadSHA()
	if err != nil {
		return err
	}

	newRef := "refs/heads/" + name
	_, _, err = gh.client.Git.CreateRef(
		context.Background(),
		gh.Owner,
		gh.Repo,
		&github.Reference{Ref: &newRef, Object: &github.GitObject{SHA: &headSHA}},
	)
	return err
}

func (gh *GitHub) editRef(ref string, sha string) error {
	_, _, err := gh.client.Git.UpdateRef(
		context.Background(),
		gh.Owner,
		gh.Repo,
		&github.Reference{Ref: &ref, Object: &github.GitObject{SHA: &sha}},
		true,
	)
	return err
}

func (gh *GitHub) updateFile(path string, opts github.RepositoryContentFileOptions) (*github.RepositoryContentResponse, error) {
	opts.Author = gh.author
	r, _, err := gh.client.Repositories.UpdateFile(
		context.Background(),
		gh.Owner,
		gh.Repo,
		path,
		&opts,
	)
	return r, err
}

// UpdateFilePR implements Repository
func (gh *GitHub) UpdateFilePR(path string, oldSHA string, newContent []byte, commitMsg string, newBranch string, prTitle string, prBody string) (prID string, err error) {
	if err := gh.createBranch(newBranch); err != nil {
		return "", fmt.Errorf("error creating new branch: %w", err)
	}
	opts := github.RepositoryContentFileOptions{
		Message: &commitMsg,
		Content: newContent,
		SHA:     &oldSHA,
		Branch:  &newBranch,
	}
	if _, err := gh.updateFile(path, opts); err != nil {
		return "", fmt.Errorf("error updating file: %w", err)
	}

	resp, _, err := gh.client.PullRequests.Create(
		context.Background(),
		gh.Owner,
		gh.Repo,
		&github.NewPullRequest{
			Title: &prTitle,
			Body:  &prBody,
			Base:  &gh.BaseBranch,
			Head:  &newBranch,
		},
	)
	if err != nil {
		return "", fmt.Errorf("error creating PR: %w", err)
	}
	if resp.Number == nil {
		return "", fmt.Errorf("PR number is nil")
	}

	if len(gh.labels) > 0 {
		resp, _, err = gh.client.PullRequests.Edit(
			context.Background(),
			gh.Owner,
			gh.Repo,
			resp.GetNumber(),
			&github.PullRequest{Labels: gh.labels},
		)
		if err != nil {
			return "", fmt.Errorf("error adding labels to PR #%d: %v", resp.GetNumber(), err)
		}
	}
	pr := &GitHubPR{pr: resp}
	return pr.ID(), nil
}

// AddPRComment implements Repository
func (gh *GitHub) AddPRComment(pr PullRequest, body string) error {
	_, _, err := gh.client.Issues.CreateComment(
		context.Background(),
		gh.Owner,
		gh.Repo,
		pr.(*GitHubPR).pr.GetNumber(),
		&github.IssueComment{
			Body: &body,
		},
	)
	return err
}

// ClosePR implements Repository
func (gh *GitHub) ClosePR(pr PullRequest) error {
	state := githubPRStateClosed
	gpr := pr.(*GitHubPR)
	_, _, err := gh.client.PullRequests.Edit(
		context.Background(),
		gh.Owner,
		gh.Repo,
		gpr.pr.GetNumber(),
		&github.PullRequest{State: &state},
	)
	if err != nil {
		gpr.wasClosed = true
	}
	return err
}

func (gh *GitHub) deleteBranch(name string) error {
	_, err := gh.client.Git.DeleteRef(
		context.Background(),
		gh.Owner,
		gh.Repo,
		"refs/heads/"+name,
	)
	return err
}

// RebasePR implements Repository
func (gh *GitHub) RebasePR(pr PullRequest, path string, oldSHA string, newContent []byte, commitMsg string) error {
	gpr := pr.(*GitHubPR)
	if gpr.pr.Head == nil {
		return errors.New("PR head was nil")
	} else if gpr.pr.Head.Ref == nil {
		return errors.New("PR head ref was nil")
	}
	branch := *gpr.pr.Head.Ref + "-temp"
	if err := gh.createBranch(branch); err != nil {
		return fmt.Errorf("error creating temp branch: %w", err)
	}

	resp, err := gh.updateFile(path, github.RepositoryContentFileOptions{
		Message: &commitMsg,
		Content: newContent,
		SHA:     &oldSHA,
		Branch:  &branch,
	})
	if err != nil {
		return err
	}
	if resp.SHA == nil {
		return errors.New("new commit SHA was nil")
	}

	if err = gh.editRef("heads/"+*gpr.pr.Head.Ref, *resp.SHA); err != nil {
		return fmt.Errorf("error editing ref: %w", err)
	}

	return gh.deleteBranch(branch)
}

// DeletePRBranch implements Repository
func (gh *GitHub) DeletePRBranch(prID string) (string, error) {
	prNumber, err := strconv.Atoi(prID)
	if err != nil || prNumber < 1 {
		return "", fmt.Errorf("%s is not a valid PR number.", prID)
	}

	pr, _, err := gh.client.PullRequests.Get(
		context.Background(),
		gh.Owner,
		gh.Repo,
		prNumber,
	)
	if err != nil {
		return "", err
	}
	if pr.Head == nil || pr.Head.Ref == nil {
		return "", errors.New("PR head ref is nil")
	}
	return *pr.Head.Ref, gh.deleteBranch(*pr.Head.Ref)
}

type GitHubFile struct {
	rc *github.RepositoryContent
}

// SHA implements File
func (f *GitHubFile) SHA() string {
	return f.rc.GetSHA()
}

// Content implements File
func (f *GitHubFile) Content() []byte {
	c, _ := f.rc.GetContent()
	return []byte(c)
}

// Path implements File
func (f *GitHubFile) Path() string {
	return f.rc.GetPath()
}

type GitHubPR struct {
	pr        *github.PullRequest
	wasClosed bool
}

// Body implements PullRequest
func (pr *GitHubPR) Body() string {
	return pr.pr.GetBody()
}

// ID implements PullRequest
func (pr *GitHubPR) ID() string {
	return fmt.Sprintf("#%d", pr.pr.GetNumber())
}

// IsMergeable implements PullRequest
func (pr *GitHubPR) IsMergeable() bool {
	return pr.pr.GetMergeable()
}

// IsOpen implements PullRequest
func (pr *GitHubPR) IsOpen() bool {
	if pr.wasClosed {
		return false
	}
	return pr.pr.GetState() == githubPRStateOpen
}
