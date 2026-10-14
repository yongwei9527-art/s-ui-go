param([Alias('lang','l')][string]$Language = 'en')
Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.ServiceProcess

function Assert-SafePath([string]$Path, [switch]$Tree) {
    if ($Path -notmatch '^[A-Za-z]:\\' -or $Path.Substring(2).Contains(':')) { throw "Not an absolute local path: $Path" }
    if ($Path -match '[<>|?*]' -or $Path -match '(\\|^)[^\\]*[. ](\\|$)') { throw "Ambiguous path refused: $Path" }
    $full = [IO.Path]::GetFullPath($Path).TrimEnd('\')
    if ($full.Length -le 3 -or $full -cne $Path.TrimEnd('\')) { throw "Noncanonical/dangerous path: $Path" }
    $cursor = $full
    while ($cursor) {
        if (Test-Path -LiteralPath $cursor) {
            $item = Get-Item -LiteralPath $cursor -Force
            if ($item.Attributes -band [IO.FileAttributes]::ReparsePoint) { throw "Reparse point refused: $cursor" }
        }
        $cursor = Split-Path -Path $cursor -Parent
    }
    if ($Tree -and (Test-Path -LiteralPath $full -PathType Container)) {
        # Walk one level at a time: never recurse through a junction before checking it.
        foreach ($child in Get-ChildItem -LiteralPath $full -Force) {
            Assert-SafePath $child.FullName
            if ($child.PSIsContainer) { Assert-SafePath $child.FullName -Tree }
        }
    }
    return
}
function Invoke-Checked([string]$Exe, [string[]]$Arguments) {
    & $Exe @Arguments
    if ($LASTEXITCODE -ne 0) { throw "$Exe failed ($LASTEXITCODE): $($Arguments[0])" }
}
function Stop-Confirmed {
    $service = Get-Service -Name 's-ui' -ErrorAction SilentlyContinue
    if ($null -ne $service) {
        if ($service.Status -ne 'Stopped') { Stop-Service -Name 's-ui' -ErrorAction Stop }
        $service.WaitForStatus([ServiceProcess.ServiceControllerStatus]::Stopped, [TimeSpan]::FromSeconds(30))
        $service.Refresh()
        if ($service.Status -ne 'Stopped') { throw 'Service did not stop; database untouched.' }
    }
    # Also reject manual/unmanaged instances, including inaccessible executable paths.
    foreach ($process in Get-Process -Name 'sui','s-ui-service' -ErrorAction SilentlyContinue) {
        throw "S-UI process still present (PID $($process.Id)); database untouched."
    }
}
function Protect-Directory([string]$Path) {
    Invoke-Checked 'icacls.exe' @($Path, '/inheritance:r', '/grant:r', '*S-1-5-18:(OI)(CI)F', '*S-1-5-32-544:(OI)(CI)F')
}
function Copy-Verified([string]$Source, [string]$Destination) {
    Assert-SafePath $Source
    Assert-SafePath $Destination
    Copy-Item -LiteralPath $Source -Destination $Destination -Force -ErrorAction Stop
    if ((Get-FileHash -LiteralPath $Source).Hash -ne (Get-FileHash -LiteralPath $Destination).Hash) { throw "Copy verification failed: $Destination" }
}
function Copy-Tree([string]$Source, [string]$Destination) {
    Assert-SafePath $Source -Tree
    Assert-SafePath $Destination -Tree
    if (-not (Test-Path -LiteralPath $Destination)) { New-Item -ItemType Directory -Path $Destination | Out-Null }
    foreach ($item in Get-ChildItem -LiteralPath $Source -Force) {
        $target = Join-Path $Destination $item.Name
        if ($item.PSIsContainer) { Copy-Tree $item.FullName $target }
        else { Copy-Verified $item.FullName $target }
    }
}
function Assert-OriginalServiceState($Service) {
    if ($null -eq $Service) { return }
    if ($Service.State -notin @('Running','Stopped')) { throw "Existing service is in an unstable state: $($Service.State)" }
    if ($Service.State -eq 'Stopped' -and ($Service.ExitCode -ne 0 -or $Service.ServiceSpecificExitCode -ne 0)) {
        throw 'Existing service has a failure exit code; repair it before upgrading.'
    }
}
function Get-StabilityTime { return [Diagnostics.Stopwatch]::GetTimestamp() / [double][Diagnostics.Stopwatch]::Frequency }
function Wait-ServiceStable {
    $service = Get-Service -Name 's-ui' -ErrorAction Stop
    $service.WaitForStatus([ServiceProcess.ServiceControllerStatus]::Running, [TimeSpan]::FromSeconds(30))
    $started = Get-StabilityTime
    $originalPid = $null
    do {
        $service.Refresh()
        $state = Get-CimInstance Win32_Service -Filter "Name='s-ui'" -ErrorAction Stop
        if ($service.Status -ne 'Running' -or $null -eq $state -or $state.State -ne 'Running' -or
            $state.ProcessId -eq 0 -or $state.ExitCode -ne 0 -or $state.ServiceSpecificExitCode -ne 0) {
            throw 'Service failed during the 15-second stability check.'
        }
        if ($null -eq $originalPid) { $originalPid = $state.ProcessId }
        elseif ($state.ProcessId -ne $originalPid) { throw 'Service restarted during the 15-second stability check.' }
        if (((Get-StabilityTime) - $started) -ge 15) { break }
        Start-Sleep -Milliseconds 250
    } while ($true)
}
function Complete-ServiceUpgrade([bool]$PreserveStopped) {
    if ($PreserveStopped) {
        Stop-Confirmed
        Write-Host 'Upgrade completed; service remains stopped as before / 升级完成，服务保持原停止状态'
    } else {
        Start-Service -Name 's-ui' -ErrorAction Stop
        Wait-ServiceStable
        Write-Host 'Installation completed; service stable for 15 seconds / 安装完成，服务已稳定运行15秒'
    }
}

function Write-ServiceConfig([string]$Source, [string]$Destination) {
    Assert-SafePath $Source
    Assert-SafePath $Destination
    $xml = New-Object System.Xml.XmlDocument
    $xml.XmlResolver = $null
    $xml.Load($Source)
    if ($null -eq $xml.SelectSingleNode('/service')) { throw 'Invalid WinSW service XML.' }
    foreach ($old in @($xml.SelectNodes('/service/env[@name="SUI_REQUIRE_PROXY_CORES"]'))) {
        [void]$old.ParentNode.RemoveChild($old)
    }
    $envNode = $xml.CreateElement('env')
    $envNode.SetAttribute('name', 'SUI_REQUIRE_PROXY_CORES')
    $envNode.SetAttribute('value', '1')
    [void]$xml.DocumentElement.AppendChild($envNode)
    $xml.Save($Destination)
}
function Install-Sui {
    $install = 'C:\Program Files\s-ui'
    $source = $PSScriptRoot
    $backup = $null
    $changed = $false
    $serviceCreated = $false
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    if (-not ([Security.Principal.WindowsPrincipal]::new($identity)).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) { throw 'Run as administrator / 请以管理员身份运行' }
    Assert-SafePath $source -Tree
    Assert-SafePath $install -Tree
    if ($source -eq $install -or $source.StartsWith($install + '\', [StringComparison]::OrdinalIgnoreCase)) { throw 'Run from a separate extracted package, not the installation directory.' }
    $required = @('sui.exe','libcronet.dll','s-ui-windows.xml','s-ui-windows.bat','uninstall-windows.bat','uninstall-windows.ps1')
    foreach ($file in $required) {
        if (-not (Test-Path -LiteralPath (Join-Path $source $file) -PathType Leaf)) { throw "Missing package file: $file" }
    }
    $existingService = Get-CimInstance Win32_Service -Filter "Name='s-ui'"
    if ($null -ne $existingService -and $existingService.PathName.Trim('"') -ine "$install\s-ui-service.exe") { throw 'Existing service uses an unexpected executable; refusing upgrade.' }
    Assert-OriginalServiceState $existingService
    $preserveStopped = $null -ne $existingService -and $existingService.State -eq 'Stopped'
    try {
        Stop-Confirmed
        Assert-SafePath $install -Tree
        $upgrade = Test-Path -LiteralPath "$install\db\s-ui.db" -PathType Leaf
        if ($upgrade -and (Get-Item -LiteralPath "$install\db\s-ui.db").Length -eq 0) { throw 'Empty existing database; manual recovery required.' }
        # Fail closed on ambiguous DB state instead of creating/resetting an administrator.
        if (-not $upgrade -and (Test-Path -LiteralPath "$install\db") -and @(Get-ChildItem -LiteralPath "$install\db" -Force).Count -gt 0) { throw 'Existing DB directory has no s-ui.db; manual recovery required.' }
        if (Test-Path -LiteralPath $install) {
            $backup = "$install.backup-$([Guid]::NewGuid().ToString('N'))"
            Assert-SafePath $backup
            New-Item -ItemType Directory -Path $backup | Out-Null
            Protect-Directory $backup
            Copy-Tree $install $backup
            Write-Host "Recovery snapshot (keep private): $backup"
        }
        $changed = $true
        if (-not (Test-Path -LiteralPath $install)) { New-Item -ItemType Directory -Path $install | Out-Null }
        Protect-Directory $install
        foreach ($dir in @('db','logs','cert')) {
            if (-not (Test-Path -LiteralPath "$install\$dir")) { New-Item -ItemType Directory -Path "$install\$dir" | Out-Null }
            Protect-Directory "$install\$dir"
        }
        foreach ($file in $required + @('README.md','PROTOCOL_GUIDE.md')) {
            if (Test-Path -LiteralPath "$source\$file") { Copy-Verified "$source\$file" "$install\$file" }
        }
        if (-not (Test-Path -LiteralPath "$install\winsw.exe")) {
            $arch = $env:PROCESSOR_ARCHITECTURE
            if ($env:PROCESSOR_ARCHITEW6432) { $arch = $env:PROCESSOR_ARCHITEW6432 }
            $asset = switch ($arch) { 'ARM64' { 'arm64' } 'AMD64' { 'x64' } 'x86' { 'x86' } default { throw "Unsupported architecture: $arch" } }
            [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12
            Invoke-WebRequest "https://github.com/winsw/winsw/releases/latest/download/WinSW-$asset.exe" -OutFile "$install\winsw.exe" -UseBasicParsing
        }
        Copy-Verified "$install\winsw.exe" "$install\s-ui-service.exe"
        Write-ServiceConfig "$install\s-ui-windows.xml" "$install\s-ui-service.xml"
        Assert-SafePath $install -Tree
        Push-Location $install
        $oldDb = $env:SUI_DB_FOLDER
        try {
            $env:SUI_DB_FOLDER = "$install\db"
            Invoke-Checked "$install\sui.exe" @('migrate')
            if (-not $upgrade) {
                $username = Read-Host 'Admin username (default admin) / 管理员用户名'
                if (-not $username) { $username = 'admin' }
                $password = Read-Host 'Admin password / 管理员密码' -AsSecureString
                if ($password.Length -eq 0) { throw 'Password cannot be empty.' }
                $ptr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($password)
                try { Invoke-Checked "$install\sui.exe" @('admin','-username',$username,'-password',[Runtime.InteropServices.Marshal]::PtrToStringBSTR($ptr)) }
                finally { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($ptr) }
            } else { Write-Host 'Existing DB: administrator and panel settings preserved / 保留管理员和面板设置' }
        } finally { $env:SUI_DB_FOLDER = $oldDb; Pop-Location }
        if ($null -eq $existingService) {
            $serviceCreated = $true
            Invoke-Checked "$install\s-ui-service.exe" @('install')
        }
        Complete-ServiceUpgrade $preserveStopped
    } catch {
        $failure = $_
        if ($changed) {
            try {
                # Never overwrite DB if stop/confirmation fails, even during rollback.
                Stop-Confirmed
                Assert-SafePath $install -Tree
                if ($serviceCreated -and (Get-Service 's-ui' -ErrorAction SilentlyContinue)) { Invoke-Checked "$install\s-ui-service.exe" @('uninstall') }
                if ($backup) {
                    Assert-SafePath $backup -Tree
                    # Preserve failed installation too; no recursive deletion of old data.
                    $failed = "$install.failed-$([Guid]::NewGuid().ToString('N'))"
                    Assert-SafePath $failed
                    Move-Item -LiteralPath $install -Destination $failed
                    Copy-Tree $backup $install
                    Write-Host "Restored old files; service left stopped. Snapshot: $backup; failed files: $failed"
                }
            } catch { Write-Warning "Recovery failed; do not restart. Old snapshot retained at $backup. $($_.Exception.Message)" }
        }
        throw $failure
    }
}
# Dot sourcing exposes functions for isolated mocks without invoking the installer.
if ($MyInvocation.InvocationName -ne '.') {
    try { Install-Sui; exit 0 } catch { Write-Error $_ -ErrorAction Continue; exit 1 }
}
