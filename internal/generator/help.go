package generator

// topLevelHelp is printed for `aspgen`, `aspgen --help`, `aspgen -h`, and `aspgen help`.
const topLevelHelp = `aspgen scaffolds ASP.NET Core Clean Architecture APIs and Prism/DryIoc WPF apps from templates.

Usage:
  aspgen new NAME [flags]        Create a new project (tree + solution + manifest)
  aspgen add KIND NAME [flags]   Add a component to an existing project
  aspgen import-db [flags]       Add entities to an existing project from a DB connection/script
  aspgen templates list|export PATH|validate PATH
  aspgen version                 Print the aspgen version

Run "aspgen new --help", "aspgen add --help", or "aspgen import-db --help" for flag details.

Flags accept three equivalent forms: --flag value, --flag:value, -flag:value
`

// newHelp is printed for `aspgen new --help`.
const newHelp = `Usage:
  aspgen new NAME --app webapi|wpf|blazor|fullstack [flags]

Flags:
  --app TARGET       webapi | wpf | blazor | fullstack (default webapi)
  --simple           Single-project Active Record CRUD (webapi/fullstack; not with --backend)
  --backend ddd      Clean Architecture DDD/CQRS layers (webapi/fullstack, or wpf for local-only DDD)
  --database DB      sqlite | postgres (default sqlite; webapi/fullstack/wpf+ddd only)
  --theme wpfui      WPF-UI Fluent theme (wpf/fullstack only)
  --seed dummy [N]   N sample records per entity, default 3 (needs --simple or --backend ddd)
  --connection CONN  Live DB connection string to import entities from (needs --provider)
  --script PATH      SQL DDL script to import entities from instead of --connection (needs --provider)
  --provider P       sqlite | postgres | sqlserver | mysql (required with --connection/--script)
  --tables T         all (default) or a comma list of table names to import
  --output PATH      Output directory (default: project NAME)
  --templates PATH   Use a custom template directory instead of the embedded set
  --dry-run          Print what would be created without writing files
  --force            Overwrite existing files

Examples:
  aspgen new MyApp --app fullstack --simple --output ./MyApp
  aspgen new MyApp --app webapi --backend ddd --database postgres
  aspgen new MyApp --app webapi --simple --script schema.sql --provider postgres --tables all
  aspgen new RenoirDemo --app blazor
`

// addHelp is printed for `aspgen add --help`.
const addHelp = `Usage:
  aspgen add KIND NAME [prop:type ...] [flags]

Kinds:
  entity NAME prop:type...      Simple-profile entity (Domain + API + seed)
  module NAME                   WPF Prism module (run "add ui" first)
  database NAME                 Register/switch the persistence provider
  service NAME                  Application service (non-simple webapi/fullstack)
  feature NAME prop:type...     Web API vertical-slice CRUD feature
  ui                            Add the WPF/Prism UI to a webapi project [--framework wpf]
  context NAME                  DDD bounded context (blazor/Renoir profile)
  aggregate NAME prop:type... --context CTX [--no-crud]
  value-object NAME prop:type... --context CTX
  domain-service NAME --context CTX
  repository NAME --aggregate AGG --context CTX
  event NAME prop:type... --context CTX

Common flags:
  --project PATH     Path to the generated project (default: search this dir and parents for .aspgen/manifest.json)
  --theme wpfui      WPF-UI Fluent theme (ui/module)
  --dry-run          Print what would change without writing files
  --force            Overwrite existing files

Examples:
  aspgen add entity Customer name:string age:int active:bool --project ./MyApp
  aspgen add context Catalog --project ./RenoirDemo
  aspgen add aggregate Product name:string price:decimal --context Catalog --project ./RenoirDemo
`

// importDBHelp is printed for `aspgen import-db --help`.
const importDBHelp = `Usage:
  aspgen import-db --project PATH --connection CONN|--script PATH --provider PROVIDER [flags]

Adds an entity per selected table to an existing project (simple or ddd
backend, webapi/fullstack/wpf profiles only — not blazor/Renoir), the same
way "add entity" would, then writes a schema.sql backup snapshot at the
project root. Does not run "dotnet ef migrations" — that remains a manual
step against the generated entities/DbContext.

Flags:
  --project PATH     Path to the generated project (default: search this dir and parents for .aspgen/manifest.json)
  --connection CONN  Live DB connection string (mutually exclusive with --script)
  --script PATH      SQL DDL script to parse instead of connecting live
  --provider P       sqlite | postgres | sqlserver | mysql (required)
  --tables T         all (default) or a comma list of table names to import
  --backend ddd      Override the project's backend profile (default: the project's own backend)
  --dry-run          Print what would change without writing files
  --force            Overwrite existing files

Examples:
  aspgen import-db --project ./MyApp --script schema.sql --provider postgres --tables all
  aspgen import-db --project ./MyApp --connection "file:demo.db" --provider sqlite --tables Customers,Orders
`
