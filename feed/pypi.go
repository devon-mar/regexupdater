package feed

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

const (
	typePyPI = "pypi"
	pypiURL  = "https://pypi.org"
)

type pypiJSON struct {
	Info struct {
		ProjectURL string `json:"project_url"`
	} `json:"info"`
	Releases map[string][]struct {
		CommentText       string      `json:"comment_text"`
		UploadTimeIso8601 time.Time   `json:"upload_time_iso_8601"`
		Yanked            bool        `json:"yanked"`
		YankedReason      interface{} `json:"yanked_reason"`
	} `json:"releases"`
}

func (p *pypiJSON) GetRelease(version string) *Release {
	rel, ok := p.Releases[version]
	if !ok {
		return nil
	}
	if len(rel) == 0 {
		return nil
	}
	if rel[0].Yanked {
		return nil
	}
	return &Release{
		Version:      version,
		ReleaseNotes: rel[0].CommentText,
		URL:          strings.TrimRight(p.Info.ProjectURL, "/") + "/" + version,
	}
}

type PyPI struct {
	URL string `cfg:"url" validate:"omitempty,url"`
}

func (p *PyPI) init() error {
	if p.URL == "" {
		p.URL = pypiURL
	}
	p.URL = strings.TrimRight(p.URL, "/")
	return nil
}

// NewConfig implements Feed
func (*PyPI) NewConfig(c map[string]interface{}) (interface{}, error) {
	return newConfig(c, &pypiConfig{})
}

type pypiConfig struct {
	Project string `cfg:"project" validate:"required"`
}

func (p *PyPI) getProjectJSON(project string) (*pypiJSON, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(fmt.Sprintf("%s/pypi/%s/json", p.URL, project))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	decoded := &pypiJSON{}
	if err := json.NewDecoder(resp.Body).Decode(decoded); err != nil {
		return nil, err
	}
	return decoded, nil
}

// GetRelease implements Feed
func (p *PyPI) GetRelease(release string, config interface{}) (*Release, error) {
	cfg := config.(*pypiConfig)

	data, err := p.getProjectJSON(cfg.Project)
	if err != nil {
		return nil, err
	}

	return data.GetRelease(release), nil
}

// GetReleases implements Feed
func (p *PyPI) GetReleases(config interface{}, done chan struct{}) (chan *Release, chan error) {
	relChan := make(chan *Release)
	errChan := make(chan error)

	go func() {
		defer close(errChan)
		defer close(relChan)

		cfg := config.(*pypiConfig)

		data, err := p.getProjectJSON(cfg.Project)
		if err != nil {
			errChan <- err
			return
		}

		releases := make([]string, 0, len(data.Releases))
		for k, v := range data.Releases {
			if len(v) == 0 || v[0].Yanked {
				continue
			}
			releases = append(releases, k)
		}
		sort.Slice(releases, func(i, j int) bool {
			return data.Releases[releases[i]][0].UploadTimeIso8601.After(data.Releases[releases[j]][0].UploadTimeIso8601)
		})

		for _, r := range releases {
			select {
			case relChan <- data.GetRelease(r):
			case <-done:
				return
			}
		}
	}()

	return relChan, errChan
}
