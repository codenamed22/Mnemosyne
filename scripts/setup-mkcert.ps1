# Mkcert setup helper for Mnemosyne (Windows)
# Run this in PowerShell as Administrator

Write-Host "🔐 Mnemosyne Certificate Setup" -ForegroundColor Cyan
Write-Host "==============================" -ForegroundColor Cyan
Write-Host ""

# Check if mkcert is installed
try {
    $null = Get-Command mkcert -ErrorAction Stop
    Write-Host "✓ mkcert is installed" -ForegroundColor Green
} catch {
    Write-Host "❌ mkcert is not installed." -ForegroundColor Red
    Write-Host ""
    Write-Host "Install it first (run as Administrator):"
    Write-Host "  choco install mkcert"
    Write-Host "  OR"
    Write-Host "  scoop install mkcert"
    Write-Host ""
    exit 1
}

Write-Host ""

# Install the local CA
Write-Host "📜 Installing local CA (may require admin)..." -ForegroundColor Yellow
mkcert -install
Write-Host ""

# Get local IPs
Write-Host "🌐 Detecting local IP addresses..." -ForegroundColor Yellow
$localIPs = (Get-NetIPAddress -AddressFamily IPv4 | Where-Object { 
    $_.IPAddress -notlike "127.*" -and 
    $_.IPAddress -notlike "169.254.*" -and
    $_.PrefixOrigin -ne "WellKnown"
}).IPAddress

$ipList = $localIPs -join " "
Write-Host "Found IPs: $ipList"
Write-Host ""

# Create certs directory
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$certDir = Join-Path (Split-Path -Parent $scriptDir) "certs"
if (-not (Test-Path $certDir)) {
    New-Item -ItemType Directory -Path $certDir | Out-Null
}

# Generate certificate
Write-Host "🔑 Generating certificates..." -ForegroundColor Yellow
Set-Location $certDir

$domains = @("localhost", "127.0.0.1") + $localIPs
$domainString = $domains -join " "
Write-Host "Domains: $domainString"
Write-Host ""

$mkcertArgs = @("-cert-file", "server.crt", "-key-file", "server.key") + $domains
& mkcert @mkcertArgs

Write-Host ""
Write-Host "✅ Certificates generated successfully!" -ForegroundColor Green
Write-Host ""
Write-Host "📁 Certificate location:" -ForegroundColor Cyan
Write-Host "   $certDir\server.crt"
Write-Host "   $certDir\server.key"
Write-Host ""
Write-Host "📱 To access from phones/tablets without warnings:" -ForegroundColor Cyan
Write-Host "   1. Find the CA file: mkcert -CAROOT"
Write-Host "   2. Copy rootCA.pem to your device"
Write-Host "   3. Install it as a trusted certificate"
Write-Host "   → See docs/TRUSTED_CERTIFICATES.md for detailed instructions"
Write-Host ""
Write-Host "⚙️  Update your config.json:" -ForegroundColor Yellow
Write-Host '   "use_mkcert": true'
Write-Host ""

