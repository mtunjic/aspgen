<#
.SYNOPSIS
    Generates, builds, and (optionally) runs the whole Northwind Trading demo
    from doc/aspgen-enterprise-developer-guide.md, end to end.

.DESCRIPTION
    Automates every `aspgen new`/`add` command from the enterprise guide's
    Section 12 "Running everything end to end" showcase script, then runs
    `dotnet restore`/`build` against each generated solution:
      - CatalogApi     (ar)         Category/Product/Supplier
      - NorthwindOps   (dm + cqrs)  Inventory (StockItem/Warehouse) + Sales
                                    (Order/Customer), one shared WPF shell
      - BillingLedger  (es)         Invoice/CreditNote, spa-ready
      - SalesPortal    (cqrs)       Order, blazor UI
      - WarehouseOps   (dm)         StockItem, mvc UI

    Uses `go run ./cmd/aspgen` against this checkout, so it always exercises
    the generator source as it currently stands, not a previously built
    aspgen.exe. Does not shell out to `dotnet ef`. Automatically patches the
    doc/aspgen-enterprise-developer-guide.md Section 6 "Seeding Catalog with
    real Northwind sample data" block into CatalogApi's Program.cs after
    generating it (see Add-CatalogSeedData below) -- aspgen itself has no
    `--seed` flag, so this is this script's own post-generation step, not
    something `aspgen new`/`add` produces on its own.

.PARAMETER OutputRoot
    Directory the five project folders are created under. Default:
    ".\northwind-trading-demo" relative to the repo root.

.PARAMETER Force
    Delete OutputRoot first if it already exists, instead of failing.

.PARAMETER SkipDotnet
    Only run the aspgen generation commands; skip `dotnet restore`/`build`.

