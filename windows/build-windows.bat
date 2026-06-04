@echo off
setlocal enabledelayedexpansion

title S-UI Windows 构建脚本

echo ========================================
echo S-UI Windows 构建脚本
echo ========================================
echo.

cd /d "%~dp0"

go version >nul 2>&1
if errorlevel 1 (
    echo 错误：未检测到 Go，或 Go 未加入 PATH。
    echo 请先安装 Go，然后重新打开终端再运行本脚本。
    pause
    exit /b 1
)

node --version >nul 2>&1
if errorlevel 1 (
    echo 错误：未检测到 Node.js，或 Node.js 未加入 PATH。
    echo 请先安装 Node.js，然后重新打开终端再运行本脚本。
    pause
    exit /b 1
)

if not exist "frontend" (
    echo 错误：未找到 frontend 目录。
    echo 请在 S-UI 源码根目录运行本脚本。
    pause
    exit /b 1
)

if not exist "main.go" (
    echo 错误：未找到 main.go。
    echo 请在 S-UI 源码根目录运行本脚本。
    pause
    exit /b 1
)

echo 正在构建前端...
cd frontend
call npm install
if errorlevel 1 (
    echo 错误：前端依赖安装失败。
    pause
    exit /b 1
)

call npm run build
if errorlevel 1 (
    echo 错误：前端构建失败。
    pause
    exit /b 1
)

cd ..

echo 正在重建 web\html 目录...
if exist "web\html" rmdir /s /q "web\html"
mkdir "web\html"

echo 正在复制前端构建产物...
xcopy "frontend\dist\*" "web\html\" /E /I /Y /Q
if errorlevel 1 (
    echo 错误：复制前端构建产物失败。
    pause
    exit /b 1
)

echo.
echo 请选择 Windows 目标架构：
echo   1. amd64  - 64 位 Intel/AMD Windows，最常用
echo   2. arm64  - ARM64 Windows
echo   3. 386    - 32 位 Windows
echo.
set /p arch_choice="请输入选项 [1-3]，默认 1: "
if "%arch_choice%"=="" set "arch_choice=1"

set "GOARCH_VALUE=amd64"
if "%arch_choice%"=="2" set "GOARCH_VALUE=arm64"
if "%arch_choice%"=="3" set "GOARCH_VALUE=386"

echo.
echo 正在构建后端：windows/%GOARCH_VALUE% ...
set CGO_ENABLED=1
set GOOS=windows
set GOARCH=%GOARCH_VALUE%
set "OUTPUT=sui-windows-%GOARCH_VALUE%.exe"

go build -ldflags "-w -s" -tags "with_quic,with_grpc,with_utls,with_acme,with_gvisor,with_tailscale" -o "%OUTPUT%" main.go
if errorlevel 1 (
    echo 警告：CGO 构建失败，正在尝试关闭 CGO 重新构建...
    set CGO_ENABLED=0
    go build -ldflags "-w -s" -tags "with_quic,with_grpc,with_utls,with_acme,with_gvisor,with_tailscale" -o "%OUTPUT%" main.go
    if errorlevel 1 (
        echo 错误：后端构建失败。
        pause
        exit /b 1
    )
    echo 构建成功：已关闭 CGO，部分依赖 CGO 的功能可能受限。
) else (
    echo 构建成功：已启用 CGO。
)

copy /y "%OUTPUT%" "sui.exe" >nul

echo.
echo ========================================
echo 构建完成
echo ========================================
echo 输出文件：%OUTPUT%
echo 兼容文件：sui.exe
echo.
pause
