#!/bin/bash
# CodeSwitch 基础设施启动脚本 (Bash)
# 用法: ./start-infra.sh [--dev] [--stop] [--clean] [--logs]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOCKER_DIR="$SCRIPT_DIR/../docker"

cd "$DOCKER_DIR"

# 参数解析
DEV=false
STOP=false
CLEAN=false
LOGS=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --dev)   DEV=true; shift ;;
        --stop)  STOP=true; shift ;;
        --clean) CLEAN=true; shift ;;
        --logs)  LOGS=true; shift ;;
        *)       echo "Unknown option: $1"; exit 1 ;;
    esac
done

# 检查 .env 文件
if [ ! -f ".env" ]; then
    echo "Creating .env from .env.example..."
    cp .env.example .env
    echo "Please edit .env file with your settings!"
fi

if $STOP; then
    echo "Stopping all services..."
    docker-compose down
    exit 0
fi

if $CLEAN; then
    echo "Stopping and cleaning all data..."
    docker-compose down -v
    echo "All data volumes removed!"
    exit 0
fi

if $LOGS; then
    docker-compose logs -f
    exit 0
fi

echo "========================================"
echo "  CodeSwitch Infrastructure Startup    "
echo "========================================"
echo ""

# 启动核心服务
echo "Starting core services (NATS, PostgreSQL, Redis)..."
docker-compose up -d nats postgres redis

# 等待服务就绪
echo "Waiting for services to be ready..."
sleep 5

# 检查服务状态
echo ""
echo "Service Status:"
docker-compose ps

# 启动开发工具
if $DEV; then
    echo ""
    echo "Starting development tools..."
    docker-compose --profile dev up -d redis-commander pgadmin
    sleep 3
fi

echo ""
echo "========================================"
echo "  Services Ready!                      "
echo "========================================"
echo ""
echo "Endpoints:"
echo "  NATS:       nats://localhost:4222"
echo "  NATS WS:    ws://localhost:8223"
echo "  NATS Mon:   http://localhost:8222"
echo "  PostgreSQL: localhost:5432"
echo "  Redis:      localhost:6379"

if $DEV; then
    echo ""
    echo "Dev Tools:"
    echo "  pgAdmin:    http://localhost:5050"
    echo "  Redis UI:   http://localhost:8081"
fi

echo ""
echo "Next steps:"
echo "  1. Initialize JetStream: bash ../nats/init-streams.sh"
echo "  2. Check PostgreSQL schema is created"
echo "  3. Start sync-service"
echo ""
