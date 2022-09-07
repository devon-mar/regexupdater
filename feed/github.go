package feed

import (
	"context"
	"errors"

	"github.com/devon-mar/regexupdater/utils/githubutil"
	"github.com/google/go-github/v45/github"
)

const (
	typeGitHub = "github"
)

type gitHubConfig struct {
	Owner              string `cfg:"owner" validate:"required"`
	Repo               string `cfg:"repo" validate:"required"`
	Tags               bool   `cfg:"tags"`
	IncludePrereleases bool   `cfg:"include_prereleases"`
}

type GitHub struct {
	githubutil.GitHubOptions `cfg:",squash"`
	PageSize                 int `cfg:"page_size" validate:"omitempty,gte=0"`
	Limit                    int `cfg:"limit" validate:"gte=0"`

	client *github.Client
}

func (g *GitHub) init() error {
	var err error

	if g.PageSize == 0 {
		// 100 is the max.
		g.PageSize = 100
	}

	if g.Limit == 0 {
		g.Limit = g.PageSize
	}

	g.client, _, _, err = githubutil.NewGitHub(&g.GitHubOptions)
	return err
}

// NewConfig implements Feed
func (*GitHub) NewConfig(c map[string]interface{}) (interface{}, error) {
	return newConfig(c, &gitHubConfig{})
}

// GetRelease implements Feed
func (g *GitHub) GetRelease(release string, config interface{}) (*Release, error) {
	cfg := config.(*gitHubConfig)
	if cfg.Tags {
		return g.getReleaseTags(release, cfg)
	}
	return g.getReleaseReleases(release, cfg)
}

func (g *GitHub) getReleaseReleases(release string, cfg *gitHubConfig) (*Release, error) {
	rel, _, err := g.client.Repositories.GetReleaseByTag(
		context.Background(),
		cfg.Owner,
		cfg.Repo,
		release,
	)
	if err != nil {
		return nil, err
	}
	if rel.GetPrerelease() && !cfg.IncludePrereleases {
		return nil, nil
	}
	return releaseFromGHRelease(rel), nil
}

func (g *GitHub) getReleaseTags(release string, cfg *gitHubConfig) (*Release, error) {
	ref, _, err := g.client.Git.GetRef(
		context.Background(),
		cfg.Owner,
		cfg.Repo,
		"tags/"+release,
	)
	if err != nil {
		return nil, err
	}

	if ref.Object == nil {
		return nil, errors.New("ref object was nil")
	}
	if ref.Object.SHA == nil {
		return nil, errors.New("ref object SHA was nil")
	}

	tag, _, err := g.client.Git.GetTag(
		context.Background(),
		cfg.Owner,
		cfg.Repo,
		*ref.Object.SHA,
	)
	if err != nil {
		return nil, err
	}

	return &Release{
		Version: tag.GetTag(),
		URL:     tag.GetURL(),
	}, nil
}

// GetReleases implements Feed
func (g *GitHub) GetReleases(config interface{}, done chan struct{}) (chan *Release, chan error) {
	relChan := make(chan *Release)
	errChan := make(chan error)

	cfg := config.(*gitHubConfig)

	go func() {
		defer close(errChan)
		defer close(relChan)
		if cfg.Tags {
			g.getReleasesTags(cfg, relChan, errChan, done)
		} else {
			g.getReleasesReleases(cfg, relChan, errChan, done)
		}
	}()

	return limit(relChan, errChan, g.Limit)
}

func (g *GitHub) getReleasesReleases(cfg *gitHubConfig, relChan chan *Release, errChan chan error, done chan struct{}) {
	listOpts := &github.ListOptions{
		PerPage: g.PageSize,
	}
	for {
		releases, resp, err := g.client.Repositories.ListReleases(
			context.Background(),
			cfg.Owner,
			cfg.Repo,
			listOpts,
		)
		if err != nil {
			errChan <- err
			return
		}

		for _, r := range releases {
			if r.GetPrerelease() && !cfg.IncludePrereleases {
				continue
			}
			select {
			case relChan <- releaseFromGHRelease(r):
			case <-done:
				return
			}
		}

		if resp.NextPage == 0 {
			break
		}
		listOpts.Page = resp.NextPage
	}
}

func (g *GitHub) getReleasesTags(cfg *gitHubConfig, relChan chan *Release, errChan chan error, done chan struct{}) {
	listOpts := &github.ListOptions{
		PerPage: g.PageSize,
	}
	for {
		tags, resp, err := g.client.Repositories.ListTags(
			context.Background(),
			cfg.Owner,
			cfg.Repo,
			listOpts,
		)
		if err != nil {
			errChan <- err
			return
		}

		for _, t := range tags {
			var url string
			if t.Commit != nil && t.Commit.HTMLURL != nil {
				url = *t.Commit.HTMLURL
			}
			rel := &Release{
				Version: t.GetName(),
				URL:     url,
			}
			select {
			case relChan <- rel:
			case <-done:
				return
			}
		}

		if resp.NextPage == 0 {
			break
		}
		listOpts.Page = resp.NextPage
	}
}

func releaseFromGHRelease(r *github.RepositoryRelease) *Release {
	return &Release{
		Version:      r.GetTagName(),
		ReleaseNotes: r.GetBody(),
		URL:          r.GetHTMLURL(),
	}
}
