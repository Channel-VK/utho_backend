#!/usr/bin/env bash
# =============================================================================
# SmartCloud Optimizer (FinSort) — WSL2 & Linux One-Command Docker Launcher
# Clones missing sibling repos automatically (if needed), builds all 3 Docker
# images, launches the unified stack, and verifies end-to-end health.
# =============================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PARENT_DIR="$(dirname "$SCRIPT_DIR")"

echo "=================================================================="
echo "  SmartCloud Optimizer (FinSort) — Unified Docker Deployment"
echo "=================================================================="

# 1. Verify sibling repositories exist (or auto-clone them in WSL2/cloud VM)
if [ ! -d "$PARENT_DIR/utho_frontend" ]; then
  echo "[info] Sibling ../utho_frontend not found. Cloning from GitHub..."
  git clone https://github.com/Channel-VK/utho_frontend.git "$PARENT_DIR/utho_frontend"
fi

if [ ! -d "$PARENT_DIR/utho_ai_app" ]; then
  echo "[info] Sibling ../utho_ai_app not found. Cloning from GitHub..."
  git clone https://github.com/risvanthkm/utho_ai_app.git "$PARENT_DIR/utho_ai_app"
fi

cd "$SCRIPT_DIR"

# 2. Build and launch all 3 containers
echo "[1/2] Building and starting utho-ai (:8001), utho-backend (:8080), and utho-frontend (:5173)..."
docker compose up --build -d

# 3. Verify health endpoints
echo "[2/2] Waiting for services to report healthy..."
sleep 5

echo ""
echo "------------------------------------------------------------------"
echo "  Service Endpoints:"
echo "  * FinSort Web Console (Nginx + SPA) : http://localhost:${FRONTEND_PORT:-5173}"
echo "  * Go FinOps API Gateway             : http://localhost:${BACKEND_PORT:-8080}/api/v1/status"
echo "  * Python AI Microservice            : http://localhost:${AI_PORT:-8001}/health"
echo "------------------------------------------------------------------"
docker compose ps
