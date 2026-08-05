# aspgen

First version of a Go template generator for ASP.NET Core Clean Architecture APIs and Prism/DryIoc WPF applications.

## Build and run

Generation is a two-step workflow: run `new` once to create the project, solution, source tree, and `.aspgen/manifest.json`; then run `add` commands against that generated project. An `add` command cannot initialize a missing project directory.

```text
go run ./cmd/aspgen new MyApp --app fullstack --simple --output ./MyApp
go run ./cmd/aspgen add entity Customer name:string age:int active:bool --project ./MyApp
go run ./cmd/aspgen add module Customers --project ./MyApp
go run ./cmd/aspgen add database postgres --project ./MyApp
go run ./cmd/aspgen add service Email --project ./MyApp
go run ./cmd/aspgen add feature CreateCustomer name:string age:int active:bool --project ./MyApp
```

If you run an `add` command without `--project`, aspgen searches the current directory and its parents for `.aspgen/manifest.json`. The explicit form is recommended in scripts and from outside the project tree.

Run `aspgen --help`, `aspgen new --help`, or `aspgen add --help` for a full flag reference. Every flag accepts three equivalent forms: `--flag value`, `--flag:value`, and `-flag:value`.

See [doc/DEVOPS.md](doc/DEVOPS.md) for how CI and tagged releases work.

## Context/arch engine (recommended)

The recommended way to start a new backend is `--context`/`--arch`, not `--app`/`--backend`/`--simple` (see "Legacy `--app`/`--backend` workflow" below for the older, still-supported path). Each bounded context picks its own architecture tier independently, so one solution can mix e.g. a simple `ar` context alongside an event-sourced `es` context:

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

A UI attaches to the whole project (one UI surfaces every context), via `-ui` on `new` or `add ui --framework` afterward. Only `spa` is currently implemented, and only for `cqrs`/`es`-tier contexts (which have a WebApi host); it wires OpenAPI/Scalar discovery and a permissive local-dev CORS policy onto the host, without scaffolding an actual frontend project:

```text
go run ./cmd/aspgen new Billing --context Sales --arch cqrs -ui spa --output ./Billing
go run ./cmd/aspgen add ui spa --framework spa --project ./Billing
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

## Legacy `--app`/`--backend` workflow

The sections below describe the original `--app webapi|wpf|blazor|fullstack` + `--backend ddd`/`--simple`/`--theme` workflow. It remains fully supported (no behavior changes), but is no longer the recommended starting point for new projects — prefer `--context`/`--arch` above.

For a WPF or fullstack project, add the WPF-UI Fluent theme with `--theme wpfui` (also accepted as `--theme:wpfui` or `-theme:wpfui`). This adds the `WPF-UI` NuGet package, theme resource dictionaries, and a themed `FluentWindow` shell. The generated app defaults to the Light palette; pass `--theme-mode dark` to generate the Dark palette instead (both are just the starting `ThemesDictionary` value — the shell also ships a runtime Light/Dark toggle).

For a Web API or fullstack project, use `--backend ddd` (or `--backend:ddd`) to enable incremental DDD CRUD generation. Adding an entity then creates its Domain entity, repository contract, EF Core repository, separate CQRS commands/queries and handlers, FluentValidation validators, and Minimal API endpoints for list, get, create, update, and delete.

Add realistic development data with `--seed dummy` (or `--seed:dummy`) together with `--simple` or `--backend ddd`. Each generated entity receives three deterministic sample records by default. Set a count with `--seed dummy 200` or `--seed:dummy:200`; values are chosen from declared types. Seeding runs at API or local DDD WPF startup and is intended for development environments.

DDD/Renoir workflow:

```text
go run ./cmd/aspgen new RenoirDemo --app blazor --output ./RenoirDemo
go run ./cmd/aspgen add context Catalog --project ./RenoirDemo
go run ./cmd/aspgen add aggregate Product name:string price:decimal active:bool published:date --context Catalog --project ./RenoirDemo
go run ./cmd/aspgen add value-object ProductCode value:string --context Catalog --project ./RenoirDemo
go run ./cmd/aspgen add domain-service PricingPolicy --context Catalog --project ./RenoirDemo
go run ./cmd/aspgen add repository ProductRepository --aggregate Product --context Catalog --project ./RenoirDemo
go run ./cmd/aspgen add event ProductPriceChanged productId:long price:decimal --context Catalog --project ./RenoirDemo
```

Aggregate generation creates the domain aggregate root, persistence mapping, application CRUD service, and a Blazor CRUD page. Controls are selected from types: strings use `InputText`, numbers use `InputNumber`, booleans use `InputCheckbox`, and dates use `InputDate`.

DDD building blocks are incremental and context-scoped. Value objects are immutable records, domain services are stateless policies, repository contracts are aggregate-specific and live in the domain layer, and events are immutable completed business facts. Use `--no-crud` on an aggregate when the use case should be modeled explicitly instead of starting with generated CRUD.

Generated Renoir CRUD keeps boundaries explicit: the application service exposes immutable `Request` and `View` records, while the Blazor page uses a local editable form model and never binds directly to the domain aggregate.

See [doc/aspgen-renoir-developer-guide.md](doc/aspgen-renoir-developer-guide.md) for a full step-by-step walkthrough of building and extending a Renoir app, with real generated code and file-tree diagrams.

Supported application targets are `webapi`, `wpf`, `blazor`, and `fullstack`.

Project names may be dotted .NET names such as `Markosoft.Commerce`. In that case aspgen keeps namespaces, project filenames, project references, and solution entries aligned: `Markosoft.Commerce.Domain.csproj`, `Markosoft.Commerce.Application.csproj`, `Markosoft.Commerce.Infrastructure.csproj`, and `Markosoft.Commerce.Desktop.csproj`.

Use `--simple` with `webapi` or `fullstack` for the Rails-style profile: one Web API project, EF Core Active Record-like models, direct CRUD endpoints, and no DDD/CQRS layer. `--simple` cannot be combined with `--backend ddd`.

Backend/profile matrix:

```text
webapi                         Clean Architecture Web API
webapi --backend ddd           Clean Architecture + DDD/CQRS CRUD
webapi --simple                Single-project Active Record-style CRUD API
wpf                            Prism/DryIoc desktop shell and local UI modules
wpf --backend ddd               Local DDD + SQLite layers with no WebApi
fullstack --backend ddd        DDD/CQRS API + Prism/DryIoc WPF modules
fullstack --simple             Simple CRUD API + WPF modules connected by HttpClient
```

For fullstack projects, generated WPF entity stores call `/api/{entity}`. The API base URL defaults to `http://localhost:5000` and can be changed with the `ASPGENT_API_URL` environment variable.

