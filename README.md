# FinSort - Go Backend API (Utho Cloud FinOps)

FinSort is a peak-safe, deterministic FinOps and cloud resource rightsizing backend engine for companies running infrastructure across **Utho Cloud** regions (`in-noida-1`, `in-bangalore-1`, `in-mumbai-1`) split across `prod`, `staging`, and `dev` environments.

Over 6 months, cloud spend rose 40% (from ₹4,85,000/mo to ₹6,79,000/mo). This platform converts 30 days of raw hourly utilization metrics across 85 resources into peak-safe (`p95`/`p99`), mathematically sound, actionable engineering decisions.

---

## 🏗️ Architecture & Tech Stack

- **Language:** Go 1.22+ (Built and tested with Go 1.26)
- **Framework:** Standard library `net/http` with Go 1.22+ routing enhancements (`Method Pattern /path/{id}`)
- **Concurrency & State:** In-memory thread-safe store (`sync.RWMutex`) seeded deterministically with `seed 42` (zero external database or cloud provider dependencies required to run locally)
- **Default Port:** `:8080` (`http://localhost:8080`)
- **CORS:** Full permissive CORS headers (`Access-Control-Allow-Origin: *`, `GET, POST, PATCH, OPTIONS`, `Content-Type, Authorization`) for integration with React Vite frontend on `:5173`.
- **Python AI Microservice Bridge:** External client targeting `http://localhost:8001` (configurable via `PYTHON_AI_URL`) with a `1500ms` strict timeout and **automatic fallback to deterministic Go rule-engine proofs**.

---

## 📁 Project Directory Structure

```text
finsort-backend/
├── go.mod
├── README.md
├── main.go                       # Root runner entrypoint
├── cmd/
│   └── server/
│       └── main.go              # Production server entrypoint, router & CORS setup (:8080)
└── internal/
    ├── models/
    │   └── types.go             # Exact JSON data contract shared with React & Python
    ├── catalog/
    │   └── pricing.go           # Official Utho Cloud INR pricing catalog & compute/storage formulas
    ├── seed/
    │   └── generator.go         # Deterministic seed 42 (85 resources, 30d 180-point timeseries)
    ├── engine/
    │   ├── percentile.go        # Fast p50, p95, p99, average, and linear slope regression
    │   ├── optimizer.go         # Peak-safe rightsizing, off-hours scheduling, zombie & orphan logic
    │   ├── anomaly.go           # Z-score and slope-based anomaly & capacity leak detection (3 alerts)
    │   ├── simulator.go         # Interactive What-If scenario engine & remediation generator
    │   └── engine_test.go       # Engine math, peak-safety guardrail, and seed tests
    ├── ai/
    │   └── client.go            # HTTP client to Python AI service (:8001) + deterministic fallback
    └── api/
        ├── handlers.go          # Strict REST API contract endpoints (/api/v1/...)
        └── api_test.go          # REST API endpoint tests & simulation validation
```

---

## ⚡ Quick Start & Run

### 1. Build and Run Server

To run directly from the command line:

```bash
# Run server directly
go run ./cmd/server

# Or build the binary
go build -o bin/server.exe ./cmd/server
./bin/server.exe
```

The server binds to `http://localhost:8080`.

### 2. Verify Server Health

```bash
curl http://localhost:8080/healthz
# Response: {"status":"ok","service":"finsort-backend"}
```

### 3. Run Test Suite

```bash
go test -v ./...
```

---

## 💰 Utho Cloud Pricing Catalog (`internal/catalog/pricing.go`)

All calculations use official INR (₹) rates:

| Plan | vCPU | RAM (GB) | Monthly Rate (INR) | Hourly Rate (INR) |
|---|---|---|---|---|
| `c2.small` | 1 | 2 | ₹950 | ₹1.30 |
| `c4.medium` | 2 | 4 | ₹1,900 | ₹2.60 |
| `m4.large` | 4 | 16 | ₹4,800 | ₹6.58 |
| `m8.xlarge` | 8 | 32 | ₹9,600 | ₹13.15 |
| `r8.xlarge` | 8 | 64 | ₹13,500 | ₹18.49 |
| `c16.2xlarge` | 16 | 32 | ₹18,200 | ₹24.93 |
| `r16.2xlarge` | 16 | 64 | ₹22,400 | ₹30.68 |

- **Block SSD:** ₹8 / GB / month
- **Public IPv4:** ₹350 / month when unattached (₹0 when attached to a running instance)
- **Grid Carbon Baseline:** `0.71 kg CO2 / kWh` (CEA India Baseline)

---

## 🛡️ The 7 Workload Archetypes (Seed 42)

The engine deterministically generates 85 resources exhibiting 7 critical enterprise infrastructure patterns:

