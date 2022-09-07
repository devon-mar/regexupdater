# Gitea

## Feed Configuration
```yaml
type: gitea
# The URL to the Gitea instance.
url: <string>
# Basic auth credentials.
[ username: <string> ]
# Must be used with username.
[ password: <string> ]
# Token auth.
[ token: <string> ]
# The page size to use when accessing paginated endpoints.
[ page_size: <int> | default = 30 ]
# Limit the number of releases returned. Defaults to the page size.
[ limit: <int> ]
```

## Update Configuration
```yaml
owner: <string>
repo: <string>
# Use tags instead of releases.
[ tags: <bool> | default = false ]
[ include_prereleases: <bool> | default = false ]
```
