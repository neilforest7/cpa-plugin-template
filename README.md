# cpa-plugin-template

Starter repository for a [CLIProxyAPI](https://github.com/router-for-me/CLIProxyAPI) plugin:
a request normalizer built as a C-ABI shared library (`.dylib` / `.so` / `.dll`) that the
host loads with `plugins.enabled: true`.

Click **Use this template**, rename, replace the capability, tag `v0.1.0`, then point a
plugin store at the repository. The default example writes configured fields into the
upstream request body (`set_fields`), which is small enough to read in one sitting and real
enough to ship.

## Layout

| file | purpose |
| --- | --- |
| `main.go` | plugin logic: registration, config, request normalization. No cgo. |
| `abi_cgo.go` | the C-ABI shim (`cliproxy_plugin_init` and friends). The only cgo file. |
| `abi_nocgo.go` | stub so `go build`, `go test` and editors work without cgo. |
| `main_test.go` | unit tests for the logic. |
| `scripts/smoke.py` | builds the library and drives it over the C ABI, no CPA install needed. |
| `Makefile` | `build`, `test`, `dist` (archive + checksums for the current platform). |
| `.github/workflows/` | `ci.yml` verifies every push, `release.yml` publishes a `v*` tag. |

## Develop

```bash
go test ./...
python3 scripts/smoke.py                  # build + register + normalize over the C ABI
make dist VERSION=0.1.0                   # dist/<id>_0.1.0_<goos>_<goarch>.zip + checksums.txt
```

A c-shared build needs a C toolchain (`CGO_ENABLED=1`); `make build` and the release
workflow set it for you. `go build` without cgo still compiles the stub so plain test runs
and editors work.

## Rename checklist

1. `pluginID`, `name`, `author`, `repository` in `main.go`.
2. `PLUGIN_ID` in the `Makefile`.
3. `PLUGIN_ID` and the metadata version handling in `.github/workflows/release.yml`.
4. `PLUGIN_ID` in `scripts/smoke.py`.
5. The `module` line in `go.mod` and this README.

`pluginID` must equal the built library file name (`<pluginID>.dylib|so|dll`), the
`plugins.configs.<pluginID>` key and the id you publish in the store registry. The metadata
version is injected from the git tag by the release build.

## Config

```yaml
plugins:
  enabled: true
  dir: "/Users/<you>/.cli-proxy-api/plugins"   # absolute and writable
  configs:
    sample-normalizer:
      enabled: true
      priority: 1
      set_fields:
        service_tier: priority
```

Empty or missing `set_fields` makes the plugin a pass-through.

## Local install

```bash
dir="${PLUGINS_DIR}/$(go env GOOS)/$(go env GOARCH)"
mkdir -p "$dir" && cp dist/sample-normalizer.dylib "$dir/"
```

Restart CLIProxyAPI and the management API lists the plugin. Installs from a plugin store
land in the same tree as `<id>-v<version>.<ext>`.

## Release and store registration

```bash
git tag v0.1.0 && git push origin v0.1.0
```

`release.yml` builds darwin/arm64, darwin/amd64, linux/amd64, linux/arm64 and windows/amd64,
packs `<id>_<version>_<goos>_<goarch>.zip` with the library at the archive root plus
`checksums.txt`, and publishes the GitHub release. Then add one entry to a store registry,
for example
[`cliproxyapi-plugins/registry.json`](https://github.com/neilforest7/cliproxyapi-plugins/blob/main/registry.json):

```json
{
  "id": "sample-normalizer",
  "name": "Sample Normalizer",
  "description": "Writes configured fields into upstream request bodies.",
  "author": "neilforest7",
  "repository": "https://github.com/neilforest7/cpa-plugin-template",
  "tags": ["normalizer"]
}
```

The store reads the repository's latest release, so a new tag is all it takes to ship an
update.

## Other capabilities

The shim stays the same for every capability. To become an executor, interceptor, scheduler,
management resource, or usage observer, change the `capabilities` block in
`pluginRegistration`, handle the matching `pluginabi.Method*` in `handleMethod`, and mirror
the payload structs from the upstream
[`examples/plugin`](https://github.com/router-for-me/CLIProxyAPI/tree/main/examples/plugin)
folder for that capability.

## 中文速览

这是一个 CLIProxyAPI 插件模板：Go 写的 C-ABI 动态库（`.dylib`/`.so`/`.dll`），默认能力是请求
规范化插件，按配置往上游请求体里写字段。改完 `pluginID` 等常量，打 `v0.1.0` tag 就会生成符合
插件商店规范的 zip 与 `checksums.txt`，然后在商店的 `registry.json` 里加一条即可。
逻辑代码在 `main.go`（不依赖 cgo），cgo 只在 `abi_cgo.go` 里。

## License

[MIT](LICENSE)
