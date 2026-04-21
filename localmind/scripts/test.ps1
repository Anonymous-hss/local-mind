# LocalMind Test Suite Runner
#
# Runs all project tests (Go unit/integration/e2e, TypeScript build/lint,
# GUI browser tests) and prints a consolidated summary.
#
# Usage:
#   .\scripts\test.ps1           # Run unit + TypeScript + GUI tests
#   .\scripts\test.ps1 -All      # Run everything including integration + e2e
#   .\scripts\test.ps1 -Unit     # Go unit tests only
#   .\scripts\test.ps1 -GUI      # GUI tests only
#   .\scripts\test.ps1 -TS       # TypeScript build/lint only

param(
    [switch]$All,
    [switch]$Unit,
    [switch]$GUI,
    [switch]$TS,
    [switch]$Integration,
    [switch]$E2E,
    [switch]$Screenshots
)

$ErrorActionPreference = "Continue"

$ProjectRoot = Split-Path -Parent $PSScriptRoot
$CoreDir = Join-Path $ProjectRoot "packages\core"
$ExtensionDir = Join-Path $ProjectRoot "packages\extension"
$ScriptsDir = $PSScriptRoot

# If no flags specified, run Unit + TS + GUI
if (-not ($All -or $Unit -or $GUI -or $TS -or $Integration -or $E2E)) {
    $Unit = $true
    $TS = $true
    $GUI = $true
}
if ($All) {
    $Unit = $true
    $TS = $true
    $GUI = $true
    $Integration = $true
    $E2E = $true
}

# --- State ---
$script:phases = @()
$script:overallPassed = $true
$startTime = Get-Date

function Add-Phase($Name, [bool]$Passed, $Details, [int]$Total = 0, [int]$PassedCount = 0, [int]$FailedCount = 0) {
    $script:phases += New-Object PSObject -Property @{
        Name    = $Name
        Passed  = $Passed
        Details = $Details
        Total   = $Total
        PassedC = $PassedCount
        FailedC = $FailedCount
    }
    if (-not $Passed) { $script:overallPassed = $false }
}

function Write-Section($title) {
    Write-Host ""
    Write-Host "=======================================================" -ForegroundColor DarkCyan
    Write-Host "  $title" -ForegroundColor Cyan
    Write-Host "=======================================================" -ForegroundColor DarkCyan
}

# ===================================================================
# PHASE 1: Go Build Check
# ===================================================================

if ($Unit -or $Integration -or $E2E) {
    Write-Section "PHASE 1: Go Build Check"

    $env:CGO_ENABLED = "0"
    Push-Location $CoreDir
    try {
        $buildOutput = & go build -tags nocgo ./... 2>&1 | Out-String
        $buildExitCode = $LASTEXITCODE
    }
    finally {
        Pop-Location
    }

    if ($buildExitCode -eq 0) {
        Write-Host "  [PASS] Build succeeded" -ForegroundColor Green
        Add-Phase -Name "Go Build" -Passed $true -Details "Clean build"
    }
    else {
        Write-Host "  [FAIL] Build failed:" -ForegroundColor Red
        Write-Host $buildOutput -ForegroundColor Yellow
        Add-Phase -Name "Go Build" -Passed $false -Details $buildOutput
    }
}

# ===================================================================
# PHASE 2: Go Unit Tests (via test runner)
# ===================================================================

