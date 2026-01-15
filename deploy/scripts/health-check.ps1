# CodeSwitch 服务健康检查脚本

$ErrorActionPreference = "SilentlyContinue"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  CodeSwitch Health Check              " -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# NATS 检查
Write-Host "Checking NATS..." -NoNewline
try {
    $response = Invoke-WebRequest -Uri "http://localhost:8222/healthz" -TimeoutSec 3
    if ($response.StatusCode -eq 200) {
        Write-Host " OK" -ForegroundColor Green

        # 获取 NATS 详细信息
        $varz = Invoke-RestMethod -Uri "http://localhost:8222/varz" -TimeoutSec 3
        Write-Host "  Version: $($varz.version)" -ForegroundColor Gray
        Write-Host "  Connections: $($varz.connections)" -ForegroundColor Gray
        Write-Host "  JetStream: $($varz.jetstream.config.store_dir)" -ForegroundColor Gray
    }
} catch {
    Write-Host " FAILED" -ForegroundColor Red
    Write-Host "  Error: Cannot connect to NATS at localhost:4222" -ForegroundColor Red
}

Write-Host ""

# PostgreSQL 检查
Write-Host "Checking PostgreSQL..." -NoNewline
try {
    $env:PGPASSWORD = "codeswitch_dev_2026"
    $result = & psql -h localhost -U codeswitch -d codeswitch -c "SELECT 1;" 2>&1
    if ($LASTEXITCODE -eq 0) {
        Write-Host " OK" -ForegroundColor Green

        # 获取表数量
        $tables = & psql -h localhost -U codeswitch -d codeswitch -t -c "SELECT count(*) FROM information_schema.tables WHERE table_schema = 'public';" 2>&1
        Write-Host "  Tables: $($tables.Trim())" -ForegroundColor Gray
    } else {
        throw "Connection failed"
    }
} catch {
    Write-Host " FAILED" -ForegroundColor Red
    Write-Host "  Error: Cannot connect to PostgreSQL at localhost:5432" -ForegroundColor Red
    Write-Host "  Tip: Make sure psql is installed or use Docker exec" -ForegroundColor Yellow
}

Write-Host ""

# Redis 检查
Write-Host "Checking Redis..." -NoNewline
try {
    $result = & redis-cli ping 2>&1
    if ($result -eq "PONG") {
        Write-Host " OK" -ForegroundColor Green

        # 获取 Redis 信息
        $info = & redis-cli info server 2>&1
        $version = ($info | Select-String "redis_version").ToString().Split(":")[1]
        Write-Host "  Version: $version" -ForegroundColor Gray

        $dbsize = & redis-cli dbsize 2>&1
        Write-Host "  Keys: $dbsize" -ForegroundColor Gray
    } else {
        throw "Ping failed"
    }
} catch {
    Write-Host " FAILED" -ForegroundColor Red
    Write-Host "  Error: Cannot connect to Redis at localhost:6379" -ForegroundColor Red
}

Write-Host ""

# Docker 容器状态
Write-Host "Docker Containers:" -ForegroundColor Cyan
docker ps --filter "name=codeswitch" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
