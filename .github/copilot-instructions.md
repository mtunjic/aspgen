# aspgen — Copilot instructions

aspgen is a Go CLI that scaffolds ASP.NET Core Clean Architecture APIs and
Prism/DryIoc WPF applications from embedded C#/XAML templates. There is no
runtime web server here — the "backend" is code generation only.

## Build, test, run

Always scope Go commands to real source; `./...` also matches example/asset
`.go` files under `agent/skills/**/assets` and `.claude/skills/**` that don't
compile standalone:

```powershell
go build ./cmd/... ./internal/...
go vet ./cmd/... ./internal/...
go test ./cmd/... ./internal/...
```

Run the CLI directly for manual testing:

```powershell
go run ./cmd/aspgen new MyApp --app fullstack --simple --output ./MyApp
go run ./cmd/aspgen add entity Customer name:string age:int active:bool --project ./MyApp
```

`new` bootstraps a project + `.aspgen/manifest.json`; `add` commands mutate an
existing generated project and require that manifest (found via `--project` or
by searching parent directories).

## Code layout

- [cmd/aspgen/main.go](cmd/aspgen/main.go) — trivial entrypoint, delegates to `generator.Run`.
- [internal/generator/generator.go](internal/generator/generator.go) — tiny dispatcher only
  (`Run`, `usage`); routes to `new`/`add`/`templates`/`version`. The CLI
  logic itself is split by concern across sibling files in
  [internal/generator/](internal/generator):
  [new_project.go](internal/generator/new_project.go) (`new` subcommand),
  [add.go](internal/generator/add.go) (`add` dispatcher) plus
  [add_entity.go](internal/generator/add_entity.go),
  [add_module.go](internal/generator/add_module.go),
  [add_webapi.go](internal/generator/add_webapi.go), and
  [add_ddd.go](internal/generator/add_ddd.go) (context/aggregate/
  value-object/domain-service/repository/event) for the individual `add`
  kinds, [flags.go](internal/generator/flags.go) (flag parsing),
  [manifest.go](internal/generator/manifest.go) (manifest read/write +
  project discovery), [project_files.go](internal/generator/project_files.go)
  (`.csproj` lookup/update), [project_markers.go](internal/generator/project_markers.go)
  (marker-comment injection like `// aspgen:services`),
  [render.go](internal/generator/render.go) (template rendering),
  [solution.go](internal/generator/solution.go) (`.sln` generation),
  [seed.go](internal/generator/seed.go) (dummy seed data), and
  [types.go](internal/generator/types.go) (shared types, `parseProperties`).
  When adding a subcommand or entity kind, find the closest existing sibling
  (e.g. `add entity` vs `add aggregate`) and mirror its pattern rather than
  inventing a new one.
- [internal/templates/templates.go](internal/templates/templates.go) — `//go:embed files`
  exposing the template tree as an `embed.FS`.
- [internal/templates/files/](internal/templates/files) — actual Go `text/template` source for
  generated C#/XAML/project files, one subdirectory per generation kind
  (`entity`, `module`, `service`, `database`, `webapi`, `wpf`, `renoir-*`,
  `seed-*`, etc.). Editing generated output means editing templates here, not
  generator.go, unless the change is about *which* files get rendered.
- [internal/generator/generator_test.go](internal/generator/generator_test.go) and
  [internal/generator/layout_integration_test.go](internal/generator/layout_integration_test.go) — unit tests for
  parsing/helpers plus integration tests that actually run generation into a
  temp dir and assert on the resulting file tree/content.

## Conventions specific to this repo

- Application targets: `webapi`, `wpf`, `blazor`, `fullstack`. Profiles:
  `--simple` (single-project Active Record CRUD, no DDD) vs `--backend ddd`
  (Domain/Application/Infrastructure/WebApi CQRS layers) — these two flags are
  mutually exclusive; check for that validation when touching flag parsing.
  `blazor` is the separate Renoir-style DDD profile driven by `add context /
  aggregate / value-object / domain-service / repository / event`.
  See the [README.md](README.md) for the full flag/profile matrix.
- Flags accept three forms: `--flag value`, `--flag:value`, `-flag:value` —
  preserve all three when adding new flags (see `seedOption` in
  generator.go for the parsing pattern).
- Property args are `name:type` pairs parsed by `parseProperties`; supported
  C# types are the fixed set in `mapType`. Unknown types must error, not
  silently pass through.
- Dotted .NET project names (e.g. `Markosoft.Commerce`) must stay consistent
  across namespace, `.csproj` filenames, project references, and `.sln`
  entries — don't special-case a "simple name" path that breaks this.
- Incremental `add` commands must produce the same `<Compile Update>` /
  `<Page Update>` and manifest entries a fresh `new` would produce, so
  Solution Explorer and repeated `add` runs stay idempotent-looking.

## Go style skills

This repo ships Go style/review skills under `agent/skills/go-*` (mirrored in
`.claude/skills`). Before finishing non-trivial Go changes, consult the
relevant skill (e.g. `go-error-handling`, `go-testing`, `go-code-review`) via
its `SKILL.md` rather than guessing at conventions.
