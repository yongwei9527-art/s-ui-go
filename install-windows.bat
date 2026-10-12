@echo off
chcp 65001 >nul
setlocal DisableDelayedExpansion
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%~dp0install-windows.ps1" %*
set "INSTALL_EXIT=%ERRORLEVEL%"
exit /b %INSTALL_EXIT%
