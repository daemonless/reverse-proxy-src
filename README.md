# reverse-proxy-src

The code of the [daemonless reverse proxy](https://github.com/daemonless/reverse-proxy):
the admin (web UI, API, `reverse-proxy hosts` CLI) and the podman-labels unit.
The container image, its Caddy build and its compose file live in
[daemonless/reverse-proxy](https://github.com/daemonless/reverse-proxy).

## Build

```sh
cd ui && npm ci && npm run build && cd ..
go test ./...
CGO_ENABLED=0 go build -trimpath -o reverse-proxy ./cmd/reverse-proxy
```

The UI is embedded in the binary.

## Releases

A `v*` tag publishes a GitHub release with `reverse-proxy_<version>_<os>_<arch>.tar.gz`
for FreeBSD and Linux (amd64, arm64) and `SHA256SUMS`. A tag with a hyphen
(`v0.4.0-beta.1`) is a pre-release; the image's `beta` variant follows those.

## License

BSD-2-Clause
