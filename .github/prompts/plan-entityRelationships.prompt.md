# Plan: Entity relationships (FK dropdowns + many-to-many) across webapi/wpf/Renoir

## TL;DR
Today aspgen's `Property`/`parseProperties`/templates are 100% scalar — there is
no FK/navigation concept anywhere (confirmed: dbschema even discards FK
constraints during SQL parsing). Approved scope is the maximal one: many-to-one
(FK dropdown), auto inverse one-to-many collection display, AND many-to-many
(join entity + multi-select), across --simple, --backend ddd webapi, and
Renoir/blazor, declared inline via `add entity`/`add aggregate` args using the
existing `name:type` syntax (type = another entity's name instead of a scalar
alias). This is genuinely a large feature, so it's broken into 6 independently
verifiable phases (0 = foundation, 1-3 = many-to-one per profile, 4 = inverse
collection display, 5 = many-to-many). Each phase after 0 can ship/PR on its
own; later phases build on Phase 0's manifest + parsing foundation.

Key design trick that keeps blast radius manageable: a many-to-one FK is
modeled as an ordinary synthetic `Property` (`CustomerId: long`/`long?`) so
every existing template that loops `.Properties` (Request/Response, Create
Handler, Repository, Store, seed literals, DataGrid columns, TryBuild
validation) needs **zero changes** — it just sees another scalar property.
Only nav-aware spots (entity/model class, EF configuration, the WPF edit
control for that one property, ViewModel ctor/collections) consult new
`Property.RelationTarget`/`RelationDisplayProperty` fields or the new
`data.Relations []Relation` list.

## Decisions
- **CLI syntax**: user names the *navigation* property, not the FK column,
  e.g. `add entity Order total:decimal customer:Customer` — generates
  `CustomerId` (long, FK) + `Customer` (nav) automatically. Optional relation:
  `customer:Customer?` → `long?` FK. Many-to-many: `add entity Post
  title:string tags:Tag[]` (suffix `[]`, chosen over prefix `[]Tag` or
  `List<Tag>` to avoid shell quoting/glob ambiguity and to mirror the
  existing `?` suffix convention).
- **Referenced entity must already exist** (added via a prior `add entity`/
  `add aggregate`) — validated against a new manifest entity registry. This
  mirrors natural FK-creation order and avoids needing forward references.
- **Inverse one-to-many is never separately declared** — declaring a
  many-to-one always implies the inverse collection on the target; this
  removes the ambiguity between "one-to-many" and "many-to-many" (any
  `Entity[]` the user types is unambiguously many-to-many, requiring a join
  entity).
- **Dropdown display value**: first `string` property of the target entity
  (fallback to `Id`), computed once from manifest metadata when the relation
  is parsed and stored as `Property.RelationDisplayProperty`.
- **Renoir bounded-context rule** (per doc/architecture-theta.md's context
  isolation conventions): many-to-one relations are only allowed when both
  aggregates are in the **same** Context (full nav property + EF `HasOne`).
  Cross-context references are rejected with an actionable error in this
  pass (no ID-only cross-context stub) — can be revisited later if needed.
- **No retroactive regeneration** of already-generated files. Inverse
  collections / multi-select patches on a pre-existing parent entity are
  applied via new marker comments that are added to the base entity/wpf
  templates themselves (Phase 4/5), the same way `// aspgen:entities` already
  sits in `AppDbContext.cs` from `new` time whether or not any entity exists
  yet. No project-upgrade/migration path is introduced or needed.
- **Excluded from this pass**: import-db automatic FK detection (dbschema
  currently discards FK constraints entirely — explicitly deferred per user
  answer), custom/non-`long` primary keys, cross-context Renoir references.

