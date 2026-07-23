# Update the Xray-core source dependency compiled into S-UI.
# Xray publishes CalVer release tags (vYY.M.D) and mirrors each release to a
# Go-compatible SemVer tag (v1.YYMMDD.0). The update is accepted only if the
# focused runtime and subscription tests still pass.
param(
    [Parameter(Mandatory = $true)]
    [ValidatePattern('^v\d{2}\.\d{1,2}\.\d{1,2}$')]
    [string]$Version
)

$ErrorActionPreference = "Stop"
$match = [regex]::Match($Version, '^v(?<year>\d{2})\.(?<month>\d{1,2})\.(?<day>\d{1,2})$')
$year = [int]$match.Groups["year"].Value
$month = [int]$match.Groups["month"].Value
$day = [int]$match.Groups["day"].Value
if ($month -lt 1 -or $month -gt 12 -or $day -lt 1 -or $day -gt 31) {
    throw "Invalid Xray CalVer release: $Version"
}

$goVersion = "v1.{0:D2}{1:D2}{2:D2}.0" -f $year, $month, $day
Push-Location $PSScriptRoot
try {
    Write-Host "Updating embedded Xray-core source: $Version -> $goVersion" -ForegroundColor Cyan
    & go get "github.com/xtls/xray-core@$goVersion"
    if ($LASTEXITCODE -ne 0) { throw "go get failed for $goVersion" }

    & go mod tidy
    if ($LASTEXITCODE -ne 0) { throw "go mod tidy failed" }

    & go test ./... -count=1 -timeout=15m
    if ($LASTEXITCODE -ne 0) {
        throw "Xray source update tests failed; review go.mod/go.sum before committing"
    }

    & go vet ./...
    if ($LASTEXITCODE -ne 0) {
        throw "go vet failed after the Xray source update"
    }

    $productionTags = "with_quic,with_grpc,with_utls,with_acme,with_gvisor,with_tailscale,with_naive_outbound,with_purego"
    & go test -tags $productionTags ./internal/xrayruntime -run '^TestSingBoxAndEmbeddedXrayOperateTogether$' -count=1 -timeout=3m
    if ($LASTEXITCODE -ne 0) {
        throw "dual-core Reality integration test failed after the Xray source update"
    }

    Write-Host "Embedded Xray-core source updated and verified: $goVersion" -ForegroundColor Green
}
finally {
    Pop-Location
}
