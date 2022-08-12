package githubutil

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/bradleyfalzon/ghinstallation/v2"
	"github.com/google/go-github/v45/github"
	"golang.org/x/oauth2"
)

type GitHubOptions struct {
	Token               string `cfg:"token"`
	AppPrivateKey       string `cfg:"app_private_key" validate:"omitempty,required_without=Token"`
	AppPrivateKeyPath   string `cfg:"app_private_key_path" validate:"omitempty,file,required_without=Token"`
	AppID               int64  `cfg:"app_id" validate:"omitempty,required_with=AppPrivateKey"`
	AppInstallationID   int64  `cfg:"app_installation_id"`
	EnterpriseURL       string `cfg:"enterprise_url" validate:"omitempty,url"`
	EnterpriseUploadURL string `cfg:"enterprise_upload_url" validate:"omitempty,url,required_with=EnterpriseURL"`

	Owner string `cfg:"-"`
	Repo  string `cfg:"-"`
}

func NewGitHub(opts *GitHubOptions) (*github.Client, *github.Client, string, error) {
	var httpClient *http.Client
	var appTransport *ghinstallation.AppsTransport
	var err error

	if opts.AppID != 0 {
		if opts.AppPrivateKey != "" {
			appTransport, err = ghinstallation.NewAppsTransport(http.DefaultTransport, opts.AppID, []byte(opts.AppPrivateKey))
		} else {
			appTransport, err = ghinstallation.NewAppsTransportKeyFromFile(http.DefaultTransport, opts.AppID, opts.AppPrivateKeyPath)
		}
		if err != nil {
			return nil, nil, "", err
		}
		httpClient = &http.Client{Transport: appTransport}
	} else if opts.Token != "" {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: opts.Token})
		httpClient = oauth2.NewClient(context.Background(), ts)
	}
	// else - no auth

	var appClient *github.Client
	var installClient *github.Client
	var appSlug string

	if opts.EnterpriseURL != "" && opts.EnterpriseUploadURL != "" {
		appClient, err = github.NewEnterpriseClient(opts.EnterpriseURL, opts.EnterpriseUploadURL, httpClient)
	} else {
		appClient, err = github.NewClient(httpClient), nil
	}
	if err != nil {
		return nil, nil, "", err
	}

	if appTransport != nil && opts.Owner != "" && opts.Repo != "" {
		install, _, err := appClient.Apps.FindRepositoryInstallation(context.Background(), opts.Owner, opts.Repo)
		if err != nil {
			return nil, nil, "", fmt.Errorf("error getting install ID: %v", err)
		}
		if install.ID == nil {
			return nil, nil, "", errors.New("app id is nil")
		}
		if install.AppSlug == nil {
			return nil, nil, "", errors.New("app slug is nil")
		}
		appSlug = *install.AppSlug
		installHttp := &http.Client{Transport: ghinstallation.NewFromAppsTransport(appTransport, *install.ID)}
		if opts.EnterpriseURL != "" && opts.EnterpriseUploadURL != "" {
			installClient, err = github.NewEnterpriseClient(opts.EnterpriseURL, opts.EnterpriseUploadURL, installHttp)
		} else {
			installClient, err = github.NewClient(installHttp), nil
		}
		if err != nil {
			return nil, nil, "", err
		}
	}
	return appClient, installClient, appSlug, nil
}
