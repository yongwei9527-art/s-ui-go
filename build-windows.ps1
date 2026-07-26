# S-UI 多系统构建脚本
param(
    [ValidateSet("windows", "linux", "darwin")]
    [string]$System = "windows",

    [ValidateSet("amd64", "386", "arm64", "arm")]
    [string]$Architecture = "amd64",

    [switch]$NoCGO,
    [switch]$SkipFrontend,
    [switch]$Package,
    [switch]$NonInteractive,
    [switch]$ListCleanCandidates,
    [switch]$Help
)

$ErrorActionPreference = 'Stop'

if ($Help) {
    Write-Host "用法："
    Write-Host "  .\build-windows.ps1 [-System <系统>] [-Architecture <架构>] [-NoCGO] [-SkipFrontend] [-Package] [-NonInteractive]"
    Write-Host "  .\build-windows.ps1 -ListCleanCandidates"
    Write-Host ""
    Write-Host "说明："
    Write-Host "  请优先使用仓库根目录下的 build-windows.ps1；windows\ 下脚本仅作旧路径/参考保留。"
    Write-Host ""
    Write-Host "系统："
    Write-Host "  windows  Windows"
    Write-Host "  linux    Linux"
    Write-Host "  darwin   macOS"
    Write-Host ""
    Write-Host "架构："
    Write-Host "  amd64    64 位 Intel/AMD"
    Write-Host "  arm64    ARM64"
    Write-Host "  386      32 位 x86，仅 Windows/Linux 常用"
    Write-Host "  arm      ARM 32 位，仅 Linux 常用"
    Write-Host ""
    Write-Host "参数："
    Write-Host "  -NoCGO                关闭 CGO，适合从 Windows 交叉构建 Linux/macOS 包"
    Write-Host "  -SkipFrontend         跳过前端构建；需要已有 frontend/dist 嵌入资源"
    Write-Host "  -Package              生成 dist/*.zip 发布压缩包"
    Write-Host "  Xray-core Go 源码依赖会编译进 sui；发布包不再附带独立 xray 文件"
    Write-Host "  -NonInteractive       构建完成后不等待输入"
    Write-Host "  -ListCleanCandidates  只列出本地可清理候选，不删除任何文件"
    Write-Host ""
    Write-Host "示例："
    Write-Host "  .\build-windows.ps1"
    Write-Host "  .\build-windows.ps1 -System windows -Architecture arm64 -Package"
    Write-Host "  .\build-windows.ps1 -System linux -Architecture amd64 -NoCGO -SkipFrontend -Package"
    Write-Host "  .\build-windows.ps1 -System darwin -Architecture arm64 -NoCGO -SkipFrontend -Package"
    Write-Host "  .\build-windows.ps1 -ListCleanCandidates"
    exit 0
}

function Stop-WithMessage($Message) {
    Write-Host "错误：$Message" -ForegroundColor Red
    if (!$NonInteractive) {
        Read-Host "按 Enter 退出 / Press Enter to exit"
    }
    exit 1
}

function Write-Sha256Sidecar([string]$ArchivePath) {
    $archive = Get-Item -LiteralPath $ArchivePath -ErrorAction Stop
    if (-not $archive.PSIsContainer) {
        $hash = (Get-FileHash -LiteralPath $archive.FullName -Algorithm SHA256 -ErrorAction Stop).Hash.ToLowerInvariant()
        $sidecarPath = "$($archive.FullName).sha256"
        if (Test-Path -LiteralPath $sidecarPath) {
            Remove-Item -LiteralPath $sidecarPath -Force -ErrorAction Stop
        }
        $sidecarContent = "$hash  $($archive.Name)`n"
        [System.IO.File]::WriteAllText($sidecarPath, $sidecarContent, [System.Text.Encoding]::ASCII)
        $recorded = [System.IO.File]::ReadAllText($sidecarPath, [System.Text.Encoding]::ASCII).Trim()
        if ($recorded -cne "$hash  $($archive.Name)") {
            throw "生成的 SHA256 文件格式无效：$sidecarPath"
        }
        Write-Host "已生成校验文件 / Checksum generated: $sidecarPath" -ForegroundColor Green
        return
    }
    throw "不能为目录生成 SHA256 文件：$ArchivePath"
}

