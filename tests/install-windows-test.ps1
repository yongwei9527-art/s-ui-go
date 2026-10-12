# Repeatable, dependency-free tests for Windows PowerShell 5.1. No real service calls.
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$installer = Join-Path (Split-Path $PSScriptRoot -Parent) 'install-windows.ps1'
$errors = $null; $tokens = $null
[void][Management.Automation.Language.Parser]::ParseFile($installer, [ref]$tokens, [ref]$errors)
if ($errors.Count) { throw ($errors | Out-String) }
foreach ($file in @($installer, $PSCommandPath)) {
    $bytes = [IO.File]::ReadAllBytes($file)
    if ($bytes[0] -ne 239 -or $bytes[1] -ne 187 -or $bytes[2] -ne 191) { throw "Missing UTF-8 BOM: $file" }
}
. $installer
function Assert($Condition, [string]$Message) { if (-not $Condition) { throw $Message } }
function Expect-Failure([scriptblock]$Action, [string]$Pattern) {
    $caught = $false
    try { & $Action } catch { $caught = $true; Assert ($_.Exception.Message -like $Pattern) "Unexpected failure: $_" }
    Assert $caught "Expected failure: $Pattern"
}
# Every service/process API used by the installer is shadowed before any test runs.
$script:mode = 'stable'; $script:time = 0; $script:samples = 0; $script:starts = 0; $script:stops = 0
function Get-Service {
    if ($script:mode -eq 'absent' -or $script:mode -eq 'unmanaged') { return $null }
    $obj = [pscustomobject]@{Status='Running'}
    if ($script:mode -eq 'stopped') { $obj.Status = 'Stopped' }
    $obj | Add-Member ScriptMethod WaitForStatus { param($Status,$Timeout) } -PassThru |
        Add-Member ScriptMethod Refresh { } -PassThru
}
function Stop-Service { $script:stops++; if ($script:mode -eq 'stop-fail') { throw 'mock stop failure' } }
function Start-Service { $script:starts++; if ($script:mode -eq 'start-fail') { throw 'mock start failure' } }
function Get-Process { if ($script:mode -eq 'unmanaged') { [pscustomobject]@{Id=123} } }
function Get-CimInstance {
    $script:samples++
    $state = [pscustomobject]@{State='Running'; ProcessId=123; ExitCode=0; ServiceSpecificExitCode=0}
    if ($script:time -ge 7) {
        if ($script:mode -eq 'crash') { $state.State = 'Stopped'; $state.ExitCode = 1 }
        if ($script:mode -eq 'restart') { $state.ProcessId = 456 }
    }
    return $state
}
function Get-StabilityTime { return $script:time }
function Start-Sleep { param($Milliseconds) $script:time += $Milliseconds / 1000.0 }
function Reset-Mock([string]$Mode) { $script:mode=$Mode; $script:time=0; $script:samples=0; $script:starts=0; $script:stops=0 }
$root = Join-Path $PSScriptRoot ('installer-test-' + [guid]::NewGuid().ToString('N'))
try {
    New-Item -ItemType Directory -Path "$root\old" | Out-Null
    Set-Content -LiteralPath "$root\old\s-ui.db" -Value 'old-admin'
    Set-Content -LiteralPath "$root\old\s-ui.db-wal" -Value 'wal'
    Set-Content -LiteralPath "$root\old\sui.exe" -Value 'old-binary'
    Copy-Tree "$root\old" "$root\snapshot"
    Assert ((Get-Content "$root\snapshot\s-ui.db") -eq 'old-admin') 'DB snapshot mismatch'
    Assert (Test-Path "$root\snapshot\s-ui.db-wal") 'Missing WAL snapshot'
    foreach ($path in @('db','C:\','C:\temp\..\db','C:\temp:stream','C:\temp.\db')) {
        Expect-Failure { Assert-SafePath $path } '*path*'
    }
    Expect-Failure { Copy-Verified "$root\missing" "$root\snapshot\sui.exe" } '*'
    Reset-Mock 'stop-fail'
    Expect-Failure { Stop-Confirmed } '*mock stop failure*'
    Assert ((Get-Content "$root\old\s-ui.db") -eq 'old-admin') 'Stop failure altered DB'
    Reset-Mock 'unmanaged'; Expect-Failure { Stop-Confirmed } '*still present*'
    Reset-Mock 'stable'; Complete-ServiceUpgrade $false
    Assert ($script:time -ge 15 -and $script:samples -ge 61 -and $script:starts -eq 1) 'Not a full 15-second check'
    Reset-Mock 'crash'; Expect-Failure { Complete-ServiceUpgrade $false } '*stability check*'
    Reset-Mock 'restart'; Expect-Failure { Complete-ServiceUpgrade $false } '*restarted*'
    Reset-Mock 'start-fail'; Expect-Failure { Complete-ServiceUpgrade $false } '*start failure*'
    Reset-Mock 'stopped'; Complete-ServiceUpgrade $true
    Assert ($script:starts -eq 0 -and $script:time -eq 0) 'Originally stopped service was started'
    Assert-OriginalServiceState ([pscustomobject]@{State='Stopped';ExitCode=0;ServiceSpecificExitCode=0})
    Expect-Failure { Assert-OriginalServiceState ([pscustomobject]@{State='Stopped';ExitCode=1;ServiceSpecificExitCode=0}) } '*failure exit code*'
    Expect-Failure { Assert-OriginalServiceState ([pscustomobject]@{State='Start Pending';ExitCode=0;ServiceSpecificExitCode=0}) } '*unstable state*'
    $template = Join-Path (Split-Path $PSScriptRoot -Parent) 's-ui-windows.xml'
    $hash = (Get-FileHash $template).Hash
    Write-ServiceConfig $template "$root\service.xml"
    [xml]$xml = Get-Content -LiteralPath "$root\service.xml" -Raw
    $node = $xml.SelectSingleNode('/service/env[@name="SUI_REQUIRE_PROXY_CORES"]')
    Assert ($null -ne $node -and $node.GetAttribute('value') -eq '1') 'Missing strict core initialization environment'
    Assert ((Get-FileHash $template).Hash -eq $hash) 'Shared template was modified'
    # Check actual filesystem reparse refusal using an unprivileged NTFS junction.
    New-Item -ItemType Junction -Path "$root\link" -Target "$root\old" | Out-Null
    Expect-Failure { Assert-SafePath "$root\link" -Tree } '*Reparse point refused*'
    [IO.Directory]::Delete("$root\link")
    Write-Host 'PASS: snapshot/WAL, paths/reparse, copy/stop failures, unmanaged process, 15s stable/crash/restart, stopped state, failed existing service, instance-only core environment, BOM/parser'
} finally {
    if (Test-Path -LiteralPath "$root\link") { [IO.Directory]::Delete("$root\link") }
    $resolved = [IO.Path]::GetFullPath($root)
    if (-not $resolved.StartsWith($PSScriptRoot + '\installer-test-', [StringComparison]::OrdinalIgnoreCase)) { throw 'Unsafe test cleanup path' }
    if (Test-Path -LiteralPath $resolved) { Remove-Item -LiteralPath $resolved -Recurse -Force }
}