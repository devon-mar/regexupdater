package feed

import (
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"
)

func newAtomServer() (*httptest.Server, error) {
	data, err := os.ReadFile("testdata/rss/logrus.atom")
	if err != nil {
		return nil, err
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(data)
	}))
	return ts, err
}

func new404Server() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "", http.StatusNotFound) }))
}

func TestRSSGetReleases(t *testing.T) {
	ts, err := newAtomServer()
	if err != nil {
		t.Fatalf("error initializing test server: %v", err)
	}
	defer ts.Close()

	r := &RSS{}

	want := []*Release{
		{Version: "v1.8.1", ReleaseNotes: "c0", URL: "https://github.com/sirupsen/logrus/releases/tag/v1.8.1"},
		{Version: "v1.8.0", ReleaseNotes: "c1", URL: "https://github.com/sirupsen/logrus/releases/tag/v1.8.0"},
		{Version: "v1.7.1", ReleaseNotes: "c2", URL: "https://github.com/sirupsen/logrus/releases/tag/v1.7.1"},
		{Version: "Release v1.6.0", ReleaseNotes: "c3", URL: "https://github.com/sirupsen/logrus/releases/tag/v1.6.0"},
		{Version: "v1.5.0", ReleaseNotes: "c4", URL: "https://github.com/sirupsen/logrus/releases/tag/v1.5.0"},
	}
	have := []*Release{}

	relChan, errChan := r.GetReleases(&rssConfig{URL: ts.URL}, nil)

outer:
	for i := 0; i < 20; i++ {
		select {
		case r, ok := <-relChan:
			if !ok {
				break outer
			}
			have = append(have, r)
		case err, ok := <-errChan:
			if !ok {
				break outer
			}
			t.Errorf("unexpected error: %v", err)
			break outer
		}
	}

	if !reflect.DeepEqual(have, want) {
		t.Errorf("got %#v, want %#v", have, want)
	}

	assertClosed(t, relChan, errChan)
}

func TestRSSGetReleasesDone(t *testing.T) {
	ts, err := newAtomServer()
	if err != nil {
		t.Fatalf("error initializing test server: %v", err)
	}
	defer ts.Close()

	r := &RSS{}

	want := []*Release{
		{Version: "v1.8.1", ReleaseNotes: "c0", URL: "https://github.com/sirupsen/logrus/releases/tag/v1.8.1"},
		{Version: "v1.8.0", ReleaseNotes: "c1", URL: "https://github.com/sirupsen/logrus/releases/tag/v1.8.0"},
	}
	have := []*Release{}

	done := make(chan struct{})
	relChan, errChan := r.GetReleases(&rssConfig{URL: ts.URL}, done)

outer:
	for i := 0; i < 2; i++ {
		select {
		case r, ok := <-relChan:
			if !ok {
				break outer
			}
			have = append(have, r)
		case err, ok := <-errChan:
			if ok {
				t.Errorf("unexpected error: %v", err)
			}
			break outer
		}
	}
	close(done)

	if !reflect.DeepEqual(have, want) {
		t.Errorf("got %#v, want %#v", have, want)
	}

	assertClosed(t, relChan, errChan)
}

func TestRSSGetReleasesError(t *testing.T) {
	ts := new404Server()
	defer ts.Close()

	r := &RSS{}

	relChan, errChan := r.GetReleases(&rssConfig{URL: ts.URL}, nil)
	var err error
	select {
	case err = <-errChan:
	case <-time.After(time.Second * 2):
	}

	if err == nil {
		t.Errorf("expected an error")
	}

	assertClosed(t, relChan, errChan)
}

func TestRSSGetReleaseError(t *testing.T) {
	ts := new404Server()
	defer ts.Close()

	r := &RSS{}

	_, err := r.GetRelease("v1.2.0", &rssConfig{URL: ts.URL})

	if err == nil {
		t.Errorf("expected an error")
	}
}

func TestRSSGetRelease(t *testing.T) {
	ts, err := newAtomServer()
	if err != nil {
		t.Fatalf("error initializing test server: %v", err)
	}
	defer ts.Close()

	tests := map[string]*Release{
		"v1.8.1": {Version: "v1.8.1", ReleaseNotes: "c0", URL: "https://github.com/sirupsen/logrus/releases/tag/v1.8.1"},
		"v1.7.1": {Version: "v1.7.1", ReleaseNotes: "c2", URL: "https://github.com/sirupsen/logrus/releases/tag/v1.7.1"},
		"v1.5.0": {Version: "v1.5.0", ReleaseNotes: "c4", URL: "https://github.com/sirupsen/logrus/releases/tag/v1.5.0"},
		"v0.1.0": nil,
	}

	r := &RSS{}

	for version, want := range tests {
		t.Run(version, func(t *testing.T) {
			have, err := r.GetRelease(version, &rssConfig{URL: ts.URL})
			if err != nil {
				t.Errorf("unexpected error")
			}
			if !reflect.DeepEqual(have, want) {
				t.Errorf("got release %#v, want %#v", have, want)
			}
		})
	}
}

func TestRSSGetNewConfig(t *testing.T) {
	tests := map[string]struct {
		config    map[string]interface{}
		wantError bool
		want      *rssConfig
	}{
		"valid": {
			config: map[string]interface{}{"url": "http://example.com"},
			want:   &rssConfig{URL: "http://example.com"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			r := &RSS{}
			cfg, err := r.NewConfig(tc.config)
			if tc.wantError && err == nil {
				t.Error("expected an error")
			} else if !tc.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			_, ok := cfg.(*rssConfig)
			if !ok {
				t.Errorf("unexpected type %T", cfg)
			}

			if !reflect.DeepEqual(cfg, tc.want) {
				t.Errorf("got config %#v, want %#v", cfg, tc.want)
			}
		})
	}
}
