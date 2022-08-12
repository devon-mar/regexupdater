package feed

import (
	"code.gitea.io/sdk/gitea"
	"github.com/devon-mar/regexupdater/utils/giteautil"
	"github.com/go-playground/validator/v10"
)

const (
	typeGiteaReleases = "gitea_releases"
	typeGiteaTags     = "gitea_tags"

	// The default page size
	// https://docs.gitea.io/en-us/config-cheat-sheet/
	giteaDefaultPageSize = 30
)

type giteaConfig struct {
	Owner string `cfg:"owner" validate:"required"`
	Repo  string `cfg:"repo" validate:"required"`
}

type giteaCommon struct {
	URL      string `cfg:"url" validate:"required,url"`
	Owner    string `cfg:"owner" validate:"required"`
	Repo     string `cfg:"repo" validate:"required"`
	PageSize int    `cfg:"page_size" validate:"omitempty,gte=0"`
	Limit    int    `cfg:"limit" validate:"gte=0"`

	client *gitea.Client
}

func (g *giteaCommon) validate() error {
	return validator.New().Struct(g)
}

func (g *giteaCommon) init() error {
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
func (*giteaCommon) NewConfig(c map[string]interface{}) (interface{}, error) {
	return newConfig(c, &giteaConfig{})
}

type GiteaReleases struct {
	giteaCommon
}

// GetRelease implements Feed
func (g *GiteaReleases) GetRelease(release string, config interface{}) (*Release, error) {
	cfg := config.(*giteaConfig)

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
func (g *GiteaReleases) GetReleases(config interface{}, done chan struct{}) (chan *Release, chan error) {
	r, e := g.getReleases(config, done)
	return limit(r, e, g.Limit)
}

func (g *GiteaReleases) getReleases(config interface{}, done chan struct{}) (chan *Release, chan error) {
	relChan := make(chan *Release)
	errChan := make(chan error)

	go func() {
		defer close(errChan)
		defer close(relChan)

		cfg := config.(*giteaConfig)

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
					break
				}
			}

			if nextPage := giteautil.NextPage(resp.Header.Get("Link")); nextPage == 0 {
				break
			} else {
				opts.Page = nextPage
			}
		}
	}()

	return relChan, errChan
}

func releaseFromGiteaRelease(r *gitea.Release) *Release {
	return &Release{
		Version:      r.TagName,
		ReleaseNotes: r.Note,
		URL:          r.HTMLURL,
	}
}

type GiteaTags struct {
	giteaCommon
}

// GetRelease implements Feed
func (g *GiteaTags) GetRelease(release string, config interface{}) (*Release, error) {
	cfg := config.(*giteaConfig)

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

// GetReleases implements Feed
func (g *GiteaTags) GetReleases(config interface{}, done chan struct{}) (chan *Release, chan error) {
	r, e := g.getReleases(config, done)
	return limit(r, e, g.Limit)
}

func (g *GiteaTags) getReleases(config interface{}, done chan struct{}) (chan *Release, chan error) {
	relChan := make(chan *Release)
	errChan := make(chan error)

	go func() {
		defer close(errChan)
		defer close(relChan)

		cfg := config.(*giteaConfig)

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
	}()

	return relChan, errChan
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
