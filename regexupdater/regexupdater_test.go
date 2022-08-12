package regexupdater

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/devon-mar/regexupdater/feed"
	"github.com/devon-mar/regexupdater/repository"
	log "github.com/sirupsen/logrus"
)

const (
	testFilePath     = "test_file"
	testFileSHA      = "12345"
	testPRID         = "#199"
	testPRSuperseded = "Superseded by " + testPRID
	testFeedName     = "test"
	testFeedRepo     = "testfeedrepo"
	testUpdateName   = "test"

	prFindErrUpdateName   = "searcherror"
	prCreateErrUpdateName = "createerror"
)

type testFile struct {
	content []byte
	path    string
}

// SHA implements repository.File
func (*testFile) SHA() string {
	return testFileSHA
}

// Content implements repository.File
func (f *testFile) Content() []byte {
	return f.content
}

// Path implements repository.File
func (f *testFile) Path() string {
	return f.path
}

type fileUpdate struct {
	content   string
	commitMsg string
	newBranch string
	prTitle   string
	prBody    string

	contentOnly string
}

type testPR struct {
	id        string
	open      bool
	comments  []string
	mergeable bool

	canClose   bool
	canComment bool

	prMeta prMetadata

	wantOpen     bool
	wantComments []string
}

// IsMergeable implements repository.PullRequest
func (pr *testPR) IsMergeable() bool {
	return pr.mergeable
}

// Body implements repository.PullRequest
func (pr *testPR) Body() string {
	return pr.prMeta.Footer()
}

// Close implements repository.PullRequest
func (pr *testPR) Close() error {
	if !pr.canClose {
		return errors.New("PR is read only")
	}
	pr.open = false
	return nil
}

// ID implements repository.PullRequest
func (pr *testPR) ID() string {
	return pr.id
}

// IsOpen implements repository.PullRequest
func (pr *testPR) IsOpen() bool {
	return pr.open
}

func (pr *testPR) assert(t *testing.T) {
	t.Helper()

	if pr.canClose && pr.wantOpen != pr.open {
		t.Errorf("got open=%t, want open=%t", pr.open, pr.wantOpen)
	}

	if pr.canComment && !reflect.DeepEqual(pr.comments, pr.wantComments) {
		t.Errorf("got comments %#v, want comments %#v", pr.comments, pr.wantComments)
	}
}

type testRepository struct {
	content string

	prs []*testPR

	haveUpdate *fileUpdate
	wantUpdate *fileUpdate
}

// DeletePRBranch implements repository.Repository
func (*testRepository) DeletePRBranch(prID string) (string, error) {
	panic("unimplemented")
}

// AddPRComment implements repository.Repository
func (r *testRepository) AddPRComment(pr repository.PullRequest, body string) error {
	tpr := pr.(*testPR)
	if !tpr.canComment {
		return errors.New("PR is read only")
	}
	tpr.comments = append(tpr.comments, body)
	return nil
}

// ClosePR implements repository.Repository
func (r *testRepository) ClosePR(pr repository.PullRequest) error {
	tpr := pr.(*testPR)
	if !tpr.canClose {
		return errors.New("PR is read only")
	}
	tpr.open = false
	return nil
}

// RebasePR implements Repository.Repository
func (*testRepository) RebasePR(pr repository.PullRequest, path string, oldSHA string, newContent []byte, commitMsg string) error {
	panic("unimplemented")
}

// FindPR implements repository.Repository
func (r *testRepository) FindPR(s string) (repository.PullRequest, error) {
	if s == getUpdateID(prFindErrUpdateName) {
		return nil, errors.New("got err search string")
	}
	for _, pr := range r.prs {
		if strings.Contains(pr.Body(), s) {
			return pr, nil
		}
	}
	return nil, nil
}

// GetFile implements repository.Repository
func (r *testRepository) GetFile(path string) (repository.File, error) {
	if path != testFilePath {
		return nil, fmt.Errorf("unknown path %q", path)
	}

	return &testFile{content: []byte(r.content), path: testFilePath}, nil
}

