param(
    [Alias('InstallDirectory')][string]$InstallDir = '',
    [switch]$RemoveData,
    [switch]$ServiceOnly,
    [switch]$NonInteractive
)
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.ServiceProcess

function Assert-UninstallAdmin {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    if (-not ([Security.Principal.WindowsPrincipal]::new($identity)).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
        throw 'Run as administrator / 请以管理员身份运行。'
    }
}
function Assert-UninstallPath([string]$Path, [switch]$Tree) {
    if ($Path -notmatch '^[A-Za-z]:\\' -or $Path.Substring(2).Contains(':') -or
        $Path -match '[<>|?*]' -or $Path -match '(\\|^)[^\\]*[. ](\\|$)') { throw "Unsafe local path: $Path" }
    $full = [IO.Path]::GetFullPath($Path).TrimEnd('\')
    if ($full.Length -le 3 -or $full -cne $Path.TrimEnd('\')) { throw "Noncanonical/root path refused: $Path" }
    $cursor = $full
    while ($cursor) {
        if (Test-Path -LiteralPath $cursor) {
            $item = Get-Item -LiteralPath $cursor -Force
            if ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw "Reparse point refused: $cursor" }
        }
        $cursor = Split-Path -Path $cursor -Parent
    }
    if ($Tree -and (Test-Path -LiteralPath $full -PathType Container)) {
        foreach ($child in Get-ChildItem -LiteralPath $full -Force) {
            Assert-UninstallPath $child.FullName
            if ($child.PSIsContainer) { Assert-UninstallPath $child.FullName -Tree }
        }
    }
}
function Get-UninstallOwnedFiles {
    return @('sui.exe','libcronet.dll','s-ui-service.exe','s-ui-service.xml','winsw.exe',
        's-ui-windows.xml','s-ui-windows.env','install-windows.bat','install-windows.ps1',
        'uninstall-windows.bat','uninstall-windows.ps1','s-ui-windows.bat',
        'README.md','PROTOCOL_GUIDE.md','RELEASE_NOTES.md','XRAY-CORE-NOTICE.md')
}
function Assert-UninstallTarget([string]$Directory, [switch]$DeleteData) {
    Assert-UninstallPath $Directory -Tree
    # An installation is a dedicated directory, never a drive/system/user root
    # or a source checkout, even if environment/configuration was corrupted.
    if ((Split-Path $Directory -Leaf) -notmatch '^s-ui(?:[-_][A-Za-z0-9._-]+)?$' -or
        (Test-Path -LiteralPath (Join-Path $Directory '.git'))) { throw "Not a dedicated S-UI installation: $Directory" }
    if (-not (Test-Path -LiteralPath $Directory -PathType Container)) { throw "Installation directory not found: $Directory" }
    $config = Join-Path $Directory 's-ui-windows.env'
    if (Test-Path -LiteralPath $config) {
        if (-not (Test-Path -LiteralPath $config -PathType Leaf)) { throw 'Invalid installation configuration file.' }
        foreach ($line in Get-Content -LiteralPath $config) {
            # Read data only; never execute configuration as a script.
            if ($line -match '^INSTALL_DIR=(.*)$' -and $Matches[1] -ine $Directory) { throw 'Configured installation directory does not match the selected target.' }
            if ($line -match '^SERVICE_NAME=(.*)$' -and $Matches[1] -ine 's-ui') { throw 'Unexpected configured service identity.' }
        }
    }
    $xmlPath = Join-Path $Directory 's-ui-service.xml'
    if (-not (Test-Path -LiteralPath $xmlPath -PathType Leaf) -or
        -not (Test-Path -LiteralPath (Join-Path $Directory 'sui.exe') -PathType Leaf)) { throw 'Missing installed program/service markers; refusing deletion.' }
    $settings = New-Object Xml.XmlReaderSettings
    $settings.DtdProcessing = [Xml.DtdProcessing]::Prohibit
    $settings.XmlResolver = $null
    $reader = [Xml.XmlReader]::Create($xmlPath, $settings)
    try { $xml = New-Object Xml.XmlDocument; $xml.XmlResolver = $null; $xml.Load($reader) } finally { $reader.Dispose() }
    $ids = @($xml.SelectNodes('/service/id')); $executables = @($xml.SelectNodes('/service/executable'))
    if ($ids.Count -ne 1 -or $executables.Count -ne 1 -or $ids[0].InnerText -ine 's-ui') { throw 'Unexpected service XML identity.' }
    $program = $executables[0].InnerText.Replace('%BASE%', $Directory)
    if ($program -ine (Join-Path $Directory 'sui.exe')) { throw 'Service XML executable does not belong to the selected installation.' }
    $owned = @(Get-UninstallOwnedFiles)
    foreach ($item in Get-ChildItem -LiteralPath $Directory -Force) {
        if ($owned -contains $item.Name -and $item.PSIsContainer) { throw "Expected installed file is a directory: $($item.Name)" }
        if ($DeleteData -and $owned -notcontains $item.Name -and @('db','logs','cert') -notcontains $item.Name) {
            throw "Unknown file/directory would be deleted: $($item.Name). Move it out or keep data."
        }
        if (@('db','logs','cert') -contains $item.Name -and -not $item.PSIsContainer) { throw "Expected data directory is a file: $($item.Name)" }
    }
}
function Get-UninstallServiceState {
    return Get-CimInstance Win32_Service -Filter "Name='s-ui'" -ErrorAction Stop
}
function Assert-UninstallServiceMatch($State, [string]$Directory) {
    if ($null -eq $State) { return }
    if ($State.PathName.Trim('"') -ine (Join-Path $Directory 's-ui-service.exe')) { throw 'Existing service belongs to a different installation; refusing to stop or delete it.' }
}
function Stop-UninstallService([string]$Directory) {
    Assert-UninstallServiceMatch (Get-UninstallServiceState) $Directory
    $service = Get-Service -Name 's-ui' -ErrorAction SilentlyContinue
    if ($null -ne $service) {
        if ($service.Status -ne 'Stopped') { Stop-Service -Name 's-ui' -ErrorAction Stop }
        $service.WaitForStatus([ServiceProcess.ServiceControllerStatus]::Stopped, [TimeSpan]::FromSeconds(30))
        $service.Refresh()
        if ($service.Status -ne 'Stopped') { throw 'Service did not stop; no files removed.' }
    }
    foreach ($process in Get-Process -Name 'sui','s-ui-service' -ErrorAction SilentlyContinue) {
        throw "S-UI process still present (PID $($process.Id)); no files removed."
    }
    $state = Get-UninstallServiceState
    Assert-UninstallServiceMatch $state $Directory
    if ($null -ne $state -and ($state.State -ne 'Stopped' -or $state.ProcessId -ne 0)) { throw 'Service stop not confirmed; no files removed.' }
}
function Invoke-UninstallServiceDelete {
    # Do not execute an installation-controlled EXE while elevated.
    & (Join-Path $env:SystemRoot 'System32\sc.exe') delete 's-ui'
    if ($LASTEXITCODE -ne 0) { throw "Service deletion failed ($LASTEXITCODE); no files removed." }
}
function Remove-UninstallService([string]$Directory) {
    $state = Get-UninstallServiceState
    Assert-UninstallServiceMatch $state $Directory
    if ($null -eq $state) { return }
    if ($state.State -ne 'Stopped' -or $state.ProcessId -ne 0) { throw 'Service restarted before deletion; no files removed.' }
    Invoke-UninstallServiceDelete
    for ($i = 0; $i -lt 40; $i++) {
        if ($null -eq (Get-UninstallServiceState)) { return }
        Start-Sleep -Milliseconds 250
    }
    throw 'Service deletion is still pending; no files removed.'
}
function Remove-UninstallFiles([string]$Directory, [bool]$DeleteData) {
    # Recheck the resolved absolute target immediately before any removal, in
    # this same PowerShell process. Nothing is enumerated then deleted by cmd.
    Assert-UninstallTarget $Directory -DeleteData:$DeleteData
    if ($DeleteData) {
        Remove-Item -LiteralPath $Directory -Recurse -Force -ErrorAction Stop
    } else {
        foreach ($name in Get-UninstallOwnedFiles) {
            $file = Join-Path $Directory $name
            if (Test-Path -LiteralPath $file -PathType Leaf) {
                Assert-UninstallPath $file
                Remove-Item -LiteralPath $file -Force -ErrorAction Stop
            }
        }
    }
}
function Uninstall-Sui([string]$Directory, [bool]$DeleteData, [bool]$OnlyService, [bool]$Unattended) {
    Assert-UninstallAdmin
    if ($OnlyService -and $DeleteData) { throw 'ServiceOnly cannot be combined with RemoveData.' }
    if (-not $Directory) {
        $Directory = $env:SUI_HOME
        if (-not $Directory) { $Directory = 'C:\Program Files\s-ui' }
    }
    # Do not canonicalize away suspicious .. segments before validating them.
    Assert-UninstallTarget $Directory -DeleteData:$DeleteData
    Write-Host "Selected installation / 目标安装目录：$Directory"
    if (-not $OnlyService -and -not $Unattended -and -not $DeleteData) {
        $answer = Read-Host 'Enter/Y = keep DB, logs, certificates / 保留数据；输入 DELETE 才删除全部数据'
        switch -CaseSensitive ($answer) {
            '' { }
            'Y' { }
            'y' { }
            'DELETE' { $DeleteData = $true }
            default { throw 'Unknown confirmation; nothing changed / 未知确认，未进行卸载。' }
        }
    }
    Assert-UninstallTarget $Directory -DeleteData:$DeleteData
    Stop-UninstallService $Directory
    # Stop/delete matching service first; any error leaves DB and certificates
    # untouched. The process list is checked again before removing files.
    Remove-UninstallService $Directory
    Stop-UninstallService $Directory
    if (-not $OnlyService) { Remove-UninstallFiles $Directory $DeleteData }
    if ($OnlyService) { Write-Host 'Service removed; installed files retained / 服务已移除，文件保留。' }
    elseif ($DeleteData) { Write-Host 'Uninstall completed / 卸载完成。' }
    else { Write-Host "Uninstall completed; DB/logs/certificates retained / 数据保留在：$Directory" }
}
# Dot sourcing is for isolated tests; never invoke a real uninstallation there.
if ($MyInvocation.InvocationName -ne '.') {
    try { Uninstall-Sui $InstallDir ([bool]$RemoveData) ([bool]$ServiceOnly) ([bool]$NonInteractive); exit 0 }
    catch { Write-Error $_ -ErrorAction Continue; exit 1 }
}