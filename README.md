# AssetGoblinLua

Optional Lua 5.1 backend for [AssetGoblin](https://github.com/sbolch/AssetGoblin).
The module provides Lua evaluation, manifest conversion and inheritance. The
host continues to validate manifests/responses and provides HTTP serving, rooted
files, declared-tool execution and caching.

Requires Go 1.27.3+. Lua is embedded through GopherLua; no external Lua installation
or Python interpreter is needed for Lua-only deployments. Install trusted plugins:
the restricted libraries and context deadlines do not provide a hard memory limit.

## Enable in a local AssetGoblin build

Standard AssetGoblin binaries and containers do not include this backend. Add this
file to the AssetGoblin application's main package:

```go
package main

import (
    "assetgoblin/plugin"
    assetgoblinlua "github.com/sybolch/AssetGoblinLua"
)

func init() {
    if err := plugin.RegisterBackend(assetgoblinlua.Backend{}); err != nil {
        panic(err)
    }
}
```

Create a Go workspace outside both checkouts, adding the AssetGoblin and
AssetGoblinLua directories with `go work init <AssetGoblin> <AssetGoblinLua>`.
Build AssetGoblin with `GOWORK=<workspace>/go.work go build .`. The module name above
is the Go import path for this package.
No `replace` directive or runtime dependency is added to the default host module.

Register before calling `plugin.Load` or `plugin.LoadWithPython`. Deploy
`assetgoblin-*.lua` beside the custom executable or use `-plugins-dir`.
Python takes precedence for identical plugin names, and inheritance cannot cross
languages. Lua `extends` executes ancestors in the leaf interpreter; only leaves
register routes/codecs. Editing any ancestor changes the cache revision.

## Lua plugins

```lua
return {
    api_version = 1,
    routes = {{path = "/files/", directory = "assets/files"}},
}
```

Lua callbacks receive request metadata as tables and return response tables using
AssetGoblin's shared plugin API. Base, string, table and math libraries are enabled.
Filesystem/process libraries, package loading, debug access and file loaders are
unavailable. Cancellation interrupts execution while retaining interpreter state.
The host serializes callbacks and closing, and supplies initialization/callback
deadlines. Direct consumers of `Backend.Load` must do the same and call the returned
closer once after callbacks have stopped; failed loads release their own interpreter.

The [examples](examples) include audio, video, QOI and QOA plugins.
Their tool requirements and media behavior match the corresponding AssetGoblin
example catalog. Shared API and deployment details remain in AssetGoblin's
[canonical plugin documentation](https://github.com/sbolch/AssetGoblin/blob/main/docs/plugins.md); this library does not duplicate the serving layer.

## Development

```sh
go build ./...
go test ./... -race
go vet ./...
```

Runtime behavior is covered by Gherkin scenarios in [features.feature](features.feature).
The module is independent of AssetGoblin and has no local module replacements.
