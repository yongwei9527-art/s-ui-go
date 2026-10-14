# Windows Reference Files

This directory keeps legacy entry points for compatibility. The BAT files now delegate to the canonical root scripts rather than maintaining independent installers or uninstallers.

## Recommended source of truth

Use the scripts in the repository root as the canonical versions:

- `build-windows.ps1`: recommended PowerShell build and packaging script
- `build-windows.bat`: simple interactive CMD build script
- `install-windows.bat`: Windows installation script
- `s-ui-windows.bat`: Windows service management script
- `uninstall-windows.bat`: Windows uninstallation script
- `s-ui-windows.xml`: Windows service configuration

If a root script and a file in this directory differ, update and run the root script first. The BAT wrappers refuse to run if the canonical root script is missing. Use the root PowerShell installer/uninstaller so safety checks and private data permissions cannot diverge.

## Install on Windows

From an extracted release package, run `install-windows.bat` as Administrator, then use `s-ui-windows.bat` for service management.

## Build from source

Run the root PowerShell script from the repository root:

```powershell
cd ..
.\build-windows.ps1 -System windows -Architecture amd64 -Package -NonInteractive
```

To list local generated-file cleanup candidates without deleting anything:

```powershell
.\build-windows.ps1 -ListCleanCandidates
```
