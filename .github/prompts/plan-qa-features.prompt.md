# Plan: QA of new feature work (import-db FK, WPF/Blazor/MVC CRUD UI)

## TL;DR
Quality-assure the feature set landed in the last working session before it is
treated as stable. The features under QA:
1. **import-db FK auto-detection** — single-column `FOREIGN KEY` constraints
   become `nav:Target` relations; tables import referenced-first; fallback to
   scalar when the target table is excluded from `--tables`.
2. **WPF edit flows** — dedicated `{Name}EditView` (Save/Cancel), quick-add
   on relation pickers, shared `ListViewModelBase`/`EditViewModelBase`/
   `IListStore`/`IAppNavigationService`, `AppStyles.xaml`.
3. **Blazor run fixes** — UI host on its own port (`ASPGENT_BLAZOR_URL`,
   default 5001) alongside the WebApi on 5000; `<base href="/">`; `FormName`
   on `EditForm` (antiforgery); `ViewDetails` `{id}` interpolation.
4. **Relation/m2m CRUD pickers** — FK dropdowns + multi-select on WPF/Blazor/
   MVC, quick-add, generated relation unit/API/MVC integration tests.
5. **Generated READMEs** — per-UI "Run with the …" sections (WebApi-first for
   blazor/wpf).

Everything below is a checklist; run the Go gate always, run the generated-app
gate after every generator change, and run the manual checks whenever a UI
flow was touched.

## Decisions
- **QA is manual + automated**: Go tests gate generator logic; `dotnet build`+
  `dotnet test` gate generated output; the manual checklist covers the browser
  and WPF interactions automation can't.
- **Scratch apps live under `%TEMP%\aspgen-qa`** (NOT `aspgen-combo`, which a
  Visual Studio instance may hold open and which `makeDemoapp.ps1` deletes).
- **Never regenerate over a locked directory**; delete the target first, and
  `dotnet build-server shutdown` if `.vs`/`bin` locks appear.
- **Stray `Properties\launchSettings.json` breaks the port contract** (hosts
  bind 5000/5001 by design; launchSettings overrides to random ports). Remove
  it if an IDE dropped one into a scratch app before judging a port failure.
- Known, tolerated: es-tier aggregates get no relation tests (event-sourced
  Id=0 concurrency); m2m join EF config logs shadow-FK warnings at startup
  (cosmetic, tests pass); `singularize` is best-effort.

## QA gates

### Gate 1 — Go suite (after any generator/template change)
```powershell
go build ./cmd/... ./internal/...
go vet   ./cmd/... ./internal/...
go test  ./cmd/... ./internal/... -count=1
```
Fast loop for one test: `go test ./internal/generator/ -run <TestName> -v`.
`./scripts/ci.ps1` runs build+vet+test(-race)+goreleaser check in one shot.

### Gate 2 — Generated-app build/test (after any generator/template change)
```powershell
go build -o aspgen.exe ./cmd/aspgen
powershell -ExecutionPolicy Bypass -File scripts\makeDemoapp.ps1   # A–G, exit codes + picker presence
```
Then `dotnet build` + `dotnet test` on each A–F solution (G is negatives).
Expect: 0 errors, all tests pass, all six combos green.

## Feature QA checklists

### F1 — import-db FK detection
Generate a scratch ar project and import a schema with FK chains:
```powershell
$exe = ".\aspgen.exe"; $root = Join-Path $env:TEMP "aspgen-qa"
& $exe new FkQa --context Catalog --arch ar --output "$root\FkQa"
& $exe import-db --project "$root\FkQa" --context Catalog --script schema.sql --provider sqlite
```
`schema.sql` fixture: `Customers` <- `Orders.CustomerId` (NOT NULL),
`Orders.ManagerId` (NULL) -> `Employees`, plus a table whose target is
excluded by `--tables` (FK must fall back to a scalar).
- [ ] Order.cs has `public long CustomerId { get; set; }` + `Customer` nav;
  `public long? ManagerId` + `Employee` nav
