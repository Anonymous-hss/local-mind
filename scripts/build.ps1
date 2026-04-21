# LocalMind Extension Build Script
# Usage: .\build.ps1 [platform]
# Platforms: windows, darwin, linux, all (default)

param(
    [string]$Platform = "all",
    [switch]$Package = $false,
    [switch]$Clean = $false
)

$ErrorActionPreference = "Stop"

$ProjectRoot = Split-Path -Parent $PSScriptRoot
$CoreDir = Join-Path $ProjectRoot "localmind\packages\core"
$ExtensionDir = Join-Path $ProjectRoot "localmind\packages\extension"
$BinDir = Join-Path $ExtensionDir "bin"

# Ensure bin directory exists
if (-not (Test-Path $BinDir)) {
    New-Item -ItemType Directory -Path $BinDir -Force | Out-Null
}

# Clean build artifacts
if ($Clean) {
    Write-Host "Cleaning build artifacts..." -ForegroundColor Yellow
    Remove-Item -Path (Join-Path $BinDir "*") -Force -ErrorAction SilentlyContinue
    Remove-Item -Path (Join-Path $ExtensionDir "*.vsix") -Force -ErrorAction SilentlyContinue
    Remove-Item -Path (Join-Path $ExtensionDir "out") -Recurse -Force -ErrorAction SilentlyContinue
    Write-Host "Clean complete" -ForegroundColor Green
    if (-not $Package -and $Platform -eq "all") {
        exit 0
    }
}

function Build-Binary {
    param(
        [string]$GOOS,
        [string]$GOARCH,
        [string]$OutputName
    )

    Write-Host "Building for $GOOS/$GOARCH..." -ForegroundColor Cyan

    $env:GOOS = $GOOS
    $env:GOARCH = $GOARCH
    $env:CGO_ENABLED = "0"

    $outputPath = Join-Path $BinDir $OutputName

    Push-Location $CoreDir
    try {
        go build -tags nocgo -ldflags="-s -w" -o $outputPath ./cmd/localmind
        if ($LASTEXITCODE -ne 0) {
            throw "Build failed for $GOOS/$GOARCH"
        }
        Write-Host "  Built: $OutputName" -ForegroundColor Green
    }
    finally {
        Pop-Location
        Remove-Item Env:GOOS -ErrorAction SilentlyContinue
        Remove-Item Env:GOARCH -ErrorAction SilentlyContinue
        Remove-Item Env:CGO_ENABLED -ErrorAction SilentlyContinue
    }
}

# Build for requested platforms
$builds = @()

switch ($Platform.ToLower()) {
    "windows" {
        $builds += @{ GOOS = "windows"; GOARCH = "amd64"; Output = "localmind-windows-amd64.exe" }
    }
    "darwin" {
        $builds += @{ GOOS = "darwin"; GOARCH = "amd64"; Output = "localmind-darwin-amd64" }
        $builds += @{ GOOS = "darwin"; GOARCH = "arm64"; Output = "localmind-darwin-arm64" }
    }
    "linux" {
        $builds += @{ GOOS = "linux"; GOARCH = "amd64"; Output = "localmind-linux-amd64" }
        $builds += @{ GOOS = "linux"; GOARCH = "arm64"; Output = "localmind-linux-arm64" }
    }
    "all" {
        $builds += @{ GOOS = "windows"; GOARCH = "amd64"; Output = "localmind-windows-amd64.exe" }
        $builds += @{ GOOS = "darwin"; GOARCH = "amd64"; Output = "localmind-darwin-amd64" }
        $builds += @{ GOOS = "darwin"; GOARCH = "arm64"; Output = "localmind-darwin-arm64" }
        $builds += @{ GOOS = "linux"; GOARCH = "amd64"; Output = "localmind-linux-amd64" }
        $builds += @{ GOOS = "linux"; GOARCH = "arm64"; Output = "localmind-linux-arm64" }
    }
    default {
        Write-Error "Unknown platform: $Platform. Use: windows, darwin, linux, or all"
        exit 1
    }
}

foreach ($build in $builds) {
    Build-Binary -GOOS $build.GOOS -GOARCH $build.GOARCH -OutputName $build.Output
}

# Build TypeScript extension
Write-Host "Building TypeScript extension..." -ForegroundColor Cyan
Push-Location $ExtensionDir
try {
    npm run compile
    if ($LASTEXITCODE -ne 0) {
        throw "TypeScript build failed"
    }
    Write-Host "  TypeScript build complete" -ForegroundColor Green
}
finally {
    Pop-Location
}

# Package VSIX if requested
if ($Package) {
    Write-Host "Packaging VSIX..." -ForegroundColor Cyan
    Push-Location $ExtensionDir
    try {
        # Check if vsce is available
        $vsceInstalled = Get-Command vsce -ErrorAction SilentlyContinue
        if (-not $vsceInstalled) {
            Write-Host "  Installing vsce..." -ForegroundColor Yellow
            npm install -g @vscode/vsce
        }

        vsce package --no-dependencies
        if ($LASTEXITCODE -ne 0) {
            throw "VSIX packaging failed"
        }

        $vsix = Get-ChildItem -Path $ExtensionDir -Filter "*.vsix" | Select-Object -First 1
        Write-Host "  Created: $($vsix.Name)" -ForegroundColor Green
    }
    finally {
        Pop-Location
    }
}

Write-Host ""
Write-Host "Build complete!" -ForegroundColor Green
Write-Host "Binaries in: $BinDir" -ForegroundColor Cyan
