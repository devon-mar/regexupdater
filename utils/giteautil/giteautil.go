package giteautil

import (
	"net/url"
	"strconv"

	"code.gitea.io/sdk/gitea"
	"github.com/devon-mar/regexupdater/utils/linkhdr"
)

func NextPage(linkHdr string) int {
	links := linkhdr.Parse(linkHdr)

	next := links["next"]
	if next == "" {
		return 0
	}

	parsed, err := url.Parse(next)
	if err != nil {
		return 0
	}
	page, _ := strconv.Atoi(parsed.Query().Get("page"))
	return page
}

type ClientOptions struct {
	URL string `cfg:"url" validate:"required"`
	// Basic Auth
	Username string `cfg:"username"`
	Password string `cfg:"password" validate:"required_with=Username"`
	// Token Auth
	Token string `cfg:"token" validate:"required_without=Username"`
}

func NewClient(opts ClientOptions) (*gitea.Client, error) {
	copts := []gitea.ClientOption{}
	if opts.Token != "" {
		copts = append(copts, gitea.SetToken(opts.Token))
	} else if opts.Username != "" && opts.Password != "" {
		copts = append(copts, gitea.SetBasicAuth(opts.Username, opts.Password))
	}
	return gitea.NewClient(opts.URL, copts...)
}
