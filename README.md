# FinSort — Go FinOps & Rightsizing Engine (`utho_backend`)

> **Jury Overview:** `utho_backend` is the high-throughput, peak-safe FinOps rule engine and API gateway for **FinSort** on **Utho Cloud** (`in-mumbai-1`, `in-noida-1`, `in-bangalore-1`). Addressing a 40% 6-month cloud spend surge (`₹4,85,000/mo → ₹6,79,000/mo`), it evaluates 30 days of continuous telemetry across **85 resources** (`Seed 42`) using **`p95`/`p99` peak-safety math**, powers interactive What-If simulations, generates **Utho Cloud Terraform HCL (`.tf`) & CLI (`.sh`) manifests**, and bridges to the Python AI service (`:8001`) with automatic zero-downtime deterministic fallback.

---

## 3-Repository System Architecture

| Repository | Tech Stack | Port | Responsibility |
| :--- | :--- | :--- | :--- |
| **`utho_backend`** *(this repo)* | Go 1.22+ (`net/http`), `sync.RWMutex` Seed 42 Store | `:8080` | REST API (`/api/v1/*`), `p95`/`p99` peak-safe optimizer, What-If simulator, Terraform HCL/CLI exporter, AI proxy |
| **`utho_frontend`** | React 19, TypeScript, Vite 8, Tailwind CSS 4 | `:5173` | Interactive FinOps Dashboard, Resource Drill-Down, Recommendations, Forecast, What-If & Terraform Modal |
| **`utho_ai_app`** | Python 3.11+, FastAPI, PyTorch TFT, Google Gemini | `:8001` | Pooled TFT `P10/P50/P90` quantile forecasting, SRE root-cause audits, Jev ordinal risk, NL What-If & AI chat |

---

## Full-Stack Setup Instructions (All 3 Repos)

### 1. Start Go FinOps Engine & API (`utho_backend` — Port `:8080`)
```bash
cd utho_backend
go run ./cmd/server
# Or build & run binary:
# go build -o finsort_server.exe ./cmd/server && ./finsort_server.exe
```
- **Healthcheck:** `curl http://localhost:8080/healthz`
- **Zero External DB Required:** Boots instantaneously with an in-memory thread-safe dataset (`Seed 42`, 85 resources × 180 4-hour intervals = 15,300 telemetry points).

### 2. Start Python AI & Forecasting Service (`utho_ai_app` — Port `:8001`)
```bash
cd utho_ai_app
python -m venv .venv
# Windows: .venv\Scripts\activate | macOS/Linux: source .venv/bin/activate
pip install -r requirements.txt
uvicorn app.main:app --host 0.0.0.0 --port 8001
```
*(Resilience Guarantee: If `:8001` is not running, `utho_backend` automatically falls back to deterministic mathematical proofs and quantile trajectories in `<2ms`.)*

### 3. Start React FinOps Frontend (`utho_frontend` — Port `:5173`)
```bash
cd utho_frontend
npm install
npm run dev
# Open http://localhost:5173
```

---

## Core FinOps Engineering Logic (For Jury Review)

### 1. Peak-Safety Guardrail (`internal/engine/optimizer.go`)
Naive FinOps tools downsize VMs based on **average** CPU, causing production outages on bursty workloads. FinSort enforces:
$$\text{Projected } p95 = \frac{\text{Current vCPU} \times \text{Observed } p95}{\text{Target vCPU}} \le 75\% \quad \text{and} \quad \text{Projected } p99 \le 85\%$$
- **Proof Case (`res-vm-021` / `prod-payment-webhook-01`):** Average CPU is `13.8%`, but daily `20:00–21:30 IST` checkout traffic spikes `p95` to `86.4%` and `p99` to `93.2%`. Halving compute would push `p99` to `186.4%` (saturation). The engine flags it `PROTECTED_PEAK_LOAD` and emits a Terraform `prevent_destroy = true` lock.

