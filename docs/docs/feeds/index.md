# Feeds

## Environment Variables

Secrets such as a feed's token may be configured using environment variables.

Environment variables are uppercase and use the following format: `FEED_<name>_<option>`

For example:
```yaml
feeds:
  github:
    type: github
    # FEED_GITHUB_TOKEN=abc
    token: abc
  othergithub:
    type: github
    # FEED_OTHERGITHUB_TOKEN=abc
    token: abc
```

!!! note

    Environment variables can only be used with string options.
