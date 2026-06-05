# S-UI legacy Windows build wrapper
# Prefer the repository root build-windows.ps1. This wrapper keeps the old windows\ path usable.
param(
    [ValidateSet('windows', 'linux', 'darwin')]
    [string]$System = 'windows',

    [ValidateSet('amd64', '386', 'arm64', 'arm')]
    [string]$Architecture = 'amd64',

    [switch]$NoCGO,
    [switch]$SkipFrontend,
    [switch]$Package,
    [switch]$NonInteractive,
    [switch]$ListCleanCandidates,
    [switch]$Help
)

$repoRoot = Split-Path -Parent $PSScriptRoot
$rootScript = Join-Path $repoRoot 'build-windows.ps1'

if (!(Test-Path $rootScript)) {
    Write-Host 'Error: repository root build-windows.ps1 was not found.' -ForegroundColor Red
    exit 1
}

$ErrorActionPreference = 'Stop'

$params = @{
    System = $System
    Architecture = $Architecture
}
if ($NoCGO) { $params.NoCGO = $true }
if ($SkipFrontend) { $params.SkipFrontend = $true }
if ($Package) { $params.Package = $true }
if ($NonInteractive) { $params.NonInteractive = $true }
if ($ListCleanCandidates) { $params.ListCleanCandidates = $true }
if ($Help) { $params.Help = $true }

Push-Location $repoRoot
try {
    & $rootScript @params
    exit $LASTEXITCODE
} catch {
    Write-Host $_ -ForegroundColor Red
    exit 1
} finally {
    Pop-Location
}
