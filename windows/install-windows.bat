@echo off
setlocal DisableDelayedExpansion
rem Legacy entry point: always delegate to the canonical root script.
if not exist "%~dp0..\install-windows.bat" (
    echo Missing canonical root script: install-windows.bat
    exit /b 1
)
pushd "%~dp0.."
if errorlevel 1 exit /b 1
call ".\install-windows.bat" %*
set "LEGACY_EXIT=%ERRORLEVEL%"
popd
exit /b %LEGACY_EXIT%