// UpdateFilePR implements repository.Repository
func (r *testRepository) UpdateFilePR(path string, oldSHA string, newContent []byte, commitMsg string, newBranch string, prTitle string, prBody string) (prID string, err error) {
	if oldSHA != testFileSHA {
		return "", fmt.Errorf("unexpected old SHA %q", oldSHA)
	}
	if strings.Contains(prTitle, prCreateErrUpdateName) {
		return "", errors.New("got pr create error title")
	}
	if path != testFilePath {
		return "", fmt.Errorf("unknown path %q", path)
	}

	if r.haveUpdate != nil {
		return "", fmt.Errorf("file has already been updated")
	}

	r.haveUpdate = &fileUpdate{
		content:   string(newContent),
		commitMsg: commitMsg,
		newBranch: newBranch,
		prTitle:   prTitle,
		prBody:    prBody,
	}

	return testPRID, nil
}

func (r *testRepository) assert(t *testing.T) {
	if r == nil {
		return
	}
	t.Helper()

	if r.wantUpdate == nil && r.haveUpdate != nil {
		t.Errorf("expected no file update but got %#v", r.haveUpdate)
		return
	}
	if r.wantUpdate != nil && r.haveUpdate == nil {
		t.Errorf("expected a file update")
		return
	}

	if r.wantUpdate != nil {
		if r.wantUpdate.contentOnly != "" && r.wantUpdate.contentOnly != r.haveUpdate.content {
			t.Errorf("got content %q, want %q", r.haveUpdate.content, r.wantUpdate.contentOnly)
		} else if r.wantUpdate.contentOnly == "" && !reflect.DeepEqual(r.haveUpdate, r.wantUpdate) {
			t.Errorf("got update %#v, want %#v", r.haveUpdate, r.wantUpdate)
		}
	}

	for _, pr := range r.prs {
		pr.assert(t)
	}
}

type testFeed struct {
	releases []*feed.Release
}

// NewConfig implements Feed
func (*testFeed) NewConfig(c map[string]interface{}) (interface{}, error) {
	panic("unimplemented")
}

// GetReleases implements feed.Feed
func (f *testFeed) GetReleases(config interface{}, done chan struct{}) (chan *feed.Release, chan error) {
	relChan := make(chan *feed.Release)
	errChan := make(chan error)

	repo := config.(string)

	go func() {
		defer close(relChan)
		defer close(errChan)
		if repo != testFeedRepo {
			errChan <- fmt.Errorf("unknown repo: %s", repo)
			return
		}
		for _, r := range f.releases {
			relChan <- r
		}
	}()

	return relChan, errChan
}

// GetRelease implements feed.Feed
func (f *testFeed) GetRelease(release string, config interface{}) (*feed.Release, error) {
	repo := config.(string)
	if repo != testFeedRepo {
		return nil, fmt.Errorf("unknown repo: %s", repo)
	}

	for _, r := range f.releases {
		if r.Version == release {
			return r, nil
		}
	}
	return nil, nil
}

func newTestFeed(versions ...string) *testFeed {
	releases := make([]*feed.Release, 0, len(versions))
	for _, v := range versions {
		releases = append(releases, &feed.Release{Version: v})
	}
	return &testFeed{releases: releases}
}

func newTestUpdate(regex string) updateConfig {
	return updateConfig{
		Name:   testUpdateName,
		Path:   testFilePath,
		Feed:   updateFeedConfig{Name: testFeedName, feedConfig: testFeedRepo},
		mregex: regexp.MustCompile(regex),
	}
}

func mustNewRegexUpdater(c *Config) *RegexUpdater {
	ru, err := NewUpdater(c)
	if err != nil {
		panic(err)
	}
	return ru
}

