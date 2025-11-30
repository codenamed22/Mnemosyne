@echo off
REM Mnemosyne Complete Setup & Start - Batch Wrapper
REM This wrapper launches the PowerShell start script

setlocal enabledelayedexpansion

cd /d "%~dp0"

REM Check if PowerShell execution policy allows script execution
powershell -Command "Write-Host 'PowerShell ready'" >nul 2>&1
if !ERRORLEVEL! neq 0 (
    echo Error: PowerShell is not available
    pause
    exit /b 1
)

REM Launch the PowerShell script
echo Starting Mnemosyne...
echo.

powershell -ExecutionPolicy Bypass -File "%~dp0start-all.ps1" %*

pause
