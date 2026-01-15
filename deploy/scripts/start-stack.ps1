# CodeSwitch Infrastructure Stack Startup Script
# Usage: .\start-stack.ps1 [-Dev] [-Down] [-Logs]

param(
    [switch]$Dev,      # Include dev tools (pgAdmin, Redis Commander)
    [switch]$Down,     # Stop all services
    [switch]$Logs,     # Follow logs
    [switch]$Build,    # Rebuild images
    [string]$Service   # Specific service to manage
)

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$DockerDir = Join-Path $ScriptDir "..\docker"
$ComposeFile = Join-Path $DockerDir "docker-compose.yml"
$EnvFile = Join-Path $DockerDir ".env"

# Check if .env exists
if (-not (Test-Path $EnvFile)) {
    Write-Host "ERROR: .env file not found!" -ForegroundColor Red
    Write-Host "Please copy .env.example to .env and configure it:" -ForegroundColor Yellow
    Write-Host "  cd $DockerDir" -ForegroundColor Cyan
    Write-Host "  cp .env.example .env" -ForegroundColor Cyan
    Write-Host "  # Edit .env with your configuration" -ForegroundColor Cyan
    exit 1
}

# Change to docker directory
Push-Location $DockerDir

try {
    if ($Down) {
        Write-Host "Stopping CodeSwitch stack..." -ForegroundColor Yellow
        if ($Service) {
            docker-compose -f $ComposeFile down $Service
        } else {
            docker-compose -f $ComposeFile --profile dev down
        }
        Write-Host "Stack stopped." -ForegroundColor Green
        exit 0
    }

    if ($Logs) {
        if ($Service) {
            docker-compose -f $ComposeFile logs -f $Service
        } else {
            docker-compose -f $ComposeFile logs -f
        }
        exit 0
    }

    # Build profiles
    $Profiles = @()
    if ($Dev) {
        $Profiles += "--profile"
        $Profiles += "dev"
        Write-Host "Including dev tools (pgAdmin, Redis Commander)..." -ForegroundColor Cyan
    }

    # Start services
    Write-Host "Starting CodeSwitch stack..." -ForegroundColor Green
    Write-Host "============================================================" -ForegroundColor Cyan

    $Args = @("-f", $ComposeFile) + $Profiles + @("up", "-d")
    if ($Build) {
        $Args += "--build"
    }
    if ($Service) {
        $Args += $Service
    }

    docker-compose @Args

    Write-Host ""
    Write-Host "============================================================" -ForegroundColor Cyan
    Write-Host "CodeSwitch Stack Started!" -ForegroundColor Green
    Write-Host "============================================================" -ForegroundColor Cyan
    Write-Host ""
    Write-Host "Services:" -ForegroundColor Yellow
    Write-Host "  PostgreSQL:      localhost:5432" -ForegroundColor White
    Write-Host "  Redis:           localhost:6379" -ForegroundColor White
    Write-Host "  NATS:            localhost:4222" -ForegroundColor White
    Write-Host "  NATS Monitor:    http://localhost:8222" -ForegroundColor White
    Write-Host "  Casdoor:         http://localhost:8000" -ForegroundColor White
    Write-Host "  Lago API:        http://localhost:3001" -ForegroundColor White
    Write-Host "  Lago UI:         http://localhost:8080" -ForegroundColor White

    if ($Dev) {
        Write-Host ""
        Write-Host "Dev Tools:" -ForegroundColor Yellow
        Write-Host "  pgAdmin:         http://localhost:5050" -ForegroundColor White
        Write-Host "  Redis Commander: http://localhost:8081" -ForegroundColor White
    }

    Write-Host ""
    Write-Host "Default Credentials:" -ForegroundColor Yellow
    Write-Host "  Casdoor:  admin / 123" -ForegroundColor White
    Write-Host "  Lago:     (first user becomes admin)" -ForegroundColor White
    Write-Host ""
    Write-Host "Next Steps:" -ForegroundColor Yellow
    Write-Host "  1. Configure Casdoor: Create application for CodeSwitch" -ForegroundColor White
    Write-Host "  2. Configure Lago: Create plans and billable metrics" -ForegroundColor White
    Write-Host "  3. Run NATS stream init: ..\nats\init-streams.sh" -ForegroundColor White
    Write-Host ""

} finally {
    Pop-Location
}
