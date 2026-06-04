@echo off
chcp 65001 >nul
setlocal enabledelayedexpansion

set "INSTALL_LANG="
call :parse_args %*
if !errorLevel! neq 0 exit /b 1
call :normalize_language
if "!INSTALL_LANG!"=="" call :select_language
call :normalize_language
if "!INSTALL_LANG!"=="" set "INSTALL_LANG=en"
call :load_messages

title !TXT_TITLE!

echo ========================================
echo !TXT_TITLE!
echo ========================================
echo.

net session >nul 2>&1
if !errorLevel! neq 0 (
    echo !TXT_ADMIN_REQUIRED_1!
    echo !TXT_ADMIN_REQUIRED_2!
    pause
    exit /b 1
)

cd /d "%~dp0"
set "INSTALL_DIR=C:\Program Files\s-ui"
set "SERVICE_NAME=s-ui"
set "CONFIG_FILE=%INSTALL_DIR%\s-ui-windows.env"

if not exist "sui.exe" (
    echo !TXT_MISSING_EXE!
    echo !TXT_MISSING_EXE_HINT!
    pause
    exit /b 1
)

echo !TXT_INSTALLING_TO! %INSTALL_DIR%
echo !TXT_INSTALL_DIR_LABEL! %INSTALL_DIR%
echo.

if not exist "%INSTALL_DIR%" mkdir "%INSTALL_DIR%"
if not exist "%INSTALL_DIR%\db" mkdir "%INSTALL_DIR%\db"
if not exist "%INSTALL_DIR%\logs" mkdir "%INSTALL_DIR%\logs"
if not exist "%INSTALL_DIR%\cert" mkdir "%INSTALL_DIR%\cert"

echo !TXT_COPY_FILES!
copy /y "sui.exe" "%INSTALL_DIR%\" >nul
copy /y "s-ui-windows.xml" "%INSTALL_DIR%\" >nul
copy /y "s-ui-windows.bat" "%INSTALL_DIR%\" >nul
copy /y "uninstall-windows.bat" "%INSTALL_DIR%\" >nul
if exist "README.md" copy /y "README.md" "%INSTALL_DIR%\" >nul
if exist "libcronet.dll" copy /y "libcronet.dll" "%INSTALL_DIR%\" >nul

set "ARCH=%PROCESSOR_ARCHITECTURE%"
if /i "%PROCESSOR_ARCHITEW6432%"=="ARM64" set "ARCH=ARM64"
if /i "%PROCESSOR_ARCHITEW6432%"=="AMD64" set "ARCH=AMD64"

set "WINSW_ASSET=WinSW-x64.exe"
if /i "%ARCH%"=="ARM64" set "WINSW_ASSET=WinSW-arm64.exe"
if /i "%ARCH%"=="x86" set "WINSW_ASSET=WinSW-x86.exe"

set "WINSW_PATH=%INSTALL_DIR%\winsw.exe"
if not exist "%WINSW_PATH%" (
    if "!INSTALL_LANG!"=="zh" (
        echo 正在下载适用于 %ARCH% 的 WinSW 服务组件...
    ) else (
        echo Downloading WinSW service component for %ARCH%...
    )
    powershell -NoProfile -ExecutionPolicy Bypass -Command "try { [Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12; Invoke-WebRequest -Uri 'https://github.com/winsw/winsw/releases/latest/download/%WINSW_ASSET%' -OutFile '%WINSW_PATH%' -UseBasicParsing; exit 0 } catch { Write-Host $_.Exception.Message; exit 1 }"
    if not exist "%WINSW_PATH%" (
        echo !TXT_WINSW_DOWNLOAD_FAILED_1!
        echo !TXT_WINSW_DOWNLOAD_FAILED_2!
    )
)

echo !TXT_MIGRATION!
cd /d "%INSTALL_DIR%"
sui.exe migrate
if !errorLevel! equ 0 (
    echo !TXT_MIGRATION_DONE!
) else (
    echo !TXT_MIGRATION_WARN!
)

echo.
echo ========================================
echo !TXT_NETWORK_CONFIG!
echo ========================================
echo !TXT_AVAILABLE_IPV4!
for /f "tokens=2 delims=:" %%i in ('ipconfig ^| findstr /i "IPv4"') do echo   %%i
echo.

