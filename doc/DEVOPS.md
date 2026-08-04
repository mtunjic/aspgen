# DevOps how-to: CI and releases

## CI (every push/PR)

`.github/workflows/ci.yml` runs on every push to `main` and every PR:

- `go build`, `go vet`, `go test -race` on Linux, Windows, and macOS.
- `goreleaser check` + `goreleaser release --snapshot --clean --skip=publish`
  to catch `.goreleaser.yaml` breakage before a real tag is cut.

Nothing to configure — just open a PR and check the Actions tab.

## Cutting a release

Releases are tag-driven. Pushing a `v*` tag on `main` builds and publishes
automatically; there is no manual build/upload step.

```powershell
git checkout main
git pull
git tag v1.2.0
git push origin v1.2.0
```

This triggers `.github/workflows/release.yml`, which:

1. Runs `go test ./cmd/... ./internal/...`.
2. Runs `goreleaser release --clean` (config in `.goreleaser.yaml`), building
   `aspgen` for linux/windows/darwin × amd64/arm64 with the version baked in
   via `-ldflags -X aspgen/internal/generator.Version=<tag>`.
3. Publishes a GitHub Release with the archives, a `checksums.txt`, and an
   auto-generated changelog (commits prefixed `doc:`/`test:`/`chore:` are
   excluded from the changelog, not from the build).

No local `goreleaser` install or `GITHUB_TOKEN` is needed — the workflow uses
the built-in `secrets.GITHUB_TOKEN`.

## Verifying a release locally before tagging

```powershell
go build ./cmd/... ./internal/...
go test ./cmd/... ./internal/...
go run ./cmd/aspgen version   # prints "aspgen dev" locally; a real tag prints "aspgen v1.2.0"
```

If `goreleaser` is installed locally, `goreleaser release --snapshot --clean --skip=publish`
reproduces exactly what CI does, without pushing a tag or touching GitHub.

## Version numbers

`aspgen version` reports `dev` for local/`go run` builds. Only binaries built
by the release workflow (or with the same `-ldflags`) report a real version,
so there's no version string to bump by hand anywhere in the repo — the git
tag *is* the version.
