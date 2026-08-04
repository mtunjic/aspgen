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

For a WPF or fullstack project, add the WPF-UI Fluent theme with `--theme wpfui` (also accepted as `--theme:wpfui` or `-theme:wpfui`). This adds the `WPF-UI` NuGet package, theme resource dictionaries, and a themed `FluentWindow` shell.

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
