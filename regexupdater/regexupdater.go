package regexupdater

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"strings"
	"text/template"

	"github.com/Masterminds/semver/v3"
	"github.com/devon-mar/regexupdater/feed"
	"github.com/devon-mar/regexupdater/repository"
	log "github.com/sirupsen/logrus"
)

const (
	defaultPRTitle = "Bump {{ .Name }} from {{ .Old }} to {{ .New}}"
	defaultPRBody  = `Bumps {{ .Name }} from {{ .Old }} to {{ .New}}

<details>
<summary>Release notes</summary>
<em>View details <a href="{{ .URL }}">here</a>.</em>
<blockquote>
{{ .ReleaseNotes }}
</blockquote>
</details>
`
	defaultCommitMsg = "Bump {{ .Name }} from {{ .Old }} to {{ .New }}"
	defaultBranch    = "update/{{ .Name }}-{{ .New }}"

	existingPRStop  = "stop"
	existingPRClose = "close"
)

type version struct {
	V  string
	SV *semver.Version
}

func (v version) String() string {
	if v.SV != nil {
		return v.SV.String()
	}
	return v.V
}

type releaseInfo struct {
	version version
	release *feed.Release

	older bool
}

type RegexUpdater struct {
	repo  repository.Repository
	feeds map[string]feed.Feed
	isDry bool

	prTitleTemplate   *template.Template
	prBodyTemplate    *template.Template
	commitMsgTemplate *template.Template
	branchTemplate    *template.Template
}

// Returns a RegexUpdater without calling NewFeed or NewRepository
// (to avoid any side effects).
func newBaseUpdater(config *Config) (*RegexUpdater, error) {
	ru := &RegexUpdater{
		isDry: config.DryRun,
		feeds: make(map[string]feed.Feed, len(config.Feeds)),
	}
	var err error

	ru.prTitleTemplate, err = newTemplate(config.Templates.PRTitle, defaultPRTitle)
	if err != nil {
		return nil, fmt.Errorf("error parsing PR title template: %w", err)
	}

	ru.prBodyTemplate, err = newTemplate(config.Templates.PRBody, defaultPRBody)
	if err != nil {
		return nil, fmt.Errorf("error parsing PR body template: %w", err)
	}

	ru.commitMsgTemplate, err = newTemplate(config.Templates.CommitMsg, defaultCommitMsg)
	if err != nil {
		return nil, fmt.Errorf("error parsing commit message template: %w", err)
	}

	ru.branchTemplate, err = newTemplate(config.Templates.Branch, defaultBranch)
	if err != nil {
		return nil, fmt.Errorf("error parsing branch name template: %w", err)
	}

	return ru, nil
}

func NewUpdater(config *Config) (*RegexUpdater, error) {
	ru, err := newBaseUpdater(config)
	if err != nil {
		return nil, err
	}

	// Empty string should be caught by the config validator. Only for testing.
	if config.Repository.Type != "" {
		ru.repo, err = repository.NewRepository(config.Repository.Type, config.Repository.Config)
		if err != nil {
			return nil, fmt.Errorf("error initializing repository: %w", err)
		}
	}

	for name, cfg := range config.Feeds {
		feed, err := feed.NewFeed(name, cfg.Type, cfg.Config)
		if err != nil {
			return nil, err
		}
		ru.feeds[name] = feed
	}

	for _, u := range config.Updates {
		// validate() already checked that the key is valid
		u.Feed.feedConfig, err = ru.feeds[u.Feed.Name].NewConfig(u.Feed.Config)
		if err != nil {
			return nil, fmt.Errorf("error creating feed config: %w", err)
		}

		if u.SecondaryFeed != nil {
			u.SecondaryFeed.Feed.feedConfig, err = ru.feeds[u.SecondaryFeed.Feed.Name].NewConfig(u.SecondaryFeed.Feed.Config)
			if err != nil {
				return nil, fmt.Errorf("error creating secondary feed config: %w", err)
			}
		}
	}
	return ru, nil
}

func ValidateConfig(c *Config) error {
	_, err := newBaseUpdater(c)
	return err
}