The `webapi` target generates Domain, Application, Infrastructure, and WebApi projects with project references in the direction Domain <- Application <- Infrastructure <- WebApi. It includes EF Core persistence (SQLite by default, PostgreSQL/Npgsql when selected), FluentValidation registration, OpenAPI, Scalar, health checks, and appsettings connection-string configuration.

SQLite is the default database for `webapi`, `fullstack`, `--simple`, and `--backend ddd` generation. Use `--database postgres` (also accepted as `--database:postgres`) when PostgreSQL is required. The selected provider is recorded in `.aspgen/manifest.json` and is emitted into the generated EF Core project, connection string, and dependency-injection setup.

### Generating entities from an existing database

Instead of hand-typing `name:type` properties, aspgen can scaffold entities (or, for the `blazor`/Renoir profile, aggregates via `--context`) from an existing database schema — a static SQL DDL script — for `webapi`/`fullstack`/`--simple`/`--backend ddd`/`blazor` projects. Supported providers are `sqlite`, `postgres`, `sqlserver`, and `mysql`.

```text
go run ./cmd/aspgen new MyApp --app webapi --simple --script schema.sql --provider postgres --tables all --output ./MyApp
go run ./cmd/aspgen import-db --project ./MyApp --connection "file:demo.db" --provider sqlite --tables Customers,Orders
```

`--tables` accepts `all` (the default) or a comma-separated list of table names. `--connection` and `--script` are mutually exclusive and both require `--provider`. Each selected table becomes one entity (its name PascalCased and best-effort singularized) via the same code path `add entity` uses, so the resulting Domain/Application/Infrastructure/WebApi/WPF layers match the target project's existing backend profile. Primary-key columns and (on `--backend ddd`) conventional audit columns (`created_on`/`updated_on` and similar) are skipped since generated entities already provide them; columns with no known type mapping (e.g. `json`, `blob`) are skipped with a warning rather than failing the whole table.

A `schema.sql` backup snapshot of the discovered tables is written at the project root on every run. It's a reference artifact only — aspgen never invokes `dotnet ef`; run `dotnet ef migrations add`/`dotnet ef database update` yourself against the generated entities and `DbContext`. Connection strings are never written to `.aspgen/manifest.json`, `schema.sql`, or any generated file.

Run `aspgen import-db --help` for the full flag reference.

Web API features are generated as vertical slices with request/response records, a FluentValidation validator, a CQRS handler returning `Result<T>`, and a Minimal API endpoint. Feature endpoints are registered incrementally in `Program.cs`.

The `blazor` target is a Renoir-style profile with DomainModel, Application, Infrastructure, Persistence, Resources, and AppBlazor projects. It includes soft-delete and `TimeStamp` domain conventions, settings binding, SQL Server EF Core persistence, and a layered Blazor host.

Templates are embedded in the executable. Export and customize them with:

```text
go run ./cmd/aspgen templates export ./my-templates
go run ./cmd/aspgen templates list
go run ./cmd/aspgen new MyApp --app webapi --templates ./my-templates
```

Generated projects contain `.aspgen/manifest.json`, allowing components to be added incrementally without regenerating the whole project.

Generated projects use SDK-style `.csproj` files. Incremental generation also adds explicit safe `<Compile Update>` and `<Page Update>` entries for generated `.cs` and `.xaml` files, so new entities and modules appear in Solution Explorer and remain owned by the correct project. The `.sln` file contains project entries; it is updated whenever a new project, such as an incremental WPF UI, is added.

The WPF target uses current Prism 9 conventions with `Prism.DryIoc`: `PrismApplication`, `IContainerRegistry`, `IContainerProvider`, `IModule`, `ConfigureModuleCatalog`, `ViewModelLocator`, `BindableBase`, `DelegateCommand`, regions, navigation registration, and typed `PubSubEvent` communication.
