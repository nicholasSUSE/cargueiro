# Development environment

Snippets to test and debug cargueiro in VSCode with the tools that mise pins. `.vscode/` is gitignored, so each developer creates these files locally. Setup of mise itself is in [`DEVELOPING.md`](DEVELOPING.md#setup).

## Tests and linting: `.vscode/settings.json`

```json
{
  "go.buildTags": "e2e",
  "go.lintTool": "golangci-lint-v2",
  "go.testEnvVars": {
    "PATH": "<home>/.local/share/mise/shims:/usr/local/bin:/usr/bin:/bin:${env:PATH}"
  },
  "go.toolsEnvVars": {
    "PATH": "<home>/.local/share/mise/shims:/usr/local/bin:/usr/bin:/bin:${env:PATH}"
  }
}
```

Replace `<home>` with your home directory. Each setting does one thing:

- `go.buildTags` lets the editor see the e2e tests. Without it, running one fails with `build constraints exclude all Go files`, because the e2e files carry the `e2e` build tag.
- `go.lintTool` makes the editor use the repository's linter and its `.golangci.yml`.
- `go.testEnvVars` sets the `PATH` of the tests run from the editor. The mise shims come first, so the pinned `helm` wins over any other one, such as a Homebrew copy; otherwise an e2e test fails with `helm CLI on PATH differs from the helm SDK in go.mod`.
- `go.toolsEnvVars` sets the `PATH` of the tools the editor runs. The Go extension installs its own tools into Go's bin directory, and an old `golangci-lint` there can come first. It rejects the repository's configuration, and the editor shows only a package header with the `go-golangci-lint-v2` source. With the shims first, the pinned one runs. Reload the window after changing it.

The system directories after the shims keep `git` and the other system tools findable even when `${env:PATH}` is empty in the editor. Without them, the tests that need `git` fail with `git is not installed`.

Starting VSCode from a shell where mise is activated, for example `code .`, works too, and then the `PATH` lines are not needed.

The e2e tests that use the local registries need `make registry-up` before they are run from the editor, and `make registry-down` after.

## Debugging: `.vscode/launch.json`

Requires [Delve](https://github.com/go-delve/delve), which mise pins.

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "cargueiro version",
      "type": "go",
      "request": "launch",
      "mode": "auto",
      "program": "${workspaceFolder}",
      "args": ["version"],
      "cwd": "${workspaceFolder}"
    },
    {
      "name": "cargueiro version (--debug)",
      "type": "go",
      "request": "launch",
      "mode": "auto",
      "program": "${workspaceFolder}",
      "args": ["--debug", "version"],
      "cwd": "${workspaceFolder}"
    },
    {
      "name": "Test current package",
      "type": "go",
      "request": "launch",
      "mode": "test",
      "program": "${fileDirname}"
    }
  ]
}
```

Open `cargueiro/` as the workspace root. Change `args` to debug other commands. Debug runs skip the Makefile `-ldflags`, so the version output shows `dev`.
