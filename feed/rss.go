package feed

import (
	"github.com/mmcdole/gofeed"
)

const (
	typeRSS = "rss"
)

type rssConfig struct {
	URL string `cfg:"url" validate:"required,url"`
}

type RSS struct{}

// NewConfig implements Feed
func (*RSS) NewConfig(c map[string]interface{}) (interface{}, error) {
	return newConfig(c, &rssConfig{})
}

// GetRelease implements Feed
func (r *RSS) GetRelease(release string, config interface{}) (*Release, error) {
	return releaseFromReleases(r, release, config)
}

// GetReleases implements Feed
func (g *RSS) GetReleases(config interface{}, done chan struct{}) (chan *Release, chan error) {
	relChan := make(chan *Release)
	errChan := make(chan error)
	go func() {
		defer close(relChan)
		defer close(errChan)

		cfg := config.(*rssConfig)

		fp := gofeed.NewParser()
		feed, err := fp.ParseURL(cfg.URL)
		if err != nil {
			errChan <- err
			return
		}

		for _, itm := range feed.Items {
			select {
			case relChan <- &Release{
				Version:      itm.Title,
				ReleaseNotes: itm.Content,
				URL:          itm.Link,
			}:
			case <-done:
				return
			}
		}
	}()
	return relChan, errChan
}
