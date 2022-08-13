package feed

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/devon-mar/regexupdater/utils/linkhdr"
)

const (
	typeContainer = "container_registry"

	wwwAuthHeader = "www-authenticate"
	authzHeader   = "Authorization"
)

type containerRegistryConfig struct {
	Repo string `cfg:"repo" validate:"required"`
}

type ContainerRegistry struct {
	URL      string `cfg:"url" validate:"required,url"`
	PageSize int    `cfg:"page_size" validate:"omitempty,gt=0"`
	Token    string `cfg:"token"`
	Limit    int    `cfg:"limit" validate:"gte=0"`
}

func (c *ContainerRegistry) init() error {
	c.URL = strings.TrimRight(c.URL, "/")
	return nil
}

// NewConfig implements Feed
func (*ContainerRegistry) NewConfig(c map[string]interface{}) (interface{}, error) {
	return newConfig(c, &containerRegistryConfig{})
}

// GetRelease implements Feed
func (c *ContainerRegistry) GetRelease(release string, config interface{}) (*Release, error) {
	return releaseFromReleases(c, release, config)
}

// GetReleases implements Feed
func (c *ContainerRegistry) GetReleases(config interface{}, done chan struct{}) (chan *Release, chan error) {
	r, e := c.getReleases(config, done)
	return limit(r, e, c.Limit)
}

func (c *ContainerRegistry) getReleases(config interface{}, done chan struct{}) (chan *Release, chan error) {
	relChan := make(chan *Release)
	errChan := make(chan error)
	go func() {
		defer close(relChan)
		defer close(errChan)

		cfg := config.(*containerRegistryConfig)

		isFirst := true

		url := c.URL + "/v2/" + cfg.Repo + "/tags/list"
		if c.PageSize != 0 {
			url += fmt.Sprintf("?n=%d", c.PageSize)
		}

		token := c.Token

		for {
			req, err := http.NewRequest(http.MethodGet, url, nil)
			if err != nil {
				errChan <- fmt.Errorf("error making new request %s: %w", url, err)
				return
			}
			if token != "" {
				req.Header.Add(authzHeader, "Bearer "+token)
			}

			client := &http.Client{Timeout: 10 * time.Second}
			resp, err := client.Do(req)
			if err != nil {
				errChan <- fmt.Errorf("error sending request %s: %w", url, err)
				return
			}
			defer resp.Body.Close()
			if isFirst && resp.StatusCode == http.StatusUnauthorized && c.Token == "" && resp.Header.Get(wwwAuthHeader) != "" {
				token, err = c.getToken(resp.Header.Get(wwwAuthHeader), client)
				if err != nil {
					errChan <- err
					return
				}
				req.Header.Set(authzHeader, "Bearer "+token)
				resp, err = client.Do(req)
				if err != nil {
					errChan <- fmt.Errorf("error sending request (with Auth): %w", err)
					return
				}
				defer resp.Body.Close()
			}
			isFirst = false

			if resp.StatusCode != http.StatusOK {
				errChan <- fmt.Errorf("HTTP status %s when retrieving tags: %s", resp.Status, url)
				return
			}

			tags := struct {
				Tags []string `json:"tags"`
			}{}
			if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
				errChan <- fmt.Errorf("error unmarshalling response: %w", err)
				return
			}

			for _, t := range tags.Tags {
				select {
				case relChan <- &Release{Version: t}:
				case <-done:
					return
				}
			}

			nextURL := linkhdr.Parse(resp.Header.Get("link"))["next"]
			if nextURL == "" {
				return
			}
			url = c.URL + nextURL
		}
	}()
	return relChan, errChan
}

func (c *ContainerRegistry) getToken(hdr string, client *http.Client) (string, error) {
	if !strings.HasPrefix(hdr, "Bearer") {
		return "", fmt.Errorf("Unsupported auth type: %s", hdr)
	}

	var realm string
	var service string
	var scope string

	// strip "Bearer "
	hdr = hdr[7:]
	for _, kv := range strings.Split(hdr, ",") {
		key, val, ok := strings.Cut(kv, "=")
		if !ok {
			return "", fmt.Errorf("Invalid KV pair %q", kv)
		}

		val = strings.Trim(val, `"`)

		switch key {
		case "realm":
			realm = val
		case "service":
			service = val
		case "scope":
			scope = val
		}
	}
	if realm == "" {
		return "", errors.New("realm is empty")
	}
	if service == "" {
		return "", errors.New("service is empty")
	}
	if scope == "" {
		return "", errors.New("scope is empty")
	}
	req, err := http.NewRequest(http.MethodGet, realm, nil)
	if err != nil {
		return "", err
	}
	q := req.URL.Query()
	q.Add("scope", scope)
	q.Add("service", service)
	req.URL.RawQuery = q.Encode()

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error sending token req: %w", err)
	}
	defer resp.Body.Close()

	token := struct {
		Token string `json:"token"`
	}{}

	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return "", err
	}
	if token.Token == "" {
		return "", errors.New("token was empty")
	}
	return token.Token, nil
}
