package repository

import (
	"fmt"
	"strings"

	"github.com/devon-mar/regexupdater/utils/envtag"
	"github.com/go-playground/validator/v10"
	"github.com/mitchellh/mapstructure"
)

const (
	configStructTag = "cfg"
)

type Repository interface {
	GetFile(path string) (File, error)
	FindPR(string) (PullRequest, error)
	UpdateFilePR(path string, oldSHA string, newContent []byte, commitMsg string, newBranch string, prTitle string, prBody string) (prID string, err error)
	// Implementations can assume that pr is their own PR type.
	ClosePR(pr PullRequest) error
	AddPRComment(pr PullRequest, body string) error
	UpdatePRFile(pr PullRequest, path string, oldSHA string, newContent []byte, commitMsg string) error
	// Return the name of the deleted branch.
	DeletePRBranch(prID string) (string, error)
}

type File interface {
	// May return nil.
	Content() []byte
	Path() string
	SHA() string
}

type PullRequest interface {
	ID() string
	Body() string
	IsOpen() bool
	IsMergeable() bool
}

func NewRepository(typ string, cfg map[string]interface{}) (Repository, error) {
	r, err := getRepository(typ, cfg)
	if err != nil {
		return nil, err
	}

	if v, ok := r.(interface{ init() error }); ok {
		if err := v.init(); err != nil {
			return nil, fmt.Errorf("error initializing repository: %w", err)
		}
	}

	return r, nil
}

func Validate(typ string, cfg map[string]interface{}) error {
	_, err := getRepository(typ, cfg)
	return err
}

func getRepository(typ string, cfg map[string]interface{}) (Repository, error) {
	var r Repository
	switch typ {
	case typeGitHub:
		r = &GitHub{}
	case typeGitea:
		r = &Gitea{}
	default:
		return nil, fmt.Errorf("unsupported repository type %q", typ)
	}

	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{Result: r, ErrorUnused: true, TagName: configStructTag})
	if err != nil {
		return nil, fmt.Errorf("error initializing config decoder: %w", err)
	}
	if err = decoder.Decode(cfg); err != nil {
		return nil, err
	}

	envtag.Unmarshal("cfg", "REPOSITORY_"+strings.ToUpper(typ)+"_", r)

	if err := validator.New().Struct(r); err != nil {
		return nil, fmt.Errorf("error validating %s repository config: %w", typ, err)
	}

	return r, nil
}
