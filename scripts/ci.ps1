<#
.SYNOPSIS
    Local CI/production verification pipeline for aspgen.

.DESCRIPTION
    Mirrors .github/workflows/ci.yml (and, with -Release, release.yml) so the
    same checks that gate a PR/tag can be run locally before pushing:
      1. go build ./cmd/... ./internal/...
      2. go vet   ./cmd/... ./internal/...
      3. go test  ./cmd/... ./internal/... -race -count=1
      4. goreleaser check (skipped if goreleaser isn't installed)
      5. goreleaser release --snapshot --clean --skip=publish (only with -Release)

    Always scopes Go commands to ./cmd/... ./internal/... — a bare ./...
    would also pick up non-compiling example/asset .go files under
    agent/skills/**/assets and .claude/skills/**.

.PARAMETER Release
    Also run the release-build verification step (goreleaser snapshot build).
    Slower; use this before cutting a real version tag.

.PARAMETER SkipRace
    Run tests without -race. Useful on machines without a working C
    toolchain/cgo, which -race requires.

.EXAMPLE
    ./scripts/ci.ps1
    ./scripts/ci.ps1 -Release
#>
[CmdletBinding()]
param(
    [switch]$Release,
    [switch]$SkipRace
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

try {
    Invoke-Step "go build ./cmd/... ./internal/..." {
        go build ./cmd/... ./internal/...
    }

    Invoke-Step "go vet ./cmd/... ./internal/..." {
        go vet ./cmd/... ./internal/...
    }

    if ($SkipRace) {
        Invoke-Step "go test ./cmd/... ./internal/... -count=1" {
            go test ./cmd/... ./internal/... -count=1
        }
    }
    else {
        Invoke-Step "go test ./cmd/... ./internal/... -race -count=1" {
            go test ./cmd/... ./internal/... -race -count=1
        }
    }

    if (Get-Command goreleaser -ErrorAction SilentlyContinue) {
        Invoke-Step "goreleaser check" {
            goreleaser check
        }

        if ($Release) {
            Invoke-Step "goreleaser release --snapshot --clean --skip=publish" {
                goreleaser release --snapshot --clean --skip=publish
            }
        }
    }
    else {
        Write-Host ""
        Write-Host "SKIPPED: goreleaser checks (goreleaser not found on PATH)" -ForegroundColor Yellow
    }

    Invoke-Step "go run ./cmd/aspgen version" {
        go run ./cmd/aspgen version
    }

    Write-Host ""
    Write-Host "All CI checks passed." -ForegroundColor Green
}
finally {
    Pop-Location
}
