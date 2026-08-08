# aspgen

First version of a Go template generator for ASP.NET Core Clean Architecture APIs and Prism/DryIoc WPF applications.

## Build and run

Generation is a two-step workflow: run `new` once to create the project, solution, source tree, and `.aspgen/manifest.json`; then run `add` commands against that generated project. An `add` command cannot initialize a missing project directory.

```text
go run ./cmd/aspgen new MyApp --context Catalog --arch ar --output ./MyApp
go run ./cmd/aspgen add entity Customer name:string age:int active:bool --context Catalog --project ./MyApp
go run ./cmd/aspgen add database postgres --project ./MyApp
```

If you run an `add` command without `--project`, aspgen searches the current directory and its parents for `.aspgen/manifest.json`. The explicit form is recommended in scripts and from outside the project tree.

Run `aspgen --help`, `aspgen new --help`, or `aspgen add --help` for a full flag reference. Every flag accepts three equivalent forms: `--flag value`, `--flag:value`, and `-flag:value`.

See [doc/DEVOPS.md](doc/DEVOPS.md) for how CI and tagged releases work.

## Context/arch engine

Each bounded context picks its own architecture tier independently, so one solution can mix e.g. a simple `ar` context alongside an event-sourced `es` context:

```text
go run ./cmd/aspgen new Accounting --context Billing --arch ar --output ./Accounting
go run ./cmd/aspgen new Accounting --context Billing --arch dm --output ./Accounting
go run ./cmd/aspgen new Billing --context Sales --arch cqrs --output ./Billing
go run ./cmd/aspgen new Billing --context Sales --arch es --output ./Billing
go run ./cmd/aspgen add context Catalog --arch dm --project ./Accounting
```

The tiers form an ordinal ladder, each a superset of the previous tier's concepts:

```text
ar    Active Record: flat entity, direct DbContext CRUD.
dm    Domain Model: aggregate root + value objects + domain services + repository + events,
      synchronous Application-layer CRUD service. No host project (class libraries only).
cqrs  dm's Domain layer + vertical-slice Application (Command/Query/Handler/Validator per
      verb) + a WebApi Minimal API host.
es    cqrs tier + an append-only event store, event-sourced aggregates (rehydrated by
      replaying events), and read-model projections.
```

`ar` and `dm` contexts use `add entity`/`add aggregate` respectively; `dm`, `cqrs`, and `es` contexts all support `add aggregate`/`add value-object`/`add domain-service`/`add repository`/`add event` (`add repository` is rejected for `es`-tier aggregates, which already have a generated event-store repository):

```text
go run ./cmd/aspgen add entity Product name:string price:decimal --context Catalog --project ./Accounting
go run ./cmd/aspgen add aggregate Order number:string total:decimal --context Sales --project ./Billing
go run ./cmd/aspgen add repository OrderRepository --aggregate Order --context Sales --project ./Billing
```