1. **Steady Over-Provisioned VMs (~20 VMs):**
   - Provisioned on `m8.xlarge` or `r16.2xlarge`.
   - Metrics: CPU avg 8–13%, p95 16–22%, p99 25%, RAM p95 < 30%.
   - Recommendation: Downsizing 1–2 tiers with verified safety headroom.

2. **Bursty Daily Peaks — The Peak Safety Trap (~10 VMs):**
   - Example: `prod-payment-webhook-01` (`m8.xlarge`, `in-mumbai-1`, `payments-core`).
   - Metrics: Average CPU is only 13.8% and RAM 21.0%. Daily 20:00–21:30 IST checkout surge spikes CPU p95 to 86.4% and p99 to 93.2%.
   - **Critical FinOps Rule:** Marked `PROTECTED_PEAK_LOAD`. Downsizing blocked to prevent checkout latency breaches and HTTP 504 gateway timeouts.

3. **Batch Jobs Idle Most of the Day (~8 VMs):**
   - Example: `prod-nightly-recon-batch` (`c16.2xlarge`). Runs at 89% CPU for ~3 hours/day (01:00–04:00 IST), idle <2% CPU for 21 hours/day.
   - Recommendation: Automated batch schedule yielding ~83% cost reduction.

4. **Dev & Staging VMs Idle Nights & Weekends (~22 VMs):**
   - Active Mon–Fri 09:00–19:00 IST (50 hrs/wk), idle 118 hrs/wk (nights + weekends).
   - Recommendation: Automated off-hours shutdown schedule saving ~65%.

5. **Zombie / Abandoned Idle VMs (~5 VMs):**
   - Created >90 days ago, CPU p99 < 3.5%, Net < 0.5 Mbps for 30 consecutive days.
   - Recommendation: Snapshot archival & termination (95% savings).

6. **Orphaned Block Volumes (12 resources):**
   - Unattached 100 GB to 1000 GB volumes with 0 IOPS.
   - Recommendation: Immediate cleanup to eliminate ₹8/GB/mo idle waste.

7. **Orphaned Public IPs (8 resources):**
   - Unattached floating IPv4 addresses incurring ₹350/mo fee each.
   - Recommendation: Immediate release to regional IP pool.

---

## 🚨 Active Anomaly Alerts (`internal/engine/anomaly.go`)

- **ALT-001 (Critical - Memory Leak):** `prod-search-indexer-04` (`in-noida-1`) RAM climbed linearly from 44.0% to 96.4% over 96 hours with 3.8x network egress spike; projected OOM in 14 hours.
- **ALT-002 (High - Cost Spike):** `staging-loadtest-cluster` (`in-bangalore-1`) 4x `r16.2xlarge` nodes running 5 days past load test window; burning ₹2,986/day (₹89,600/mo overrun).
- **ALT-003 (Medium - Capacity Trend):** `prod-postgres-wal-volume` (`in-mumbai-1`) disk usage growing +0.9%/day, currently at 91.2%, projected full in 9 days.

---

## 🔌 Strict REST API Specification

All endpoints are prefixed with `/api/v1` and return `application/json`:

### 1. `GET /api/v1/dashboard`
Returns executive KPI summary, current vs projected monthly spend in INR, accepted savings, peak protection metrics, 6-month historical trajectory, 60-day forecast, regional/env breakdowns, and sustainability metrics.

### 2. `GET /api/v1/resources`
Query parameters:
- `region`: Filter by `in-noida-1`, `in-bangalore-1`, `in-mumbai-1`
- `env`: Filter by `prod`, `staging`, `dev`
- `team`: Filter by team name
- `type`: Filter by `vm`, `block_volume`, `public_ip`
- `status`: Filter by `PROTECTED_PEAK_LOAD`, `OVERPROVISIONED`, `IDLE_SCHEDULE_CANDIDATE`, `ZOMBIE`, `ORPHANED`
- `search`: Substring search on resource ID, name, or plan

### 3. `GET /api/v1/resources/{id}`
Returns full resource details, 180 metric points of 30-day timeseries, active recommendation, and the **AI/Engineering Analysis** proof.

### 4. `GET /api/v1/recommendations`
Returns recommendation summary (counts, accepted savings, breakdowns by env and action) and array of recommendation items.

### 5. `PATCH /api/v1/recommendations/{id}`
Update recommendation status:
```json
{
  "status": "accepted"
}
```
Valid statuses: `"accepted"`, `"rejected"`, `"pending"`.

### 6. `POST /api/v1/recommendations/bulk`
Bulk approve or reject recommendations:
```json
{
  "status": "accepted",
  "filter_risk": "LOW_ONLY"
}
```

### 7. `POST /api/v1/simulate`
Interactive What-If FinOps scenario simulation:
```json
{
  "dev_off_hours_enabled": true,
  "dev_shutdown_hours_per_week": 118,
  "staging_weekend_off_enabled": true,
  "safety_headroom_buffer_pct": 25,
  "purge_orphaned_older_than_days": 14
}
```
Returns simulated spend, savings, affected VM count, newly eligible/ineligible resources when moving safety sliders, and carbon reduction.

