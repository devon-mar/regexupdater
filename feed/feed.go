package feed

import (
	"fmt"
	"strings"

	"github.com/devon-mar/regexupdater/utils/envtag"
	"github.com/go-playground/validator/v10"
	"github.com/mitchellh/mapstructure"
)

const (
	cfgTag = "cfg"
)

var validate = validator.New()

type Feed interface {
	// Return the given release or nil if not found.
	GetRelease(release string, config interface{}) (*Release, error)
	GetReleases(config interface{}, done chan struct{}) (chan *Release, chan error)
	NewConfig(c map[string]interface{}) (interface{}, error)
}

type Release struct {
	Version      string
	ReleaseNotes string
	URL          string
}

func NewFeed(name string, typ string, cfg map[string]interface{}) (Feed, error) {
	f, err := getFeed(name, typ, cfg)
	if err != nil {
		return nil, err
	}

	if v, ok := f.(interface{ init() error }); ok {
		if err := v.init(); err != nil {
			return nil, fmt.Errorf("error initializing feed: %w", err)
		}
	}

	return f, nil
}

func Validate(name string, typ string, cfg map[string]interface{}) error {
	_, err := getFeed(name, typ, cfg)
	return err
}

func ValidateUpdate(typ string, cfg map[string]interface{}) error {
	f, err := getFeedType(typ)
	if err != nil {
		return err
	}
	_, err = f.NewConfig(cfg)
	if err != nil {
		return err
	}

	return nil
}

// Returns a new empty Feed for the given typ.
func getFeedType(typ string) (Feed, error) {
	switch typ {
	case typeGitHubReleases:
		return &GitHubReleases{}, nil
	case typeGitHubTags:
		return &GitHubTags{}, nil
	case typeGiteaReleases:
		return &GiteaReleases{}, nil
	case typeGiteaTags:
		return &GiteaTags{}, nil
	case typePyPI:
		return &PyPI{}, nil
	case typeRSS:
		return &RSS{}, nil
	case typeContainer:
		return &ContainerRegistry{}, nil
	default:
		return nil, fmt.Errorf("unsupported feed type %q", typ)
	}
}

// Validates and returns the feed.
func getFeed(name string, typ string, cfg map[string]interface{}) (Feed, error) {
	f, err := getFeedType(typ)
	if err != nil {
		return nil, err
	}

	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{Result: f, ErrorUnused: true, TagName: cfgTag})
	if err != nil {
		return nil, err
	}
	if err = decoder.Decode(cfg); err != nil {
		return nil, err
	}

	envtag.Unmarshal(cfgTag, "FEED_"+strings.ToUpper(name)+"_", f)

	if err := validate.Struct(f); err != nil {
		return nil, err
	}

	return f, nil
}

func releaseFromReleases(f Feed, release string, config interface{}) (*Release, error) {
	done := make(chan struct{})
	defer close(done)
	releases, errors := f.GetReleases(config, done)
	for {
		select {
		case rel, ok := <-releases:
			if !ok {
				return nil, nil
			}
			if rel.Version == release {
				return rel, nil
			}
		case err, ok := <-errors:
			if !ok {
				return nil, nil
			}
			return nil, err
		}
	}
}

func newConfig(configMap map[string]interface{}, config interface{}) (interface{}, error) {
	d, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{Result: config, ErrorUnused: true, TagName: cfgTag})
	if err != nil {
		return nil, fmt.Errorf("error initializing config decoder: %w", err)
	}
	if err = d.Decode(configMap); err != nil {
		return nil, fmt.Errorf("error unmarshalling update feed config: %w", err)
	}
	if err = validate.Struct(config); err != nil {
		return nil, err
	}
	return config, nil
}

// Limit the number of releases to limit.
func limit(relChan chan *Release, errChan chan error, limit int) (chan *Release, chan error) {
	ourRel := make(chan *Release)
	ourErr := make(chan error)

	go func() {
		defer close(ourRel)
		defer close(ourErr)

		var releasesSent int
		for {
			select {
			case r, ok := <-relChan:
				if !ok {
					return
				}
				ourRel <- r
				releasesSent++
				if releasesSent == limit {
					return
				}
			case e, ok := <-errChan:
				if !ok {
					return
				}
				ourErr <- e
			}
		}
	}()
	return ourRel, ourErr
}
