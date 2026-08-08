$ErrorActionPreference = "Continue"

# 1. Build fresh binary (run from the aspgen repo root)
Set-Location C:\Users\Marko\Projects\aspgen
go build -o aspgen.exe ./cmd/aspgen
$exe = Join-Path (Get-Location) "aspgen.exe"

$root = Join-Path $env:TEMP "aspgen-combo"
if (Test-Path $root) { Remove-Item -Recurse -Force $root }

# NOTE: args are passed as an explicit array literal ('a','b',...) so
# PowerShell doesn't swallow the --flag tokens as function parameters.
function Invoke-Aspgen([string[]]$Arguments) {
    $out = & $exe $Arguments 2>&1 | Out-String
    [pscustomobject]@{ Code = $LASTEXITCODE; Output = $out.Trim() }
}
$failures = @()
function Check {
    param($Label, $Want, $Result)
    $ok = $Result.Code -eq $Want
    if (-not $ok) { $script:failures += $Label }
    "{0}  {1}  (exit {2}, want {3})" -f $(if ($ok) { "OK  " } else { "FAIL" }), $Label, $Result.Code, $Want
    if ($Result.Output) { ($Result.Output -split "`n") | ForEach-Object { "      $_" } }
}

# --- A: dm + WPF, optional many-to-one + many-to-many in ONE command ---
$p = "$root\A-dm-wpf"
Check "A new dm+wpf"      0 (Invoke-Aspgen ('new','A','--context','Blog','--arch','dm','-ui','wpf','--output',$p))
Check "A Customer"        0 (Invoke-Aspgen ('add','aggregate','Customer','name:string','--context','Blog','--project',$p))
Check "A Tag"             0 (Invoke-Aspgen ('add','aggregate','Tag','name:string','--context','Blog','--project',$p))
Check "A Post combo"      0 (Invoke-Aspgen ('add','aggregate','Post','title:string','body:string?','customer:Customer?','tags:Tag[]','--context','Blog','--project',$p))

# --- B: cqrs + WPF (HTTP) ---
$p = "$root\B-cqrs-wpf"
Check "B new cqrs+wpf"    0 (Invoke-Aspgen ('new','B','--context','Blog','--arch','cqrs','-ui','wpf','--output',$p))
Check "B Customer"        0 (Invoke-Aspgen ('add','aggregate','Customer','name:string','--context','Blog','--project',$p))
Check "B Tag"             0 (Invoke-Aspgen ('add','aggregate','Tag','name:string','--context','Blog','--project',$p))
Check "B Post"            0 (Invoke-Aspgen ('add','aggregate','Post','title:string','customer:Customer','tags:Tag[]','--context','Blog','--project',$p))

# --- C: cqrs + Blazor ---
$p = "$root\C-cqrs-blazor"
Check "C new cqrs+blazor" 0 (Invoke-Aspgen ('new','C','--context','Blog','--arch','cqrs','-ui','blazor','--output',$p))
Check "C Tag"             0 (Invoke-Aspgen ('add','aggregate','Tag','name:string','--context','Blog','--project',$p))
Check "C Post"            0 (Invoke-Aspgen ('add','aggregate','Post','title:string','tags:Tag[]','--context','Blog','--project',$p))

# --- D: dm + MVC ---
$p = "$root\D-dm-mvc"
Check "D new dm+mvc"      0 (Invoke-Aspgen ('new','D','--context','Blog','--arch','dm','-ui','mvc','--output',$p))
Check "D Tag"             0 (Invoke-Aspgen ('add','aggregate','Tag','name:string','--context','Blog','--project',$p))
Check "D Post"            0 (Invoke-Aspgen ('add','aggregate','Post','title:string','tags:Tag[]','--context','Blog','--project',$p))

