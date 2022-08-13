package feed

import (
	"code.gitea.io/sdk/gitea"
	"github.com/devon-mar/regexupdater/utils/giteautil"
	"github.com/go-playground/validator/v10"
)

const (
	typeGitea = "gitea"

	// The default page size
	// https://docs.gitea.io/en-us/config-cheat-sheet/
	giteaDefaultPageSize = 30
)

type giteaConfig struct {
	Owner string `cfg:"owner" validate:"required"`
	Repo  string `cfg:"repo" validate:"required"`
	Tags  bool   `cfg:"tags"`
}

type Gitea struct {
	URL      string `cfg:"url" validate:"required,url"`
	Owner    string `cfg:"owner" validate:"required"`
	Repo     string `cfg:"repo" validate:"required"`
	PageSize int    `cfg:"page_size" validate:"omitempty,gte=0"`
	Limit    int    `cfg:"limit" validate:"gte=0"`

	client *gitea.Client
}

func (g *Gitea) validate() error {
	return validator.New().Struct(g)
}

func (g *Gitea) init() error {
	var err error
	if err = g.validate(); err != nil {
		return err
	}

	if g.PageSize == 0 {
		g.PageSize = giteaDefaultPageSize
	}

	if g.Limit == 0 {
		g.Limit = g.PageSize
	}

	g.client, err = gitea.NewClient(g.URL)
	return err
}

// NewConfig implements Feed
func (*Gitea) NewConfig(c map[string]interface{}) (interface{}, error) {
	return newConfig(c, &giteaConfig{})
}

// GetRelease implements Feed
func (g *Gitea) GetRelease(release string, config interface{}) (*Release, error) {
	cfg := config.(*giteaConfig)
	if cfg.Tags {
		return g.getReleaseTags(release, cfg)
	}
	return g.getReleaseReleases(release, cfg)
}

func (g *Gitea) getReleaseReleases(release string, cfg *giteaConfig) (*Release, error) {
	rel, _, err := g.client.GetReleaseByTag(
		cfg.Owner,
		cfg.Repo,
		release,
	)
	if err != nil {
		return nil, err
	}
	return releaseFromGiteaRelease(rel), err
}

// GetReleases implements Feed
func (g *Gitea) GetReleases(config interface{}, done chan struct{}) (chan *Release, chan error) {
	relChan := make(chan *Release)
	errChan := make(chan error)

	cfg := config.(*giteaConfig)

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

func (g *Gitea) getReleasesReleases(cfg *giteaConfig, relChan chan *Release, errChan chan error, done chan struct{}) {
	opts := gitea.ListReleasesOptions{
		ListOptions: gitea.ListOptions{PageSize: g.PageSize},
	}
	for {
		releases, resp, err := g.client.ListReleases(
			cfg.Owner,
			cfg.Repo,
			opts,
		)
		if err != nil {
			errChan <- err
			return
		}

		for _, r := range releases {
			select {
			case relChan <- releaseFromGiteaRelease(r):
			case <-done:
				return
			}
		}

		if nextPage := giteautil.NextPage(resp.Header.Get("Link")); nextPage == 0 {
			break
		} else {
			opts.Page = nextPage
		}
	}
}

func (g *Gitea) getReleaseTags(release string, cfg *giteaConfig) (*Release, error) {
	tag, _, err := g.client.GetTag(
		cfg.Owner,
		cfg.Repo,
		release,
	)
	if err != nil {
		return nil, err
	}
	return releaseFromGiteaTag(tag), nil
}

func (g *Gitea) getReleasesTags(cfg *giteaConfig, relChan chan *Release, errChan chan error, done chan struct{}) {
	opts := gitea.ListRepoTagsOptions{
		ListOptions: gitea.ListOptions{PageSize: g.PageSize},
	}
	for {
		tags, resp, err := g.client.ListRepoTags(
			cfg.Owner,
			cfg.Repo,
			opts,
		)
		if err != nil {
			errChan <- err
		}

		for _, t := range tags {
			select {
			case relChan <- releaseFromGiteaTag(t):
			case <-done:
				return
			}
		}

		if nextPage := giteautil.NextPage(resp.Header.Get("Link")); nextPage == 0 {
			break
		} else {
			opts.Page = nextPage
		}
	}
}

func releaseFromGiteaRelease(r *gitea.Release) *Release {
	return &Release{
		Version:      r.TagName,
		ReleaseNotes: r.Note,
		URL:          r.HTMLURL,
	}
}

func releaseFromGiteaTag(t *gitea.Tag) *Release {
	var url string
	if t.Commit != nil {
		url = t.Commit.URL
	}
	return &Release{
		Version:      t.Name,
		ReleaseNotes: t.Message,
		URL:          url,
	}
}