if ($Unit) {
    Write-Section "PHASE 2: Go Unit Tests"

    $testRunnerArgs = @()
    if ($Integration) { $testRunnerArgs += "-integration" }
    if ($E2E) { $testRunnerArgs += "-e2e" }

    Push-Location $CoreDir
    try {
        $jsonOutput = & go run ./cmd/testrunner @testRunnerArgs 2>$null | Out-String
        $testExitCode = $LASTEXITCODE
    }
    finally {
        Pop-Location
    }

    if ($jsonOutput.Trim()) {
        try {
            $report = $jsonOutput | ConvertFrom-Json

            $total = $report.summary.total
            $passed = $report.summary.passed
            $failed = $report.summary.failed
            $skipped = $report.summary.skipped

            Write-Host "  Total:   $total" -ForegroundColor White
            Write-Host "  Passed:  $passed" -ForegroundColor Green
            if ($failed -gt 0) {
                Write-Host "  Failed:  $failed" -ForegroundColor Red
            }
            if ($skipped -gt 0) {
                Write-Host "  Skipped: $skipped" -ForegroundColor Yellow
            }
            Write-Host "  Duration: $($report.duration)" -ForegroundColor DarkGray

            # Show failed tests
            if ($failed -gt 0) {
                Write-Host ""
                Write-Host "  FAILURES:" -ForegroundColor Red
                foreach ($pkg in $report.packages) {
                    foreach ($test in $pkg.tests) {
                        if ((-not $test.passed) -and (-not $test.skipped)) {
                            Write-Host "    [X] $($pkg.name)/$($test.name)" -ForegroundColor Red
                            if ($test.output) {
                                $lines = $test.output -split "`n" | Select-Object -First 5
                                foreach ($line in $lines) {
                                    Write-Host "      $line" -ForegroundColor Yellow
                                }
                            }
                        }
                    }
                }
            }

            # Show build errors
            if ($report.build_errors -and $report.build_errors.Count -gt 0) {
                Write-Host ""
                Write-Host "  BUILD ERRORS:" -ForegroundColor Red
                foreach ($err in $report.build_errors) {
                    Write-Host "    $err" -ForegroundColor Yellow
                }
            }

            $goTestsPassed = ($failed -eq 0)
            if ($report.build_errors -and $report.build_errors.Count -gt 0) {
                $goTestsPassed = $false
            }
            Add-Phase -Name "Go Tests" -Passed $goTestsPassed -Details $jsonOutput -Total $total -PassedCount $passed -FailedCount $failed

            # Save raw JSON report for AI consumption
            $reportPath = Join-Path $CoreDir "test_report.json"
            $jsonOutput | Out-File -FilePath $reportPath -Encoding UTF8
            Write-Host ""
            Write-Host "  Full report: $reportPath" -ForegroundColor DarkGray
        }
        catch {
            Write-Host "  [WARN] Failed to parse test runner output" -ForegroundColor Yellow
            Write-Host $jsonOutput
            Add-Phase -Name "Go Tests" -Passed $false -Details "JSON parse error: $($_.Exception.Message)"
        }
    }
    else {
        Write-Host "  [WARN] No output from test runner (exit code: $testExitCode)" -ForegroundColor Yellow
        Add-Phase -Name "Go Tests" -Passed $false -Details "No test runner output"
    }
}

# ===================================================================
# PHASE 3: TypeScript Build + Lint
# ===================================================================

if ($TS) {
    Write-Section "PHASE 3: TypeScript Build + Lint"

    Push-Location $ExtensionDir
    try {
        # Compile
        Write-Host "  Compiling TypeScript..." -ForegroundColor White
        $compileOutput = & npm run compile 2>&1 | Out-String
        $compileCode = $LASTEXITCODE

        if ($compileCode -eq 0) {
            Write-Host "  [PASS] TypeScript compile succeeded" -ForegroundColor Green
        }
        else {
            Write-Host "  [FAIL] TypeScript compile failed:" -ForegroundColor Red
            Write-Host $compileOutput -ForegroundColor Yellow
        }

        # Lint
        Write-Host "  Linting TypeScript..." -ForegroundColor White
        $lintOutput = & npm run lint 2>&1 | Out-String
        $lintCode = $LASTEXITCODE

        if ($lintCode -eq 0) {
            Write-Host "  [PASS] TypeScript lint passed" -ForegroundColor Green
        }
        else {
            Write-Host "  [FAIL] TypeScript lint failed:" -ForegroundColor Red
            Write-Host $lintOutput -ForegroundColor Yellow
        }

        $tsPassed = ($compileCode -eq 0) -and ($lintCode -eq 0)
        $details = ""
        if ($compileCode -ne 0) { $details += "Compile errors: $compileOutput`n" }
        if ($lintCode -ne 0) { $details += "Lint errors: $lintOutput" }
        if ($tsPassed) { $details = "Clean" }

        Add-Phase -Name "TypeScript" -Passed $tsPassed -Details $details
    }
    finally {
        Pop-Location
    }
}

# ===================================================================
# PHASE 4: GUI Tests (Puppeteer)
# ===================================================================