func TestProcess(t *testing.T) {
	const (
		testSecondaryFeed = "feed2"
	)
	testUpdateID := getUpdateID("test")
	tests := map[string]struct {
		u         updateConfig
		r         *testRepository
		f         *testFeed
		f2        *testFeed
		ru        *RegexUpdater
		wantError bool
	}{
		"semver update": {
			u: newTestUpdate(`(?m)^two: v([\d\.]+)$`),
			r: &testRepository{
				content: `one: v3.0.1
two: v1.2.0
three: v1.4.0
`,
				wantUpdate: &fileUpdate{
					contentOnly: `one: v3.0.1
two: v1.3.0
three: v1.4.0
`,
				},
			},
			f: newTestFeed("1.3.0"),
		},
		"semver update UseSemver=true": {
			u: updateConfig{
				Name:      "test",
				Path:      testFilePath,
				Feed:      updateFeedConfig{Name: testFeedName, feedConfig: testFeedRepo},
				mregex:    regexp.MustCompile("^v(.*)$"),
				UseSemver: true,
			},
			r: &testRepository{
				content: "v1.2.0",
				wantUpdate: &fileUpdate{
					contentOnly: "v2.0.0",
				},
			},
			f: newTestFeed("2.0"),
		},
		"FindPR error": {
			wantError: true,
			u: updateConfig{
				Name:   prFindErrUpdateName,
				Path:   testFilePath,
				Feed:   updateFeedConfig{Name: testFeedName, feedConfig: testFeedRepo},
				mregex: regexp.MustCompile("^v(.*)$"),
			},
			r: &testRepository{content: "v1.2.0"},
			f: newTestFeed("2.0"),
		},
		"Create PR error": {
			wantError: true,
			u: updateConfig{
				Name:   prCreateErrUpdateName,
				Path:   testFilePath,
				Feed:   updateFeedConfig{Name: testFeedName, feedConfig: testFeedRepo},
				mregex: regexp.MustCompile("^v(.*)$"),
			},
			r: &testRepository{content: "v1.2.0"},
			f: newTestFeed("2.0"),
		},
		"invalid file": {
			wantError: true,
			u: updateConfig{
				Name:      "test",
				Path:      "404",
				Feed:      updateFeedConfig{Name: testFeedName, feedConfig: testFeedRepo},
				mregex:    regexp.MustCompile("^v(.*)$"),
				UseSemver: true,
			},
			r: &testRepository{content: "v1.2.0"},
			f: newTestFeed("1.3.0"),
		},
		"existing PR metadata error (empty version)": {
			u: updateConfig{
				Name:   "test",
				Path:   testFilePath,
				Feed:   updateFeedConfig{Name: testFeedName, feedConfig: testFeedRepo},
				mregex: regexp.MustCompile("^(.*)$"),
			},
			r: &testRepository{
				content: "1.2.0",
				// The existing PR is ignored and the update is created anyways.
				wantUpdate: &fileUpdate{contentOnly: "1.3.0"},
				prs: []*testPR{
					{open: true, prMeta: prMetadata{ID: testUpdateID}},
				},
			},
			f: newTestFeed("1.3.0"),
		},
		"existing open and mergeable PR for update": {
			u: updateConfig{
				Name:   "test",
				Path:   testFilePath,
				Feed:   updateFeedConfig{Name: testFeedName, feedConfig: testFeedRepo},
				mregex: regexp.MustCompile("^(.*)$"),
			},
			r: &testRepository{
				content: "1.2.0",
				prs: []*testPR{
					{open: true, mergeable: true, prMeta: prMetadata{ID: testUpdateID, Version: "1.3.0"}},
				},
			},
			f: newTestFeed("1.3.0"),
		},
		"existing closed PR for update": {
			u: updateConfig{
				Name:   "test",
				Path:   testFilePath,
				Feed:   updateFeedConfig{Name: testFeedName, feedConfig: testFeedRepo},
				mregex: regexp.MustCompile("^(.*)$"),
			},
			r: &testRepository{
				content: "1.2.0",
				prs: []*testPR{
					{prMeta: prMetadata{ID: testUpdateID, Version: "1.3.0"}},
				},
			},
			f: newTestFeed("1.3.0"),
		},
		"existing closed PR for older version": {
			u: updateConfig{
				Name:   "test",
				Path:   testFilePath,
				Feed:   updateFeedConfig{Name: testFeedName, feedConfig: testFeedRepo},
				mregex: regexp.MustCompile("^(.*)$"),
			},
			r: &testRepository{
				content:    "1.2.0",
				wantUpdate: &fileUpdate{contentOnly: "1.3.0"},
				prs: []*testPR{
					{prMeta: prMetadata{ID: testUpdateID, Version: "1.0"}},
				},
			},
			f: newTestFeed("1.3.0"),
		},
		"existing open PR for older version action=stop": {
			u: updateConfig{
				Name:       "test",
				Path:       testFilePath,
				Feed:       updateFeedConfig{Name: testFeedName, feedConfig: testFeedRepo},
				mregex:     regexp.MustCompile("^(.*)$"),
				ExistingPR: existingPRStop,
			},
			r: &testRepository{
				content: "1.2.0",
				prs: []*testPR{
					{open: true, prMeta: prMetadata{ID: testUpdateID, Version: "1.0"}},
				},
			},
			f: newTestFeed("1.3.0"),
		},
		"existing open PR for older version action=close": {
			u: updateConfig{
				Name:       "test",
				Path:       testFilePath,
				Feed:       updateFeedConfig{Name: testFeedName, feedConfig: testFeedRepo},
				mregex:     regexp.MustCompile("^(.*)$"),
				ExistingPR: existingPRClose,
			},
			r: &testRepository{
				content:    "1.2.0",
				wantUpdate: &fileUpdate{contentOnly: "1.3.0"},
				prs: []*testPR{
					{
						canClose:     true,
						canComment:   true,
						open:         true,
						prMeta:       prMetadata{ID: testUpdateID, Version: "1.0"},
						wantOpen:     false,
						wantComments: []string{testPRSuperseded},
					},
				},
			},
			f: newTestFeed("1.3.0"),
		},
		"existing open PR for older version action=close DRY RUN": {
			ru: mustNewRegexUpdater(&Config{DryRun: true}),
			u: updateConfig{
				Name:       "test",
				Path:       testFilePath,
				Feed:       updateFeedConfig{Name: testFeedName, feedConfig: testFeedRepo},
				mregex:     regexp.MustCompile("^(.*)$"),
				ExistingPR: existingPRClose,
			},
			r: &testRepository{
				content: "1.2.0",
				// We should get no update
				// wantUpdate: &fileUpdate{contentOnly: "1.3.0"},
				prs: []*testPR{
					{
						// The old PR should stay open.
						open:         true,
						prMeta:       prMetadata{ID: testUpdateID, Version: "1.0"},
						wantOpen:     false,
						wantComments: []string{testPRSuperseded},
					},
				},
			},
			f: newTestFeed("1.3.0"),
		},
		"existing open PR for older version action=close with error commenting": {
			wantError: true,
			u: updateConfig{
				Name:       "test",
				Path:       testFilePath,
				Feed:       updateFeedConfig{Name: testFeedName, feedConfig: testFeedRepo},
				mregex:     regexp.MustCompile("^(.*)$"),
				ExistingPR: existingPRClose,
			},
			r: &testRepository{
				content:    "1.2.0",
				wantUpdate: &fileUpdate{contentOnly: "1.3.0"},
				prs: []*testPR{
					{
						canComment: false,
						canClose:   false,
						open:       true,
						prMeta:     prMetadata{ID: testUpdateID, Version: "1.0"},
					},
				},
			},
			f: newTestFeed("1.3.0"),
		},
		"existing open PR for older version action=close with error closing": {
			wantError: true,
			u: updateConfig{
				Name:       "test",
				Path:       testFilePath,
				Feed:       updateFeedConfig{Name: testFeedName, feedConfig: testFeedRepo},
				mregex:     regexp.MustCompile("^(.*)$"),
				ExistingPR: existingPRClose,
			},
			r: &testRepository{
				content:    "1.2.0",
				wantUpdate: &fileUpdate{contentOnly: "1.3.0"},
				prs: []*testPR{
					{
						canComment:   true,
						canClose:     false,
						open:         true,
						prMeta:       prMetadata{ID: testUpdateID, Version: "1.0"},
						wantComments: []string{testPRSuperseded},
					},
				},
			},
			f: newTestFeed("1.3.0"),
		},
		"skip prerelease": {
			u: newTestUpdate("^(.*)$"),
			r: &testRepository{content: "1.2.0", wantUpdate: &fileUpdate{contentOnly: "1.3.0"}},
			f: newTestFeed("1.4.0-beta", "1.3.0"),
		},
		"include prerelease": {
			u: updateConfig{
				Name:       "test",
				Path:       testFilePath,
				Feed:       updateFeedConfig{Name: testFeedName, feedConfig: testFeedRepo},
				Prerelease: true,
				mregex:     regexp.MustCompile("^(.*)$"),
			},
			r: &testRepository{content: "1.2.0", wantUpdate: &fileUpdate{contentOnly: "1.4.0-beta"}},
			f: newTestFeed("1.4.0-beta", "1.3.0"),
		},
		"semver only same version avail": {
			u: newTestUpdate("^v(.*)$"),
			r: &testRepository{content: "v1.2.0"},
			f: newTestFeed("1.2.0"),
		},
		"semver only old version avail": {
			u: newTestUpdate("^v(.*)$"),
			r: &testRepository{content: "v1.2.0"},
			f: newTestFeed("1.1.0"),
		},
		"no regex match": {
			wantError: true,
			u:         newTestUpdate(`^\d(.*)$`),
			r:         &testRepository{content: "v1.2.0"},
			f:         newTestFeed("1.1.0"),
		},
		"no regex match on new content": {
			wantError: true,
			u: updateConfig{
				Name:       "test",
				Path:       testFilePath,
				Feed:       updateFeedConfig{Name: testFeedName, feedConfig: testFeedRepo},
				Prerelease: true,
				mregex:     regexp.MustCompile(`^([\d\.]+)$`),
			},
			r: &testRepository{content: "1.2.0"},
			f: newTestFeed("1.4.0-beta"),
		},
		"file with invalid semver": {
			wantError: true,
			u:         newTestUpdate("^(.*)$"),
			r:         &testRepository{content: "1.20a"},
			f:         newTestFeed("1.2.0"),
		},
		"release version not semver": {
			wantError: true,
			u:         newTestUpdate("^(.*)$"),
			r:         &testRepository{content: "1.2.0"},
			f:         newTestFeed("1.2a0"),
		},
		"not semver update": {
			u: updateConfig{
				Name:        "test",
				IsNotSemver: true,
				Path:        testFilePath,
				Feed:        updateFeedConfig{Name: testFeedName, feedConfig: testFeedRepo},
				mregex:      regexp.MustCompile("^(.*)$"),
			},
			r: &testRepository{content: "2022-07-10", wantUpdate: &fileUpdate{contentOnly: "2022-07-12"}},
			f: newTestFeed("2022-07-12", "2022-07-11"),
		},
		"not semver and no update": {
			u: updateConfig{
				Name:        "test",
				IsNotSemver: true,
				Path:        testFilePath,
				Feed:        updateFeedConfig{Name: testFeedName, feedConfig: testFeedRepo},
				mregex:      regexp.MustCompile("^(.*)$"),
			},
			r: &testRepository{content: "2022-07-10"},
			f: newTestFeed("2022-07-10"),
		},
		"release feed exhausted": {
			u: newTestUpdate("^(.*)$"),
			r: &testRepository{content: "1.0"},
			f: &testFeed{
				releases: []*feed.Release{{Version: "1.1.0-beta"}, {Version: "1.1.0-alpha"}, {Version: "1.0.1-beta"}},
			},
		},
		"custom templates": {
			ru: mustNewRegexUpdater(&Config{Templates: templateConfig{
				PRTitle:   "prTitle-{{ .Name }}-{{ .Old }}-{{ .Old.V }}-{{ .New }}-{{ .New.V }}",
				PRBody:    "prBody-{{ .Name }}-{{ .Old }}-{{ .Old.V }}-{{ .New }}-{{ .New.V }}",
				CommitMsg: "commitMsg-{{ .Name }}-{{ .Old }}-{{ .Old.V }}-{{ .New }}-{{ .New.V }}",
				Branch:    "branch-{{ .Name }}-{{ .Old }}-{{ .Old.V }}-{{ .New }}-{{ .New.V }}",
			}}),
			u: newTestUpdate("^(.*)$"),
			r: &testRepository{
				content: "1.0",
				wantUpdate: &fileUpdate{
					content:   "2.0",
					commitMsg: "commitMsg-test-1.0.0-1.0-2.0.0-2.0",
					newBranch: "branch-test-1.0.0-1.0-2.0.0-2.0",
					prTitle:   "prTitle-test-1.0.0-1.0-2.0.0-2.0",
					prBody: "prBody-test-1.0.0-1.0-2.0.0-2.0\n" + prMetadata{
						ID:      testUpdateID,
						Update:  testUpdateName,
						Version: "2.0.0",
					}.Footer(),
				},
			},
			f: &testFeed{
				releases: []*feed.Release{{Version: "2.0", URL: "http://127.0.0.1"}},
			},
		},
		"invalid title template": {
			wantError: true,
			ru:        mustNewRegexUpdater(&Config{Templates: templateConfig{PRTitle: "{{ .invalid }}"}}),
			u:         newTestUpdate("^(.*)$"),
			r:         &testRepository{content: "1.0"},
			f:         newTestFeed("2.0"),
		},
		"invalid body template": {
			wantError: true,
			ru:        mustNewRegexUpdater(&Config{Templates: templateConfig{PRBody: "{{ .invalid }}"}}),
			u:         newTestUpdate("^(.*)$"),
			r:         &testRepository{content: "1.0"},
			f:         newTestFeed("2.0"),
		},
		"invalid commit template": {
			wantError: true,
			ru:        mustNewRegexUpdater(&Config{Templates: templateConfig{CommitMsg: "{{ .invalid }}"}}),
			u:         newTestUpdate("^(.*)$"),
			r:         &testRepository{content: "1.0"},
			f:         newTestFeed("2.0"),
		},
		"invalid branch template": {
			wantError: true,
			ru:        mustNewRegexUpdater(&Config{Templates: templateConfig{Branch: "{{ .invalid }}"}}),
			u:         newTestUpdate("^(.*)$"),
			r:         &testRepository{content: "1.0"},
			f:         newTestFeed("2.0"),
		},
		"semver update, secondary feed has release": {
			u: updateConfig{
				Name: "test",
				Path: testFilePath,
				Feed: updateFeedConfig{Name: testFeedName, feedConfig: testFeedRepo},
				SecondaryFeed: &SecondaryFeedConfig{
					Feed: &updateFeedConfig{
						Name:       testSecondaryFeed,
						feedConfig: testFeedRepo,
					},
				},
				mregex: regexp.MustCompile("^(.*)$"),
			},
			r: &testRepository{
				content:    "1.0",
				wantUpdate: &fileUpdate{contentOnly: "1.3"},
			},
			f:  newTestFeed("1.3"),
			f2: newTestFeed("1.3"),
		},
		"semver update, secondary feed has release with replace": {
			u: updateConfig{
				Name: "test",
				Path: testFilePath,
				Feed: updateFeedConfig{Name: testFeedName, feedConfig: testFeedRepo},
				SecondaryFeed: &SecondaryFeedConfig{
					Feed: &updateFeedConfig{
						Name:       testSecondaryFeed,
						feedConfig: testFeedRepo,
					},
					Replace: &Replace{
						Replace: "abcd$1",
						regex:   regexp.MustCompile("(.*)"),
					},
				},
				mregex: regexp.MustCompile("^(.*)$"),
			},
			r: &testRepository{
				content:    "1.0",
				wantUpdate: &fileUpdate{contentOnly: "1.3"},
			},
			f:  newTestFeed("1.3"),
			f2: newTestFeed("abcd1.3"),
		},
		"semver update, secondary feed DOES NOT have release": {
			u: updateConfig{
				Name: "test",
				Path: testFilePath,
				Feed: updateFeedConfig{Name: testFeedName, feedConfig: testFeedRepo},
				SecondaryFeed: &SecondaryFeedConfig{
					Feed: &updateFeedConfig{
						Name:       testSecondaryFeed,
						feedConfig: testFeedRepo,
					},
				},
				mregex: regexp.MustCompile("^(.*)$"),
			},
			r:  &testRepository{content: "1.0"},
			f:  newTestFeed("1.3", "1.2"),
			f2: newTestFeed("1.2"),
		},
		"semver update, secondary feed error": {
			u: updateConfig{
				Name: "test",
				Path: testFilePath,
				Feed: updateFeedConfig{Name: testFeedName, feedConfig: testFeedRepo},
				SecondaryFeed: &SecondaryFeedConfig{
					Feed: &updateFeedConfig{
						Name:       testSecondaryFeed,
						feedConfig: "error",
					},
				},
				mregex: regexp.MustCompile("^(.*)$"),
			},
			r:         &testRepository{content: "1.0"},
			f:         newTestFeed("1.3"),
			f2:        newTestFeed("1.2"),
			wantError: true,
		},
		"PR for version in the file": {
			u: newTestUpdate(`(.*)`),
			r: &testRepository{
				content: "1.3.0",
				prs: []*testPR{
					{
						prMeta:   prMetadata{ID: testUpdateID, Version: "1.3.0"},
						canClose: true, canComment: true, open: true,
						wantComments: []string{fmt.Sprintf("`%s` is already using this version. This PR is no longer necessary.", testUpdateName)},
					},
				},
			},
			f: newTestFeed(),
		},
		"PR for version in the file with update as well": {
			u: newTestUpdate(`(.*)`),
			r: &testRepository{
				content:    "1.3.0",
				wantUpdate: &fileUpdate{contentOnly: "1.4.0"},
				prs: []*testPR{
					{
						prMeta:   prMetadata{ID: testUpdateID, Version: "1.3.0"},
						canClose: true, canComment: true, open: true,
						wantComments: []string{fmt.Sprintf("`%s` is already using this version. This PR is no longer necessary.", testUpdateName)},
					},
				},
			},
			f: newTestFeed("1.4.0"),
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			if name == "semver update" {
				t.Log("debug")
			}
			var err error
			if tc.ru == nil {
				tc.ru, err = NewUpdater(&Config{})
			}
			if err != nil {
				t.Fatalf("error initializing RegexUpdater: %v", err)
			}
			tc.ru.repo = tc.r
			tc.ru.feeds = map[string]feed.Feed{testFeedName: tc.f}
			if tc.f2 != nil {
				tc.ru.feeds[testSecondaryFeed] = tc.f2
			}

			err = tc.ru.Process(&tc.u, log.WithField("test", name))
			if tc.wantError && err == nil {
				t.Error("expected an error")
			} else if !tc.wantError && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}

			tc.r.assert(t)
		})
	}
}