set /p panel_port="!TXT_PANEL_PORT_PROMPT!"
if "!panel_port!"=="" set "panel_port=2095"

set /p panel_path="!TXT_PANEL_PATH_PROMPT!"
if "!panel_path!"=="" set "panel_path=/app/"

set /p sub_port="!TXT_SUB_PORT_PROMPT!"
if "!sub_port!"=="" set "sub_port=2096"

set /p sub_path="!TXT_SUB_PATH_PROMPT!"
if "!sub_path!"=="" set "sub_path=/sub/"

echo !TXT_APPLYING_CONFIG!
sui.exe setting -port !panel_port! -path "!panel_path!" -subPort !sub_port! -subPath "!sub_path!"
if !errorLevel! neq 0 echo !TXT_CONFIG_WARN!

echo.
echo ========================================
echo !TXT_ADMIN_CONFIG!
echo ========================================
set /p admin_username="!TXT_ADMIN_USERNAME_PROMPT!"
if "!admin_username!"=="" set "admin_username=admin"

for /f "usebackq delims=" %%p in (`powershell -NoProfile -ExecutionPolicy Bypass -Command "$p = Read-Host '!TXT_ADMIN_PASSWORD_PROMPT!' -AsSecureString; $b = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($p); try { [Runtime.InteropServices.Marshal]::PtrToStringBSTR($b) } finally { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($b) }"`) do set "admin_password=%%p"

if "!admin_password!"=="" (
    echo !TXT_PASSWORD_EMPTY!
    pause
    exit /b 1
)

echo !TXT_SETTING_ADMIN!
sui.exe admin -username "!admin_username!" -password "!admin_password!"
if !errorLevel! neq 0 echo !TXT_ADMIN_WARN!
set "admin_password="

(
    echo INSTALL_DIR=%INSTALL_DIR%
    echo SERVICE_NAME=%SERVICE_NAME%
    echo PANEL_PORT=!panel_port!
    echo PANEL_PATH=!panel_path!
    echo SUB_PORT=!sub_port!
    echo SUB_PATH=!sub_path!
) > "%CONFIG_FILE%"

if exist "%WINSW_PATH%" (
    echo !TXT_INSTALL_SERVICE!
    copy /y "%WINSW_PATH%" "%INSTALL_DIR%\s-ui-service.exe" >nul
    copy /y "%INSTALL_DIR%\s-ui-windows.xml" "%INSTALL_DIR%\s-ui-service.xml" >nul
    "%INSTALL_DIR%\s-ui-service.exe" install
    if !errorLevel! equ 0 (
        echo !TXT_SERVICE_INSTALLED!
    ) else (
        echo !TXT_SERVICE_INSTALL_WARN!
    )
)

echo !TXT_SETTING_PERMISSIONS!
icacls "%INSTALL_DIR%" /grant "Users:(OI)(CI)RX" /T >nul
icacls "%INSTALL_DIR%\db" /grant "Users:(OI)(CI)F" /T >nul
icacls "%INSTALL_DIR%\logs" /grant "Users:(OI)(CI)F" /T >nul
icacls "%INSTALL_DIR%\cert" /grant "Users:(OI)(CI)F" /T >nul

echo !TXT_SETTING_ENV!
setx SUI_HOME "%INSTALL_DIR%" /M >nul

echo !TXT_CREATING_SHORTCUT!
set "DESKTOP=%USERPROFILE%\Desktop"
if exist "%DESKTOP%" powershell -NoProfile -ExecutionPolicy Bypass -Command "$s=(New-Object -ComObject WScript.Shell).CreateShortcut('%DESKTOP%\S-UI.lnk'); $s.TargetPath='%INSTALL_DIR%\s-ui-windows.bat'; $s.WorkingDirectory='%INSTALL_DIR%'; $s.Description='S-UI Control Panel'; $s.Save()"
set "START_MENU=%APPDATA%\Microsoft\Windows\Start Menu\Programs"
if exist "%START_MENU%" (
    if not exist "%START_MENU%\S-UI" mkdir "%START_MENU%\S-UI"
    powershell -NoProfile -ExecutionPolicy Bypass -Command "$s=(New-Object -ComObject WScript.Shell).CreateShortcut('%START_MENU%\S-UI\S-UI Control Panel.lnk'); $s.TargetPath='%INSTALL_DIR%\s-ui-windows.bat'; $s.WorkingDirectory='%INSTALL_DIR%'; $s.Description='S-UI Control Panel'; $s.Save()"
)

