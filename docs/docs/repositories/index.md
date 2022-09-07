# Repositories

## Environment Variables

Secrets such as a repository's token may be configured using environment variables.

Environment variables are uppercase and use the following format: `REPOSITORY_<type>_<option>`

For example:
```yaml
feeds:
  github:
    type: github_releases
repository:
  type: github
  # REPOSITORY_GITHUB_TOKEN=abc
  token: abc
```
```yaml
repository:
  type: gitea
  # REPOSITORY_GITEA_TOKEN=abc
  token: abc
```

!!! note

    Environment variables can only be used with string options.