# --- E: retrofit (aggregates first, WPF attached later) ---
$p = "$root\E-retrofit-wpf"
Check "E new (no ui)"     0 (Invoke-Aspgen ('new','E','--context','Blog','--arch','dm','--output',$p))
Check "E Tag"             0 (Invoke-Aspgen ('add','aggregate','Tag','name:string','--context','Blog','--project',$p))
Check "E Post"            0 (Invoke-Aspgen ('add','aggregate','Post','title:string','tags:Tag[]','--context','Blog','--project',$p))
Check "E add ui wpf"      0 (Invoke-Aspgen ('add','ui','wpf','--framework','wpf','--project',$p))

# --- F: es tier + WPF (event-sourced) ---
$p = "$root\F-es-wpf"
Check "F new es+wpf"      0 (Invoke-Aspgen ('new','F','--context','Blog','--arch','es','-ui','wpf','--output',$p))
Check "F Tag"             0 (Invoke-Aspgen ('add','aggregate','Tag','name:string','--context','Blog','--project',$p))
Check "F Post"            0 (Invoke-Aspgen ('add','aggregate','Post','title:string','tags:Tag[]','--context','Blog','--project',$p))

# --- G: negative cases -- these MUST fail (want exit 1) ---
$p = "$root\G-negatives"
Check "G new"             0 (Invoke-Aspgen ('new','G','--context','Blog','--arch','dm','--output',$p))
Check "G add context"     0 (Invoke-Aspgen ('add','context','Sales','--arch','dm','--project',$p))
Check "G Tag"             0 (Invoke-Aspgen ('add','aggregate','Tag','name:string','--context','Blog','--project',$p))
Check "G cross-context (EXPECTED error)" 1 (Invoke-Aspgen ('add','aggregate','Post','title:string','tags:Tag[]','--context','Sales','--project',$p))
Check "G unknown type (EXPECTED error)"  1 (Invoke-Aspgen ('add','aggregate','Post2','title:string','tags:DoesNotExist[]','--context','Blog','--project',$p))

# --- verify the pickers actually landed in each generated app ---
Write-Host "`n--- verify pickers ---"
$ver = @(
    @{ L="A dm+wpf dropdown+multiselect"; F="$root\A-dm-wpf\src\Desktop\Modules\Post\Views\PostView.xaml";         P=@("CustomerItems","TagOptions") },
    @{ L="B cqrs+wpf multiselect";        F="$root\B-cqrs-wpf\src\Desktop\Modules\Post\Views\PostView.xaml";       P=@("TagOptions") },
    @{ L="C blazor multiselect";          F="$root\C-cqrs-blazor\src\C.AppBlazor\Components\Pages\Blog\PostCrud.razor"; P=@("option.Selected","SyncTagsAsync") },
    @{ L="D mvc multiselect";             F="$root\D-dm-mvc\src\D.WebMvc\Views\Post\Create.cshtml";                P=@("selectedTagIds") },
    @{ L="E retrofit multiselect";        F="$root\E-retrofit-wpf\src\Desktop\Modules\Post\Views\PostView.xaml";  P=@("TagOptions") },
    @{ L="F es+wpf multiselect";          F="$root\F-es-wpf\src\Desktop\Modules\Post\Views\PostView.xaml";        P=@("TagOptions") }
)
foreach ($v in $ver) {
    $text = Get-Content -Raw $v.F -ErrorAction SilentlyContinue
    $missing = @($v.P | Where-Object { -not $text.Contains($_) })
    if ($missing.Count -eq 0) { Write-Host "OK   $($v.L)" } else { Write-Host "FAIL $($v.L) missing: $($missing -join ', ')"; $script:failures += $v.L }
}

#if ($failures.Count -eq 0) { Write-Host "`nALL CHECKS PASSED" } else { Write-Host "`nFAILURES: $($failures -join ', ')" }

if ($failures.Count -eq 0) {
    Write-Host "`nALL CHECKS PASSED"
    Write-Host "Opening: $root"
    explorer $root
} else {
    Write-Host "`nFAILURES: $($failures -join ', ')"
}