### 2. The 7 Enterprise Workload Archetypes (`internal/seed/generator.go`)
| Archetype | Count | Detection Rule | Action & Savings |
| :--- | :---: | :--- | :--- |
| **Over-Provisioned VMs** | 20 | `p95 < 25%`, `p99 < 35%` across 30d | Rightsize down 1–2 Utho tiers (`~50%` saved) |
| **Bursty Peak Load (Trap)** | 10 | Low avg (`~14%`), high `p95 > 80%` / `p99 > 90%` | **Protected** — Downsizing blocked |
| **Nightly Batch VMs** | 8 | Active 3h/day (`01:00–04:00 IST`), idle 21h | Batch cron schedule (`~83%` saved) |
| **Dev/Staging Workhours** | 22 | Idle 118h/168h per week (nights + weekends) | Off-hours schedule (`~65%` saved) |
| **Abandoned Zombie VMs** | 5 | 30d `p99 CPU < 3.5%`, `Net < 0.5 Mbps` | Snapshot + Decommission (`~95%` saved) |
| **Orphaned Block Volumes** | 12 | `AttachedTo == ""` & `0 IOPS` for 30d | Purge unattached SSD (`₹8/GB/mo` saved) |
| **Orphaned Floating IPs** | 8 | Unassigned Public IPv4 | Release to regional pool (`₹350/mo` saved) |

### 3. Official Utho Cloud INR Pricing (`internal/catalog/pricing.go`)
- **Compute Plans:** `c2.small` (`₹950`), `c4.medium` (`₹1,900`), `m4.large` (`₹4,800`), `m8.xlarge` (`₹9,600`), `r8.xlarge` (`₹13,500`), `c16.2xlarge` (`₹18,200`), `r16.2xlarge` (`₹22,400`).
- **Storage & Network:** Block SSD at `₹8/GB/mo`; Unattached IPv4 at `₹350/mo`; India CEA Grid Carbon factor at `0.71 kg CO2/kWh`.

---

## REST API Endpoints (`/api/v1`)

| Method & Path | Description |
| :--- | :--- |
| `GET /api/v1/dashboard` | Executive KPIs, Before/After monthly spend, regional/env breakdowns, 6-mo history & forecast |
| `GET /api/v1/resources` | Filterable inventory (`?region=&env=&team=&type=&status=&search=`) |
| `GET /api/v1/resources/{id}` | 30-day timeseries (180 points), recommendation, SRE proof & per-resource Terraform HCL |
| `GET /api/v1/recommendations` | All actionable recommendations with live accepted/pending totals and assumptions |
| `PATCH /api/v1/recommendations/{id}` | Accept/Reject a recommendation (`{"status": "accepted" \| "rejected" \| "pending"}`) |
| `POST /api/v1/recommendations/bulk` | Bulk accept low-risk recommendations (`{"status": "accepted", "filter_risk": "LOW_ONLY"}`) |
| `POST /api/v1/simulate` | Interactive What-If simulation (off-hours hours/wk, headroom buffer %, orphan age) |
| `GET /api/v1/alerts` | Active anomaly alerts (Memory leak OOM, runaway load-test cluster, WAL disk exhaustion) |
| `GET /api/v1/export/remediation` | Full Utho Cloud Terraform HCL (`terraform_hcl`), CLI runbook (`utho_cli_script`), & CSV (`csv_report`) |
| `GET /api/v1/ai/forecast` | 30/60/90-day P10/P50/P90 quantile forecast (`?horizon_days=30\|60\|90`) |
| `POST /api/v1/ai/what-if` | Natural-language What-If scenario parser & simulator |
| `POST /api/v1/ai/chat` | Grounded FinOps Copilot chat with source citations |

---

## Verification & Test Suite

```bash
go vet ./...
go test -v ./...
```
Covers percentile math (`p50`/`p95`/`p99`), deterministic `Seed 42` generation, peak-safety downsizing guardrails, What-If simulation math, and all `/api/v1/*` HTTP handlers.