### 8. `GET /api/v1/alerts`
Returns array of the 3 active anomaly detection alerts.

### 9. `GET /api/v1/assumptions`
Returns pricing catalog, mathematical safety formulas, off-hours schedule parameters, and CEA India grid carbon factor.

### 10. `GET /api/v1/export/remediation`
Generates ready-to-use:
- `utho_cli_script`: Executable bash script executing `utho vm resize`, `utho vm schedule`, `utho volume delete`, etc.
- `terraform_hcl`: Infrastructure as Code HCL snippet for state reconfigurations.
- `csv_report`: FinOps export table for finance and executive teams.

---

## 🤖 Python AI Microservice Webhook Contract (`internal/ai/client.go`)

The backend connects to our teammates' Python AI service configured via the `PYTHON_AI_URL` environment variable (default: `http://localhost:8001`).

### Client Configuration
- **Endpoint:** `POST $PYTHON_AI_URL/ai/explain-resource`
- **Timeout:** `1500ms` strict HTTP client context timeout.
- **Resilience Policy:** If the Python microservice is offline, times out, or returns a non-200 code, the Go backend **automatically falls back to its deterministic rule-engine math**, guaranteeing 100% uptime and zero dropped requests.

### Request Payload (Sent by Go to Python)
```json
{
  "resource": {
    "id": "res-vm-021",
    "name": "prod-payment-webhook-01",
    "type": "vm",
    "plan": "m8.xlarge",
    "region": "in-mumbai-1",
    "env": "prod",
    "team": "payments-core",
    "vcpu": 8,
    "ram_gb": 32,
    "disk_gb": 200,
    "created_at": "2025-11-10",
    "monthly_cost_inr": 9600,
    "optimization_status": "PROTECTED_PEAK_LOAD",
    "status_reason": "Avg CPU is 13.8%, but daily 20:00-21:30 IST checkout peak hits p99 93.2%. Downsizing blocked for safety.",
    "metrics_summary": {
      "cpu_avg": 13.8,
      "cpu_p50": 11.2,
      "cpu_p95": 86.4,
      "cpu_p99": 93.2,
      "mem_avg": 21.0,
      "mem_p50": 19.5,
      "mem_p95": 68.0,
      "mem_p99": 74.5,
      "disk_iops_avg": 420.0,
      "disk_iops_p95": 1850.0,
      "net_mbps_avg": 45.2,
      "net_mbps_p95": 310.0
    }
  }
}
```

### Expected Response Payload (Returned by Python to Go)
```json
{
  "resource_id": "res-vm-021",
  "provider": "Python-AI-Service",
  "peak_headroom_proof": "Mathematical proof verifying p99 peak load headroom...",
  "safety_buffer_analysis": "Target safety buffer requires 25% headroom above p95 peak...",
  "workload_classification": "Bursty Daily Peak Load (Peak-Safety Guardrail Active)",
  "downsizing_safe": false,
  "risk_score": 92,
  "technical_explanation": "Webhook service experiences severe checkout traffic bursts between 20:00-21:30 IST...",
  "blast_radius_analysis": "Downsizing directly impacts checkout conversion rate with upstream gateway timeouts...",
  "actionable_next_steps": [
    "Maintain m8.xlarge tier for peak capacity",
    "Implement asynchronous Redis/Kafka queueing buffer",
    "Configure horizontal autoscaling rather than vertical downsizing"
  ]
}
```

---

## 🧪 Verification & Test Results

All tests pass cleanly:
```text
=== RUN   TestGetDashboard
--- PASS: TestGetDashboard (0.00s)
=== RUN   TestGetResources
--- PASS: TestGetResources (0.00s)
=== RUN   TestGetResourceDetailAndAnalysis
--- PASS: TestGetResourceDetailAndAnalysis (0.02s)
=== RUN   TestPatchAndBulkRecommendations
--- PASS: TestPatchAndBulkRecommendations (0.00s)
=== RUN   TestSimulateEndpoint
--- PASS: TestSimulateEndpoint (0.00s)
=== RUN   TestAlertsAndAssumptionsAndExport
--- PASS: TestAlertsAndAssumptionsAndExport (0.00s)
PASS
ok      utho_backend/internal/api       (cached)
=== RUN   TestPercentiles
--- PASS: TestPercentiles (0.00s)
=== RUN   TestSeedDatasetGeneration
--- PASS: TestSeedDatasetGeneration (0.00s)
=== RUN   TestPeakSafetyRule
--- PASS: TestPeakSafetyRule (0.00s)
=== RUN   TestSimulationAndRemediation
--- PASS: TestSimulationAndRemediation (0.00s)
=== RUN   TestPricingTiers
--- PASS: TestPricingTiers (0.00s)
PASS
ok      utho_backend/internal/engine    (cached)
```