if ($GUI) {
    Write-Section "PHASE 4: GUI Tests (Browser)"

    # Check if puppeteer is installed
    $puppeteerPath = Join-Path $ProjectRoot "node_modules\puppeteer"
    if (-not (Test-Path $puppeteerPath)) {
        Write-Host "  Installing Puppeteer..." -ForegroundColor Yellow
        Push-Location $ProjectRoot
        try {
            & npm install --save-dev puppeteer 2>&1 | Out-Null
        }
        finally {
            Pop-Location
        }
    }

    $guiArgs = @()
    if ($Screenshots) { $guiArgs += "--screenshots" }

    $guiScript = Join-Path $ScriptsDir "gui_test.js"

    Push-Location $ProjectRoot
    try {
        $guiOutput = & node $guiScript @guiArgs 2>$null | Out-String
        $guiExitCode = $LASTEXITCODE
    }
    finally {
        Pop-Location
    }

    if ($guiOutput.Trim()) {
        try {
            $guiReport = $guiOutput | ConvertFrom-Json

            $guiTotal = $guiReport.summary.total
            $guiPassed = $guiReport.summary.passed
            $guiFailed = $guiReport.summary.failed

            Write-Host "  Total:   $guiTotal" -ForegroundColor White
            Write-Host "  Passed:  $guiPassed" -ForegroundColor Green
            if ($guiFailed -gt 0) {
                Write-Host "  Failed:  $guiFailed" -ForegroundColor Red
                Write-Host ""
                Write-Host "  FAILURES:" -ForegroundColor Red
                foreach ($test in $guiReport.tests) {
                    if (-not $test.passed) {
                        Write-Host "    [X] $($test.name)" -ForegroundColor Red
                        Write-Host "      $($test.error)" -ForegroundColor Yellow
                        if ($test.screenshot) {
                            Write-Host "      Screenshot: $($test.screenshot)" -ForegroundColor DarkGray
                        }
                    }
                }
            }

            Add-Phase -Name "GUI Tests" -Passed ($guiFailed -eq 0) -Details $guiOutput -Total $guiTotal -PassedCount $guiPassed -FailedCount $guiFailed

            # Save GUI report
            $guiReportPath = Join-Path $ExtensionDir "test\gui_report.json"
            $guiOutput | Out-File -FilePath $guiReportPath -Encoding UTF8
            Write-Host ""
            Write-Host "  Full report: $guiReportPath" -ForegroundColor DarkGray
        }
        catch {
            Write-Host "  [WARN] Failed to parse GUI test output" -ForegroundColor Yellow
            Write-Host $guiOutput
            Add-Phase -Name "GUI Tests" -Passed $false -Details "JSON parse error: $($_.Exception.Message)"
        }
    }
    else {
        Write-Host "  [WARN] No output from GUI tests (exit code: $guiExitCode)" -ForegroundColor Yellow
        Add-Phase -Name "GUI Tests" -Passed $false -Details "No GUI test output"
    }
}

# ===================================================================
# FINAL SUMMARY
# ===================================================================

$elapsed = (Get-Date) - $startTime

Write-Host ""
Write-Host "+-------------------------------------------------------+" -ForegroundColor Cyan
Write-Host "|             LOCALMIND TEST RESULTS                     |" -ForegroundColor Cyan
Write-Host "+-------------------------------------------------------+" -ForegroundColor Cyan

foreach ($phase in $phases) {
    if ($phase.Passed) {
        $icon = "[PASS]"
        $color = "Green"
    }
    else {
        $icon = "[FAIL]"
        $color = "Red"
    }
    $count = ""
    if ($phase.Total -gt 0) {
        $count = " ($($phase.PassedC)/$($phase.Total))"
    }
    $line = "  $icon $($phase.Name)$count"
    Write-Host "| $($line.PadRight(53)) |" -ForegroundColor $color
}

Write-Host "+-------------------------------------------------------+" -ForegroundColor Cyan

if ($script:overallPassed) {
    Write-Host "|   ALL PASS                                            |" -ForegroundColor Green
}
else {
    Write-Host "|   SOME TESTS FAILED                                   |" -ForegroundColor Red
}

$timeLine = "  Time: $([math]::Round($elapsed.TotalSeconds, 1))s"
Write-Host "| $($timeLine.PadRight(53)) |" -ForegroundColor DarkGray
Write-Host "+-------------------------------------------------------+" -ForegroundColor Cyan
Write-Host ""

if ($script:overallPassed) {
    exit 0
}
else {
    exit 1
}
