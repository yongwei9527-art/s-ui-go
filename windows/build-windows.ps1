# S-UI 多系统构建脚本
param(
    [ValidateSet("windows", "linux", "darwin")]
    [string]$System = "windows",

    [ValidateSet("amd64", "386", "arm64", "arm")]
    [string]$Architecture = "amd64",

    [switch]$NoCGO,
    [switch]$SkipFrontend,
    [switch]$Package,
    [switch]$Help
)

if ($Help) {
    Write-Host "用法："
    Write-Host "  .\build-windows.ps1 [-System <系统>] [-Architecture <架构>] [-NoCGO] [-SkipFrontend] [-Package]"
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
    Write-Host "示例："
    Write-Host "  .\build-windows.ps1"
    Write-Host "  .\build-windows.ps1 -System windows -Architecture arm64 -Package"
    Write-Host "  .\build-windows.ps1 -System linux -Architecture amd64 -NoCGO"
    Write-Host "  .\build-windows.ps1 -System darwin -Architecture arm64 -NoCGO"
    exit 0
}

function Stop-WithMessage($Message) {
    Write-Host "错误：$Message" -ForegroundColor Red
    Read-Host "按 Enter 退出"
    exit 1
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
        npm install
        if ($LASTEXITCODE -ne 0) { throw "前端依赖安装失败" }

        Write-Host "正在执行前端构建..." -ForegroundColor Cyan
        npm run build
        if ($LASTEXITCODE -ne 0) { throw "前端构建失败" }
    } catch {
        Pop-Location
        Stop-WithMessage $_
    }
    Pop-Location

    Write-Host "正在重建 web/html 目录..." -ForegroundColor Yellow
    if (Test-Path "web\html") {
        Remove-Item "web\html" -Recurse -Force
    }
    New-Item -ItemType Directory -Path "web\html" -Force | Out-Null

    Write-Host "正在复制前端构建产物..." -ForegroundColor Yellow
    Copy-Item "frontend\dist\*" "web\html\" -Recurse -Force
} else {
    Write-Host "已跳过前端构建。" -ForegroundColor Yellow
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
$tags = "with_quic,with_grpc,with_utls,with_acme,with_gvisor,with_tailscale"
$buildArgs = @("build", "-ldflags", "-w -s", "-tags", $tags, "-o", $output, "main.go")

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
    }
} catch {
    Stop-WithMessage $_
}

if ($System -eq "windows") {
    Copy-Item $output "sui.exe" -Force
}

if ($Package) {
    $packageDir = "dist\s-ui-$System-$Architecture"
    if (Test-Path $packageDir) { Remove-Item $packageDir -Recurse -Force }
    New-Item -ItemType Directory -Path $packageDir -Force | Out-Null
    Copy-Item $output $packageDir -Force

    if ($System -eq "windows") {
        Copy-Item "s-ui-windows.xml" $packageDir -Force -ErrorAction SilentlyContinue
        Copy-Item "install-windows.bat" $packageDir -Force -ErrorAction SilentlyContinue
        Copy-Item "uninstall-windows.bat" $packageDir -Force -ErrorAction SilentlyContinue
        Copy-Item "s-ui-windows.bat" $packageDir -Force -ErrorAction SilentlyContinue
        Copy-Item "README.md" $packageDir -Force -ErrorAction SilentlyContinue
        if (Test-Path "libcronet.dll") { Copy-Item "libcronet.dll" $packageDir -Force }
        Copy-Item $output "$packageDir\sui.exe" -Force
        Compress-Archive -Path "$packageDir\*" -DestinationPath "dist\s-ui-$System-$Architecture.zip" -Force
        Write-Host "已生成压缩包：dist\s-ui-$System-$Architecture.zip" -ForegroundColor Green
    } elseif ($System -eq "linux") {
        Copy-Item $output "$packageDir\sui" -Force
        Copy-Item "README.md" $packageDir -Force -ErrorAction SilentlyContinue
        Copy-Item "install-linux.sh" $packageDir -Force -ErrorAction SilentlyContinue
        Copy-Item "uninstall-linux.sh" $packageDir -Force -ErrorAction SilentlyContinue
        Copy-Item "s-ui.sh" $packageDir -Force -ErrorAction SilentlyContinue
        Copy-Item "s-ui.service" $packageDir -Force -ErrorAction SilentlyContinue
        Compress-Archive -Path "$packageDir\*" -DestinationPath "dist\s-ui-$System-$Architecture.zip" -Force
        Write-Host "已生成压缩包：dist\s-ui-$System-$Architecture.zip" -ForegroundColor Green
    } else {
        Copy-Item $output "$packageDir\sui" -Force
        Copy-Item "README.md" $packageDir -Force -ErrorAction SilentlyContinue
        Copy-Item "install-macos.sh" $packageDir -Force -ErrorAction SilentlyContinue
        Copy-Item "uninstall-macos.sh" $packageDir -Force -ErrorAction SilentlyContinue
        Copy-Item "com.s-ui.plist" $packageDir -Force -ErrorAction SilentlyContinue
        Compress-Archive -Path "$packageDir\*" -DestinationPath "dist\s-ui-$System-$Architecture.zip" -Force
        Write-Host "已生成压缩包：dist\s-ui-$System-$Architecture.zip" -ForegroundColor Green
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

Read-Host "按 Enter 退出"
