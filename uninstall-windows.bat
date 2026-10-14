@echo off
chcp 65001 >nul
setlocal DisableDelayedExpansion
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0uninstall-windows.ps1" %*
set "UNINSTALL_EXIT=%ERRORLEVEL%"
exit /b %UNINSTALL_EXIT%