# Architecture Theta

Architecture Theta is the reusable architecture profile for `aspgen`. When a future request says **use Architecture Theta**, apply the rules in this document to generated or modified projects.

## Purpose

Generate maintainable applications that can contain an ASP.NET Core Web API, a WPF desktop client, or both in one solution. The backend follows Clean Architecture, DDD, Vertical Slice Architecture, and CQRS. The desktop client follows Prism MVVM with DryIoc, modular composition, regions, navigation, typed events, commands, and dialogs.

## Solution shape

### Backend

`WebApi -> Infrastructure -> Application -> Domain`

- **Domain**: entities, aggregate roots, value objects, domain services, domain events, and repository contracts. It has no dependency on other layers.
- **Application**: use cases, commands, queries, handlers, immutable request/response records, validators, and application ports. It depends only on Domain.
- **Infrastructure**: EF Core, SQLite by default or PostgreSQL/Npgsql when selected, repositories, migrations, external services, health checks, audit interception, and persistence configuration. It depends on Application and Domain.
- **WebApi**: Minimal API endpoints, dependency composition, middleware, OpenAPI, Scalar, and health endpoints. It depends on Application and Infrastructure.

### Desktop

`Desktop shell -> Prism modules -> application/API services`

- **Shell**: Prism application startup, DryIoc composition root, global resources, shell regions, navigation, dialogs, and global commands.
- **Module**: an independently deployable feature area containing views, ViewModels, services, registrations, navigation keys, and module-specific events.
- **Shared**: API clients, contracts, typed events, converters, validation helpers, dialog helpers, and shared styles.

The Web API and Desktop projects may be generated together. They communicate through explicit contracts and services, never through direct references between unrelated UI modules.

Desktop-only DDD is also supported with `--app wpf --backend ddd`. In that profile the graph is `Desktop -> Infrastructure -> Application -> Domain`, SQLite is local by default, and no WebApi or HTTP client is generated.

## Domain-Driven Design

### Bounded contexts

Every context owns its language, aggregates, persistence mapping, application services, and external boundary. Contexts should not share mutable domain entities. Shared concepts belong in explicit shared-kernel contracts or integration events.

### Aggregate roots

An aggregate root is the only entry point for modifying the aggregate. It protects invariants and exposes behavior-oriented methods rather than public mutable state.

Each aggregate should have:

- an identity;
- private or protected state where practical;
- invariant enforcement in constructors and methods;
- domain events for meaningful state changes;
- a repository contract in Domain;
- EF Core mapping in Infrastructure;
- application CRUD use cases when CRUD is appropriate;
- API endpoints and desktop screens generated from contracts, not directly from EF entities.

### Entities and value objects

Entities inherit from `AuditableEntity` and contain `CreatedOn` and `UpdatedOn`. Audit values are assigned by an audit interceptor.

Value objects are immutable records with value equality. They validate themselves at creation and should be represented as owned types or strongly typed conversions in persistence.

Domain services are stateless services for business rules that do not naturally belong to one entity or aggregate. They must not become a place for application orchestration or database access.

### Domain events

Domain events are immutable records or classes. They describe facts that happened inside a bounded context. Dispatching and integration translation belong outside Domain.

## Application and CQRS

Organize application code by vertical feature slice:

```text
Application/Features/Book/CreateBook/
  CreateBookRequest.cs
  CreateBookResponse.cs
  CreateBookHandler.cs
  CreateBookValidator.cs
  CreateBookEndpoint.cs
```

- Commands change state: Create, Update, Delete.
- Queries read state: GetAll, GetById, Search.
- Handlers contain business orchestration and use asynchronous APIs.
- Handlers implement `IHandler<TRequest, TResponse>`.
- Handlers return `Result<T>` with success or typed failure.
- Endpoints only map HTTP input/output and delegate to handlers.
- All request and response DTOs are C# records.
- Every request has a FluentValidation validator.
- Application must not depend on ASP.NET or EF Core implementation details.

## API conventions

- Use ASP.NET Core Minimal APIs.
- Use thin endpoint extension methods grouped by feature.
- Return appropriate status codes: `201` for creation, `200` for reads/updates, `204` for successful deletion, `400` for validation, `404` for missing resources, and `409` for conflicts.
- Use OpenAPI at `/openapi/v1.json` and Scalar at `/scalar/v1`.
- Expose `/health` for service and database health.
- Use async database calls and cancellation tokens.
- Keep the selected database configuration in `appsettings.json`; SQLite is the default and PostgreSQL is enabled with `--database postgres`. Production settings remain user-owned.

## WPF and MVVM

Views are XAML composition surfaces. ViewModels own state, commands, validation, navigation participation, and orchestration. Code-behind should be limited to view lifecycle or unavoidable visual behavior.

- Use `BindableBase` or an equivalent `INotifyPropertyChanged` base.
- Prefer `ObservableCollection<T>` for bindable collections.
- Use `DelegateCommand` and `DelegateCommand<T>` instead of click handlers.
- Use `ObservesProperty`, `ObservesCanExecute`, or `RaiseCanExecuteChanged` to keep command enablement correct.
- Use `IValueConverter` for presentation-only transformations; `ConvertBack` should explicitly reject unsupported reverse conversions.
- Use `ICollectionView` for sorting, filtering, grouping, and display-specific collection behavior.
- Use shared resource dictionaries for colors, typography, styles, templates, and control themes.
- Use typed controls based on the property type: text, numeric, checkbox, date, enum, lookup, and multiline text.
- Use immediate source updates for edit forms when validation should be visible while typing.
- Use validation and change tracking for edit screens, including accept/reject behavior and unsaved-change checks.

