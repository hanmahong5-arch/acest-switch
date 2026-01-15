@echo off
REM CodeSwitch 基础设施启动脚本 (Windows CMD)
REM 用法: start-infra.bat [dev]

setlocal enabledelayedexpansion

cd /d "%~dp0..\docker"

REM 检查 Docker
docker --version >nul 2>&1
if errorlevel 1 (
    echo ERROR: Docker not found. Please install Docker Desktop.
    exit /b 1
)

REM 检查 .env 文件
if not exist ".env" (
    echo Creating .env from .env.example...
    copy .env.example .env
    echo Please edit .env file with your settings!
)

echo ========================================
echo   CodeSwitch Infrastructure Startup
echo ========================================
echo.

REM 启动核心服务
echo Starting core services (NATS, PostgreSQL, Redis)...
docker-compose up -d nats postgres redis

REM 等待服务就绪
echo Waiting for services to be ready...
timeout /t 5 /nobreak >nul

REM 检查服务状态
echo.
echo Service Status:
docker-compose ps

REM 启动开发工具
if "%1"=="dev" (
    echo.
    echo Starting development tools...
    docker-compose --profile dev up -d redis-commander pgadmin
    timeout /t 3 /nobreak >nul
)

echo.
echo ========================================
echo   Services Ready!
echo ========================================
echo.
echo Endpoints:
echo   NATS:       nats://localhost:4222
echo   NATS WS:    ws://localhost:8223
echo   NATS Mon:   http://localhost:8222
echo   PostgreSQL: localhost:5432
echo   Redis:      localhost:6379

if "%1"=="dev" (
    echo.
    echo Dev Tools:
    echo   pgAdmin:    http://localhost:5050
    echo   Redis UI:   http://localhost:8081
)

echo.
echo Next steps:
echo   1. Initialize JetStream streams
echo   2. Check PostgreSQL schema is created
echo   3. Start sync-service
echo.

endlocal
