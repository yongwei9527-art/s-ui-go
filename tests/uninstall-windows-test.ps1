# Isolated Windows PowerShell 5.1 regression tests. Never touches real services.
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
$source = Join-Path (Split-Path $PSScriptRoot -Parent) 'uninstall-windows.ps1'
$errors=$null; $tokens=$null
[void][Management.Automation.Language.Parser]::ParseFile($source,[ref]$tokens,[ref]$errors)
if ($errors.Count) { throw ($errors | Out-String) }
. $source
function Assert($Condition,[string]$Message) { if (-not $Condition) { throw $Message } }
function Expect-Failure([scriptblock]$Action,[string]$Pattern) {
    $failed=$false
    try { & $Action } catch { $failed=$true; Assert ($_.Exception.Message -like $Pattern) "Unexpected failure: $_" }
    Assert $failed "Expected failure: $Pattern"
}
# Every service/admin API is mocked before any uninstall is exercised.
function Assert-UninstallAdmin { }
$script:exists=$true; $script:state='Running'; $script:mode='ok'; $script:stops=0; $script:deletes=0; $script:target=''
function Get-UninstallServiceState {
    if (-not $script:exists) { return $null }
    $exe = Join-Path $script:target 's-ui-service.exe'
    if ($script:mode -eq 'wrong-service') { $exe='C:\other\s-ui-service.exe' }
    return [pscustomobject]@{ PathName='"'+$exe+'"'; State=$script:state; ProcessId=$(if ($script:state -eq 'Stopped') {0} else {123}) }
}
function Get-Service {
    if (-not $script:exists) { return $null }
    $obj=[pscustomobject]@{ Status=$script:state }
    $obj | Add-Member ScriptMethod WaitForStatus { param($State,$Timeout) } -PassThru |
        Add-Member ScriptMethod Refresh { $this.Status=$script:state } -PassThru
}
function Stop-Service {
    $script:stops++
    if ($script:mode -eq 'stop-fail') { throw 'mock stop failure' }
    if ($script:mode -ne 'still-running') { $script:state='Stopped' }
}
function Get-Process { if ($script:mode -eq 'unmanaged') { [pscustomobject]@{Id=456} } }
function Invoke-UninstallServiceDelete {
    $script:deletes++
    if ($script:mode -eq 'delete-fail') { throw 'mock delete failure' }
    if ($script:mode -ne 'pending-delete') { $script:exists=$false }
}
function Start-Sleep { }
function Read-Host { return 'unknown-answer' }
function Reset-Mock([string]$Mode,[string]$Directory) {
    $script:mode=$Mode; $script:target=$Directory; $script:state='Running'; $script:exists=$true; $script:stops=0; $script:deletes=0
}
$root=Join-Path $PSScriptRoot ('uninstaller-test-'+[guid]::NewGuid().ToString('N'))
$resolved=[IO.Path]::GetFullPath($root)
if (-not $resolved.StartsWith($PSScriptRoot+'\uninstaller-test-',[StringComparison]::OrdinalIgnoreCase)) { throw 'Unsafe test root' }
function New-Fixture([string]$Name) {
    $directory=Join-Path $root ('s-ui-'+$Name)
    foreach ($part in @('db','logs','cert')) { New-Item -ItemType Directory -Path (Join-Path $directory $part) -Force | Out-Null }
    Set-Content -LiteralPath (Join-Path $directory 'sui.exe') -Value 'mock program'
    Set-Content -LiteralPath (Join-Path $directory 'db\s-ui.db') -Value 'existing administrator and nodes'
    Set-Content -LiteralPath (Join-Path $directory 'cert\private.key') -Value 'keep private key'
    Copy-Item -LiteralPath (Join-Path (Split-Path $PSScriptRoot -Parent) 's-ui-windows.xml') -Destination (Join-Path $directory 's-ui-service.xml')
    return $directory
}
$junction=''
try {
    foreach ($mode in @('stop-fail','still-running','unmanaged','delete-fail','pending-delete','wrong-service')) {
        $directory=New-Fixture $mode
        Reset-Mock $mode $directory
        $hash=(Get-FileHash -LiteralPath (Join-Path $directory 'db\s-ui.db')).Hash
        Expect-Failure { Uninstall-Sui $directory $true $false $true } '*'
        Assert (Test-Path -LiteralPath (Join-Path $directory 'sui.exe')) "$mode deleted the program"
        Assert ((Get-FileHash -LiteralPath (Join-Path $directory 'db\s-ui.db')).Hash -eq $hash) "$mode changed DB"
        Assert (Test-Path -LiteralPath (Join-Path $directory 'cert\private.key')) "$mode deleted key"
        if ($mode -eq 'wrong-service') { Assert ($script:stops -eq 0 -and $script:deletes -eq 0) 'Touched wrong service' }
    }
    $directory=New-Fixture 'unknown'
    Set-Content -LiteralPath (Join-Path $directory 'personal.txt') -Value 'unrelated file'
    Reset-Mock 'ok' $directory
    Expect-Failure { Uninstall-Sui $directory $true $false $true } '*Unknown file*'
    Assert ($script:stops -eq 0 -and (Test-Path -LiteralPath (Join-Path $directory 'personal.txt'))) 'Unknown content removed'
    Expect-Failure { Uninstall-Sui $directory $false $false $false } '*Unknown confirmation*'
    Assert ($script:stops -eq 0) 'Unknown confirmation stopped service'
    foreach ($path in @('db','C:\','C:\Windows','C:\temp\..\s-ui','C:\temp:stream','C:\temp.\s-ui')) {
        Expect-Failure { Assert-UninstallTarget $path -DeleteData } '*'
    }
    $directory=New-Fixture 'env-mismatch'
    Set-Content -LiteralPath (Join-Path $directory 's-ui-windows.env') -Value 'INSTALL_DIR=C:\other\s-ui'
    Reset-Mock 'ok' $directory
    Expect-Failure { Uninstall-Sui $directory $true $false $true } '*does not match*'
    Assert ($script:stops -eq 0) 'Environment redirected operation'
    $directory=New-Fixture 'xml-mismatch'
    [xml]$xml=Get-Content -LiteralPath (Join-Path $directory 's-ui-service.xml') -Raw
    $xml.service.executable='C:\other\sui.exe'; $xml.Save((Join-Path $directory 's-ui-service.xml'))
    Expect-Failure { Assert-UninstallTarget $directory } '*executable*'
    $directory=New-Fixture 'reparse'
    $junction=Join-Path $directory 'cert\link'
    New-Item -ItemType Junction -Path $junction -Target (Join-Path $directory 'db') | Out-Null
    Expect-Failure { Assert-UninstallTarget $directory -DeleteData } '*Reparse point*'
    [IO.Directory]::Delete($junction); $junction=''
    $directory=New-Fixture 'service-only'
    Reset-Mock 'ok' $directory
    Uninstall-Sui $directory $false $true $true
    Assert (-not $script:exists -and (Test-Path -LiteralPath (Join-Path $directory 'sui.exe'))) 'Service-only deleted files'
    $directory=New-Fixture 'keep'
    Reset-Mock 'ok' $directory
    Uninstall-Sui $directory $false $false $true
    Assert (-not (Test-Path -LiteralPath (Join-Path $directory 'sui.exe'))) 'Program not removed'
    Assert (Test-Path -LiteralPath (Join-Path $directory 'db\s-ui.db')) 'DB not kept'
    Assert (Test-Path -LiteralPath (Join-Path $directory 'cert\private.key')) 'Key not kept'
    $directory=New-Fixture 'remove'
    Reset-Mock 'ok' $directory
    Uninstall-Sui $directory $true $false $true
    Assert (-not (Test-Path -LiteralPath $directory)) 'Explicit data removal incomplete'
    Write-Host 'PASS: target/service identity, reparse, invalid confirmation, stop/process/delete failures, pending deletion, retain data, service-only, explicit removal'
} finally {
    if ($junction -and (Test-Path -LiteralPath $junction)) { [IO.Directory]::Delete($junction) }
    if (-not ([IO.Path]::GetFullPath($root)).StartsWith($PSScriptRoot+'\uninstaller-test-',[StringComparison]::OrdinalIgnoreCase)) { throw 'Unsafe cleanup target' }
    Assert-UninstallPath $root -Tree
    if (Test-Path -LiteralPath $root) { Remove-Item -LiteralPath $root -Recurse -Force }
}