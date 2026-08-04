package generator

// topLevelHelp is printed for `aspgen`, `aspgen --help`, `aspgen -h`, and `aspgen help`.
const topLevelHelp = `aspgen scaffolds ASP.NET Core Clean Architecture APIs and Prism/DryIoc WPF apps from templates.

Usage:
  aspgen new NAME [flags]        Create a new project (tree + solution + manifest)
  aspgen add KIND NAME [flags]   Add a component to an existing project
  aspgen templates list|export PATH|validate PATH
  aspgen version                 Print the aspgen version

Run "aspgen new --help" or "aspgen add --help" for flag details.

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
  --output PATH      Output directory (default: project NAME)
  --templates PATH   Use a custom template directory instead of the embedded set
  --dry-run          Print what would be created without writing files
  --force            Overwrite existing files

Examples:
  aspgen new MyApp --app fullstack --simple --output ./MyApp
  aspgen new MyApp --app webapi --backend ddd --database postgres
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
