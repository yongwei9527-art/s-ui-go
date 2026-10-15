$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.IO.Compression
Add-Type -AssemblyName System.IO.Compression.FileSystem
$root = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$source = [IO.File]::ReadAllText((Join-Path $root 'build-windows.ps1'))
$tokens = $null; $parseErrors = $null
$ast = [Management.Automation.Language.Parser]::ParseInput($source, [ref]$tokens, [ref]$parseErrors)
if ($parseErrors.Count) { throw 'Cannot parse packaging script.' }
# Load only validation functions, never the build/install script's top-level code.
foreach ($name in @('Test-PackageArchive', 'Test-LinuxPackageArchive')) {
    $nodes = @($ast.FindAll({ param($node) $node -is [Management.Automation.Language.FunctionDefinitionAst] -and $node.Name -eq $name }, $true))
    if ($nodes.Count -ne 1) { throw "Expected one function: $name" }
    Invoke-Expression $nodes[0].Extent.Text
}
$parent = [IO.Path]::GetFullPath((Join-Path $root '.verify-build'))
if ((Test-Path -LiteralPath $parent) -and ((Get-Item -LiteralPath $parent).Attributes -band [IO.FileAttributes]::ReparsePoint)) { throw 'Redirected fixture parent.' }
$fixture = Join-Path $parent ('package-license-' + [Guid]::NewGuid().ToString('N'))
[void](New-Item -ItemType Directory -Path $fixture)
$previous = $PWD.Path
$docs = @('README.md', 'RELEASE_NOTES.md', 'PROTOCOL_GUIDE.md', 'XRAY-CORE-NOTICE.md', 'LICENSE')
function New-FixtureZip([string]$Name, [bool]$IncludeLicense, [bool]$Tamper) {
    $path = Join-Path $fixture $Name
    $zip = [IO.Compression.ZipFile]::Open($path, [IO.Compression.ZipArchiveMode]::Create)
    try {
        foreach ($name in @('sui', 'install.sh', 'install-linux.sh', 'uninstall-linux.sh', 's-ui.sh') + $docs) {
            if ($name -eq 'LICENSE' -and -not $IncludeLicense) { continue }
            $stream = $zip.CreateEntry($name).Open()
            try {
                if ($name -eq 'sui') {
                    [byte[]]$bytes = New-Object byte[] 512
                    $bytes[0] = 0x7f; $bytes[1] = 0x45; $bytes[2] = 0x4c; $bytes[3] = 0x46; $bytes[18] = 0x3e
                } elseif ($name -in $docs) {
                    $bytes = [IO.File]::ReadAllBytes((Join-Path $root $name))
                    if ($Tamper -and $name -eq 'LICENSE') { $bytes = [Text.Encoding]::UTF8.GetBytes('truncated license') }
                } else { $bytes = [Text.Encoding]::UTF8.GetBytes("#!/bin/sh`nexit 0`n") }
                $stream.Write($bytes, 0, $bytes.Length)
            } finally { $stream.Dispose() }
        }
    } finally { $zip.Dispose() }
    return $path
}
function Assert-LicenseRejected([scriptblock]$Check) {
    $rejected = $false
    try { & $Check } catch { if ($_.Exception.Message -notmatch 'LICENSE') { throw }; $rejected = $true }
    if (-not $rejected) { throw 'Missing/tampered LICENSE was accepted.' }
}
try {
    Set-Location -LiteralPath $root
    $missing = New-FixtureZip 'missing.zip' $false $false
    Assert-LicenseRejected { Test-PackageArchive $missing 'linux' 'amd64' }
    Assert-LicenseRejected { Test-LinuxPackageArchive $missing 'amd64' }
    $tampered = New-FixtureZip 'tampered.zip' $true $true
    Assert-LicenseRejected { Test-PackageArchive $tampered 'linux' 'amd64' }
    $valid = New-FixtureZip 'valid.zip' $true $false
    Test-PackageArchive $valid 'linux' 'amd64'
    Test-LinuxPackageArchive $valid 'amd64'
    # Check all platform copy lists, and the independent CI document contract.
    if ([regex]::Matches($source, "foreach \(\`$document in @\([^\r\n]*'LICENSE'").Count -ne 2) { throw 'Windows/macOS copy lists must include LICENSE.' }
    $release = [IO.File]::ReadAllText((Join-Path $root '.github/workflows/release-linux-xray.yml'))
    if ($release -notmatch 'docs = \([^\r\n]*"LICENSE"') { throw 'Independent release verification must require LICENSE.' }
    Write-Host 'Package license regressions passed (5 archive checks + platform/CI contracts).'
} finally {
    Set-Location -LiteralPath $previous
    $resolved = [IO.Path]::GetFullPath($fixture)
    if (-not $resolved.StartsWith($parent + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) { throw 'Fixture cleanup escaped the audit directory.' }
    Remove-Item -LiteralPath $resolved -Recurse -Force
}
