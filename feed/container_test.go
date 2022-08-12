package feed

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
)

const (
	testContainerToken       = "12345abc"
	testContainerAuthService = "auth.example.com"
	testContainerScope       = "repository:library/alpine:pull"
)

func newTestRegistry() (string, func(), error) {
	auth := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			if r.URL.Query().Get("scope") == testContainerScope && r.URL.Query().Get("service") == testContainerAuthService {
				_, _ = w.Write([]byte(fmt.Sprintf(`{
    "token": "%s",
    "access_token": "%s",
    "expires_in": 300,
    "issued_at": "2022-07-19T16:48:50.352866197Z"
}`, testContainerToken, testContainerToken)))
				w.Header().Add("content-type", "application/json")
			} else {
				http.Error(w, "", http.StatusBadRequest)
			}
		default:
			http.Error(w, "", http.StatusNotFound)
		}
	}))

	alpine1, err := os.ReadFile("testdata/container/alpine_1.json")
	if err != nil {
		return "", nil, err
	}
	alpine2, err := os.ReadFile("testdata/container/alpine_2.json")
	if err != nil {
		return "", nil, err
	}
	registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v2/library/alpine/tags/list") && r.Header.Get(authzHeader) != "Bearer "+testContainerToken {
			w.Header().Add(wwwAuthHeader, fmt.Sprintf(`Bearer realm="%s/token",service="%s",scope="%s"`, auth.URL, testContainerAuthService, testContainerScope))
			w.WriteHeader(http.StatusUnauthorized)
		}

		switch r.URL.Path + "?" + r.URL.RawQuery {
		case "/v2/library/alpine/tags/list?n=2":
			w.Header().Add("content-type", "application/json")
			w.Header().Add("link", `</v2/library/alpine/tags/list?last=2.7&n=2>; rel="next"`)
			_, _ = w.Write(alpine1)
		case "/v2/library/alpine/tags/list?last=2.7&n=2":
			w.Header().Add("content-type", "application/json")
			_, _ = w.Write(alpine2)
		case "/v2/library/autherror/tags/list?":
			w.Header().Add(wwwAuthHeader, fmt.Sprintf(`Bearer realm="%s/token",service="other",scope="other"`, auth.URL))
			http.Error(w, "", http.StatusUnauthorized)
		case "/v2/library/noservice/tags/list?":
			w.Header().Add(wwwAuthHeader, fmt.Sprintf(`Bearer realm="%s/token",scope="other"`, auth.URL))
			http.Error(w, "", http.StatusUnauthorized)
		case "/v2/library/norealm/tags/list?":
			w.Header().Add(wwwAuthHeader, `Bearer service="other",scope="other"`)
			http.Error(w, "", http.StatusUnauthorized)
		case "/v2/library/noscope/tags/list?":
			w.Header().Add(wwwAuthHeader, fmt.Sprintf(`Bearer realm="%s/token",service="something"`, auth.URL))
			http.Error(w, "", http.StatusUnauthorized)
		case "/v2/library/basicauth/tags/list?":
			w.Header().Add(wwwAuthHeader, "Basic")
			http.Error(w, "", http.StatusUnauthorized)
		case "/v2/library/noauth/tags/list?":
			w.Header().Add("content-type", "application/json")
			_, _ = w.Write(alpine2)
		case "/v2/library/invalidjson/tags/list?":
			w.Header().Add("content-type", "application/json")
			_, _ = w.Write([]byte(`{"isjson":false`))
		case "/v2/library/noauthhdr/tags/list?n=2":
			if r.Header.Get(authzHeader) != "Bearer "+testContainerToken {
				http.Error(w, "", http.StatusUnauthorized)
				return
			}
			w.Header().Add("content-type", "application/json")
			_, _ = w.Write(alpine1)
		default:
			http.Error(w, "", http.StatusNotFound)
		}
	}))

	c := func() {
		registry.Close()
		auth.Close()
	}
	return registry.URL, c, nil
}

func TestGetReleases(t *testing.T) {
	tests := map[string]struct {
		token      string
		pageSize   int
		config     *containerRegistryConfig
		wantError  bool
		iterations int

		wantReleases []*Release
	}{
		"library/invalidjson": {
			config:       &containerRegistryConfig{Repo: "library/invalidjson"},
			wantError:    true,
			wantReleases: []*Release{},
		},
		"library/autherror": {
			config:       &containerRegistryConfig{Repo: "library/autherror"},
			wantError:    true,
			wantReleases: []*Release{},
		},
		"library/basicauth": {
			config:       &containerRegistryConfig{Repo: "library/basicauth"},
			wantError:    true,
			wantReleases: []*Release{},
		},
		"library/norealm": {
			config:       &containerRegistryConfig{Repo: "library/norealm"},
			wantError:    true,
			wantReleases: []*Release{},
		},
		"library/noscope": {
			config:       &containerRegistryConfig{Repo: "library/noscope"},
			wantError:    true,
			wantReleases: []*Release{},
		},
		"library/noservice": {
			config:       &containerRegistryConfig{Repo: "library/noservice"},
			wantError:    true,
			wantReleases: []*Release{},
		},
		"invalid URL": {
			config:       &containerRegistryConfig{Repo: "::"},
			wantError:    true,
			wantReleases: []*Release{},
		},
		"library/noauth no token": {
			config: &containerRegistryConfig{Repo: "library/noauth"},
			wantReleases: []*Release{
				{Version: "3"},
			},
		},
		"library/alpine no token": {
			config:   &containerRegistryConfig{Repo: "library/alpine"},
			pageSize: 2,
			wantReleases: []*Release{
				{Version: "2.6"},
				{Version: "2.7"},
				{Version: "3"},
			},
		},
		"library/noauthhdr without token": {
			config:       &containerRegistryConfig{Repo: "library/noauthhdr"},
			pageSize:     2,
			wantError:    true,
			wantReleases: []*Release{},
		},
		"library/noauthhdr with token": {
			token:    testContainerToken,
			config:   &containerRegistryConfig{Repo: "library/noauthhdr"},
			pageSize: 2,
			wantReleases: []*Release{
				{Version: "2.6"},
				{Version: "2.7"},
			},
		},
	}

	url, close, err := newTestRegistry()
	if err != nil {
		t.Fatalf("error initializing server: %v", err)
	}
	defer close()

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			c := &ContainerRegistry{
				URL:      url,
				Token:    tc.token,
				PageSize: tc.pageSize,
			}
			relChan, errChan := c.GetReleases(tc.config, nil)

			have := []*Release{}

			err = nil

			iterations := tc.iterations
			if iterations == 0 {
				iterations = 20
			}
		outer:
			for i := 0; i < iterations; i++ {
				select {
				case r, ok := <-relChan:
					if !ok {
						break outer
					}
					have = append(have, r)
				case err = <-errChan:

					break outer
				}
			}

			if !reflect.DeepEqual(have, tc.wantReleases) {
				t.Errorf("got releases %#v, want %#v", have, tc.wantReleases)
			}

			if err == nil && tc.wantError {
				t.Errorf("expected an error")
			} else if err != nil && !tc.wantError {
				t.Errorf("expected no error but got: %v", err)
			}

			assertClosed(t, relChan, errChan)
		})
	}
}
