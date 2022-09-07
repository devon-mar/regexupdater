---
title: GitHub
---

## Feed Configuration
```yaml
type: github
# The page size to use when accessing paginated endpoints.
[ page_size: <int> | default = 100 ]
# Limit the number of releases returned. Defaults to the page size.
[ limit: <int> ]
# Personal access token.
[ token: <string> ]
# Alternative to app_private_key_path. The contents of the app private key.
# Should be passed as a environment variable instead of directly in the config file.
[ app_private_key: <string> ]
# Required when not using a token.
[ app_private_key_path: <path> ]
# GitHub app auth.
[ app_id: <string> ]
# The GitHub enterprise URL.
[ enterprise_url: <url> ]
# Requried with enterprise_url.
[ enterprise_upload_url: <url> ]
```

## Update Configuration
```yaml
owner: <string>
repo: <string>
# Use tags instead of releases.
[ tags: <bool> | default = false ]
[ include_prereleases: <bool> | default = false ]
```