echo !TXT_STARTING_SERVICE!
net start %SERVICE_NAME%
if !errorLevel! neq 0 echo !TXT_SERVICE_START_WARN!

echo.
echo ========================================
echo !TXT_INSTALL_DONE!
echo ========================================
echo !TXT_INSTALL_DIR_LABEL! %INSTALL_DIR%
echo !TXT_PANEL_URL_LABEL! http://localhost:!panel_port!!panel_path!
echo !TXT_SUB_URL_LABEL! http://localhost:!sub_port!!sub_path!
echo !TXT_ADMIN_USERNAME_LABEL! !admin_username!
echo !TXT_SERVICE_NAME_LABEL! %SERVICE_NAME%
echo.
echo !TXT_LAN_ACCESS!
for /f "tokens=2 delims=:" %%i in ('ipconfig ^| findstr /i "IPv4"') do (
    set "ip=%%i"
    set "ip=!ip: =!"
    echo   Panel: http://!ip!:!panel_port!!panel_path!
    echo   Subscription: http://!ip!:!sub_port!!sub_path!
)
echo.
pause
exit /b 0

:parse_args
if "%~1"=="" exit /b 0
set "ARG=%~1"
if /i "!ARG!"=="--lang" (
    if "%~2"=="" (
        echo Missing language value after %~1 / 缺少语言参数
        exit /b 1
    )
    set "INSTALL_LANG=%~2"
    shift
    shift
    goto parse_args
)
if /i "!ARG!"=="-l" (
    if "%~2"=="" (
        echo Missing language value after %~1 / 缺少语言参数
        exit /b 1
    )
    set "INSTALL_LANG=%~2"
    shift
    shift
    goto parse_args
)
if /i "!ARG!"=="/lang" (
    if "%~2"=="" (
        echo Missing language value after %~1 / 缺少语言参数
        exit /b 1
    )
    set "INSTALL_LANG=%~2"
    shift
    shift
    goto parse_args
)
if /i "!ARG:~0,7!"=="--lang=" set "INSTALL_LANG=!ARG:~7!"
if /i "!ARG:~0,6!"=="/lang:" set "INSTALL_LANG=!ARG:~6!"
if /i "!ARG:~0,6!"=="/lang=" set "INSTALL_LANG=!ARG:~6!"
shift
goto parse_args

:normalize_language
if "!INSTALL_LANG!"=="" exit /b 0
set "RAW_LANG=!INSTALL_LANG!"
set "INSTALL_LANG="
if /i "!RAW_LANG!"=="zh" set "INSTALL_LANG=zh" & exit /b 0
if /i "!RAW_LANG!"=="zh-cn" set "INSTALL_LANG=zh" & exit /b 0
if /i "!RAW_LANG!"=="zh_cn" set "INSTALL_LANG=zh" & exit /b 0
if /i "!RAW_LANG!"=="zh-hans" set "INSTALL_LANG=zh" & exit /b 0
if /i "!RAW_LANG!"=="zh_hans" set "INSTALL_LANG=zh" & exit /b 0
if /i "!RAW_LANG!"=="cn" set "INSTALL_LANG=zh" & exit /b 0
if /i "!RAW_LANG!"=="chinese" set "INSTALL_LANG=zh" & exit /b 0
if /i "!RAW_LANG!"=="中文" set "INSTALL_LANG=zh" & exit /b 0
if /i "!RAW_LANG!"=="en" set "INSTALL_LANG=en" & exit /b 0
if /i "!RAW_LANG!"=="en-us" set "INSTALL_LANG=en" & exit /b 0
if /i "!RAW_LANG!"=="en_us" set "INSTALL_LANG=en" & exit /b 0
if /i "!RAW_LANG!"=="english" set "INSTALL_LANG=en" & exit /b 0
exit /b 0

