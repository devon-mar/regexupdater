# Gitea

## Repository Configuration
```yaml
repository:
  type: gitea
  # Gitea configuration options:

  # The URL to the Gitea instance.
  url: <string>
  # The owner of the repository.
  owner: <string>
  # The name of the repository.
  repo: <string>
  # The base branch to use when creating PRs.
  # Defaults to the default branch of the repo.
  [ base_branch: <string> ]
  # Basic auth credentials.
  [ username: <string> ]
  # Must be used with username.
  [ password: <string> ]
  # Must be specified when not using basic auth.
  [ token: <string> ]
  # The name to use for commits.
  [ committer_name: string> ]
  # The email to use for commits.
  # Required when committer_name is present.
  [ committer_email: <string> ]
  # Labels to apply to created PRs.
  # They must exist beforehand.
  [ labels: [<string>, ...] ]
  # The page size to use when accessing paginated API endpoints.
  [ page_size: <int> | default = 30 ]
```


# Notes
- Gitea does not support editing refs through the API. Therefore, whenever a PR needs to be rebased, a new PR will be created and the old one closed.
