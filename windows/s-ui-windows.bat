@echo off
setlocal DisableDelayedExpansion
rem Legacy entry point: always delegate to the canonical root script.
if not exist "%~dp0..\s-ui-windows.bat" (
    echo Missing canonical root script: s-ui-windows.bat
    exit /b 1
)
pushd "%~dp0.."
if errorlevel 1 exit /b 1
call ".\s-ui-windows.bat" %*
set "LEGACY_EXIT=%ERRORLEVEL%"
popd
exit /b %LEGACY_EXIT%