:select_language
set "SUI_CULTURE="
for /f "usebackq delims=" %%l in (`powershell -NoProfile -ExecutionPolicy Bypass -Command "try { (Get-Culture).Name } catch { '' }"`) do set "SUI_CULTURE=%%l"
set "DEFAULT_CHOICE=2"
if /i "!SUI_CULTURE:~0,2!"=="zh" set "DEFAULT_CHOICE=1"
echo 请选择安装语言 / Please choose installation language
echo 1. 中文
echo 2. English
set /p lang_choice="请输入选项 / Enter choice [1-2] (default: !DEFAULT_CHOICE!): "
if "!lang_choice!"=="" set "lang_choice=!DEFAULT_CHOICE!"
if /i "!lang_choice!"=="1" set "INSTALL_LANG=zh" & exit /b 0
if /i "!lang_choice!"=="zh" set "INSTALL_LANG=zh" & exit /b 0
if /i "!lang_choice!"=="cn" set "INSTALL_LANG=zh" & exit /b 0
if /i "!lang_choice!"=="中文" set "INSTALL_LANG=zh" & exit /b 0
set "INSTALL_LANG=en"
exit /b 0

:load_messages
if /i "!INSTALL_LANG!"=="zh" (
    set "TXT_TITLE=S-UI Windows 安装程序"
    set "TXT_ADMIN_REQUIRED_1=错误：请以管理员身份运行本脚本。"
    set "TXT_ADMIN_REQUIRED_2=错误：请右键本文件，选择“以管理员身份运行”。"
    set "TXT_MISSING_EXE=错误：当前目录未找到 sui.exe。"
    set "TXT_MISSING_EXE_HINT=请将 Windows 版 S-UI 主程序命名为 sui.exe 后再运行安装。"
    set "TXT_INSTALLING_TO=正在安装 S-UI 到："
    set "TXT_INSTALL_DIR_LABEL=安装目录："
    set "TXT_COPY_FILES=正在复制文件..."
    set "TXT_WINSW_DOWNLOAD_FAILED_1=警告：WinSW 下载失败。"
    set "TXT_WINSW_DOWNLOAD_FAILED_2=警告：WinSW 下载失败，将跳过服务安装；你仍可手动运行 sui.exe。"
    set "TXT_MIGRATION=正在执行数据库迁移..."
    set "TXT_MIGRATION_DONE=数据库迁移完成。"
    set "TXT_MIGRATION_WARN=警告：数据库迁移失败，或当前是新数据库。"
    set "TXT_NETWORK_CONFIG=网络配置"
    set "TXT_AVAILABLE_IPV4=可用 IPv4 地址："
    set "TXT_PANEL_PORT_PROMPT=面板端口，默认 2095："
    set "TXT_PANEL_PATH_PROMPT=面板路径，默认 /app/："
    set "TXT_SUB_PORT_PROMPT=订阅端口，默认 2096："
    set "TXT_SUB_PATH_PROMPT=订阅路径，默认 /sub/："
    set "TXT_APPLYING_CONFIG=正在应用配置..."
    set "TXT_CONFIG_WARN=警告：网络配置应用失败。"
    set "TXT_ADMIN_CONFIG=管理员配置"
    set "TXT_ADMIN_USERNAME_PROMPT=管理员用户名，默认 admin："
    set "TXT_ADMIN_PASSWORD_PROMPT=管理员密码"
    set "TXT_PASSWORD_EMPTY=错误：密码不能为空。"
    set "TXT_SETTING_ADMIN=正在设置管理员账号..."
    set "TXT_ADMIN_WARN=警告：管理员账号设置失败。"
    set "TXT_INSTALL_SERVICE=正在安装 Windows 服务..."
    set "TXT_SERVICE_INSTALLED=服务安装成功。"
    set "TXT_SERVICE_INSTALL_WARN=警告：服务安装失败。你可以稍后通过控制面板手动运行 S-UI。"
    set "TXT_SETTING_PERMISSIONS=正在设置目录权限..."
    set "TXT_SETTING_ENV=正在设置环境变量..."
    set "TXT_CREATING_SHORTCUT=正在创建快捷方式..."
    set "TXT_STARTING_SERVICE=正在启动 S-UI 服务..."
    set "TXT_SERVICE_START_WARN=警告：服务启动失败。你可以稍后通过 s-ui-windows.bat 启动。"
    set "TXT_INSTALL_DONE=安装完成"
    set "TXT_PANEL_URL_LABEL=面板地址："
    set "TXT_SUB_URL_LABEL=订阅地址："
    set "TXT_ADMIN_USERNAME_LABEL=管理员用户名："
    set "TXT_SERVICE_NAME_LABEL=服务名称："
    set "TXT_LAN_ACCESS=局域网访问地址："
) else (
    set "TXT_TITLE=S-UI Windows Installer"
    set "TXT_ADMIN_REQUIRED_1=Error: please run this script as administrator."
    set "TXT_ADMIN_REQUIRED_2=Error: right-click this file and choose Run as administrator."
    set "TXT_MISSING_EXE=Error: sui.exe was not found in the current directory."
    set "TXT_MISSING_EXE_HINT=Please name the Windows S-UI executable sui.exe before running the installer."
    set "TXT_INSTALLING_TO=Installing S-UI to:"
    set "TXT_INSTALL_DIR_LABEL=Installation directory:"
    set "TXT_COPY_FILES=Copying files..."
    set "TXT_WINSW_DOWNLOAD_FAILED_1=Warning: WinSW download failed."
    set "TXT_WINSW_DOWNLOAD_FAILED_2=Warning: WinSW download failed. Service installation will be skipped; you can still run sui.exe manually."
    set "TXT_MIGRATION=Running database migration..."
    set "TXT_MIGRATION_DONE=Database migration completed."
    set "TXT_MIGRATION_WARN=Warning: database migration failed, or this is a new database."
    set "TXT_NETWORK_CONFIG=Network configuration"
    set "TXT_AVAILABLE_IPV4=Available IPv4 addresses:"
    set "TXT_PANEL_PORT_PROMPT=Panel port, default 2095: "
    set "TXT_PANEL_PATH_PROMPT=Panel path, default /app/: "
    set "TXT_SUB_PORT_PROMPT=Subscription port, default 2096: "
    set "TXT_SUB_PATH_PROMPT=Subscription path, default /sub/: "
    set "TXT_APPLYING_CONFIG=Applying configuration..."
    set "TXT_CONFIG_WARN=Warning: network configuration failed."
    set "TXT_ADMIN_CONFIG=Admin configuration"
    set "TXT_ADMIN_USERNAME_PROMPT=Admin username, default admin: "
    set "TXT_ADMIN_PASSWORD_PROMPT=Admin password"
    set "TXT_PASSWORD_EMPTY=Error: password cannot be empty."
    set "TXT_SETTING_ADMIN=Setting admin credentials..."
    set "TXT_ADMIN_WARN=Warning: admin credentials setup failed."
    set "TXT_INSTALL_SERVICE=Installing Windows service..."
    set "TXT_SERVICE_INSTALLED=Service installed successfully."
    set "TXT_SERVICE_INSTALL_WARN=Warning: service installation failed. You can manually run S-UI later from the control script."
    set "TXT_SETTING_PERMISSIONS=Setting directory permissions..."
    set "TXT_SETTING_ENV=Setting environment variable..."
    set "TXT_CREATING_SHORTCUT=Creating shortcuts..."
    set "TXT_STARTING_SERVICE=Starting S-UI service..."
    set "TXT_SERVICE_START_WARN=Warning: service startup failed. You can start it later through s-ui-windows.bat."
    set "TXT_INSTALL_DONE=Installation completed"
    set "TXT_PANEL_URL_LABEL=Panel URL:"
    set "TXT_SUB_URL_LABEL=Subscription URL:"
    set "TXT_ADMIN_USERNAME_LABEL=Admin username:"
    set "TXT_SERVICE_NAME_LABEL=Service name:"
    set "TXT_LAN_ACCESS=LAN access URLs:"
)
exit /b 0
