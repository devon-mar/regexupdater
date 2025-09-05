package regexupdater

import (
	"errors"
	"fmt"
)

func (ru *RegexUpdater) CurrentVersion(u *updateConfig) (*version, error) {
	file, err := ru.repo.GetFile(u.Path)
	if err != nil {
		return nil, fmt.Errorf("error retrieving file: %w", err)
	}
	if file == nil {
		return nil, errors.New("file was nil")
	}

	origContent := file.Content()

	match := u.mregex.FindSubmatchIndex(origContent)
	if len(match) != 4 {
		return nil, errors.New("no matches found")
	}

	matchL := match[2]
	matchR := match[3]

	return &version{V: string(origContent[matchL:matchR])}, nil
}