## Prism and DryIoc

Use modern Prism 9 with `Prism.DryIoc`. Older reference samples using Unity or Prism 7 are conceptual references only.

### Startup

- `App` derives from `PrismApplication`.
- `CreateShell` resolves the shell from the container.
- `RegisterTypes` registers services, clients, dialogs, and navigation views.
- `ConfigureModuleCatalog` registers required or on-demand modules.
- The shell defines uniquely named regions such as `MenuRegion`, `HeaderRegion`, and `ContentRegion`.

### Modules

Each module implements `IModule` and owns its registrations and UI. Its `RegisterTypes` method registers services and navigation views. Its initialization method performs region registration or startup navigation. Modules should be independently removable and addable without regenerating unrelated layers.

Support module loading by code, directory, configuration, and on-demand catalog entries where the project requires it.

### Regions and composition

- Use view discovery when a region should automatically receive a view.
- Use view injection when the ViewModel must explicitly add, remove, activate, or deactivate instances.
- Use region navigation for screen transitions.
- Use custom `RegionAdapter<T>` only for controls not supported by Prism's standard adapters.
- Region names must be unique and documented as shell contracts.

### ViewModels and navigation

Use Prism's ViewModelLocator convention by default. Register custom mappings only when the convention cannot express the relationship.

Use `INavigationAware` for loading and cleanup around navigation. Use `IsNavigationTarget` to reuse an existing instance when appropriate. Use typed navigation parameters and avoid passing domain entities directly to views.

Use `IConfirmNavigationRequest` for unsaved changes or permission checks. Use the navigation journal for back/forward behavior. Use `IRegionMemberLifetime` when a view should be kept alive or removed.

### Communication

Modules communicate through interfaces and typed `PubSubEvent<T>` events from a shared contracts area. Event payloads should be immutable records. Use filters for selective subscriptions and retain a subscription token or unsubscribe during disposal/lifecycle cleanup to prevent stale subscriptions and memory leaks.

Use `CompositeCommand` for shell-level operations such as Save All. Individual modules register local commands and expose active-aware participation when only the active module should respond.

### Dialogs

Dialogs are UserControls hosted by Prism's dialog service. ViewModels implement `IDialogAware`, receive typed parameters, expose commands, and return a typed result. Register dialogs with DryIoc and provide shared helper methods for common notifications and confirmations. Dialog styles should be centralized and configurable per dialog.

## UI generation rules

For an entity property declaration such as:

```text
name:string age:int birthDate:date active:bool price:decimal
```

generate:

- a strongly typed Domain property;
- immutable request and response records;
- FluentValidation rules;
- persistence mapping;
- Web API CRUD slices when requested;
- WPF list and edit controls selected by type;
- display formatting and converters where needed;
- commands for Create, Edit, Save, Delete, Cancel, Refresh, and navigation;
- confirmation dialogs for destructive actions.

Nullable syntax uses `?`, for example `middleName:string?` or `birthDate:date?`. Type parsing must be deterministic and reject unknown types rather than silently generating incorrect code.

## Generator behavior

- `new` is the initialization command. It creates the project tree, solution, host files, and `.aspgen/manifest.json`.
- `add` is incremental only. It requires an existing generated project and updates the files owned by that project; it does not create a missing project directory or manifest.
- When `--project` is omitted, `add` searches the current directory and its parents for `.aspgen/manifest.json`. Use `--project PATH` for explicit and scriptable generation.
- Use Go `text/template` templates embedded with `go:embed`.
- Keep templates readable, small, and easy to edit.
- Put generated-project-specific logic in templates and keep orchestration in Go.
- Make generation idempotent.
- Preserve user-owned files and never modify `bin/`, `obj/`, migrations, production settings, or project files unless the command explicitly owns that operation.
- Use manifest metadata to track generated components, contexts, aggregates, modules, and features.
- Support full generation and modular additions: UI only, one backend layer, database, service, context, aggregate, feature, or module.
- Use marker comments for safe updates to registration lists, module catalogs, and endpoint hosts.
- Register generated `.cs` and `.xaml` files in their owning SDK-style `.csproj` with `Update` items so Visual Studio project membership is explicit without creating duplicate compile items. Solution files contain projects, not individual source files.
- Provide dry-run, force, template listing/export, template validation, and clear diagnostics.
- Generated names must be validated and converted consistently to namespaces, file names, routes, and database identifiers.

## Verification checklist

Before considering a generator change complete:

1. Run Go unit tests.
2. Validate every embedded template.
3. Generate a minimal Web API and build it.
4. Generate a WPF/Prism project and build it when the environment supports WPF.
5. Generate a combined Web API + Desktop project.
6. Add an entity with mixed typed properties.
7. Add a context, aggregate, value object, repository, domain event, feature, and module incrementally.
8. Run generation twice and verify no unwanted duplication.
9. Review architecture boundaries and generated project references.
10. Report package restore warnings separately from compile errors.

## Reference vocabulary

When discussing future work:

- **Theta backend** means Clean Architecture + DDD + Vertical Slice + CQRS with SQLite by default; PostgreSQL is selected with `--database postgres`.
- **Theta desktop** means WPF + Prism 9 + DryIoc + MVVM + modules.
- **Theta feature** means a self-contained command/query slice with records, handler, validator, endpoint, and optional UI.
- **Theta module** means an independently registered Prism module with View, ViewModel, commands, services, navigation, and event contracts.
- **Theta CRUD** means aggregate-safe CRUD with API contracts, validation, persistence mapping, and typed WPF controls.