func (ru *RegexUpdater) Process(u *updateConfig, logger *log.Entry) error {
	file, err := ru.repo.GetFile(u.Path)
	if err != nil {
		return fmt.Errorf("Error retrieving file: %w", err)
	}
	if file == nil {
		return errors.New("file was nil")
	}

	origContent := file.Content()

	match := u.mregex.FindSubmatchIndex(origContent)
	if len(match) != 4 {
		return errors.New("No matches found")
	}

	matchL := match[2]
	matchR := match[3]

	currentVer := version{V: string(origContent[matchL:matchR])}

	if !u.IsNotSemver {
		currentVer.SV, err = semver.NewVersion(currentVer.V)
		if err != nil {
			return fmt.Errorf("error parsing %q as a semantic version: %w", currentVer, err)
		}
	}

	updateHash := getUpdateID(u.Name)
	existingPR, err := ru.repo.FindPR(updateHash)
	var closeExistingPR bool
	if err != nil {
		return fmt.Errorf("error searching for existing PR: %w", err)
	}
	var prMeta prMetadata
	if existingPR != nil {
		prMeta = parsePRMeta(existingPR.Body())
	}
	if prMeta.Version == currentVer.String() {
		logger.Infof("Closing existing PR %s (redundant)", existingPR.ID())
		err := ru.repo.AddPRComment(existingPR, fmt.Sprintf("`%s` is already using this version. This PR is no longer necessary.", u.Name))
		if err != nil {
			return fmt.Errorf("error leaving comment on PR %s: %w", existingPR.ID(), err)
		}
		err = ru.repo.ClosePR(existingPR)
		if err != nil {
			return fmt.Errorf("error closing existing PR %s: %w", existingPR.ID(), err)
		}
	}

	newRel, err := ru.findNewRelease(u, currentVer, logger)
	if err != nil {
		return fmt.Errorf("error searching for release: %w", err)
	}

	if newRel == nil {
		logger.Info("Already up to date.")
		return nil
	}

	secondaryHasRel, err := ru.checkSecondaryFeed(u, newRel.version.V)
	if err != nil {
		return fmt.Errorf("error checking secondary feed: %w", err)
	}
	if !secondaryHasRel {
		logger.Warnf("Secondary feed does not have version %q", newRel.version.V)
		return nil
	}

	logger.Infof("Updating from %q to %q", currentVer, newRel.version)

	var replaceWith string
	if u.UseSemver {
		replaceWith = newRel.version.String()
	} else {
		replaceWith = newRel.version.V
	}
	newContent := append(make([]byte, 0, matchL+len(replaceWith)+len(origContent)-matchR), origContent[:matchL]...)
	newContent = append(newContent, []byte(replaceWith)...)
	newContent = append(newContent, origContent[matchR:]...)
	// Make sure that the new content matches the regex
	if len(u.mregex.FindSubmatchIndex(newContent)) != 4 {
		return errors.New("new content did not match the regex")
	}

	data := struct {
		Name         string
		URL          string
		Old          version
		New          version
		ReleaseNotes string
	}{
		Name:         u.Name,
		URL:          newRel.release.URL,
		Old:          currentVer,
		New:          newRel.version,
		ReleaseNotes: newRel.release.ReleaseNotes,
	}

	if existingPR != nil {
		if prMeta.ID != "" && prMeta.Version != "" {
			if prMeta.Version == newRel.version.String() {
				logger.Infof("Found existing PR %s for the same version", existingPR.ID())
				if existingPR.IsOpen() {
					if err := ru.fixIfUnmergeable(file, existingPR, newContent, data, logger); err != nil {
						return fmt.Errorf("error fixing unmergeable PR: %v", err)
					}
				}
				return nil
			} else if !existingPR.IsOpen() {
				// The PR is for a different version but closed.
				// Therefore, we can ignore it.
				logger.Infof("Found closed PR %s for (older) version %s", existingPR.ID(), prMeta.Version)
			} else if u.ExistingPR == existingPRStop {
				logger.Infof("Found PR %s for an older version and the action is STOP.", existingPR.ID())
				return nil
			} else if u.ExistingPR == existingPRClose {
				logger.Infof("Closing PR %s for an older version", existingPR.ID())
				closeExistingPR = true
			}
			// the default action is 'ignore"...
		} else {
			logger.Warnf("Exisiting PR %s metadata is invalid", existingPR.ID())
		}
	}

	newPRID, err := ru.createPR(
		data, file, newContent, prMetadata{ID: getUpdateID(u.Name), Update: u.Name, Version: newRel.version.String()}, logger,
	)
	if err != nil {
		return fmt.Errorf("error creating PR: %w", err)
	}

	if closeExistingPR {
		if err := ru.supersedePR(newPRID, existingPR, logger); err != nil {
			return err
		}
	}
	return nil
}

func (ru *RegexUpdater) findNewRelease(u *updateConfig, currentVer version, logger *log.Entry) (*releaseInfo, error) {
	feed := ru.feeds[u.Feed.Name]

	done := make(chan struct{})
	defer close(done)
	relChan, errChan := feed.GetReleases(u.Feed.feedConfig, done)

	var i int

	for {
		i++

		select {
		case r, ok := <-relChan:
			if !ok {
				// No more...
				return nil, nil
			}
			ri, err := ru.checkRelease(r, currentVer, u, logger)
			if err != nil {
				return nil, err
			}
			if ri == nil {
				// It didn't match some constraint.
				continue
			}
			if ri.older {
				// The release is older. Other releases sent on the
				// channel (should) be lesser so we can stop searching.
				return nil, nil
			} else {
				return ri, nil
			}
		case err, ok := <-errChan:
			if !ok {
				return nil, nil
			}
			return nil, err
		}
	}
}

