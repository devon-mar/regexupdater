# GitHub

## Repository Configuration
```yaml
repository:
  type: github
  # GitHub configuration options:

  # Personal access token. Required if not using app auth.
  [ token: <string> ]
  # Alternative to app_private_key_path. The contents of the app private key.
  # Should be passed as a environment variable instead of directly in the config file.
  [ app_private_key: <string> ]
  # Required when not using a token.
  [ app_private_key_path: <path> ]
  # Required when not using a token.
  [ app_id: <string> ]
  # The GitHub enterprise URL.
  [ enterprise_url: <url> ]
  # Requried with enterprise_url.
  [ enterprise_upload_url: <url> ]
  # The owner of the repository.
  owner: <string>
  # The name of the repository.
  repo: <string>
  # The base branch to use when creating PRs.
  # Defaults to the default branch of the repo.
  [ base_branch: <string> ]
  # Labels to apply to created PRs.
  # They must exist beforehand.
  [ labels: [<string>, ...] ]
  # The name to use for commits.
  [ committer_name: <string> ]
  # The email to use for commits.
  # Required when committer_name is present.
  [ committer_email: <string> ]
```
