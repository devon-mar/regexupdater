package feed

import (
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"testing"
	"time"
)

func newTestPyPI() (*PyPI, func(), error) {
	data, err := os.ReadFile("testdata/pypi/pip.json")
	if err != nil {
		return nil, nil, err
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/pypi/pip/json":
			w.Header().Add("content-type", "application/json")
			_, _ = w.Write(data)
		default:
			http.Error(w, "", http.StatusNotFound)
		}
	}))

	p := &PyPI{URL: ts.URL}
	if err = p.init(); err != nil {
		return nil, nil, err
	}
	return p, ts.Close, nil
}

func TestPyPIGetReleasesInvalid(t *testing.T) {
	pypi, cleanup, err := newTestPyPI()
	if err != nil {
		t.Fatalf("error initializing test PyPI: %v", err)
	}
	defer cleanup()

	relChan, errChan := pypi.GetReleases(&pypiConfig{Project: "invalid"}, nil)

	select {
	case err = <-errChan:
	case <-time.After(time.Second * 2):
	}

	if err == nil {
		t.Errorf("expected an error")
	}

	assertClosed(t, relChan, errChan)
}

func TestPyPIGetReleases(t *testing.T) {
	pypi, cleanup, err := newTestPyPI()
	if err != nil {
		t.Fatalf("error initializing test PyPI: %v", err)
	}
	defer cleanup()

	haveReleases := []*Release{}
	wantReleases := []*Release{
		{Version: "20.1.1", URL: "https://pypi.org/project/pip/20.1.1"},
		{Version: "20.1", URL: "https://pypi.org/project/pip/20.1"},
		// This version is yanked
		// {Version: "20.0", URL: "https://pypi.org/project/pip/20.0"},
		{Version: "20.1b1", URL: "https://pypi.org/project/pip/20.1b1"},
		{Version: "20.0.1", URL: "https://pypi.org/project/pip/20.0.1"},
		{Version: "1.5.5", URL: "https://pypi.org/project/pip/1.5.5"},
	}

	relChan, errChan := pypi.GetReleases(&pypiConfig{Project: "pip"}, nil)
outer:
	for i := 0; i < 20; i++ {
		select {
		case r, ok := <-relChan:
			if !ok {
				break outer
			}
			haveReleases = append(haveReleases, r)
		case err, ok := <-errChan:
			if !ok {
				break outer
			}
			t.Errorf("unexpected error: %v", err)
			break outer
		}
	}

	if !reflect.DeepEqual(haveReleases, wantReleases) {
		t.Errorf("got releases %#v, want %#v", haveReleases, wantReleases)
	}

	assertClosed(t, relChan, errChan)
}

func TestPyPIGetReleasesDone(t *testing.T) {
	pypi, cleanup, err := newTestPyPI()
	if err != nil {
		t.Fatalf("error initializing test PyPI: %v", err)
	}
	defer cleanup()

	haveReleases := []*Release{}
	wantReleases := []*Release{
		{Version: "20.1.1", URL: "https://pypi.org/project/pip/20.1.1"},
		{Version: "20.1", URL: "https://pypi.org/project/pip/20.1"},
	}

	done := make(chan struct{})
	relChan, errChan := pypi.GetReleases(&pypiConfig{Project: "pip"}, done)
outer:
	for i := 0; i < 2; i++ {
		select {
		case r, ok := <-relChan:
			if !ok {
				break outer
			}
			haveReleases = append(haveReleases, r)
		case err, ok := <-errChan:
			if !ok {
				break outer
			}
			t.Errorf("unexpected error: %v", err)
			break outer
		}
	}
	close(done)

	if !reflect.DeepEqual(haveReleases, wantReleases) {
		t.Errorf("got releases %#v, want %#v", haveReleases, wantReleases)
	}

	assertClosed(t, relChan, errChan)
}

func TestPyPIGetRelease(t *testing.T) {
	pypi, cleanup, err := newTestPyPI()
	if err != nil {
		t.Fatalf("error initializing test PyPI: %v", err)
	}
	defer cleanup()

	tests := map[string]*Release{
		"20.1":  {Version: "20.1", URL: "https://pypi.org/project/pip/20.1"},
		"20.0":  nil, // yanked
		"0.1.0": nil, // doesn't exist
	}
	for version, want := range tests {
		t.Run(version, func(t *testing.T) {
			have, err := pypi.GetRelease(version, &pypiConfig{Project: "pip"})
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(have, want) {
				t.Errorf("got %#v, want %#v", have, want)
			}
		})
	}
}

func TestPyPIGetReleaseInvalid(t *testing.T) {
	pypi, cleanup, err := newTestPyPI()
	if err != nil {
		t.Fatalf("error initializing test PyPI: %v", err)
	}
	defer cleanup()
	_, err = pypi.GetRelease("20.1", &pypiConfig{Project: "invalid"})
	if err == nil {
		t.Errorf("expected an error")
	}
}

func TestPyPiGetNewConfig(t *testing.T) {
	tests := map[string]struct {
		config    map[string]interface{}
		wantError bool
		want      *pypiConfig
	}{
		"valid": {
			config: map[string]interface{}{"project": "http://example.com"},
			want:   &pypiConfig{Project: "http://example.com"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			r := &PyPI{}
			cfg, err := r.NewConfig(tc.config)
			if tc.wantError && err == nil {
				t.Error("expected an error")
			} else if !tc.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			_, ok := cfg.(*pypiConfig)
			if !ok {
				t.Errorf("unexpected type %T", cfg)
			}

			if !reflect.DeepEqual(cfg, tc.want) {
				t.Errorf("got config %#v, want %#v", cfg, tc.want)
			}
		})
	}
}
