@echo off
setlocal enabledelayedexpansion

title S-UI Windows 卸载程序

echo ========================================
echo S-UI Windows 卸载程序
echo ========================================
echo.

net session >nul 2>&1
if %errorLevel% neq 0 (
    echo 错误：请以管理员身份运行本脚本。
    echo 错误：请右键本文件，选择“以管理员身份运行”。
    pause
    exit /b 1
)

set "INSTALL_DIR=%SUI_HOME%"
if "%INSTALL_DIR%"=="" set "INSTALL_DIR=C:\Program Files\s-ui"
set "SERVICE_NAME=s-ui"
set "CONFIG_FILE=%INSTALL_DIR%\s-ui-windows.env"

if exist "%CONFIG_FILE%" (
    for /f "usebackq tokens=1,* delims==" %%a in ("%CONFIG_FILE%") do (
        if /i "%%a"=="INSTALL_DIR" set "INSTALL_DIR=%%b"
        if /i "%%a"=="SERVICE_NAME" set "SERVICE_NAME=%%b"
    )
)

echo 正在从以下目录卸载 S-UI：%INSTALL_DIR%
echo 卸载目录：%INSTALL_DIR%
echo.

if exist "%INSTALL_DIR%\s-ui-service.exe" (
    echo 正在停止并移除 Windows 服务...
    net stop %SERVICE_NAME% >nul 2>&1
    cd /d "%INSTALL_DIR%"
    s-ui-service.exe uninstall >nul 2>&1
    if %errorLevel% equ 0 (
        echo 服务移除成功。
    ) else (
        echo 警告：服务移除失败，或服务本来就没有安装。
    )
) else (
    sc query %SERVICE_NAME% >nul 2>&1
    if %errorLevel% equ 0 (
        echo 警告：检测到服务存在，但未找到服务组件。如需手动删除，请执行：sc delete %SERVICE_NAME%
    )
)

echo 正在删除快捷方式...
set "DESKTOP=%USERPROFILE%\Desktop"
if exist "%DESKTOP%\S-UI.lnk" del "%DESKTOP%\S-UI.lnk" >nul 2>&1
set "START_MENU=%APPDATA%\Microsoft\Windows\Start Menu\Programs\S-UI"
if exist "%START_MENU%" rmdir /s /q "%START_MENU%" >nul 2>&1

echo 正在删除环境变量...
reg delete "HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Control\Session Manager\Environment" /v SUI_HOME /f >nul 2>&1

echo 正在删除防火墙规则...
netsh advfirewall firewall delete rule name="S-UI Panel" >nul 2>&1
netsh advfirewall firewall delete rule name="S-UI Subscription" >nul 2>&1

echo.
set /p keep_data="是否保留数据库、日志和证书等数据文件？[Y/n]："
if "%keep_data%"=="" set "keep_data=Y"

if /i "%keep_data%"=="Y" (
    echo 正在保留数据文件，仅删除程序和服务文件...
    if exist "%INSTALL_DIR%\sui.exe" del "%INSTALL_DIR%\sui.exe" >nul 2>&1
    if exist "%INSTALL_DIR%\libcronet.dll" del "%INSTALL_DIR%\libcronet.dll" >nul 2>&1
    if exist "%INSTALL_DIR%\s-ui-service.exe" del "%INSTALL_DIR%\s-ui-service.exe" >nul 2>&1
    if exist "%INSTALL_DIR%\s-ui-service.xml" del "%INSTALL_DIR%\s-ui-service.xml" >nul 2>&1
    if exist "%INSTALL_DIR%\winsw.exe" del "%INSTALL_DIR%\winsw.exe" >nul 2>&1
    if exist "%INSTALL_DIR%\s-ui-windows.xml" del "%INSTALL_DIR%\s-ui-windows.xml" >nul 2>&1
    if exist "%INSTALL_DIR%\s-ui-windows.env" del "%INSTALL_DIR%\s-ui-windows.env" >nul 2>&1
    if exist "%INSTALL_DIR%\README.md" del "%INSTALL_DIR%\README.md" >nul 2>&1
    if exist "%INSTALL_DIR%\s-ui-windows.bat" del "%INSTALL_DIR%\s-ui-windows.bat" >nul 2>&1
    echo 数据文件已保留在：%INSTALL_DIR%
) else (
    echo 正在删除所有文件...
    if exist "%INSTALL_DIR%" (
        rmdir /s /q "%INSTALL_DIR%" >nul 2>&1
        if exist "%INSTALL_DIR%" (
            echo 警告：部分文件未能删除，请手动删除：%INSTALL_DIR%
        ) else (
            echo 所有文件已删除。
        )
    )
)

echo.
echo ========================================
echo 卸载完成
echo ========================================
if /i "%keep_data%"=="Y" (
    echo 你的数据已保留在：%INSTALL_DIR%
)
echo.
pause