A UI attaches to the whole project (one UI surfaces every context), via `-ui` on `new` or `add ui --framework` afterward. `spa` and `blazor` are implemented for `cqrs`/`es`-tier contexts (which have a WebApi host, called over HTTP). `wpf` works on `cqrs`/`es` (HTTP, same as blazor) **or** `dm` (headless - no WebApi host - so the Desktop app calls each aggregate's CrudService directly, in-process, via DryIoc). `mvc` is `dm`-tier only, also in-process. `spa` wires OpenAPI/Scalar discovery and a permissive local-dev CORS policy onto the host, without scaffolding an actual frontend project. `wpf`, `blazor`, and `mvc` each scaffold a full list/edit/details CRUD screen for every aggregate already in the project, and keep generating a screen automatically for every aggregate added afterward:

```text
go run ./cmd/aspgen new Billing --context Sales --arch cqrs -ui spa --output ./Billing
go run ./cmd/aspgen add ui spa --framework spa --project ./Billing

go run ./cmd/aspgen new Billing --context Sales --arch cqrs -ui wpf --output ./Billing
go run ./cmd/aspgen add ui wpf --framework wpf --project ./Billing

go run ./cmd/aspgen new Billing --context Sales --arch cqrs -ui blazor --output ./Billing
go run ./cmd/aspgen add ui blazor --framework blazor --project ./Billing

go run ./cmd/aspgen new Accounting --context Billing --arch dm -ui mvc --output ./Accounting
go run ./cmd/aspgen add ui mvc --framework mvc --project ./Accounting

go run ./cmd/aspgen new Accounting --context Billing --arch dm -ui wpf --theme wpfui --output ./Accounting
go run ./cmd/aspgen add ui wpf --framework wpf --project ./Accounting
```

`--database sqlite|postgres` works unchanged under this engine (`dm`-tier projects are class libraries with no host attaching the provider yet). Run `aspgen new --help`/`aspgen add --help` for the full context/arch flag and kind reference.

Every `new --context/--arch` project also generates `tests\{Project}.UnitTests` (a DbContext smoke test against an in-memory provider) and, for `ar`/`cqrs`/`es` (any tier with a WebApi host), `tests\{Project}.IntegrationTests` (a `WebApplicationFactory<Program>` smoke test hitting the root endpoint); `dm` is headless so it only gets `UnitTests`. Pass `--no-tests` to skip generating these:

```text
go run ./cmd/aspgen new Accounting --context Billing --arch cqrs --no-tests --output ./Accounting
```

Every `new --context/--arch` project also generates `scripts\ci.ps1`, a local CI/production build
driver (restore, build, test, and optionally publish the WebApi host) — see the generated
project's README for usage:

```powershell
.\scripts\ci.ps1              # restore, build, test
.\scripts\ci.ps1 -Publish     # ...and publish the WebApi host (skipped for headless dm-tier)
.\scripts\ci.ps1 -SkipTests -Configuration Debug
```

## DDD building blocks: value objects, domain services, repositories, events

`add context`/`add aggregate` (shown above) are joined by four more incremental commands for `dm`/`cqrs`/`es`-tier contexts:

```text
go run ./cmd/aspgen new Catalog --context Catalog --arch dm --output ./Catalog
go run ./cmd/aspgen add aggregate Product name:string price:decimal active:bool published:date --context Catalog --project ./Catalog
go run ./cmd/aspgen add value-object ProductCode value:string --context Catalog --project ./Catalog
go run ./cmd/aspgen add domain-service PricingPolicy --context Catalog --project ./Catalog
go run ./cmd/aspgen add repository ProductRepository --aggregate Product --context Catalog --project ./Catalog
go run ./cmd/aspgen add event ProductPriceChanged productId:long price:decimal --context Catalog --project ./Catalog
```

Aggregate generation creates the domain aggregate root, persistence mapping, and an Application-layer CRUD service (plus, for `cqrs`/`es` tiers, a vertical-slice Command/Query/Handler layer and Minimal API endpoints; for `dm`, a Controller/Views set or WPF module once `-ui mvc`/`-ui wpf` is attached).

DDD building blocks are incremental and context-scoped. Value objects are immutable records, domain services are stateless policies, repository contracts are aggregate-specific and live in the domain layer (`cqrs`-tier also registers the repository with the WebApi host's DI container), and events are immutable completed business facts. Use `--no-crud` on an aggregate when the use case should be modeled explicitly instead of starting with generated CRUD.

Generated CRUD keeps boundaries explicit: the CRUD service exposes immutable `Request` and `View` records, and validators use FluentValidation rules per property type.

Project names may be dotted .NET names such as `Markosoft.Commerce`. In that case aspgen keeps namespaces, project filenames, project references, and solution entries aligned: `Markosoft.Commerce.Domain.csproj`, `Markosoft.Commerce.Application.csproj`, `Markosoft.Commerce.Infrastructure.csproj`, and `Markosoft.Commerce.Desktop.csproj`.

SQLite is the default database for every arch tier. Use `--database postgres` (also accepted as `--database:postgres`) when PostgreSQL is required. The selected provider is recorded in `.aspgen/manifest.json` and is emitted into the generated EF Core project, connection string, and dependency-injection setup.

### Entity relations: FK dropdowns and many-to-many multi-select

The `add entity`/`add aggregate` `name:type` syntax accepts another entity's
name as the type to declare a relation to an already-added entity. The UI then
adapts to the relation automatically — a dropdown that displays the target's
first `string` property (falling back to `Id`) and stores the foreign key id:

```text
go run ./cmd/aspgen add aggregate Customer name:string --context Sales --project ./Billing
go run ./cmd/aspgen add aggregate Order total:decimal customer:Customer --context Sales --project ./Billing
```

- `customer:Customer` — required many-to-one (nullable FK with `customer:Customer?`).
  The `Order` edit form renders a dropdown of customers; the grid and details
  screens show the customer's name instead of the raw id.
- `tags:Tag[]` — many-to-many. aspgen materializes a `PostTag` join aggregate
  (full CRUD + API endpoints of its own) and the `Post` edit form renders a
  checkbox multi-select of tags; saving syncs the join rows (adds new links,
  removes deselected ones). Blazor, WPF, and MVC all render the multi-select,
  and the details screen lists the related names. Many-to-many only applies to
  dm+ aggregates via `add aggregate`; ar-tier `add entity` supports many-to-one
  only.

```text
go run ./cmd/aspgen add aggregate Tag name:string --context Blog --project ./Billing
go run ./cmd/aspgen add aggregate Post title:string tags:Tag[] --context Blog --project ./Billing
```

Both relation kinds are restricted to targets in the same bounded context, and
a relation target must already exist (mirroring natural FK-creation order).
Adding a UI later (`add ui wpf|blazor|mvc`) retrofits the pickers for
pre-existing aggregates from the manifest's recorded relation metadata.

#### Nested relation search

The advanced filters on every list page also search *through* a relation's
target by its display property — e.g. a **"Customer name contains"** box on the
Posts list that filters posts whose related customer's name matches:

```text
go run ./cmd/aspgen add aggregate Post title:string customer:Customer --context Blog --project ./Billing
```

WPF, Blazor, and MVC all render this alongside the exact-id dropdown. dm/cqrs
filter by traversing the aggregate's navigation property; es uses a subquery
into the target's read model.

#### Quick-add on the edit form

Each relation dropdown on the edit form has a **"+"** that lets you create the
related record without leaving the form. Click the dropdown (WPF) or "+" (Blazor)
to type a name, then confirm — the record is created, added to the picker, and
selected. If it needs its own parent first (e.g. adding a Customer that requires
a Region), you get a friendly notice instead of a crash:

```text
go run ./cmd/aspgen add aggregate Region name:string --context Blog --project ./Billing
go run ./cmd/aspgen add aggregate Customer name:string region:Region --context Blog --project ./Billing
```

### Generating entities from an existing database

Instead of hand-typing `name:type` properties, aspgen can scaffold `ar`-tier entities from an existing database schema — a static SQL DDL script — via the `import-db` verb against an already-`new`-ed project. Supported providers are `sqlite`, `postgres`, `sqlserver`, and `mysql`.

```text
go run ./cmd/aspgen new MyApp --context Catalog --arch ar --output ./MyApp
go run ./cmd/aspgen import-db --project ./MyApp --context Catalog --script schema.sql --provider postgres --tables all
```

`--tables` accepts `all` (the default) or a comma-separated list of table names. Each selected table becomes one `ar`-tier entity (its name PascalCased and best-effort singularized) via the same code path `add entity` uses. Primary-key columns are skipped since generated entities already provide them; columns with no known type mapping (e.g. `json`, `blob`) are skipped with a warning rather than failing the whole table.

Single-column foreign keys (`FOREIGN KEY (col) REFERENCES tbl(col)`, inline or table-level) are auto-detected: instead of a scalar column the FK becomes a `nav:Target` relation (nullable if the column is nullable), with tables imported referenced-first so relations resolve. When the referenced table is excluded via `--tables`, the FK column falls back to a scalar instead.

A `schema.sql` backup snapshot of the discovered tables is written at the project root on every run. It's a reference artifact only — aspgen never invokes `dotnet ef`; run `dotnet ef migrations add`/`dotnet ef database update` yourself against the generated entities and `DbContext`. Connection strings are never written to `.aspgen/manifest.json`, `schema.sql`, or any generated file.

Run `aspgen import-db --help` for the full flag reference.

Templates are embedded in the executable. Export and customize them with:

```text
go run ./cmd/aspgen templates export ./my-templates
go run ./cmd/aspgen templates list
go run ./cmd/aspgen new MyApp --context Catalog --arch ar --templates ./my-templates
```

Generated projects contain `.aspgen/manifest.json`, allowing components to be added incrementally without regenerating the whole project.

Generated projects use SDK-style `.csproj` files. Incremental generation also adds explicit safe `<Compile Update>` and `<Page Update>` entries for generated `.cs` and `.xaml` files, so new entities and modules appear in Solution Explorer and remain owned by the correct project. The `.sln` file contains project entries; it is updated whenever a new project, such as an incremental WPF UI, is added.

The WPF target uses current Prism 9 conventions with `Prism.DryIoc`: `PrismApplication`, `IContainerRegistry`, `IContainerProvider`, `IModule`, `ConfigureModuleCatalog`, `ViewModelLocator`, `BindableBase`, `DelegateCommand`, regions, navigation registration, and typed `PubSubEvent` communication.

## Generated UI workflow

All three frontends share the same **list → edit → details** workflow:

- **List page** — search box + advanced filters (including nested relation search), a
  card-styled list/table, per-row **Edit**/**Delete**, double-click (WPF) or row-click
  (Blazor/MVC) for details, pagination, and a **+ Add new** button.
- **Edit page** — the create/edit form (per-type inputs, relation dropdowns with
  quick-add, many-to-many checkboxes) with **Save/Cancel** back to the list.
- **Details page** — read-only record with related names resolved.

WPF views live under `src/Desktop/Modules/{Name}/` as `{Name}View` (list),
`{Name}EditView`, and `{Name}DetailsView`, registered for navigation as
`{Name}List` / `{Name}Edit` / `{Name}Details`.

## WPF architecture

The Desktop shell ships a shared layer under `src/Desktop/Shared/` so every
module stays small and consistent:

- **Base ViewModels** — `ListViewModelBase` (search/filter lifecycle, pagination,
  navigation, delete confirm, reload-on-navigation) and `EditViewModelBase`
  (record loading, form reset, save + many-to-many sync, validation). Each
  module's ViewModel is a thin subclass that only supplies its properties,
  filters, and relations.
- **Design system** — `src/Desktop/Themes/AppStyles.xaml` defines `CardStyle`,
  `CardSecondaryStyle`, `SectionHeaderStyle`, `ListHeaderStyle`, and
  `HeaderBandStyle`, merged into `App.xaml` and referenced from every view, so a
  look change is a one-line edit instead of a change in each module.
- **Navigation** — `IAppNavigationService` wraps `RequestNavigate` behind
  `GoTo(viewName[, id])`, keeping navigation out of the ViewModels.
- **Stores & models** — `I{{ Name }}Store` derives from a generic
  `IListStore<TRow, TCriteria, TPage>`; rows implement `IEntityRow` and page
  results implement `IListPageResult<TRow>`.

WPF apps default to the **wpfui** theme (light mode); pass `--theme:wpfui
--theme-mode:dark` for dark, and `-ui wpf` uses wpfui even when no `--theme` is
given.

## Generated tests

Besides the schema smoke test, every aggregate with relations also generates
tests that exercise the relationship flow end-to-end:

- `tests/{Project}.UnitTests` — `{Aggregate}RelationTests.cs` creates the
  related entities (including transitive prerequisites) and join rows through
  the DbContext, then queries them back by foreign key.
- `tests/{Project}.IntegrationTests` (cqrs/es) — `{Aggregate}RelationApiTests.cs`
  drives the same flow over the WebApi HTTP contract, including the nested
  relation search filter.
- dm-tier MVC projects get a WebMvc integration test that posts the Create form
  (with many-to-many values) through the real controller and verifies the join
  rows.