// Returns the version string and optional semver if the release matches the constraints.
func (ru *RegexUpdater) checkRelease(r *feed.Release, currentVer version, u *updateConfig, logger *log.Entry) (*releaseInfo, error) {
	ri := &releaseInfo{release: r}

	ri.version.V = u.PreReplace.Do(r.Version)

	if u.IsNotSemver {
		if ri.version.V == currentVer.V {
			ri.older = true
			return ri, nil
		}
		// We assume that the release feed is in order...
		return ri, nil
	}

	var err error
	ri.version.SV, err = semver.NewVersion(ri.version.V)
	if err != nil {
		return nil, fmt.Errorf("error parsing %q as a semantic version: %w", ri.version, err)
	}

	if ri.version.SV.Prerelease() != "" && !u.Prerelease {
		logger.Infof("Skipping version %s: is a prerelease", ri.version.SV.String())
		return nil, nil
	}

	cmp := currentVer.SV.Compare(ri.version.SV)

	if cmp == 0 {
		logger.Debugf("%s == %s", ri.version.SV.String(), currentVer.SV.String())
		ri.older = true
	} else if cmp < 0 {
		logger.Debugf("%s > %s", ri.version.SV.String(), currentVer.SV.String())
	} else {
		logger.Debugf("%s < %s", ri.version.SV.String(), currentVer.SV.String())
		ri.older = true
	}
	return ri, nil
}

func templateString(t *template.Template, data any) (string, error) {
	buf := &bytes.Buffer{}
	err := t.Execute(buf, data)
	return buf.String(), err
}

func (ru *RegexUpdater) createPR(data any, file repository.File, newContent []byte, meta prMetadata, logger *log.Entry) (string, error) {
	title, err := templateString(ru.prTitleTemplate, data)
	if err != nil {
		return "", err
	}

	body, err := templateString(ru.prBodyTemplate, data)
	if err != nil {
		return "", err
	}
	body += "\n" + meta.Footer()

	commitMsg, err := templateString(ru.commitMsgTemplate, data)
	if err != nil {
		return "", err
	}
	newBranch, err := templateString(ru.branchTemplate, data)
	if err != nil {
		return "", err
	}

	if ru.isDry {
		logger.Infof("DRY RUN: Creating PR %q, updating file %q", title, file.Path())
		return "", nil
	}

	prID, err := ru.repo.UpdateFilePR(file.Path(), file.SHA(), newContent, commitMsg, newBranch, title, body)
	logger.Infof("Created PR %s", prID)
	return prID, err
}

func (ru *RegexUpdater) supersedePR(newID string, oldPR repository.PullRequest, logger *log.Entry) error {
	if ru.isDry {
		logger.Infof("DRY RUN: Superseding PR %s with %s", oldPR.ID(), newID)
		return nil
	}

	if err := ru.repo.AddPRComment(oldPR, "Superseded by "+newID); err != nil {
		return fmt.Errorf("Error adding comment to old PR %s: %w", oldPR.ID(), err)
	}
	if err := ru.repo.ClosePR(oldPR); err != nil {
		return fmt.Errorf("Error closing old PR %s: %w", oldPR.ID(), err)
	}
	return nil
}

func newTemplate(user string, def string) (*template.Template, error) {
	t := template.New("").Funcs(template.FuncMap{"lower": strings.ToLower, "upper": strings.ToUpper})
	if user != "" {
		return t.Parse(user)
	}
	return t.Parse(def)
}

func getUpdateID(s string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(s)))
}

func (ru *RegexUpdater) fixIfUnmergeable(file repository.File, pr repository.PullRequest, newContent []byte, templateData any, logger *log.Entry) error {
	if pr.IsMergeable() {
		return nil
	}
	logger.Infof("PR %s is unmergeable", pr.ID())
	commitMsg, err := templateString(ru.commitMsgTemplate, templateData)
	if err != nil {
		return fmt.Errorf("error templating commit message: %w", err)
	}
	if err := ru.repo.UpdatePRFile(pr, file.Path(), file.SHA(), newContent, commitMsg); err != nil {
		return err
	}

	logger.Infof("Successfully rebased PR %s", pr.ID())
	return nil
}

func (ru *RegexUpdater) checkSecondaryFeed(u *updateConfig, version string) (bool, error) {
	if u.SecondaryFeed == nil {
		return true, nil
	}
	feed := ru.feeds[u.SecondaryFeed.Feed.Name]
	replaced := u.SecondaryFeed.Replace.Do(version)
	rel, err := feed.GetRelease(replaced, u.SecondaryFeed.Feed.feedConfig)
	return rel != nil, err
}