## Phase 0 — Manifest & parsing foundation (blocks all other phases)
1. [internal/generator/manifest.go](internal/generator/manifest.go#L11-L26):
   extend `Manifest` with `Entities []EntityMeta`. Add:
   ```
   type EntityMeta struct { Name, Context string; Properties []Property; Relations []Relation }
   ```
   (reuse the existing `Property` struct as-is — add json tags for
   consistency with `Manifest`'s existing tagged fields, no need for a
   separate metadata type). `Context` is empty for simple/ddd-webapi
   entities, set for Renoir aggregates.
2. New `internal/generator/relations.go`:
   - `type Relation struct { Kind, Name, Target, FKProperty string; Optional bool }` —
     `Kind` is `"many-to-one"` or `"many-to-many"`.
   - Extend `Property` ([types.go](internal/generator/types.go#L11-L16)) with
     `RelationTarget string` and `RelationDisplayProperty string` (both empty
     for plain scalars).
   - `splitRelationArgs(args []string, entities []EntityMeta, context string) (remaining []string, relations []Relation, err error)`:
     scans `name:type` tokens *before* `parseProperties` runs; if `type`
     (after stripping trailing `[]`/`?`) case-matches an existing
     `EntityMeta.Name` (same `Context` for Renoir, or empty context for
     simple/ddd), builds a `Relation` and removes the token from
     `remaining`; otherwise passes the token through unchanged so
     `parseProperties` behaves exactly as today for pure scalars. Returns a
     clear error for unknown target names, duplicate relation names, and
     Renoir cross-context targets.
   - `synthesizeRelationProperty(rel Relation) Property` builds the FK
     property (`{Name}Id`, `long`/`long?`, `RelationTarget`/
     `RelationDisplayProperty` set) to be appended into the normal
     `[]Property` slice returned to callers.
   - `resolveDisplayProperty(target EntityMeta) string`: first `Properties[i].CSharpType == "string"`, else `"Id"`.
3. [internal/generator/render.go](internal/generator/render.go#L13-L25):
   add `Relations []Relation` to the template `data` struct (entity-level
   nav/config-only additions — per-property control choice instead uses the
   new `Property` fields directly so most existing `range .Properties`
   template loops need no changes).
4. Wire into callers: [internal/generator/add_entity.go](internal/generator/add_entity.go)
   (`addEntityCmd`) and [internal/generator/add_ddd.go](internal/generator/add_ddd.go)
   (`addAggregateCmd`) call `splitRelationArgs` before `parseProperties`,
   append the synthesized FK properties, set `d.Relations`, and append a new
   `EntityMeta` to `m.Entities` before `saveManifest`.
5. Unit tests in [internal/generator/generator_test.go](internal/generator/generator_test.go):
   table-driven tests for `splitRelationArgs` (unknown target error, `?`
   optional, `[]` many-to-many detection, Renoir cross-context rejection,
   duplicate relation name).

## Phase 1 — Many-to-one for --simple profile (webapi + wpf-entity)
*Depends on Phase 0.*
1. [internal/templates/files/simple-entity/.../Models/{{ .Name }}.cs.tmpl](internal/templates/files/simple-entity/src/WebApi/Models/{{%20.Name%20}}.cs.tmpl):
   after the existing `range .Properties` loop, add
   `{{ range .Relations }}{{ if eq .Kind "many-to-one" }}public {{ .Target }}? {{ .Name }} { get; set; }{{ end }}{{ end }}`
   nav property.
2. AppDbContext for simple backend (marker `// aspgen:entities` file, per
   [project_markers.go](internal/generator/project_markers.go#L196-L216)):
   add an `OnModelCreating` override (currently absent for --simple) with a
   new `// aspgen:relations` marker; add `updateSimpleDbContextRelations`
   (or extend `updateSimpleDbContext`) to insert a
   `modelBuilder.Entity<Order>().HasOne(x => x.Customer).WithMany().HasForeignKey(x => x.CustomerId);`
   line per many-to-one relation, only emitted when `d.Relations` is
   non-empty (existing entities with no relations get an empty/no-op
   `OnModelCreating`, keeping current generated output byte-identical
   otherwise).
3. `{{ .Name }}Endpoints.cs.tmpl` — verify FK property flows through
   unchanged (it's just another `Property`); add nothing unless the
   Explore-confirmed property loop needs the nav property excluded from
   request binding (nav is `[JsonIgnore]`/not settable via API — only the
   FK id is).
4. WPF: [wpf-entity View.xaml.tmpl](internal/templates/files/wpf-entity/src/Desktop/Modules/{{%20.Name%20}}/Views/{{%20.Name%20}}View.xaml.tmpl)
   per-property edit-control loop — add a new branch checked *before* the
   numeric branch: `{{ if .RelationTarget }}` → plain-theme `<ComboBox
   ItemsSource="{Binding {{ .RelationTarget }}Items}" DisplayMemberPath="{{ .RelationDisplayProperty }}" SelectedValuePath="Id" SelectedValue="{Binding Form.{{ .Name }}, Mode=TwoWay}" />`;
   wpfui-theme `<ui:ComboBox ... />` (mirror existing per-type branch
   pattern/theme conditional already in this file).
5. [wpf-entity ViewModel.cs.tmpl](internal/templates/files/wpf-entity/src/Desktop/Modules/{{%20.Name%20}}/ViewModels/{{%20.Name%20}}ViewModel.cs.tmpl):
   `{{ range .Relations }}` block adds: constructor param
   `I{{ .Target }}Store {{ camel .Target }}Store`, field, property
   `public ObservableCollection<{{ .Target }}Row> {{ .Target }}Items { get; } = []`,
   populate call in ctor/Reload (`{{ .Target }}Items.Clear(); foreach (var r in {{ camel .Target }}Store.GetAll()) {{ .Target }}Items.Add(r);`).
   `{{ .Name }}Module.cs.tmpl` DI registration for the extra store is
   already-established (Prism/DryIoc auto-resolves; existing
   `I{{ .Target }}Store` registration from the target entity's own module
   generation covers it — verify both modules are registered in the same
   container, true for all profiles here).
6. Seed data ([internal/generator/seed.go](internal/generator/seed.go)):
   the synthetic FK `Property` needs a *relation-aware* seed literal instead
   of the generic numeric literal — must reference an already-seeded
   parent's row id (e.g. `1`, cycling through however many parent rows were
   seeded) rather than an arbitrary incrementing number that might not
   exist. Add a branch in `seedLiteral`/`renderSeedBlock`: if
   `property.RelationTarget != ""`, seed with
   `(row % parentSeedCount) + 1` instead of the default numeric formula, and
   ensure the parent entity's seed block always renders *before* the child's
   in `DatabaseSeeder.cs` (parent-must-exist ordering already guaranteed by
   the "target must already exist" rule from Phase 0).
7. Tests: extend [layout_integration_test.go](internal/generator/layout_integration_test.go)
   with a `simple` case that adds `Customer` then `Order total:decimal
   customer:Customer`, asserting `CustomerId` in `Order.cs`, the `HasOne`
   line in `AppDbContext.cs`, and a `ComboBox`/`ui:ComboBox` in
   `OrderView.xaml`.

## Phase 2 — Many-to-one for --backend ddd webapi
*Depends on Phase 0 (parallel with Phase 1/3 once Phase 0 lands).*
1. Domain entity template (webapi-ddd-entity's Domain layer, per Explore
   report — file under `internal/templates/files/entity/src/Domain/Entities/{{ .Name }}.cs.tmpl`):
   add nav property analogous to Phase 1 step 1.
2. Request/Response/Create-Handler templates
   ([webapi-ddd-entity/.../{{ .Name }}Request.cs.tmpl](internal/templates/files/webapi-ddd-entity/src/Application/Features/{{%20.Name%20}}/{{%20.Name%20}}Request.cs.tmpl),
   `{{ .Name }}Response.cs.tmpl`, `Create{{ .Name }}Handler.cs.tmpl`) — no
   changes needed, FK flows through as an ordinary `Property` per the
   synthetic-property trick.
3. **New**: `{{ .Name }}Configuration.cs.tmpl` (IEntityTypeConfiguration<T>)
   under Infrastructure/Persistence, generated only when `d.Relations` is
   non-empty for this entity (keeps current no-relation output/tests
   unchanged), containing the `HasOne/WithMany/HasForeignKey` fluent config.
4. AppDbContext (ddd) — add one-time `modelBuilder.ApplyConfigurationsFromAssembly(typeof(AppDbContext).Assembly)`
   call via a new marker (only inserted the first time any entity
   configuration file is generated; idempotent check like existing marker
   functions in [project_markers.go](internal/generator/project_markers.go)).
5. Repository — no change (GetAll/GetById already generic).
6. Seed — same relation-aware literal as Phase 1 step 6, reused via the
   shared `seedLiteral`/`renderSeedBlock` helpers.
7. Tests: extend `layout_integration_test.go` ddd-webapi case with the same
   Customer/Order sequence; assert `{{ .Name }}Configuration.cs` exists only
   for `Order` (not `Customer`, which has no relations).

## Phase 3 — Many-to-one for Renoir/blazor aggregates
*Depends on Phase 0 (parallel with Phase 1/2).*
1. [add_ddd.go](internal/generator/add_ddd.go) `addAggregateCmd` — call
   `splitRelationArgs` scoped to the aggregate's own `--context`; reject
   (clear error) if target aggregate's `EntityMeta.Context` differs from the
   current one per the Decisions "same-context only" rule.
2. Aggregate template (`{{ .Aggregate }}.cs.tmpl` under
   [renoir-aggregate/.../Aggregates/](internal/templates/files/renoir-aggregate/src/{{%20.Project%20}}.DomainModel/Contexts/{{%20.Context%20}}/Aggregates/{{%20.Aggregate%20}}.cs.tmpl)):
   add nav property in the constructor-assignment + property-declaration
   loops (mirror existing per-property loops shown in the Explore report).
3. `{{ .Aggregate }}Configuration.cs.tmpl` — add `.HasOne(...).WithMany().HasForeignKey(...)`
   inside the existing `Configure()` method, guarded by `{{ range .Relations }}`.
4. Seed / renoir-crud templates — same relation-aware literal treatment as
   Phase 1/2.
5. Tests: add a Renoir case to `layout_integration_test.go` (two aggregates,
   same context, relation between them) + a negative test asserting the
   cross-context rejection error.

## Phase 4 — Auto inverse one-to-many collection display (all 3 profiles)
*Depends on Phases 1-3 having landed for whichever profile is being extended
(can be done per-profile incrementally).*
1. Add a forward-looking marker to the *target* side of every entity/
   aggregate/wpf-entity template, unconditionally at generation time (mirrors
   `// aspgen:entities` in `AppDbContext.cs` existing from `new` time):
   - Model/Domain/Aggregate class: `// aspgen:navigation` marker for extra
     inverse `ICollection<T>` properties.
   - wpf-entity `{{ .Name }}View.xaml.tmpl`: `<!-- aspgen:related --> `
     marker for an appended read-only child `DataGrid`/`ui:DataGrid` block.
   - wpf-entity `{{ .Name }}ViewModel.cs.tmpl`: `// aspgen:relatedStores`
     marker for extra injected child-entity store fields/ctor params/
     population calls.
2. New `internal/generator/project_markers.go` functions (mirror
   `updateModuleCatalog`'s idempotent read-check-inject-write pattern):
   `updateInverseNavigation`, `updateRelatedGrid`, `updateRelatedStore`,
   called from `addEntityCmd`/`addAggregateCmd` immediately after the child
   relation is parsed, targeting the **already-generated parent's** files
   (path derived the same way `add`'s discovery/path helpers already work).
3. Tests: extend the Phase 1-3 relationship tests to also assert
   `CustomerView.xaml` gained a read-only Orders grid and
   `CustomerViewModel.cs` gained an injected `IOrderStore`.

## Phase 5 — Many-to-many (join entity + multi-select UI)
*Depends on Phase 0; largest/most isolated phase — can ship independently
of Phase 4.*
1. `splitRelationArgs` (Phase 0) already detects `Entity[]` args and
   classifies them `Kind: "many-to-many"` — extend it to compute a
   deterministic join entity name `{Declaring}{Target}` (declaring-entity
   name first, no alphabetical sort, since the command that creates it
   names the direction).
2. New template dir `internal/templates/files/join-entity/` (name TBD to
   match sibling conventions) rendering: a minimal join Model/Domain class
   with two FK `long` properties, an EF configuration with composite key or
   surrogate `Id` + two `HasOne/WithMany`, and a `DbSet<T>` marker insertion
   — reuse `project_markers.go`'s existing entity-DbSet marker functions
   rather than inventing new ones.
3. Both sides' WPF View/ViewModel gain a multi-select control (plain WPF:
   `ListBox` with `CheckBox` per item via `ItemContainerStyle`; wpfui: `ui:ListView`
   equivalent) bound to an `ObservableCollection<SelectableItem<TRow>>`
   wrapper (new small generated helper type) hydrated from the join table via
   the injected join-entity store.
4. Seed — join rows seeded after both sides' seed blocks (ordering
   dependency, same principle as Phase 1 step 6).
5. Tests: `layout_integration_test.go` case with `Post title:string
   tags:Tag[]`, asserting the join entity/model/config files, DbSet marker
   entries on both sides, and multi-select controls in both `PostView.xaml`
   and `TagView.xaml`.

## Relevant files (cross-phase)
- `internal/generator/manifest.go` — `Manifest`/new `EntityMeta`, load/save.
- `internal/generator/types.go` — `Property` struct gets `RelationTarget`/`RelationDisplayProperty`.
- `internal/generator/relations.go` — new file, all relation-parsing logic.
- `internal/generator/render.go` — `data` struct gets `Relations []Relation`.
- `internal/generator/add_entity.go`, `add_ddd.go` — wire relation parsing + manifest updates.
- `internal/generator/project_markers.go` — new marker functions for Phase 4/5.
- `internal/generator/seed.go` — relation-aware seed literals.
- `internal/templates/files/simple-entity/**`, `entity/**` (ddd Domain),
  `webapi-ddd-entity/**`, `renoir-aggregate/**`, `wpf-entity/**` — template edits per phase above.
- `internal/generator/generator_test.go`, `layout_integration_test.go` — new tests per phase.
- `doc/architecture-theta.md` — should get a short new section documenting
  the relationship syntax/conventions once implemented (mirrors how Renoir
  conventions are documented there today).

## Verification (per phase, before moving to the next)
1. `go build ./cmd/... ./internal/...` and `go vet ./cmd/... ./internal/...`.
2. `go test ./cmd/... ./internal/...` including new relationship test cases.
3. Manual generation: `go run ./cmd/aspgen new RelDemo --app fullstack
   --simple --theme wpfui --output ./RelDemo`, then `add entity Customer
   name:string`, then `add entity Order total:decimal customer:Customer`,
   inspect generated `Order.cs`/`AppDbContext.cs`/`OrderView.xaml`/
   `OrderViewModel.cs` by hand.
4. Repeat the same manual check for `--backend ddd` and for a Renoir
   (`--app blazor`) two-aggregate-same-context case once their phases land.
5. Run twice (idempotency) — confirm no duplicate marker insertions or
   duplicate manifest entries.
6. `dotnet build` the generated solution if the environment supports it, to
   catch actual C#/EF compile errors the Go-side tests can't see.

## Further Considerations
1. Phase ordering/priority: recommend landing Phase 0 + Phase 1 first (
   highest value, smallest surface, matches most-used --simple profile) and
   treating Phases 2-5 as separate follow-up PRs rather than one giant
   change — reduces review risk given the size of this feature. Confirm
   this staged-delivery approach is acceptable, or whether all phases should
   be implemented together in one pass.
2. Renoir cross-context references are rejected outright in this plan; if a
   real use case needs cross-context relations later, that would need an
   ID-only reference convention (store the id, no nav property, resolved via
   the target's repository in the Application layer) as a separate follow-up.
