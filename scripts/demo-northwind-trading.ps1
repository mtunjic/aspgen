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
    aspgen.exe. Does not shell out to `dotnet ef` and never seeds real data
    for you (see the guide's Section 6 "Seeding Catalog with real Northwind
    sample data" — that block is C# hand-added to Program.cs, not something
    a generator flag can produce).

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

try {
    $outputRootFull = [System.IO.Path]::GetFullPath($OutputRoot, $repoRoot)

    if (Test-Path $outputRootFull) {
        if (-not $Force) {
            throw "$outputRootFull already exists; pass -Force to delete and regenerate it."
        }
        Write-Host "Removing existing $outputRootFull ..." -ForegroundColor Yellow
        Remove-Item -Recurse -Force $outputRootFull
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
        foreach ($project in $projects) {
            Invoke-Step "dotnet restore/build $($project.Name)" {
                Push-Location $project.Path
                try {
                    dotnet restore
                    if ($LASTEXITCODE -ne 0) { return }
                    dotnet build "$($project.Name).sln"
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

        $hosts = @(
            @{ Label = "CatalogApi WebApi";        Project = Join-Path $catalogApi "src\WebApi" }
            @{ Label = "NorthwindOps WebApi";       Project = Join-Path $northwindOps "src\WebApi" }
            @{ Label = "NorthwindOps Desktop (WPF)"; Project = Join-Path $northwindOps "src\Desktop\NorthwindOps.Desktop.csproj" }
            @{ Label = "BillingLedger WebApi";      Project = Join-Path $billingLedger "src\WebApi" }
            @{ Label = "SalesPortal AppBlazor";     Project = Join-Path $salesPortal "src\SalesPortal.AppBlazor\SalesPortal.AppBlazor.csproj" }
            @{ Label = "WarehouseOps WebMvc";       Project = Join-Path $warehouseOps "src\WarehouseOps.WebMvc\WarehouseOps.WebMvc.csproj" }
        )

        Write-Host ""
        Write-Host "Launching every app in its own window (dotnet run) ..." -ForegroundColor Cyan
        foreach ($h in $hosts) {
            Write-Host "  - $($h.Label): $($h.Project)"
            Start-Process powershell -ArgumentList @(
                "-NoExit", "-Command",
                "Write-Host '$($h.Label)' -ForegroundColor Cyan; dotnet run --project `"$($h.Project)`""
            )
        }

        Write-Host ""
        Write-Host "Six windows opened. NorthwindOps' WebApi/Desktop pair and CatalogApi/BillingLedger each need their WebApi host running before Invoke-RestMethod calls or the Desktop shell will work — give them a few seconds to start." -ForegroundColor Yellow
        Write-Host "See doc/aspgen-enterprise-developer-guide.md Section 12 for the exact endpoints/routes to verify (health checks, /api/... routes, /scalar/v1, /sales/orders, /inventory/stock-item)." -ForegroundColor Yellow
    }
    else {
        Write-Host "Pass -Run to launch every host/app in its own window, or see doc/aspgen-enterprise-developer-guide.md Section 12 to run them yourself." -ForegroundColor Yellow
    }
}
finally {
    Pop-Location
}
