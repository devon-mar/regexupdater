package regexupdater

import (
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"

	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"
)

type Config struct {
	// This will be set by CLI flag
	DryRun bool `yaml:"-"`

	Repository typeConfig            `yaml:"repository" validate:"required"`
	Updates    []*updateConfig       `yaml:"updates" validate:"required,min=1"`
	Feeds      map[string]typeConfig `yaml:"feeds" validate:"required,min=1"`

	Templates templateConfig
}

type templateConfig struct {
	PRTitle   string `mapstructure:"pr_title"`
	PRBody    string `mapstructure:"pr_body"`
	CommitMsg string `mapstructure:"commit_msg"`
	Branch    string `mapstructure:"branch"`
}

func (c *Config) init() error {
	for _, u := range c.Updates {
		if err := u.init(); err != nil {
			return err
		}
	}
	return nil
}

func (c *Config) validate() error {
	if err := validator.New().Struct(c); err != nil {
		return err
	}
	for _, u := range c.Updates {
		if err := u.validate(c); err != nil {
			return err
		}
	}
	return nil
}

type typeConfig struct {
	Type   string                 `validate:"required"`
	Config map[string]interface{} `validate:"required"`
}

func (tc *typeConfig) UnmarshalYAML(value *yaml.Node) error {
	tmp := struct {
		Type string `yaml:"type"`
	}{}
	if err := value.Decode(&tmp); err != nil {
		return err
	}
	tc.Type = tmp.Type

	tc.Config = map[string]interface{}{}
	// Decode the rest...
	if err := value.Decode(tc.Config); err != nil {
		return err
	}
	delete(tc.Config, "type")

	return nil
}

type Replace struct {
	Find    string `yaml:"find" validate:"required"`
	Replace string `yaml:"replace" validate:"required"`

	// To be populated by init()
	regex *regexp.Regexp
}

func (r *Replace) init() error {
	if r == nil {
		return nil
	}
	var err error
	r.regex, err = regexp.Compile(r.Find)
	return err
}

func (r *Replace) Do(s string) string {
	if r == nil {
		return s
	}
	return r.regex.ReplaceAllString(s, r.Replace)
}

type updateConfig struct {
	Name  string `validate:"required"`
	Path  string `validate:"required"`
	Regex string `validate:"required"`

	Feed updateFeedConfig `validate:"required"`

	IsNotSemver bool `yaml:"is_not_semver"`

	UseSemver bool

	PreReplace    *Replace             `yaml:"pre_replace"`
	SecondaryFeed *SecondaryFeedConfig `yaml:"secondary_feed"`
	ExistingPR    string               `yaml:"existing_pr" validate:"oneof=stop close ignore"`
	Prerelease    bool                 `yaml:"prerelease"`

	// This will be filled in by init()
	mregex *regexp.Regexp
}

type updateFeedConfig struct {
	Name   string                 `validate:"required"`
	Config map[string]interface{} `validate:"required"`
	// The actual config that will be passed to the feed
	feedConfig interface{}
}

func (ufc *updateFeedConfig) UnmarshalYAML(value *yaml.Node) error {
	tmp := struct {
		Name string `yaml:"name"`
	}{}
	// First get the name
	if err := value.Decode(&tmp); err != nil {
		return err
	}
	ufc.Name = tmp.Name

	ufc.Config = map[string]interface{}{}
	// Decode the rest...
	if err := value.Decode(ufc.Config); err != nil {
		return err
	}
	delete(ufc.Config, "name")

	return nil
}

func (ufc *updateFeedConfig) validate(cfg *Config) error {
	if _, ok := cfg.Feeds[ufc.Name]; !ok {
		return fmt.Errorf("feed %q does not exist", ufc.Name)
	}
	return nil
}

func (uc *updateConfig) validate(cfg *Config) error {
	if err := uc.Feed.validate(cfg); err != nil {
		return err
	}

	if len(uc.mregex.SubexpNames()) != 2 {
		return errors.New("the regex must have exactly 1 capture group")
	}

	if err := uc.SecondaryFeed.validate(cfg); err != nil {
		return err
	}
	return nil
}

func (c *updateConfig) init() error {
	var err error
	if c.mregex, err = regexp.Compile("(?m)" + c.Regex); err != nil {
		return err
	}

	if err = c.PreReplace.init(); err != nil {
		return err
	}

	return c.SecondaryFeed.init()
}

type SecondaryFeedConfig struct {
	Feed    *updateFeedConfig `yaml:"feed" validate:"required"`
	Replace *Replace          `yaml:"replace"`
}

func (c *SecondaryFeedConfig) init() error {
	if c == nil {
		return nil
	}
	return c.Replace.init()
}

func (c *SecondaryFeedConfig) validate(cfg *Config) error {
	if c == nil {
		return nil
	}
	return c.Feed.validate(cfg)
}

func ReadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	d := yaml.NewDecoder(f)
	d.KnownFields(true)

	c := &Config{}
	// https://github.com/go-yaml/yaml/issues/639#issuecomment-666935833
	if err := d.Decode(c); err != nil && err != io.EOF {
		return nil, err
	}

	if err = c.init(); err != nil {
		return nil, err
	}

	if err = c.validate(); err != nil {
		return nil, err
	}

	return c, nil
}
