# Download the pinned Xray-core binary used by the Linux amd64 release bundle.
# The archive is verified against the official .dgst SHA2-256 before extraction.
param(
    [string]$OutputDirectory = "release-assets\\xray-core-v26.3.27-linux-amd64"
)

$ErrorActionPreference = "Stop"
$version = "v26.3.27"
$asset = "Xray-linux-64.zip"
$expectedSha256 = "23cd9af937744d97776ee35ecad4972cf4b2109d1e0fe6be9930467608f7c8ae"
$archiveUrl = "https://github.com/XTLS/Xray-core/releases/download/$version/$asset"

New-Item -ItemType Directory -Path $OutputDirectory -Force | Out-Null
$archive = Join-Path $OutputDirectory $asset
$downloadRequired = -not (Test-Path -LiteralPath $archive) -or (Get-Item -LiteralPath $archive).Length -eq 0
if ($downloadRequired) {
    Remove-Item -LiteralPath $archive -Force -ErrorAction SilentlyContinue
    Write-Host "Downloading Xray-core $version Linux amd64 from the official release..." -ForegroundColor Cyan
    Invoke-WebRequest -Uri $archiveUrl -OutFile $archive
}

$actualSha256 = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant()
if ($actualSha256 -ne $expectedSha256) {
    Remove-Item -LiteralPath $archive -Force -ErrorAction SilentlyContinue
    throw "Xray archive SHA-256 verification failed. The invalid archive was removed. Expected $expectedSha256, got $actualSha256"
}

$extractDir = Join-Path $OutputDirectory "extracted"
if (Test-Path -LiteralPath $extractDir) { Remove-Item -LiteralPath $extractDir -Recurse -Force }
Expand-Archive -LiteralPath $archive -DestinationPath $extractDir -Force
$xray = Get-ChildItem -LiteralPath $extractDir -Recurse -File | Where-Object { $_.Name -eq "xray" } | Select-Object -First 1
if ($null -eq $xray) { throw "The official Xray archive did not contain the xray executable" }
Copy-Item -LiteralPath $xray.FullName -Destination (Join-Path $OutputDirectory "xray") -Force

Write-Host "Verified and extracted: $(Join-Path $OutputDirectory 'xray')" -ForegroundColor Green
Write-Host "Xray-core version: $version" -ForegroundColor Green
Write-Host "SHA-256: $actualSha256" -ForegroundColor Green
