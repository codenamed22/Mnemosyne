# Mnemosyne Complete Setup & Start Script
# This script rebuilds the Go application, starts the Python CLIP service, and launches the main server

param(
    [switch]$SkipBuild = $false,
    [switch]$SkipCLIP = $false
)

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path

Write-Host "====================================================================" -ForegroundColor Cyan
Write-Host "          Mnemosyne - Photo Cloud with AI Features" -ForegroundColor Cyan
Write-Host "====================================================================" -ForegroundColor Cyan
Write-Host ""

# Step 1: Import Chocolatey environment
Write-Host "[1/4] Setting up environment..." -ForegroundColor Yellow
try {
    Import-Module $env:ChocolateyInstall\helpers\chocolateyProfile.psm1 -ErrorAction SilentlyContinue
    refreshenv
} catch {
    Write-Host "Warning: Could not import Chocolatey profile (non-critical)" -ForegroundColor DarkYellow
}

# Step 2: Enable CGO and build Go application
if (-not $SkipBuild) {
    Write-Host "[2/4] Building Mnemosyne Go application..." -ForegroundColor Yellow
    
    cd $ScriptDir
    
    # Set CGO_ENABLED for go-sqlite3
    $env:CGO_ENABLED = 1
    
    Write-Host "       Building executable..." -ForegroundColor Cyan
    go build -o mnemosyne.exe .
    
    if ($LASTEXITCODE -ne 0) {
        Write-Host "X Build failed!" -ForegroundColor Red
        exit 1
    }
    
    Write-Host "[OK] Build successful!" -ForegroundColor Green
}
else {
    Write-Host "[2/4] Skipping build (use -SkipBuild to skip)" -ForegroundColor Gray
}

# Step 3: Start Python CLIP Embedding Service
if (-not $SkipCLIP) {
    Write-Host "[3/4] Starting CLIP Embedding Service..." -ForegroundColor Yellow
    
    $embeddingsDir = Join-Path $ScriptDir "embeddings"
    
    # Check if Python packages are installed
    Write-Host "       Checking Python dependencies..." -ForegroundColor Cyan
    $pythonExe = "C:/Users/navne/AppData/Local/Microsoft/WindowsApps/python3.10.exe"
    
    if (-not (Test-Path $pythonExe)) {
        Write-Host "X Python not found at $pythonExe" -ForegroundColor Red
        exit 1
    }
    
    # Start CLIP service in a new window
    Write-Host "       Launching CLIP service on http://127.0.0.1:8081..." -ForegroundColor Cyan
    Start-Process powershell -ArgumentList "-NoExit", "-Command", "cd '$embeddingsDir'; & '$pythonExe' main.py" -WindowStyle Normal
    
    # Give the service time to start
    Write-Host "       Waiting for CLIP service to initialize..." -ForegroundColor Cyan
    Start-Sleep -Seconds 3
    
    Write-Host "[OK] CLIP service started!" -ForegroundColor Green
}
else {
    Write-Host "[3/4] Skipping CLIP service (already running?)" -ForegroundColor Gray
}

# Step 4: Start Mnemosyne Server
Write-Host "[4/4] Starting Mnemosyne Server..." -ForegroundColor Yellow
Write-Host ""

cd $ScriptDir

Write-Host "====================================================================" -ForegroundColor Green
Write-Host "                  MNEMOSYNE IS STARTING..." -ForegroundColor Green
Write-Host "====================================================================" -ForegroundColor Green
Write-Host ""

# Run the server
& .\mnemosyne.exe

# If we get here, the server was stopped
Write-Host ""
Write-Host "====================================================================" -ForegroundColor Yellow
Write-Host "                   SERVER STOPPED" -ForegroundColor Yellow
Write-Host "====================================================================" -ForegroundColor Yellow
