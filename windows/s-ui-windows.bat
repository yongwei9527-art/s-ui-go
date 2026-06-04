@echo off
setlocal enabledelayedexpansion

title S-UI Windows 控制面板
cd /d "%~dp0"

set "SERVICE_NAME=s-ui"
set "INSTALL_DIR=%SUI_HOME%"
if "%INSTALL_DIR%"=="" set "INSTALL_DIR=%~dp0"
for %%i in ("%INSTALL_DIR%\.") do set "INSTALL_DIR=%%~fi"
set "CONFIG_FILE=%INSTALL_DIR%\s-ui-windows.env"
set "PANEL_PORT=2095"
set "PANEL_PATH=/app/"
set "SUB_PORT=2096"
set "SUB_PATH=/sub/"

if exist "%CONFIG_FILE%" (
    for /f "usebackq tokens=1,* delims==" %%a in ("%CONFIG_FILE%") do (
        if /i "%%a"=="INSTALL_DIR" set "INSTALL_DIR=%%b"
        if /i "%%a"=="SERVICE_NAME" set "SERVICE_NAME=%%b"
        if /i "%%a"=="PANEL_PORT" set "PANEL_PORT=%%b"
        if /i "%%a"=="PANEL_PATH" set "PANEL_PATH=%%b"
        if /i "%%a"=="SUB_PORT" set "SUB_PORT=%%b"
        if /i "%%a"=="SUB_PATH" set "SUB_PATH=%%b"
    )
)

:menu
cls
echo ========================================
echo S-UI Windows 控制面板
echo ========================================
echo 安装目录：%INSTALL_DIR%
echo 面板地址：http://localhost:%PANEL_PORT%%PANEL_PATH%
echo.
echo 1. 启动 S-UI 服务
echo 2. 停止 S-UI 服务
echo 3. 重启 S-UI 服务
echo 4. 查看服务状态
echo 5. 查看服务日志
echo 6. 在浏览器打开面板
echo 7. 手动运行 S-UI
echo 8. 安装或卸载服务
echo 9. 打开安装目录
echo 10. 显示当前配置
echo 11. 显示访问地址
echo 0. 退出
echo.
echo ========================================
set /p choice="请选择功能 [0-11]："

if "%choice%"=="1" goto start_service
if "%choice%"=="2" goto stop_service
if "%choice%"=="3" goto restart_service
if "%choice%"=="4" goto check_status
if "%choice%"=="5" goto view_logs
if "%choice%"=="6" goto open_panel
if "%choice%"=="7" goto run_manual
if "%choice%"=="8" goto service_management
if "%choice%"=="9" goto open_directory
if "%choice%"=="10" goto show_config
if "%choice%"=="11" goto show_urls
if "%choice%"=="0" goto exit
goto invalid_choice

:start_service
echo 正在启动 S-UI 服务...
net start %SERVICE_NAME%
if %errorLevel% equ 0 (echo 服务启动成功。) else (echo 服务启动失败，错误代码：%errorLevel%)
pause
goto menu

:stop_service
echo 正在停止 S-UI 服务...
net stop %SERVICE_NAME%
if %errorLevel% equ 0 (echo 服务停止成功。) else (echo 服务停止失败，错误代码：%errorLevel%)
pause
goto menu

:restart_service
echo 正在重启 S-UI 服务...
net stop %SERVICE_NAME% >nul 2>&1
timeout /t 2 /nobreak >nul
net start %SERVICE_NAME%
if %errorLevel% equ 0 (echo 服务重启成功。) else (echo 服务重启失败，错误代码：%errorLevel%)
pause
goto menu

:check_status
echo 正在查看 S-UI 服务状态...
sc query %SERVICE_NAME%
echo.
if exist "%INSTALL_DIR%\s-ui-service.exe" "%INSTALL_DIR%\s-ui-service.exe" status
pause
goto menu

:view_logs
echo 正在打开 S-UI 日志目录...
if exist "%INSTALL_DIR%\logs" (start "" "%INSTALL_DIR%\logs") else (echo 未找到日志目录：%INSTALL_DIR%\logs)
pause
goto menu

:open_panel
echo 正在浏览器中打开 S-UI 面板...
start "" "http://localhost:%PANEL_PORT%%PANEL_PATH%"
pause
goto menu

:run_manual
echo 正在手动运行 S-UI...
if exist "%INSTALL_DIR%\sui.exe" (
    cd /d "%INSTALL_DIR%"
    echo 按 Ctrl+C 可以停止手动运行。
    echo.
    sui.exe
) else (
    echo 未找到 S-UI 主程序：%INSTALL_DIR%\sui.exe
)
pause
goto menu

:service_management
cls
echo ========================================
echo 服务管理
echo ========================================
echo 1. 安装 Windows 服务
echo 2. 卸载 Windows 服务
echo 3. 返回主菜单
echo.
set /p service_choice="请选择功能 [1-3]："
if "%service_choice%"=="1" goto install_service
if "%service_choice%"=="2" goto uninstall_service
if "%service_choice%"=="3" goto menu
goto invalid_choice

:install_service
echo 正在安装 Windows 服务...
if exist "%INSTALL_DIR%\s-ui-service.exe" (
    cd /d "%INSTALL_DIR%"
    s-ui-service.exe install
    if %errorLevel% equ 0 (echo 服务安装成功。) else (echo 服务安装失败，错误代码：%errorLevel%)
) else (
    echo 未找到服务组件，请先运行 install-windows.bat。
)
pause
goto service_management

:uninstall_service
echo 正在卸载 Windows 服务...
if exist "%INSTALL_DIR%\s-ui-service.exe" (
    cd /d "%INSTALL_DIR%"
    net stop %SERVICE_NAME% >nul 2>&1
    s-ui-service.exe uninstall
    if %errorLevel% equ 0 (echo 服务卸载成功。) else (echo 服务卸载失败，错误代码：%errorLevel%)
) else (
    echo 未找到服务组件。
)
pause
goto service_management

:open_directory
if exist "%INSTALL_DIR%" (start "" "%INSTALL_DIR%") else (echo 未找到安装目录：%INSTALL_DIR%)
pause
goto menu

:show_config
echo.
echo ========================================
echo 当前配置
echo ========================================
echo 安装目录：%INSTALL_DIR%
echo 服务名称：%SERVICE_NAME%
echo 面板地址：http://localhost:%PANEL_PORT%%PANEL_PATH%
echo 订阅地址：http://localhost:%SUB_PORT%%SUB_PATH%
echo.
if exist "%INSTALL_DIR%\sui.exe" (
    cd /d "%INSTALL_DIR%"
    sui.exe setting -show
    echo.
    sui.exe admin -show
) else (
    echo 未找到 S-UI 主程序。
)
pause
goto menu

:show_urls
echo.
echo ========================================
echo 访问地址
echo ========================================
echo 本机访问：
echo   Panel: http://localhost:%PANEL_PORT%%PANEL_PATH%
echo   Subscription: http://localhost:%SUB_PORT%%SUB_PATH%
echo.
echo 局域网访问：
for /f "tokens=2 delims=:" %%i in ('ipconfig ^| findstr /i "IPv4"') do (
    set "ip=%%i"
    set "ip=!ip: =!"
    echo   Panel: http://!ip!:%PANEL_PORT%%PANEL_PATH%
    echo   Subscription: http://!ip!:%SUB_PORT%%SUB_PATH%
)
echo.
pause
goto menu

:invalid_choice
echo 输入无效，请重新选择。
pause
goto menu

:exit
echo 感谢使用 S-UI Windows 控制面板。
exit /b 0