.PARAMETER Run
    After a successful build, launch every runnable host/app in its own new
    PowerShell window (CatalogApi/NorthwindOps/BillingLedger WebApi hosts,
    the NorthwindOps WPF Desktop shell, SalesPortal's AppBlazor host, and
    WarehouseOps' WebMvc host). Each window stays open after `dotnet run`
    exits so you can read any errors; close them yourself when done.

.EXAMPLE
    ./scripts/demo-northwind-trading.ps1
    ./scripts/demo-northwind-trading.ps1 -Force -Run
    ./scripts/demo-northwind-trading.ps1 -OutputRoot C:\demo\northwind -SkipDotnet
#>
[CmdletBinding()]
param(
    [string]$OutputRoot = ".\northwind-trading-demo",
    [switch]$Force,
    [switch]$SkipDotnet,
    [switch]$Run
)

$ErrorActionPreference = "Stop"
Set-StrictMode -Version Latest

$repoRoot = Split-Path -Parent $PSScriptRoot
Push-Location $repoRoot

$stepNumber = 0

function Invoke-Step {
    param(
        [Parameter(Mandatory)][string]$Name,
        [Parameter(Mandatory)][scriptblock]$Script
    )
    $script:stepNumber++
    Write-Host ""
    Write-Host "== [$script:stepNumber] $Name ==" -ForegroundColor Cyan
    & $Script
    if ($LASTEXITCODE -ne 0) {
        Write-Host "FAILED: $Name (exit code $LASTEXITCODE)" -ForegroundColor Red
        Pop-Location
        exit $LASTEXITCODE
    }
    Write-Host "OK: $Name" -ForegroundColor Green
}

function Invoke-Aspgen {
    param([Parameter(Mandatory, ValueFromRemainingArguments)][string[]]$Args)
    & go run ./cmd/aspgen @Args
    if ($LASTEXITCODE -ne 0) {
        throw "aspgen $($Args -join ' ') failed with exit code $LASTEXITCODE"
    }
}

function Add-CatalogSeedData {
    # Patches the exact seed block from doc/aspgen-enterprise-developer-guide.md
    # Section 6 into CatalogApi's Program.cs at the `// aspgen:seed` marker, so
    # this demo doesn't require the hand-edit step the guide describes as
    # optional/manual -- Database.EnsureCreated() already runs earlier in
    # Program.cs (added unconditionally to every host), so this block only
    # needs to check for existing data, not create the schema itself.
    param([Parameter(Mandatory)][string]$CatalogApiPath)
    $programPath = Join-Path $CatalogApiPath "src\WebApi\Program.cs"
    $content = Get-Content -Raw -LiteralPath $programPath

    $usingAnchor = "using CatalogApi.WebApi.Data;"
    $usingLine = "using CatalogApi.WebApi.Models.Catalog;"
    if ($content -notlike "*$usingLine*") {
        $content = $content.Replace($usingAnchor, "$usingAnchor`r`n$usingLine")
    }

    $seedBlock = @'
// aspgen:seed
using (var scope = app.Services.CreateScope())
{
    var db = scope.ServiceProvider.GetRequiredService<AppDbContext>();
    if (!db.Categorys.Any())
    {
        var categories = new[]
        {
            new Category { Name = "Beverages" },
            new Category { Name = "Condiments" },
            new Category { Name = "Confections" },
            new Category { Name = "Dairy Products" },
            new Category { Name = "Grains/Cereals" },
            new Category { Name = "Meat/Poultry" },
            new Category { Name = "Produce" },
            new Category { Name = "Seafood" },
        };
        db.Categorys.AddRange(categories);
        db.SaveChanges();

        db.Products.AddRange(
            new Product { Name = "Chai", Sku = "BEV-001", Price = 18.00m, Active = true, CategoryId = categories[0].Id },
            new Product { Name = "Chang", Sku = "BEV-002", Price = 19.00m, Active = true, CategoryId = categories[0].Id },
            new Product { Name = "Aniseed Syrup", Sku = "CON-001", Price = 10.00m, Active = true, CategoryId = categories[1].Id },
            new Product { Name = "Chef Anton's Cajun Seasoning", Sku = "CON-002", Price = 22.00m, Active = true, CategoryId = categories[1].Id },
            new Product { Name = "Pavlova", Sku = "SWT-001", Price = 17.45m, Active = true, CategoryId = categories[2].Id },
            new Product { Name = "Teatime Chocolate Biscuits", Sku = "SWT-002", Price = 9.20m, Active = true, CategoryId = categories[2].Id },
            new Product { Name = "Queso Cabrales", Sku = "DAI-001", Price = 21.00m, Active = true, CategoryId = categories[3].Id },
            new Product { Name = "Mozzarella di Giovanni", Sku = "DAI-002", Price = 34.80m, Active = true, CategoryId = categories[3].Id },
            new Product { Name = "Gustaf's Knackebrod", Sku = "GRN-001", Price = 21.00m, Active = true, CategoryId = categories[4].Id },
            new Product { Name = "Tunnbrod", Sku = "GRN-002", Price = 9.00m, Active = true, CategoryId = categories[4].Id },
            new Product { Name = "Mishi Kobe Niku", Sku = "MEA-001", Price = 97.00m, Active = true, CategoryId = categories[5].Id },
            new Product { Name = "Alice Mutton", Sku = "MEA-002", Price = 39.00m, Active = true, CategoryId = categories[5].Id },
            new Product { Name = "Uncle Bob's Organic Dried Pears", Sku = "PRD-001", Price = 30.00m, Active = true, CategoryId = categories[6].Id },
            new Product { Name = "Tofu", Sku = "PRD-002", Price = 23.25m, Active = true, CategoryId = categories[6].Id },
            new Product { Name = "Ikura", Sku = "SEA-001", Price = 31.00m, Active = true, CategoryId = categories[7].Id },
            new Product { Name = "Konbu", Sku = "SEA-002", Price = 6.00m, Active = true, CategoryId = categories[7].Id }
        );
        db.SaveChanges();
    }
}
'@
    $content = $content.Replace("// aspgen:seed", $seedBlock.Trim())
    Set-Content -LiteralPath $programPath -Value $content -NoNewline
}

function Add-NorthwindOpsSeedData {
    # Patches NorthwindOps' WebApi Program.cs (the shared host for the Sales
    # cqrs context) at the `// aspgen:seed` marker with sample Inventory
    # (dm-tier) and Sales (cqrs-tier) records, so the WPF Desktop shell shows
    # real rows instead of empty grids the first time it's opened.
    #
    # Inventory's StockItem/WarehouseCrudService are only DI-registered in
    # the WPF Desktop app's own DryIoc container (dm-tier aggregates never
    # get wired into the WebApi host's ASP.NET Core DI - see
    # add_ddd.go's renderAggregateCrud "default" case), so they're
    # constructed directly here instead of resolved via services.
    # GetRequiredService. Sales' CustomerCrudService/OrderCrudService ARE
    # DI-registered (the cqrs case in the same switch), so those two are
    # resolved normally.
    param([Parameter(Mandatory)][string]$NorthwindOpsPath)
    $programPath = Join-Path $NorthwindOpsPath "src\WebApi\Program.cs"
    $content = Get-Content -Raw -LiteralPath $programPath

    $usingAnchor = "using NorthwindOps.Persistence;"
    $usingLine = "using NorthwindOps.Persistence.Repositories;"
    if ($content -notlike "*$usingLine*") {
        $content = $content.Replace($usingAnchor, "$usingAnchor`r`n$usingLine")
    }

    $seedBlock = @'
// aspgen:seed
using (var scope = app.Services.CreateScope())
{
    var services = scope.ServiceProvider;
    var database = services.GetRequiredService<NorthwindOpsDatabase>();

    var stockItems = new StockItemCrudService(database, new StockItemValidator(), new StockItemRepository(database));
    if (stockItems.GetAllAsync().GetAwaiter().GetResult().Count == 0)
    {
        stockItems.CreateAsync(new StockItemRequest("SKU-1001", 240, 50)).GetAwaiter().GetResult();
        stockItems.CreateAsync(new StockItemRequest("SKU-1002", 60, 25)).GetAwaiter().GetResult();
        stockItems.CreateAsync(new StockItemRequest("SKU-1003", 500, 100)).GetAwaiter().GetResult();
    }

    var warehouses = new WarehouseCrudService(database, new WarehouseValidator(), new WarehouseRepository(database));
    if (warehouses.GetAllAsync().GetAwaiter().GetResult().Count == 0)
    {
        warehouses.CreateAsync(new WarehouseRequest("Central Distribution", "Chicago")).GetAwaiter().GetResult();
        warehouses.CreateAsync(new WarehouseRequest("West Coast Hub", "Reno")).GetAwaiter().GetResult();
    }

    var customers = services.GetRequiredService<CustomerCrudService>();
    if (customers.GetAllAsync().GetAwaiter().GetResult().Count == 0)
    {
        customers.CreateAsync(new CustomerRequest("Contoso Retail", "orders@contoso.example")).GetAwaiter().GetResult();
        customers.CreateAsync(new CustomerRequest("Fabrikam Wholesale", "purchasing@fabrikam.example")).GetAwaiter().GetResult();
        customers.CreateAsync(new CustomerRequest("Northwind Traders", "buyer@northwindtraders.example")).GetAwaiter().GetResult();
    }

    var orders = services.GetRequiredService<OrderCrudService>();
    if (orders.GetAllAsync().GetAwaiter().GetResult().Count == 0)
    {
        orders.CreateAsync(new OrderRequest("SO-100001", "Contoso Retail", 1845.00m, new DateOnly(2026, 7, 2))).GetAwaiter().GetResult();
        orders.CreateAsync(new OrderRequest("SO-100002", "Fabrikam Wholesale", 5320.50m, new DateOnly(2026, 7, 9))).GetAwaiter().GetResult();
        orders.CreateAsync(new OrderRequest("SO-100003", "Northwind Traders", 990.25m, new DateOnly(2026, 7, 15))).GetAwaiter().GetResult();
    }
}
'@
    $content = $content.Replace("// aspgen:seed", $seedBlock.Trim())
    Set-Content -LiteralPath $programPath -Value $content -NoNewline
}

function Stop-DemoProcesses {
    # A previous "-Run" leaves every dotnet run window open (-NoExit) until you
    # close it yourself, and those processes keep their own built DLLs/EXEs
    # locked -- a later -Force delete of the same OutputRoot fails with "The
    # process cannot access the file ... because it is being used by another
    # process". Matching only Name='dotnet.exe' misses the actual lock holder
    # for exe-producing projects (WPF Desktop, WebMvc, AppBlazor all launch
    # their own apphost .exe, e.g. NorthwindOps.Desktop.exe, NOT dotnet.exe
    # itself) -- so scan every process for either an executable path or a
    # command line that points inside this OutputRoot, never anything outside it.
    param([Parameter(Mandatory)][string]$OutputRootFull)
    $procs = Get-CimInstance Win32_Process -ErrorAction SilentlyContinue | Where-Object {
        ($_.ExecutablePath -and $_.ExecutablePath.StartsWith($OutputRootFull, [System.StringComparison]::OrdinalIgnoreCase)) -or
        ($_.CommandLine -and $_.CommandLine.Contains($OutputRootFull))
    }
    foreach ($p in $procs) {
        Write-Host "  Stopping leftover process from a previous run (PID $($p.ProcessId)): $($p.Name) -- $($p.CommandLine)" -ForegroundColor Yellow
        Stop-Process -Id $p.ProcessId -Force -ErrorAction SilentlyContinue
    }

    # MSBuild/Roslyn keep persistent background "build server" processes alive
    # across builds for speed (`dotnet.exe ... MSBuild.dll /nodeReuse:true`,
    # VBCSCompiler.exe) -- these are invisible as "running apps" (no window, not
    # tied to any dotnet run/apphost process above, never closed by closing a
    # terminal) but still hold file-handle locks on every project's build
    # output, including ones under $OutputRootFull. Shutting them down is safe
    # and just costs a slightly slower next build (they respawn automatically).
    & dotnet build-server shutdown *> $null

    if ($procs) { Start-Sleep -Seconds 2 }
}

try {
    # [System.IO.Path]::GetFullPath(path, basePath) is a .NET Core-only overload;
    # Windows PowerShell 5.1 (.NET Framework) only has the single-arg form, so
    # resolve relative paths against $repoRoot manually first for compatibility
    # with both Windows PowerShell and PowerShell 7.
    $outputRootFull = if ([System.IO.Path]::IsPathRooted($OutputRoot)) { $OutputRoot } else { Join-Path $repoRoot $OutputRoot }
    $outputRootFull = [System.IO.Path]::GetFullPath($outputRootFull)

    if (Test-Path $outputRootFull) {
        if (-not $Force) {
            throw "$outputRootFull already exists; pass -Force to delete and regenerate it."
        }
        Write-Host "Removing existing $outputRootFull ..." -ForegroundColor Yellow
        Stop-DemoProcesses -OutputRootFull $outputRootFull
        $removed = $false
        $maxAttempts = 15
        for ($attempt = 1; $attempt -le $maxAttempts -and -not $removed; $attempt++) {
            try {
                Remove-Item -Recurse -Force $outputRootFull -ErrorAction Stop
                $removed = $true
            }
            catch {
                if ($attempt -eq $maxAttempts) {
                    throw "Could not fully delete $outputRootFull after $attempt attempts: $($_.Exception.Message)`nStop-DemoProcesses already tried to close every dotnet/apphost process pointing at this folder. A DIFFERENT file getting locked on each retry (as opposed to the same one every time) usually means a background scanner is walking the tree, not a single stuck process -- check Windows Search indexing, antivirus/EDR real-time scanning, or OneDrive/cloud-sync (this path is under your user profile, a common OneDrive sync root) before assuming it's a leftover terminal or File Explorer window with its cwd inside $outputRootFull. Pausing whichever of those applies, or simply re-running this script again a minute later, both work -- then re-run."
                }
                if ($attempt % 5 -eq 0) { Stop-DemoProcesses -OutputRootFull $outputRootFull }
                Write-Host "  Delete attempt $attempt/$maxAttempts failed ($($_.Exception.Message.Split("`n")[0])) - retrying ..." -ForegroundColor Yellow
                Start-Sleep -Seconds 2
            }
        }
    }
    New-Item -ItemType Directory -Path $outputRootFull -Force | Out-Null

    $catalogApi = Join-Path $outputRootFull "CatalogApi"
    $northwindOps = Join-Path $outputRootFull "NorthwindOps"
    $billingLedger = Join-Path $outputRootFull "BillingLedger"
    $salesPortal = Join-Path $outputRootFull "SalesPortal"
    $warehouseOps = Join-Path $outputRootFull "WarehouseOps"

    Invoke-Step "Generate CatalogApi (ar)" {
        Invoke-Aspgen new CatalogApi --context Catalog --arch ar --database sqlite --output $catalogApi
        Invoke-Aspgen add entity Category name:string --context Catalog --project $catalogApi
        Invoke-Aspgen add entity Product name:string sku:string price:decimal active:bool category:Category --context Catalog --project $catalogApi
        Invoke-Aspgen add entity Supplier name:string contactEmail:string --context Catalog --project $catalogApi
        Add-CatalogSeedData -CatalogApiPath $catalogApi
    }

    Invoke-Step "Generate NorthwindOps (dm + cqrs)" {
        Invoke-Aspgen new NorthwindOps --context Sales --arch cqrs --database sqlite --output $northwindOps
        Invoke-Aspgen add context Inventory --arch dm --project $northwindOps
        Invoke-Aspgen add aggregate StockItem sku:string quantityOnHand:int reorderPoint:int --context Inventory --project $northwindOps
        Invoke-Aspgen add value-object BinLocation aisle:string shelf:string --context Inventory --project $northwindOps
        Invoke-Aspgen add domain-service ReplenishmentPolicy --context Inventory --project $northwindOps
        Invoke-Aspgen add repository StockItemRepository --aggregate StockItem --context Inventory --project $northwindOps
        Invoke-Aspgen add event StockDepletedEvent stockItemId:long quantityOnHand:int --context Inventory --project $northwindOps
        Invoke-Aspgen add aggregate Warehouse name:string city:string --context Inventory --project $northwindOps
        Invoke-Aspgen add repository WarehouseRepository --aggregate Warehouse --context Inventory --project $northwindOps
        Invoke-Aspgen add aggregate Order number:string customer:string total:decimal placedOn:date --context Sales --project $northwindOps
        Invoke-Aspgen add repository OrderRepository --aggregate Order --context Sales --project $northwindOps
        Invoke-Aspgen add aggregate Customer name:string email:string --context Sales --project $northwindOps
        Invoke-Aspgen add repository CustomerRepository --aggregate Customer --context Sales --project $northwindOps
        Invoke-Aspgen add ui wpf --framework wpf --theme wpfui --theme-mode light --project $northwindOps
        Add-NorthwindOpsSeedData -NorthwindOpsPath $northwindOps
    }

    Invoke-Step "Generate BillingLedger (es)" {
        Invoke-Aspgen new BillingLedger --context Billing --arch es --database sqlite --output $billingLedger
        Invoke-Aspgen add aggregate Invoice number:string customer:string amount:decimal issuedOn:date --context Billing --project $billingLedger
        Invoke-Aspgen add aggregate CreditNote number:string customer:string amount:decimal issuedOn:date --context Billing --project $billingLedger
        Invoke-Aspgen add ui spa --framework spa --project $billingLedger
    }

    Invoke-Step "Generate SalesPortal (cqrs + blazor)" {
        Invoke-Aspgen new SalesPortal --context Sales --arch cqrs -ui blazor --output $salesPortal
        Invoke-Aspgen add aggregate Order number:string customer:string total:decimal placedOn:date --context Sales --project $salesPortal
    }

    Invoke-Step "Generate WarehouseOps (dm + mvc)" {
        Invoke-Aspgen new WarehouseOps --context Inventory --arch dm -ui mvc --output $warehouseOps
        Invoke-Aspgen add aggregate StockItem sku:string quantityOnHand:int reorderPoint:int --context Inventory --project $warehouseOps
    }

    $projects = @(
        @{ Name = "CatalogApi"; Path = $catalogApi }
        @{ Name = "NorthwindOps"; Path = $northwindOps }
        @{ Name = "BillingLedger"; Path = $billingLedger }
        @{ Name = "SalesPortal"; Path = $salesPortal }
        @{ Name = "WarehouseOps"; Path = $warehouseOps }
    )

    if (-not $SkipDotnet) {
        # Force a clean MSBuild/Roslyn state before building anything: a stale
        # persistent MSBuild node (kept alive by /nodeReuse:true, the default)
        # can wrongly report a XAML project as already up to date after this
        # script deletes and regenerates its source tree, skipping the
        # XAML-to-.g.cs markup compile step entirely and failing with
        # CS2001 "Source file '...\Shell.g.cs' could not be found." -nodeReuse:false
        # below additionally stops each of these builds from creating a NEW
        # persistent node that could cause the same problem on a future run.
        & dotnet build-server shutdown *> $null
        foreach ($project in $projects) {
            Invoke-Step "dotnet restore/build $($project.Name)" {
                Push-Location $project.Path
                try {
                    dotnet restore
                    if ($LASTEXITCODE -ne 0) { return }
                    dotnet build "$($project.Name).sln" -nodeReuse:false /p:UseSharedCompilation=false
                }
                finally {
                    Pop-Location
                }
            }
        }
    }
    else {
        Write-Host ""
        Write-Host "SKIPPED: dotnet restore/build (-SkipDotnet passed)" -ForegroundColor Yellow
    }

    Write-Host ""
    Write-Host "Northwind Trading generated at $outputRootFull" -ForegroundColor Green

    if ($Run) {
        if ($SkipDotnet) {
            throw "-Run requires a built solution; drop -SkipDotnet."
        }

        # Every generated host binds Kestrel's own default (http://localhost:5000) when
        # nothing else says otherwise -- none of the generated projects ship a
        # launchSettings.json or --urls override. Running more than one host at once
        # without giving each its own port means only the first process to start
        # actually binds; every other one throws "address already in use" and dies
        # immediately after "Now listening on..." never prints. ApiUrl points
        # HTTP-client apps (Desktop/AppBlazor) at their own backing WebApi's port via
        # ASPGENT_API_URL, which those templates already read (default http://localhost:5000).
        $hosts = @(
            @{ Label = "CatalogApi WebApi";        Project = Join-Path $catalogApi "src\WebApi"; Url = "http://localhost:5000" }
            @{ Label = "NorthwindOps WebApi";       Project = Join-Path $northwindOps "src\WebApi"; Url = "http://localhost:5010" }
            @{ Label = "NorthwindOps Desktop (WPF)"; Project = Join-Path $northwindOps "src\Desktop\NorthwindOps.Desktop.csproj"; ApiUrl = "http://localhost:5010" }
            @{ Label = "BillingLedger WebApi";      Project = Join-Path $billingLedger "src\WebApi"; Url = "http://localhost:5020" }
            @{ Label = "SalesPortal WebApi";        Project = Join-Path $salesPortal "src\WebApi"; Url = "http://localhost:5030" }
            @{ Label = "SalesPortal AppBlazor";     Project = Join-Path $salesPortal "src\SalesPortal.AppBlazor\SalesPortal.AppBlazor.csproj"; Url = "http://localhost:5031"; ApiUrl = "http://localhost:5030" }
            @{ Label = "WarehouseOps WebMvc";       Project = Join-Path $warehouseOps "src\WarehouseOps.WebMvc\WarehouseOps.WebMvc.csproj"; Url = "http://localhost:5040" }
        )

        Write-Host ""
        Write-Host "Launching every app in its own window (dotnet run), each on its own port ..." -ForegroundColor Cyan
        foreach ($h in $hosts) {
            # Use hashtable indexer access, not dot notation: Set-StrictMode -Version
            # Latest (top of this script) throws "The property '...' cannot be found on
            # this object" for dot access to a key a given $hosts entry doesn't define
            # (e.g. entries with no ApiUrl) -- indexer access returns $null instead.
            $portNote = if ($h['Url']) { $h['Url'] } else { "in-process, no listener of its own" }
            Write-Host "  - $($h['Label']): $($h['Project']) [$portNote]"
            $command = "Write-Host '$($h['Label'])' -ForegroundColor Cyan; "
            if ($h['ApiUrl']) { $command += "`$env:ASPGENT_API_URL = '$($h['ApiUrl'])'; " }
            $command += "dotnet run --project `"$($h['Project'])`""
            if ($h['Url']) { $command += " --urls $($h['Url'])" }
            Start-Process powershell -ArgumentList @("-NoExit", "-Command", $command)
        }

        Write-Host ""
        Write-Host "Seven windows opened, each on its own port (see the list above) so no host fights another for port 5000 -- give them a few seconds to start before calling any endpoint." -ForegroundColor Yellow
        Write-Host "See doc/aspgen-enterprise-developer-guide.md Section 12 for the exact endpoints/routes to verify (health checks, /api/... routes, /scalar/v1, /sales/orders, /inventory/stock-item) -- adjust the port in each URL to match the list above." -ForegroundColor Yellow
    }
    else {
        Write-Host "Pass -Run to launch every host/app in its own window, or see doc/aspgen-enterprise-developer-guide.md Section 12 to run them yourself." -ForegroundColor Yellow
    }
}
finally {
    Pop-Location
}
