<#
.SYNOPSIS
    SmartCloud Optimizer - Unified 3-Service Launcher
    Starts:
      1. Python AI Service (:8001)  [FastAPI + Uvicorn]
      2. Go FinOps Backend (:8080)  [smartcloud_server.exe]
      3. React Frontend (:5173)     [Vite Dev Server]
#>

$ErrorActionPreference = "Stop"

$PythonExe   = "C:\Users\Vishak Krishnan\AppData\Local\Python\pythoncore-3.14-64\python.exe"
$GoExe       = "C:\Users\Vishak Krishnan\sdk\go1.26.5\bin\go.exe"
$AiDir       = "c:\Users\Vishak Krishnan\PycharmProjects\utho_ai_app"
$BackendDir  = "c:\Users\Vishak Krishnan\GolandProjects\utho_backend"
$FrontendDir = "c:\Users\Vishak Krishnan\GolandProjects\utho_frontend"

Write-Host "==========================================================" -ForegroundColor Cyan
Write-Host "  SmartCloud Optimizer - Unified FinOps & AI Platform" -ForegroundColor Cyan
Write-Host "==========================================================" -ForegroundColor Cyan

# Helper to free ports if already bound
function Stop-PortProcess([int]$Port) {
    $conns = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
    if ($conns) {
        foreach ($c in $conns) {
            if ($c.OwningProcess -gt 4) {
                Write-Host "Stopping existing process $($c.OwningProcess) on port :$Port..." -ForegroundColor Yellow
                Stop-Process -Id $c.OwningProcess -Force -ErrorAction SilentlyContinue
            }
        }
        Start-Sleep -Milliseconds 600
    }
}

Stop-PortProcess 8001
Stop-PortProcess 8080
Stop-PortProcess 5173

# 1. Start Python AI Service (:8001)
Write-Host "`n[1/3] Starting Python AI Microservice on http://127.0.0.1:8001 ..." -ForegroundColor Green
Start-Process -FilePath $PythonExe `
    -ArgumentList "-m uvicorn app.main:app --host 0.0.0.0 --port 8001" `
    -WorkingDirectory $AiDir `
    -NoNewWindow

# 2. Build & Start Go FinOps Backend (:8080)
Write-Host "[2/3] Starting Go FinOps Backend on http://127.0.0.1:8080 ..." -ForegroundColor Green
Push-Location $BackendDir
& $GoExe build -o smartcloud_server.exe .\cmd\server
$env:PORT = "8080"
$env:PYTHON_AI_URL = "http://127.0.0.1:8001"
Start-Process -FilePath "$BackendDir\smartcloud_server.exe" `
    -WorkingDirectory $BackendDir `
    -NoNewWindow
Pop-Location

# 3. Start React Frontend (:5173)
Write-Host "[3/3] Starting React FinOps Console on http://localhost:5173 ..." -ForegroundColor Green
Start-Process -FilePath "cmd.exe" `
    -ArgumentList "/c npm.cmd run dev -- --port 5173" `
    -WorkingDirectory $FrontendDir `
    -NoNewWindow

Start-Sleep -Seconds 3
Write-Host "`nAll 3 services launched:" -ForegroundColor Green
Write-Host "  * Python AI Engine : http://localhost:8001/health"
Write-Host "  * Go FinOps API    : http://localhost:8080/api/v1/status"
Write-Host "  * React Console    : http://localhost:5173"
