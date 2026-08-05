# new-gen-refactorings branch summary

Work done on the `new-gen-refactorings` branch, in addition to the
context/arch generation engine itself.

## Generated-project CI script

Every `aspgen new --context/--arch` project (tiers `ar`/`dm`/`cqrs`/`es`) now
also generates `scripts/ci.ps1`, independent of `--no-tests`:

- New template group: `internal/templates/files/ci/scripts/ci.ps1.tmpl`,
  rendered from `newContextProject` in
  [internal/generator/new_project.go](../internal/generator/new_project.go).
- Discovers the project's `.sln` dynamically and runs
  `dotnet restore` -> `dotnet build` -> `dotnet test`, with an optional
  `-Publish` switch that publishes `src\WebApi\*.csproj` when the tier has a
  WebApi host (skipped gracefully for the headless `dm` tier).
- Documented in the `dm`/`cqrs`/`es` README templates unconditionally, and in
  the shared `simple-webapi` README template only for the `ar` tier (guarded
  by `{{ if eq .Arch "ar" }}`, since the legacy `--app webapi --simple`
  profile shares that template but does not get the script).
- Also documented in the top-level [README.md](../README.md) and
  `aspgen new --help` (`internal/generator/help.go`).
- Verified end-to-end: generated `ar`/`dm`/`cqrs` sample projects and ran the
  rendered `scripts/ci.ps1 -Publish` against real `dotnet` tooling (restore,
  build, test, and publish all succeeded; publish was skipped correctly for
  the headless `dm` sample).

The aspgen repository itself also gained an equivalent
[scripts/ci.ps1](../scripts/ci.ps1) (build/vet/test, optional goreleaser
check/snapshot) — see [doc/DEVOPS.md](DEVOPS.md) for usage.

## Dead-code / duplication cleanup

Ran a `golangci-lint` pass (`unused`, `ineffassign`, `staticcheck`, `unparam`)
over `internal/generator` and fixed what it found:

- Removed a dead `namespace` local variable in `patchEntityField` and an
  unused `namespace` parameter from `patchDDDWebApiFeature` and
  `updateEntityDependencyInjection`.
- Kept the unused `d *data` parameter on `addContextCmd`/`addEntityFieldCmd`
  (required by the `addHandlers` dispatch-table's uniform signature) but
  annotated it with `//nolint:unparam` and a reason instead of deleting it.
- Extracted a legacy `--app`/`--backend` validation condition, repeated 4
  times in `new_project.go`, into `isLegacyDDDTarget(app, backend)`.
- Extracted a `DatabaseSeeder.cs` path-selection block, duplicated verbatim
  in two files, into a shared `databaseSeederPath(project, seedBackend)`
  helper in `internal/generator/seed.go`.
- Converted a few `if/else-if` string-comparison chains into tagged `switch`
  statements per staticcheck's `QF1003` suggestion.

No behavior change intended; `go build`/`go vet`/`go test` (scoped to
`cmd`/`internal`) and a full `golangci-lint run` are clean after these
changes.
