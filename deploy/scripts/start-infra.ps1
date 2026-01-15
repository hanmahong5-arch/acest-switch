# CodeSwitch 基础设施启动脚本 (PowerShell)
# 用法: .\start-infra.ps1 [-Dev] [-Stop] [-Clean]

param(
    [switch]$Dev,      # 启动开发工具 (pgAdmin, Redis Commander)
    [switch]$Stop,     # 停止所有服务
    [switch]$Clean,    # 清理数据卷
    [switch]$Logs      # 查看日志
)

$ErrorActionPreference = "Stop"
$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$DockerDir = Join-Path $ScriptDir "..\docker"

Push-Location $DockerDir

try {
    # 检查 .env 文件
    if (-not (Test-Path ".env")) {
        Write-Host "Creating .env from .env.example..." -ForegroundColor Yellow
        Copy-Item ".env.example" ".env"
        Write-Host "Please edit .env file with your settings!" -ForegroundColor Yellow
    }

    if ($Stop) {
        Write-Host "Stopping all services..." -ForegroundColor Cyan
        docker-compose down
        exit 0
    }

    if ($Clean) {
        Write-Host "Stopping and cleaning all data..." -ForegroundColor Red
        docker-compose down -v
        Write-Host "All data volumes removed!" -ForegroundColor Red
        exit 0
    }

    if ($Logs) {
        docker-compose logs -f
        exit 0
    }

    Write-Host "========================================" -ForegroundColor Green
    Write-Host "  CodeSwitch Infrastructure Startup    " -ForegroundColor Green
    Write-Host "========================================" -ForegroundColor Green
    Write-Host ""

    # 启动核心服务
    Write-Host "Starting core services (NATS, PostgreSQL, Redis)..." -ForegroundColor Cyan
    docker-compose up -d nats postgres redis

    # 等待服务就绪
    Write-Host "Waiting for services to be ready..." -ForegroundColor Yellow
    Start-Sleep -Seconds 5

    # 检查服务状态
    Write-Host ""
    Write-Host "Service Status:" -ForegroundColor Green
    docker-compose ps

    # 启动开发工具
    if ($Dev) {
        Write-Host ""
        Write-Host "Starting development tools..." -ForegroundColor Cyan
        docker-compose --profile dev up -d redis-commander pgadmin
        Start-Sleep -Seconds 3
    }

    Write-Host ""
    Write-Host "========================================" -ForegroundColor Green
    Write-Host "  Services Ready!                      " -ForegroundColor Green
    Write-Host "========================================" -ForegroundColor Green
    Write-Host ""
    Write-Host "Endpoints:" -ForegroundColor Cyan
    Write-Host "  NATS:       nats://localhost:4222" -ForegroundColor White
    Write-Host "  NATS WS:    ws://localhost:8223" -ForegroundColor White
    Write-Host "  NATS Mon:   http://localhost:8222" -ForegroundColor White
    Write-Host "  PostgreSQL: localhost:5432" -ForegroundColor White
    Write-Host "  Redis:      localhost:6379" -ForegroundColor White

    if ($Dev) {
        Write-Host ""
        Write-Host "Dev Tools:" -ForegroundColor Cyan
        Write-Host "  pgAdmin:    http://localhost:5050" -ForegroundColor White
        Write-Host "  Redis UI:   http://localhost:8081" -ForegroundColor White
    }

    Write-Host ""
    Write-Host "Next steps:" -ForegroundColor Yellow
    Write-Host "  1. Initialize JetStream: bash ../nats/init-streams.sh"
    Write-Host "  2. Check PostgreSQL schema is created"
    Write-Host "  3. Start sync-service"
    Write-Host ""

} finally {
    Pop-Location
}