- [ ] Orders imported after Customers/Employees (targets exist first)
- [ ] `--tables Orders` only: `customer_id` falls back to a scalar, no nav
- [ ] `schema.sql` backup contains `REFERENCES Customers`
- [ ] `dotnet build` + `dotnet test` on FkQa pass
- [ ] join-table (composite PK of FKs) → two plain relations, no crash

### F2 — WPF edit + quick-add flows
Generate `B-cqrs-wpf`-style app (Customer, Tag, then Post with
`customer:Customer` + `tags:Tag[]`); `dotnet build` + `dotnet test`.
- [ ] `{Name}EditView.xaml` exists per module; list has an Edit button that
      navigates to it; Save persists, Cancel returns
- [ ] Relation dropdown renders options; quick-add "+" expands, typing +
      Enter commits and selects the new row
- [ ] m2m multi-select list shows all target rows, persists selection
- [ ] `IAppNavigationService`/`AppStyles.xaml` compile; no WPF-UI
      `INavigationService` clash
- [ ] Run the app (WPF, needs WebApi on 5000 first) — no `FormatException`
      on empty FK, no NRE on reset

### F3 — Blazor run + antiforgery + assets
Generate a `cqrs -ui blazor` app (Tag + Post `tags:Tag[]`); build + test;
start WebApi (5000) then AppBlazor (5001).
- [ ] `dotnet run --project src\WebApi` binds 5000; `src\C.AppBlazor` binds
      5001 (defaults; `ASPGENT_BLAZOR_URL`/`ASPGENT_API_URL` override)
- [ ] No port collision when both are up (earlier failure mode)
- [ ] `/_framework/blazor.web.js` loads on sub-routes (`/blog/tags`, not just
      `/`) — `<base href="/">` present
- [ ] Create + Edit POSTs succeed (no "which form is being submitted" error) —
      `FormName="{Aggregate}Edit"` on the EditForm
- [ ] Row click navigates to `/blog/tags/{id}` (interpolated, not literal)
- [ ] m2m sync on save: link rows created/deleted in PostTag correctly
- [ ] README has a "Run with the Blazor UI" section (WebApi-first)

### F4 — MVC / dm flows
Generate `dm -ui mvc` (Tag + Post `tags:Tag[]`); build + test.
- [ ] `dotnet run --project src\D.WebMvc` boots in-process (no WebApi needed)
- [ ] Create form POST binds `selectedTagIds`; validation notice only when
      empty; m2m saved
- [ ] Home redirects to first list; navbar links reachable
- [ ] README has a "Run with the MVC UI" section

### F5 — Retrofit (`add ui`)
Generate headless cqrs/es/dm, `add aggregate` Post with relations, then
`add ui wpf` (E-style). Rebuild + retest.
- [ ] Pickers appear for aggregates that pre-date the UI attach
- [ ] README gains the UI run section after `add ui`

### F6 — Regression sanity
- [ ] `scripts/makeDemoapp.ps1` A–G all pass (including G negatives)
- [ ] Go suite + vet green
- [ ] No stray `*.db*`/`aspgen.exe` in `git status` after QA runs

## Relevant files
- `internal/generator/import_db.go` / `internal/dbschema/script.go` — FK
  parsing + synthesis (F1)
- `internal/templates/files/wpf-entity/**` + `wpf/src/Desktop/Shared/**`,
  `Themes/AppStyles.xaml.tmpl` — WPF flows (F2)
- `internal/templates/files/blazor-context/**` (`Program.cs.tmpl`,
  `App.razor.tmpl`), `blazor-context-crud/**` (`*Edit.razor.tmpl`) — Blazor
  fixes (F3)
- `internal/generator/readme.go` — generated README run sections (F3/F4/F5)
- `internal/generator/m2m_ui_test.go`, `layout_integration_test.go`,
  `import_db*_test.go` — Go-side coverage to extend when a QA check fails
- `scripts/makeDemoapp.ps1` — A–G generator gate (F6)

## Exit criteria
All Go gates + all A–F generated-app build/test green, every manual checkbox
above checked once, and any failing checklist item either fixed (with a
regression test) or explicitly logged as a known limitation in the list above.
