# Container Registry

The container registry feed supports the [Docker Registry HTTP API V2](https://docs.docker.com/registry/spec/api/).
It uses a container's tag as releases.

## Feed Configuration
```yaml
type: container_registry
url: <url>
# The page size to use when fetching tags.
[ page_size: <url> ]
# Bearer token to use.
[ token: <string> ]
# Limit the number of releases returned. Defaults to no limit.
[ limit: <int> ]
```

## Update Configuration
```yaml
repo: <string>
```

## Example
```yaml
feeds:
  docker_hub:
    type: container_registry
    url: https://registry.hub.docker.com
  quay:
    type: container_registry
    url: https://quay.io

updates:
  - name: alpine
    feed:
      name: docker_hub
      repo: library/alpine
  - name: node-exporter
    feed:
      name: quay
      repo: prometheus/node-exporter
```

## Notes
- The tags returned are not ordered. Therefore, the limit should usually be set to 0 so that all tags are considered.