function Copy-RequiredPackageFile([string]$Source, [string]$DestinationDirectory, [string]$DestinationName) {
    if (-not (Test-Path -LiteralPath $Source -PathType Leaf)) {
        throw "发布包缺少必需文件：$Source"
    }
    if ([string]::IsNullOrWhiteSpace($DestinationName)) {
        $DestinationName = [System.IO.Path]::GetFileName($Source)
    }
    $destination = Join-Path $DestinationDirectory $DestinationName
    Copy-Item -LiteralPath $Source -Destination $destination -Force -ErrorAction Stop
    if (-not (Test-Path -LiteralPath $destination -PathType Leaf)) {
        throw "必需文件复制失败：$Source -> $destination"
    }
}

function Test-LinuxPackageArchive([string]$ArchivePath, [string]$Architecture) {
    $archive = Get-Item -LiteralPath $ArchivePath -ErrorAction Stop
    Add-Type -AssemblyName System.IO.Compression.FileSystem -ErrorAction Stop
    $zip = [System.IO.Compression.ZipFile]::OpenRead($archive.FullName)
    try {
        $entries = @($zip.Entries)
        $fileNames = [System.Collections.Generic.List[string]]::new()
        foreach ($entry in $entries) {
            $entryName = $entry.FullName
            if ([string]::IsNullOrWhiteSpace($entryName) -or
                $entryName.IndexOf([char]0) -ge 0 -or
                $entryName.StartsWith('/') -or
                $entryName.StartsWith('\') -or
                $entryName.Contains('\') -or
                $entryName -match '^[A-Za-z]:') {
                throw "发布 ZIP 包含不安全路径：$entryName"
            }
            if (($entryName -split '/') -contains '..') {
                throw "发布 ZIP 包含路径遍历项：$entryName"
            }
            if ($entryName.EndsWith('/')) {
                throw "发布 ZIP 不应包含目录项：$entryName"
            }
            $unixFileType = (($entry.ExternalAttributes -shr 16) -band 0xF000)
            if ($unixFileType -eq 0xA000) {
                throw "发布 ZIP 不应包含符号链接项：$entryName"
            }
            if ($unixFileType -ne 0 -and $unixFileType -ne 0x8000) {
                throw "发布 ZIP 包含异常文件类型：$entryName"
            }
            [void]$fileNames.Add($entryName)
        }

        $duplicateEntries = @($fileNames | Group-Object | Where-Object { $_.Count -ne 1 })
        if ($duplicateEntries.Count -gt 0) {
            throw "发布 ZIP 包含重复文件项：$($duplicateEntries.Name -join ', ')"
        }

        $requiredFiles = @(
            'sui',
            'install.sh',
            'install-linux.sh',
            'uninstall-linux.sh',
            's-ui.sh',
            'README.md',
            'PROTOCOL_GUIDE.md',
            'RELEASE_NOTES.md',
            'XRAY-CORE-NOTICE.md'
        )
        foreach ($requiredFile in $requiredFiles) {
            if (@($fileNames | Where-Object { $_ -ceq $requiredFile }).Count -ne 1) {
                throw "发布 ZIP 缺少或重复必需文件：$requiredFile"
            }
        }

        $unexpectedEntries = @($fileNames | Where-Object { $requiredFiles -cnotcontains $_ })
        if ($unexpectedEntries.Count -gt 0) {
            throw "发布 ZIP 包含未声明的冗余文件：$($unexpectedEntries -join ', ')"
        }

        $forbiddenBasenames = @('xray', 'xray.exe', 'sui.exe', "sui-linux-$Architecture", "sui-linux-$Architecture.exe")
        $forbiddenEntries = @($fileNames | Where-Object {
            $baseName = ($_ -split '/')[-1].ToLowerInvariant()
            $forbiddenBasenames -contains $baseName
        })
        if ($forbiddenEntries.Count -gt 0) {
            throw "发布 ZIP 包含独立 Xray 或重复二进制：$($forbiddenEntries -join ', ')"
        }

        $staticUnitEntries = @($fileNames | Where-Object { (($_ -split '/')[-1]) -ieq 's-ui.service' })
        if ($staticUnitEntries.Count -gt 0) {
            throw "发布 ZIP 不应携带静态 s-ui.service：$($staticUnitEntries -join ', ')"
        }
    } finally {
        $zip.Dispose()
    }
    Write-Host 'Linux 发布 ZIP 文件集、结构和路径安全检查通过。' -ForegroundColor Green
}
function Test-Sha256Sidecar([string]$ArchivePath) {
    $archive = Get-Item -LiteralPath $ArchivePath -ErrorAction Stop
    $sidecarPath = "$($archive.FullName).sha256"
    if ($archive.PSIsContainer -or -not (Test-Path -LiteralPath $sidecarPath -PathType Leaf)) { throw "Missing SHA-256 sidecar: $sidecarPath" }
    $sidecar = [IO.File]::ReadAllText($sidecarPath, [Text.Encoding]::ASCII)
    $match = [regex]::Match($sidecar, "^([0-9a-fA-F]{64})  $([regex]::Escape($archive.Name))\r?\n$")
    if (-not $match.Success) { throw "Invalid SHA-256 sidecar format: $sidecarPath" }
    $actual = (Get-FileHash -LiteralPath $archive.FullName -Algorithm SHA256 -ErrorAction Stop).Hash.ToLowerInvariant()
    if ($match.Groups[1].Value.ToLowerInvariant() -cne $actual) { throw "SHA-256 validation failed: $($archive.Name)" }
}

function Test-PackageArchive([string]$ArchivePath, [string]$System, [string]$Architecture) {
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $allDocs = @('README.md','RELEASE_NOTES.md','PROTOCOL_GUIDE.md','XRAY-CORE-NOTICE.md')
    $definitions = @{
        windows = [pscustomobject]@{ Required = @('sui.exe','s-ui-windows.xml','install-windows.bat','uninstall-windows.bat','s-ui-windows.bat') + $allDocs; Optional = @('libcronet.dll'); Binary = 'sui.exe' }
        linux = [pscustomobject]@{ Required = @('sui','install.sh','install-linux.sh','uninstall-linux.sh','s-ui.sh') + $allDocs; Optional = @(); Binary = 'sui' }
        darwin = [pscustomobject]@{ Required = @('sui','install-macos.sh','uninstall-macos.sh','com.s-ui.plist') + $allDocs; Optional = @(); Binary = 'sui' }
    }
    $definition = $definitions[$System]; if ($null -eq $definition) { throw "Unsupported package system: $System" }
    $archive = Get-Item -LiteralPath $ArchivePath -ErrorAction Stop; if ($archive.PSIsContainer -or $archive.Length -le 0) { throw "Invalid ZIP archive: $ArchivePath" }
    $zip = [IO.Compression.ZipFile]::OpenRead($archive.FullName)
    try {
        $names = New-Object 'System.Collections.Generic.List[string]'
        foreach ($entry in $zip.Entries) {
            $name = $entry.FullName
            if ([string]::IsNullOrWhiteSpace($name) -or $name.EndsWith('/') -or $name.EndsWith('\') -or $name.IndexOf([char]0) -ge 0 -or $name.StartsWith('/') -or $name.StartsWith('\') -or $name.Contains('\') -or $name -match '^[A-Za-z]:' -or $name -match '(^|/)\.\.(/|$)') { throw "Unsafe or directory ZIP entry: $name" }
            $type = (([uint32]$entry.ExternalAttributes -shr 16) -band 0xF000); if ($type -eq 0xA000 -or ($type -ne 0 -and $type -ne 0x8000)) { throw "Unsupported ZIP entry type: $name" }
            [void]$names.Add($name)
        }
        $duplicates = @($names | Group-Object | Where-Object { $_.Count -ne 1 }); if ($duplicates.Count -gt 0) { throw "Duplicate ZIP entries: $($duplicates.Name -join ', ')" }
        foreach ($required in $definition.Required) { if (@($names | Where-Object { $_ -ceq $required }).Count -ne 1) { throw "Missing required ZIP entry: $required" } }
        $extra = @($names | Where-Object { (@($definition.Required) + @($definition.Optional)) -cnotcontains $_ }); if ($extra.Count -gt 0) { throw "Unexpected ZIP entries: $($extra -join ', ')" }
        if (@($names | Where-Object { @('xray','xray.exe') -contains (($_ -split '/')[-1]).ToLowerInvariant() }).Count -gt 0) { throw 'Standalone Xray binary is not allowed in a release ZIP.' }
        foreach ($doc in $allDocs) {
            $entry = $zip.GetEntry($doc); if ($null -eq $entry) { throw "Missing embedded document: $doc" }
            $memory = New-Object IO.MemoryStream; $stream = $entry.Open(); try { $stream.CopyTo($memory) } finally { $stream.Dispose() }
            $embeddedHash = [BitConverter]::ToString([Security.Cryptography.SHA256]::Create().ComputeHash($memory.ToArray())).Replace('-',''); $memory.Dispose()
            if ($embeddedHash -cne (Get-FileHash -LiteralPath $doc -Algorithm SHA256 -ErrorAction Stop).Hash) { throw "Embedded document differs from source: $doc" }
        }
        if ($Architecture -eq 'amd64') {
            $entry = $zip.GetEntry($definition.Binary); if ($null -eq $entry) { throw "Missing release binary: $($definition.Binary)" }
            [byte[]]$bytes = New-Object byte[] 512; $stream = $entry.Open(); try { $count = $stream.Read($bytes,0,$bytes.Length) } finally { $stream.Dispose() }; if ($count -lt 32) { throw "Release binary is too short: $($definition.Binary)" }
            switch ($System) {
                windows { $offset=[BitConverter]::ToInt32($bytes,0x3c); if ($bytes[0] -ne 0x4d -or $bytes[1] -ne 0x5a -or $offset -lt 0 -or ($offset+6) -gt $count -or $bytes[$offset] -ne 0x50 -or $bytes[$offset+1] -ne 0x45 -or [BitConverter]::ToUInt16($bytes,$offset+4) -ne 0x8664) { throw 'Windows binary is not amd64 PE.' } }
                linux { if ($bytes[0] -ne 0x7f -or $bytes[1] -ne 0x45 -or $bytes[2] -ne 0x4c -or $bytes[3] -ne 0x46 -or [BitConverter]::ToUInt16($bytes,18) -ne 0x003e) { throw 'Linux binary is not amd64 ELF.' } }
                darwin { if ($bytes[0] -ne 0xcf -or $bytes[1] -ne 0xfa -or $bytes[2] -ne 0xed -or $bytes[3] -ne 0xfe -or [BitConverter]::ToUInt32($bytes,4) -ne 0x01000007) { throw 'macOS binary is not amd64 Mach-O.' } }
            }
        }
    } finally { $zip.Dispose() }
    Write-Host "$System/$Architecture ZIP structure, documents, and binary format validated." -ForegroundColor Green
}
function Test-BuildModuleMetadata([string]$BinaryPath) {
    $metadata = @(& go version -m $BinaryPath)
    if ($LASTEXITCODE -ne 0) {
        throw "无法读取构建元数据：$BinaryPath"
    }
    $modulePath = (& go list -m -f '{{.Path}}').Trim()
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($modulePath)) {
        throw '无法读取当前 Go 模块路径。'
    }
    $expectedPathLine = "path`t$modulePath"
    if (@($metadata | Where-Object { $_.Trim() -ceq $expectedPathLine }).Count -ne 1) {
        throw "构建产物不是模块包 $modulePath；请使用 go build .，不要使用 go build main.go。"
    }
    $productModuleLines = @($metadata | Where-Object { $_.Trim().StartsWith("mod`t$modulePath`t", [System.StringComparison]::Ordinal) })
    if ($productModuleLines.Count -ne 1) {
        throw "构建产物缺少主模块元数据：$modulePath"
    }
    if ($productModuleLines[0] -match '(?:^|\t)v?1\.4(?:[.\-+]|$)') {
        throw "构建产物仍携带旧版 1.4 产品元数据：$($productModuleLines[0])"
    }

    $xrayModule = (& go list -m -f '{{.Path}} {{.Version}}' github.com/xtls/xray-core).Trim()
    if ($LASTEXITCODE -ne 0 -or [string]::IsNullOrWhiteSpace($xrayModule)) {
        throw '无法读取 github.com/xtls/xray-core 的模块版本。'
    }
    $xrayFields = @($xrayModule -split '\s+', 2)
    if ($xrayFields.Count -ne 2 -or [string]::IsNullOrWhiteSpace($xrayFields[0]) -or [string]::IsNullOrWhiteSpace($xrayFields[1])) {
        throw "Xray-core 模块元数据格式无效：$xrayModule"
    }
    $expectedXrayPrefix = "dep`t$($xrayFields[0])`t$($xrayFields[1])`t"
    if (@($metadata | Where-Object { $_.Trim().StartsWith($expectedXrayPrefix, [System.StringComparison]::Ordinal) }).Count -ne 1) {
        throw "构建产物缺少精确的 Xray-core 模块元数据：$($xrayFields[0]) $($xrayFields[1])。"
    }
    Write-Host "已验证模块构建元数据：$modulePath（不含旧版 1.4，包含 $($xrayFields[0]) $($xrayFields[1])）。" -ForegroundColor Green
}
function Write-CleanCandidate($Path, $Description, [switch]$Cautious) {
    if (Test-Path $Path) {
        $label = "可清理"
        $color = "Yellow"
        if ($Cautious) {
            $label = "谨慎检查"
            $color = "Magenta"
        }
        Write-Host "[$label] $Path - $Description" -ForegroundColor $color
    }
}

function Show-CleanCandidates {
    Write-Host "========================================" -ForegroundColor Green
    Write-Host "S-UI 本地清理候选清单" -ForegroundColor Green
    Write-Host "========================================" -ForegroundColor Green
    Write-Host "仅列出候选项，不会删除任何文件。请确认不再需要后再手动清理。" -ForegroundColor Cyan
    Write-Host ""

    Write-Host "构建二进制 / Build binaries:" -ForegroundColor Cyan
    Get-ChildItem -Path "." -File -ErrorAction SilentlyContinue |
        Where-Object { $_.Name -eq "sui" -or $_.Name -eq "sui.exe" -or $_.Name -like "sui-windows-*" -or $_.Name -like "sui-linux-*" -or $_.Name -like "sui-darwin-*" -or $_.Name -like "*.bak-*" } |
        ForEach-Object { Write-Host "[可清理] $($_.FullName) - 构建输出或备份文件" -ForegroundColor Yellow }

    Write-Host ""
    Write-Host "生成目录 / Generated directories:" -ForegroundColor Cyan
    Write-CleanCandidate "frontend\dist" "前端 Vite 构建输出"

    if (Test-Path "dist") {
        Get-ChildItem -Path "dist" -Directory -ErrorAction SilentlyContinue |
            Where-Object { $_.Name -like "s-ui-*" } |
            ForEach-Object { Write-Host "[可清理] $($_.FullName) - 发布包临时展开目录" -ForegroundColor Yellow }
    }

    Write-Host ""
    Write-Host "运行数据 / Runtime data:" -ForegroundColor Cyan
    Write-CleanCandidate "db" "本地 SQLite 运行数据，删除会丢失本机数据" -Cautious
    Write-CleanCandidate "dev-db" "本地开发 SQLite 运行数据，删除会丢失本机数据" -Cautious
    Write-CleanCandidate "logs" "本地日志" -Cautious
    Write-CleanCandidate "cert" "本地证书文件" -Cautious

    Write-Host ""
    Write-Host "提示：此命令只列清单。如需清理，请逐项确认后手动删除。" -ForegroundColor Cyan
}

if ($ListCleanCandidates) {
    Show-CleanCandidates
    exit 0
}

Write-Host "========================================" -ForegroundColor Green
Write-Host "S-UI 多系统构建脚本" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
Write-Host "目标系统：$System/$Architecture" -ForegroundColor Cyan
Write-Host ""

try {
    $goVersion = go version 2>$null
    if ($LASTEXITCODE -ne 0) { throw "Go not found" }
    Write-Host "已检测到 Go：$goVersion" -ForegroundColor Green
} catch {
    Stop-WithMessage "未检测到 Go，或 Go 未加入 PATH。请先安装 Go。"
}

if (!$SkipFrontend) {
    try {
        $nodeVersion = node --version 2>$null
        if ($LASTEXITCODE -ne 0) { throw "Node.js not found" }
        Write-Host "已检测到 Node.js：$nodeVersion" -ForegroundColor Green
    } catch {
        Stop-WithMessage "未检测到 Node.js，或 Node.js 未加入 PATH。请先安装 Node.js，或使用 -SkipFrontend 跳过前端构建。"
    }
}

if (!(Test-Path "main.go")) {
    Stop-WithMessage "未找到 main.go。请在 S-UI 源码根目录运行本脚本。"
}

if (!$SkipFrontend) {
    if (!(Test-Path "frontend")) {
        Stop-WithMessage "未找到 frontend 目录。请在 S-UI 源码根目录运行本脚本。"
    }

    Write-Host "正在构建前端..." -ForegroundColor Yellow
    Push-Location frontend
    try {
        Write-Host "正在安装前端依赖..." -ForegroundColor Cyan
        npm ci
        if ($LASTEXITCODE -ne 0) { throw "前端依赖安装失败" }

        Write-Host "正在执行前端代码检查..." -ForegroundColor Cyan
        npm run lint
        if ($LASTEXITCODE -ne 0) { throw "前端代码检查失败" }
        Write-Host "正在执行前端构建..." -ForegroundColor Cyan
        npm run build
        if ($LASTEXITCODE -ne 0) { throw "前端构建失败" }
    } catch {
        Pop-Location
        Stop-WithMessage $_
    }
    Pop-Location

    Write-Host "前端构建产物将由 Go 直接嵌入 frontend/dist。" -ForegroundColor Green
} else {
    if (!(Test-Path "frontend\dist\index.html")) {
        Stop-WithMessage "已跳过前端构建，但未找到 frontend/dist/index.html。"
    }
    Write-Host "已跳过前端构建，使用现有 frontend/dist 嵌入资源。" -ForegroundColor Yellow
}

$env:GOOS = $System
$env:GOARCH = $Architecture
if ($NoCGO) {
    $env:CGO_ENABLED = "0"
    Write-Host "正在关闭 CGO 构建..." -ForegroundColor Yellow
} else {
    $env:CGO_ENABLED = "1"
    Write-Host "正在启用 CGO 构建..." -ForegroundColor Yellow
}

$exeSuffix = ""
if ($System -eq "windows") { $exeSuffix = ".exe" }
$output = "sui-$System-$Architecture$exeSuffix"
if (Test-Path $output) {
    Write-Host "正在移除旧输出文件：$output" -ForegroundColor Yellow
    Remove-Item $output -Force
}
$tags = "with_quic,with_grpc,with_utls,with_acme,with_gvisor,with_tailscale,with_naive_outbound,with_purego"
# Build the module package and suppress VCS-derived pseudo-versions from legacy Git tags.
$buildArgs = @("build", "-buildvcs=false", "-ldflags", "-w -s", "-tags", $tags, "-o", $output, ".")

try {
    Write-Host "正在构建后端：$System/$Architecture ..." -ForegroundColor Yellow
    & go @buildArgs
    if ($LASTEXITCODE -ne 0) {
        throw "后端构建失败"
    } else {
        if ($env:CGO_ENABLED -eq "1") {
            Write-Host "构建成功：已启用 CGO。" -ForegroundColor Green
        } else {
            Write-Host "构建成功：已关闭 CGO。" -ForegroundColor Green
        }
        if ($System -eq "linux") {
            Test-BuildModuleMetadata $output
        }
    }
} catch {
    Stop-WithMessage $_
}

if ($System -eq "windows") {
    if (Test-Path "sui.exe") {
        Write-Host "正在移除旧兼容文件：sui.exe" -ForegroundColor Yellow
        Remove-Item "sui.exe" -Force
    }
    Copy-Item -LiteralPath $output -Destination "sui.exe" -Force -ErrorAction Stop
}

if ($Package) {
    $packageSystem = $System
    if ($System -eq "darwin") { $packageSystem = "macos" }
    $packageDir = "dist\s-ui-$packageSystem-$Architecture"
    if (Test-Path -LiteralPath $packageDir) { Remove-Item -LiteralPath $packageDir -Recurse -Force -ErrorAction Stop }
    New-Item -ItemType Directory -Path $packageDir -Force -ErrorAction Stop | Out-Null
    $archivePath = "dist\s-ui-$packageSystem-$Architecture.zip"

    if ($System -eq "windows") {
        foreach ($document in @('RELEASE_NOTES.md', 'PROTOCOL_GUIDE.md', 'XRAY-CORE-NOTICE.md', 'README.md')) {
            Copy-RequiredPackageFile $document $packageDir $document
        }
        foreach ($script in @('s-ui-windows.xml', 'install-windows.bat', 'uninstall-windows.bat', 's-ui-windows.bat')) {
            Copy-RequiredPackageFile $script $packageDir $script
        }
        if (Test-Path -LiteralPath "libcronet.dll" -PathType Leaf) {
            Copy-Item -LiteralPath "libcronet.dll" -Destination (Join-Path $packageDir 'libcronet.dll') -Force -ErrorAction Stop
        }
        Copy-RequiredPackageFile $output $packageDir 'sui.exe'
        Compress-Archive -Path "$packageDir\*" -DestinationPath $archivePath -Force -ErrorAction Stop
        Write-Sha256Sidecar $archivePath
        Test-PackageArchive $archivePath $System $Architecture
        Test-Sha256Sidecar $archivePath
        Write-Host "已生成压缩包 / Package generated: $archivePath" -ForegroundColor Green
    } elseif ($System -eq "linux") {
        Copy-RequiredPackageFile $output $packageDir 'sui'
        foreach ($requiredFile in @(
            'install.sh',
            'install-linux.sh',
            'uninstall-linux.sh',
            's-ui.sh',
            'README.md',
            'PROTOCOL_GUIDE.md',
            'RELEASE_NOTES.md',
            'XRAY-CORE-NOTICE.md'
        )) {
            Copy-RequiredPackageFile $requiredFile $packageDir $requiredFile
        }
        # The installer creates the unit for its selected installation path. Do not ship the
        # repository's static s-ui.service, which could make a package's service path misleading.
        Write-Host "Xray-core Go 源码依赖已编译进 sui；未附带独立 xray 文件。" -ForegroundColor Green
        Compress-Archive -Path "$packageDir\*" -DestinationPath $archivePath -Force -ErrorAction Stop
        Write-Sha256Sidecar $archivePath
        Test-PackageArchive $archivePath $System $Architecture
        Test-Sha256Sidecar $archivePath
        Write-Host "已生成压缩包 / Package generated: $archivePath" -ForegroundColor Green
    } else {
        foreach ($document in @('RELEASE_NOTES.md', 'PROTOCOL_GUIDE.md', 'XRAY-CORE-NOTICE.md', 'README.md')) {
            Copy-RequiredPackageFile $document $packageDir $document
        }
        foreach ($script in @('install-macos.sh', 'uninstall-macos.sh', 'com.s-ui.plist')) {
            Copy-RequiredPackageFile $script $packageDir $script
        }
        Copy-RequiredPackageFile $output $packageDir 'sui'
        Compress-Archive -Path "$packageDir\*" -DestinationPath $archivePath -Force -ErrorAction Stop
        Write-Sha256Sidecar $archivePath
        Test-PackageArchive $archivePath $System $Architecture
        Test-Sha256Sidecar $archivePath
        Write-Host "已生成压缩包 / Package generated: $archivePath" -ForegroundColor Green
    }
}
Write-Host ""
Write-Host "========================================" -ForegroundColor Green
Write-Host "构建完成" -ForegroundColor Green
Write-Host "========================================" -ForegroundColor Green
Write-Host "输出文件：$output" -ForegroundColor Green
if (Test-Path $output) {
    $fileInfo = Get-Item $output
    Write-Host "文件大小：$([math]::Round($fileInfo.Length / 1MB, 2)) MB" -ForegroundColor Cyan
    Write-Host "创建时间：$($fileInfo.CreationTime)" -ForegroundColor Cyan
}

if (!$NonInteractive) {
    Read-Host "按 Enter 退出 / Press Enter to exit"
}
