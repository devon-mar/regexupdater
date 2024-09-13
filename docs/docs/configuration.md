# Configuration

Regex updater uses a YAML configuration file.

`[...]` indicate that a parameter is optional.

```yaml
---
repository:
  # The type of repository.
  # See the repositories section.
  type: <string>
  # Repository configuration options should follow.

# A dictionary of the (arbitrary) feed name and feed configuration.
feeds:
  # Feed name
  github:
    # The type of update feed.
    # See the Feeds section for options.
    type: <string>
    # Feed configuration options should follow.
      # This configuration is specific to the repository specified above.

[ templates: <template_config> ]

updates:
    # The name of the update.
  - name: <string>
    # The path to the file.
    path: <string>
    # The regular expression to use to extract the version in the above file.
    # It should have exactly one capture group which should capture the version.
    regex: <regex>
    feed:
      # The name of the feed. This is not the feed type.
      name: <string>
      # The feed update config should follow.
      # This configuration is specific to the feed specified above.
    # Set to true to skip parsing the version as a semantic version.
    [ is_not_semver: <bool> | default = false ]
    # Use the semantic version when replacing the version in the file.
    #
    # For example, if the feed returns version "1.0", "1.0.0" will be used in the file.
    [ use_semver: <bool> | default = false ]
    # Skip versions that can't be parsed as a semantic version when use_semver=True.
    [ skip_unparsable: <bool> | default = false ]
    # Replace the version returned by the feed before parsing as a semantic version.
    # The replaced text will also be used when updating the file.
    [ pre_replace: <replace_config> ]
    # A second feed to check for the version returned by the primary feed.
    # If the version does not exist in the secondary feed, the file will not be updated.
    [ secondary_feed: <secondary_feed> ]
    # The action to take if an existing PR for an older version is open when a new version is available..
    # Options:
    #   stop: Don't create a new PR, leave the old one open.
    #   close: Close the old PR and leave a comment with a link to the new one.
    #   ignore: Ignore the old PR and leave it open while creating a new PR.
    [ existing_pr: <string> | default = ignore ]
    # Consider version with a "prerelease" field in the semantic version.
    [ prerelease: <bool> | default = false ]
```

## `<replace_config>`
```yaml
find: <regex>
# Replace may use capture groups from `find`.
replace: <string>
```

## `<template_config>`

Regex Updater uses the [text/template](https://pkg.go.dev/text/template) package.

```yaml
[ pr_title: <template> ]
[ pr_body: <template> ]
[ commit_msg: <template> ]
[ branch: <template> ]
```

The following data is available for use in each template:

- `Name` The name of the update
- `URL` The url to the release, if any.
- `Old` The old version. [(`version` struct)](#version-struct)
- `New` The new version. [(`version` struct)](#version-struct)
- `ReleaseNotes` Release notes.

## `version` struct
The `String()` method will return the semantic version string if not nil or fallback to the raw version.

Additionally, the struct has the following fields:

- `.V` The raw version string.
- `.SV` The [`semver.Version`](https://pkg.go.dev/github.com/Masterminds/semver/v3).

    !!! warning

        `.SV` may be `nil`!
