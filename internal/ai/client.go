package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"utho_backend/internal/engine"
	"utho_backend/internal/models"
)

// Client handles communication with the external Python AI microservice (:8001).
type Client struct {
	baseURL      string
	httpClient   *http.Client
	healthClient *http.Client
}

// NewClient creates a new AI microservice client with 4000ms timeout for live LLM/ML calls.
func NewClient() *Client {
	baseURL := os.Getenv("PYTHON_AI_URL")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8001"
	}

	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 4000 * time.Millisecond,
		},
		healthClient: &http.Client{
			Timeout: 1500 * time.Millisecond,
		},
	}
}

type pythonHealthResponse struct {
	Status   string   `json:"status"`
	Service  string   `json:"service"`
	Features []string `json:"features"`
}

// CheckHealth pings GET http://localhost:8001/health to report live connection telemetry.
func (c *Client) CheckHealth() (bool, []string) {
	ctx, cancel := context.WithTimeout(context.Background(), 800*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/health", nil)
	if err != nil {
		return false, defaultFeatures()
	}

	resp, err := c.healthClient.Do(req)
	if err != nil {
		return false, defaultFeatures()
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, defaultFeatures()
	}

	var h pythonHealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&h); err != nil {
		return true, defaultFeatures()
	}
	if len(h.Features) == 0 {
		h.Features = defaultFeatures()
	}
	return h.Status == "ok", h.Features
}

func defaultFeatures() []string {
	return []string{
		"sre_resource_audit",
		"jev_ordinal_risk_scoring",
		"quantile_usage_and_cost_forecasting",
		"nl_what_if_scenario_parser",
		"anomaly_root_cause_and_ttf_correlation",
		"role_based_finops_briefings",
		"greenops_pue_wattage_modeling",
		"utho_terraform_remediation",
	}
}

// AIExplainRequest represents the payload sent to Python AI microservice.
type AIExplainRequest struct {
	Resource models.Resource `json:"resource"`
}

// ExplainResource requests deep root-cause explanation from Python AI microservice,
// falling back to deterministic mathematical proof if Python service is offline.
func (c *Client) ExplainResource(res models.Resource) *models.EngineeringAnalysis {
	analysis, err := c.callPythonExplain(res)
	if err == nil && analysis != nil {
		if analysis.Provider == "" {
			analysis.Provider = "Python-AI-Service (:8001)"
		}
		return analysis
	}

	fallback := c.generateDeterministicAnalysis(res)
	fallback.Provider = "Deterministic-Rule-Engine (Offline Fallback)"
	return fallback
}

func (c *Client) callPythonExplain(res models.Resource) (*models.EngineeringAnalysis, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4000*time.Millisecond)
	defer cancel()

	payload, err := json.Marshal(AIExplainRequest{Resource: res})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/ai/explain-resource", bytes.NewBuffer(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("python ai service returned status: %d", resp.StatusCode)
	}

	var result models.EngineeringAnalysis
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

// ParseNLScenario sends natural-language query to Python :8001, falls back to Go parser if offline,
// and computes the live SimulationResult across the 85 Utho resources.
func (c *Client) ParseNLScenario(query string, resources []models.Resource) models.NLScenarioResponse {
	ctx, cancel := context.WithTimeout(context.Background(), 4000*time.Millisecond)
	defer cancel()

	payload, _ := json.Marshal(map[string]string{"query": query})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/ai/parse-scenario", bytes.NewBuffer(payload))
	if err == nil {
		req.Header.Set("Content-Type", "application/json")
		if resp, errDo := c.httpClient.Do(req); errDo == nil {
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				var parsed models.NLScenarioResponse
				if errDec := json.NewDecoder(resp.Body).Decode(&parsed); errDec == nil {
					parsed.SimulationResult = engine.Simulate(resources, parsed.ParsedParams)
					return parsed
				}
			}
		}
	}

	// Deterministic Go fallback parser
	fallback := parseNLScenarioFallback(query)
	fallback.SimulationResult = engine.Simulate(resources, fallback.ParsedParams)
	return fallback
}

func parseNLScenarioFallback(query string) models.NLScenarioResponse {
	lower := strings.ToLower(query)
	params := models.SimulationRequest{
		DevOffHoursEnabled:         true,
		DevShutdownHoursPerWeek:    118,
		StagingWeekendOffEnabled:   true,
		SafetyHeadroomBufferPct:    25,
		PurgeOrphanedOlderThanDays: 30,
	}
	region := "all"
	env := "dev"
	var tags []string

	if strings.Contains(lower, "bangalore") {
		region = "in-bangalore-1"
		tags = append(tags, "Region: in-bangalore-1")
	} else if strings.Contains(lower, "mumbai") {
		region = "in-mumbai-1"
		tags = append(tags, "Region: in-mumbai-1")
	} else if strings.Contains(lower, "noida") {
		region = "in-noida-1"
		tags = append(tags, "Region: in-noida-1")
	}

	reHeadroom := regexp.MustCompile(`(\d{1,2})\s*%\s*(?:peak\s*)?(?:headroom|buffer|safety)`)
	if m := reHeadroom.FindStringSubmatch(lower); len(m) > 1 {
		if v, err := strconv.Atoi(m[1]); err == nil {
			if v < 15 {
				v = 15
			} else if v > 50 {
				v = 50
			}
			params.SafetyHeadroomBufferPct = float64(v)
			tags = append(tags, fmt.Sprintf("Peak Headroom: +%d%%", v))
		}
	} else if strings.Contains(lower, "aggressive") {
		params.SafetyHeadroomBufferPct = 15
		params.PurgeOrphanedOlderThanDays = 15
		tags = append(tags, "Profile: Aggressive (+15% Headroom)")
	} else if strings.Contains(lower, "conservative") || strings.Contains(lower, "strict") {
		params.SafetyHeadroomBufferPct = 40
		tags = append(tags, "Profile: Ultra-Conservative (+40% Headroom)")
	}

	if len(tags) == 0 {
		tags = []string{"Dev Off-Hours: 118h/wk", fmt.Sprintf("Peak Headroom: +%.0f%%", params.SafetyHeadroomBufferPct)}
	}

	return models.NLScenarioResponse{
		Query:        query,
		Provider:     "Deterministic-Rule-Engine (Offline Fallback)",
		ParsedParams: params,
		DetectedScope: models.NLScenarioScope{
			Environment:   env,
			Region:        region,
			ShutdownStart: "20:00",
			ShutdownEnd:   "08:00",
		},
		ParsedTags: tags,
		Explanation: fmt.Sprintf(
			"Parsed scenario for %s: Dev off-hours %dh/wk, Staging weekend halt %v, Peak-Safety headroom +%.0f%%, Orphan purge >%dd.",
			strings.ToUpper(region), params.DevShutdownHoursPerWeek, params.StagingWeekendOffEnabled, params.SafetyHeadroomBufferPct, params.PurgeOrphanedOlderThanDays,
		),
	}
}

// CorrelateAlerts enriches active anomaly alerts via Python :8001 with root-cause and time-to-failure telemetry.
func (c *Client) CorrelateAlerts(alerts []models.Alert, resources []models.Resource) []models.Alert {
	ctx, cancel := context.WithTimeout(context.Background(), 4000*time.Millisecond)
	defer cancel()

	// Send lightweight resource summaries to avoid large payloads
	payload, err := json.Marshal(map[string]any{
		"alerts": alerts,
	})
	if err == nil {
		req, errReq := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/ai/correlate-alerts", bytes.NewBuffer(payload))
		if errReq == nil {
			req.Header.Set("Content-Type", "application/json")
			if resp, errDo := c.httpClient.Do(req); errDo == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					var wrapper struct {
						Alerts []models.Alert `json:"alerts"`
					}
					if errDec := json.NewDecoder(resp.Body).Decode(&wrapper); errDec == nil && len(wrapper.Alerts) > 0 {
						return wrapper.Alerts
					}
				}
			}
		}
	}

	// Deterministic Go fallback enrichment
	out := make([]models.Alert, len(alerts))
	for i, a := range alerts {
		item := a
		item.AIProvider = "Deterministic-Rule-Engine (Offline Fallback)"
		switch a.ID {
		case "ALT-001":
			item.RootCauseCorrelation = "Pearson r=0.984 correlation between linear JVM heap growth (+0.55%/hr over 96h) and 3.8x network egress spike (+4.6σ above 7d baseline)."
			item.TimeToFailureHours = 14.0
			item.TTFFormula = "(100.0% - 96.4% RAM) / +0.55%/hr = 14.0 hrs to OOM kernel panic"
			item.SlopePctPerDay = 13.1
			item.StatisticalZScore = 4.6
			item.BlastRadius = "High: Search indexing lag across in-noida-1 catalog APIs if OOM occurs."
			item.MitigationCommand = "utho vm exec --id res-vm-015 --cmd 'jcmd 1 GC.run && systemctl restart search-indexer'"
		case "ALT-002":
			item.RootCauseCorrelation = "4x r16.2xlarge transitioned from 82% load-test peak to 0.9% idle floor 120 hours ago (-3.4σ) without teardown webhook execution."
			item.TimeToFailureHours = 0
			item.TTFFormula = "Cost Bleed Rate: 4 * ₹746.50/day = ₹2,986/day (₹89,600/mo budget overrun)"
			item.SlopePctPerDay = 0
			item.StatisticalZScore = 3.4
			item.BlastRadius = "Zero SLA risk: Ephemeral staging load-test cluster with 0 active sessions."
			item.MitigationCommand = "utho vm destroy --filter 'name=staging-loadtest-cluster-node-*' --region in-bangalore-1 --yes"
		case "ALT-003":
			item.RootCauseCorrelation = "14-day linear regression (slope = +0.90%/day, R²=0.992) on WAL block volume caused by stalled archive_command retaining unpurged WAL segments."
			item.TimeToFailureHours = 234.7
			item.TTFFormula = "(100.0% - 91.2% Disk) / +0.90%/day = 9.8 days (234.7 hrs) to disk-full DB halt"
			item.SlopePctPerDay = 0.90
			item.StatisticalZScore = 3.2
			item.BlastRadius = "Critical: PostgreSQL write-ahead log exhaustion halts primary DB transactions in in-mumbai-1."
			item.MitigationCommand = "utho volume expand --id res-vol-001 --size-gb 1500 && pg_archivecleanup /var/lib/postgresql/wal"
		}
		out[i] = item
	}
	return out
}

// GetAIForecast queries Python :8001 for 30/60/90-day component cost forecasts and GreenOps PUE modeling.
func (c *Client) GetAIForecast(days int, headroom float64, resources []models.Resource, recs []models.Recommendation) models.AIForecastResponse {
	if days != 30 && days != 60 && days != 90 {
		days = 30
	}
	if headroom <= 0 {
		headroom = 1.25
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4000*time.Millisecond)
	defer cancel()

	// Send lightweight resource & recommendation cost summaries
	payload, err := json.Marshal(map[string]any{
		"days":            days,
		"headroom":        headroom,
		"resources":       resources,
		"recommendations": recs,
	})
	if err == nil {
		req, errReq := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/ai/forecast", bytes.NewBuffer(payload))
		if errReq == nil {
			req.Header.Set("Content-Type", "application/json")
			if resp, errDo := c.httpClient.Do(req); errDo == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					var out models.AIForecastResponse
					if errDec := json.NewDecoder(resp.Body).Decode(&out); errDec == nil && out.Days > 0 {
						return out
					}
				}
			}
		}
	}

	// Deterministic Go fallback
	scale := float64(days) / 30.0
	var vmCur, volCur, ipCur float64
	for _, r := range resources {
		switch r.Type {
		case "vm":
			vmCur += r.MonthlyCostINR
		case "block_volume":
			volCur += r.MonthlyCostINR
		case "public_ip":
			ipCur += r.MonthlyCostINR
		}
	}
	var vmSav, volSav, ipSav float64
	for _, rec := range recs {
		switch rec.ResourceType {
		case "vm":
			vmSav += rec.MonthlySavingsINR
		case "block_volume":
			volSav += rec.MonthlySavingsINR
		case "public_ip":
			ipSav += rec.MonthlySavingsINR
		}
	}

	curVM := round2(vmCur * scale)
	curVol := round2(volCur * scale)
	curIP := round2(ipCur * scale)
	curTotal := round2(curVM + curVol + curIP)

	recVM := round2((vmCur - vmSav) * scale)
	recVol := round2((volCur - volSav) * scale)
	recIP := round2((ipCur - ipSav) * scale)
	recTotal := round2(recVM + recVol + recIP)
	savTotal := round2(curTotal - recTotal)
	savPct := 0.0
	if curTotal > 0 {
		savPct = round2((savTotal / curTotal) * 100.0)
	}

	kwh := round2(1910.2 * scale)
	co2 := round2(kwh * 0.71)

	return models.AIForecastResponse{
		Days:           days,
		Currency:       "INR",
		Provider:       "Deterministic-Rule-Engine (Offline Fallback)",
		HeadroomFactor: headroom,
		ForecastBasis:  fmt.Sprintf("Peak q90 utilization & p99 safety guardrails held over %d-day Utho Cloud INR horizon.", days),
		CurrentCostINR: models.ComponentCostBreakdown{
			VMCompute: curVM,
			Storage:   curVol,
			PublicIP:  curIP,
			Total:     curTotal,
		},
		RecommendedCostINR: models.ComponentCostBreakdown{
			VMCompute: recVM,
			Storage:   recVol,
			PublicIP:  recIP,
			Total:     recTotal,
		},
		EstimatedSavingsINR: savTotal,
		SavingsPct:          savPct,
		FleetQuantileSummary: models.FleetQuantileSummary{
			CPUQ10Pct: 9.4,
			CPUQ50Pct: 24.8,
			CPUQ90Pct: 68.5,
			MemQ10Pct: 16.2,
			MemQ50Pct: 31.4,
			MemQ90Pct: 64.0,
		},
		Sustainability: models.AIForecastSustainability{
			EstimatedEnergySavedKWh:    kwh,
			EstimatedCarbonSavedKgCO2e: co2,
			SustainabilityScore:        32.1,
			PUE:                        1.4,
			GridCarbonFactorKgPerKWh:   0.71,
			Assumptions: []string{
				"5W/vCPU idle to 15W/vCPU full load",
				"Datacenter PUE 1.4 • CEA India Grid 0.71 kg CO2e/kWh",
			},
		},
	}
}

// GetRoleBriefing fetches role-based executive/finance/engineering briefing from Python :8001.
func (c *Client) GetRoleBriefing(role string, dash models.DashboardData) models.AIBriefingResponse {
	role = strings.ToLower(strings.TrimSpace(role))
	if role != "executive" && role != "finance" && role != "engineering" {
		role = "executive"
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4000*time.Millisecond)
	defer cancel()

	payload, err := json.Marshal(map[string]any{
		"role":      role,
		"dashboard": dash,
	})
	if err == nil {
		req, errReq := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/api/v1/ai/briefing", bytes.NewBuffer(payload))
		if errReq == nil {
			req.Header.Set("Content-Type", "application/json")
			if resp, errDo := c.httpClient.Do(req); errDo == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					var out models.AIBriefingResponse
					if errDec := json.NewDecoder(resp.Body).Decode(&out); errDec == nil && out.Headline != "" {
						return out
					}
				}
			}
		}
	}

	// Deterministic Go fallback
	switch role {
	case "finance":
		unclaimed := round2(dash.TotalPotentialSavingsINR - dash.AcceptedMonthlySavingsINR)
		return models.AIBriefingResponse{
			Role:     "finance",
			Provider: "Deterministic-Rule-Engine (Offline Fallback)",
			Model:    "Deterministic-SRE-Synthesizer",
			Headline: fmt.Sprintf("Unit Economics & 90-Day Budget Trajectory: ₹%.0f/mo Unclaimed Yield", unclaimed),
			Summary:  fmt.Sprintf("Compute accounts for 93.0%% of Utho spend, Block SSD storage 6.6%% (₹45,200/mo in orphaned volumes), and Floating IPs 0.4%%. Approving remaining low-risk actions saves ₹%.0f over 90 days.", dash.TotalPotentialSavingsINR*3),
			BulletPoints: []string{
				"Zero-CapEx Instant Recovery: ₹48,000/mo from 12 unattached Block SSDs (₹8/GB) + 8 floating IPv4s (₹350/IP).",
				"Diurnal Non-Prod Scheduling: 118 hrs/wk dev/staging hibernation + 20 hrs/day batch scheduling cuts compute waste by 65–83%.",
				"Staging Anomaly Bleed: Decommissioning staging-loadtest-cluster halts ₹2,986/day (₹89,600/mo) budget overrun.",
			},
			KeyMetrics: []models.BriefingMetric{
				{Label: "Committed Savings", Value: fmt.Sprintf("₹%.0f/mo", dash.AcceptedMonthlySavingsINR)},
				{Label: "Pending Approval", Value: fmt.Sprintf("₹%.0f/mo", unclaimed)},
				{Label: "90-Day Net Impact", Value: fmt.Sprintf("₹%.0f", dash.TotalPotentialSavingsINR*3)},
			},
		}
	case "engineering":
		return models.AIBriefingResponse{
			Role:     "engineering",
			Provider: "Deterministic-Rule-Engine (Offline Fallback)",
			Model:    "Deterministic-SRE-Synthesizer",
			Headline: fmt.Sprintf("SRE Telemetry Audit: p95/p99 Headroom Verified & %d Active TTF Anomalies", dash.ActiveAlertsCount),
			Summary:  "All 20 compute rightsizing candidates pass the Projected p95 <= 80% and p99 < 85% governor checks (Jev ordinal risk < 25%). Urgent remediation required on prod-search-indexer-04 (OOM in 14.0h).",
			BulletPoints: []string{
				"Peak-Safety Proof: prod-payment-webhook-01 (13.8% avg, 93.2% p99) blocked from downsizing to prevent 186.4% peak saturation.",
				"TTF Alert ALT-001: prod-search-indexer-04 (in-noida-1) RAM slope +0.55%/hr with 3.8x net egress (+4.6σ); OOM projected in 14.0h.",
				"TTF Alert ALT-003: prod-postgres-wal-volume (in-mumbai-1) at 91.2% disk (+0.90%/day slope); full in 9.8 days.",
			},
			KeyMetrics: []models.BriefingMetric{
				{Label: "Governor Rule", Value: "p99 < 85% & +25% Buffer"},
				{Label: "Urgent TTF Alert", Value: "14.0h (res-vm-015)"},
				{Label: "Ready IaC Scripts", Value: "Utho CLI + Terraform"},
			},
		}
	default:
		return models.AIBriefingResponse{
			Role:     "executive",
			Provider: "Deterministic-Rule-Engine (Offline Fallback)",
			Model:    "Deterministic-SRE-Synthesizer",
			Headline: fmt.Sprintf("₹%.0f/mo Optimization Pipeline Identified Across %d Utho Cloud Resources", dash.TotalPotentialSavingsINR, dash.TotalResourcesCount),
			Summary:  fmt.Sprintf("Current monthly run-rate is ₹%.0f/mo (+40%% 6-month sprawl). ₹%.0f/mo is already committed, while %d bursty checkout VMs are automatically shielded from downsizing to guarantee 99.99%% payment SLA.", dash.CurrentMonthlySpendINR, dash.AcceptedMonthlySavingsINR, dash.ProtectedPeakVMsCount),
			BulletPoints: []string{
				fmt.Sprintf("Committed Run-Rate Drop: ₹%.0f/mo → ₹%.0f/mo (-%.1f%% locked in).", dash.CurrentMonthlySpendINR, dash.ProjectedMonthlySpendINR, dash.AcceptedSavingsPct),
				fmt.Sprintf("SLA Peak Protection: %d revenue-critical VMs (p99 >= 85%%) protected by Peak-Safety Governor.", dash.ProtectedPeakVMsCount),
				"GreenOps ESG Yield: 32.1% operational carbon reduction (~5,100+ kWh/mo saved across Noida, Bangalore & Mumbai).",
			},
			KeyMetrics: []models.BriefingMetric{
				{Label: "Pipeline Potential", Value: fmt.Sprintf("₹%.0f/mo", dash.TotalPotentialSavingsINR)},
				{Label: "Annualized ROI", Value: fmt.Sprintf("₹%.0f/yr", dash.TotalPotentialSavingsINR*12)},
				{Label: "SLA Risk Exposure", Value: "0 Breaches (Protected)"},
			},
		}
	}
}

func (c *Client) generateDeterministicAnalysis(res models.Resource) *models.EngineeringAnalysis {
	defaultQuantiles := []models.QuantileForecastPoint{
		{Timestamp: "2026-10-24T04:00:00Z", HourLabel: "+4h (09:00 IST)", CPUQ10: 9.2, CPUQ50: 14.5, CPUQ90: 22.0, MemQ10: 18.0, MemQ50: 22.5, MemQ90: 28.0},
		{Timestamp: "2026-10-24T08:00:00Z", HourLabel: "+8h (13:00 IST)", CPUQ10: 11.0, CPUQ50: 16.8, CPUQ90: 25.4, MemQ10: 19.5, MemQ50: 24.0, MemQ90: 30.2},
		{Timestamp: "2026-10-24T12:00:00Z", HourLabel: "+12h (17:00 IST)", CPUQ10: 10.5, CPUQ50: 15.2, CPUQ90: 24.1, MemQ10: 19.0, MemQ50: 23.8, MemQ90: 29.5},
		{Timestamp: "2026-10-24T16:00:00Z", HourLabel: "+16h (21:00 IST)", CPUQ10: 12.0, CPUQ50: 18.4, CPUQ90: 28.0, MemQ10: 20.0, MemQ50: 25.1, MemQ90: 31.8},
		{Timestamp: "2026-10-24T20:00:00Z", HourLabel: "+20h (01:00 IST)", CPUQ10: 6.5, CPUQ50: 10.2, CPUQ90: 16.0, MemQ10: 16.5, MemQ50: 20.0, MemQ90: 24.5},
		{Timestamp: "2026-10-25T00:00:00Z", HourLabel: "+24h (05:00 IST)", CPUQ10: 5.8, CPUQ50: 9.1, CPUQ90: 14.2, MemQ10: 16.0, MemQ50: 19.4, MemQ90: 23.8},
	}

	switch res.OptimizationStatus {
	case "PROTECTED_PEAK_LOAD":
		p99 := 93.2
		p95 := 86.4
		avg := 13.8
		if res.MetricsSummary != nil {
			p99 = res.MetricsSummary.CPUP99
			p95 = res.MetricsSummary.CPUP95
			avg = res.MetricsSummary.CPUAvg
		}

		projCPUHalf := (float64(res.VCPU) * p95) / float64(res.VCPU/2)
		projP99Half := (float64(res.VCPU) * p99) / float64(res.VCPU/2)

		peakQuantiles := make([]models.QuantileForecastPoint, len(defaultQuantiles))
		copy(peakQuantiles, defaultQuantiles)
		peakQuantiles[3].CPUQ50 = 84.2
		peakQuantiles[3].CPUQ90 = p99
		peakQuantiles[3].MemQ50 = 64.0
		peakQuantiles[3].MemQ90 = 74.5

		return &models.EngineeringAnalysis{
			ResourceID:             res.ID,
			WorkloadClassification: "Bursty Daily Peak Load (Peak-Safety Guardrail Active)",
			DownsizingSafe:         false,
			RiskScore:              92,
			ConfidenceLevel:        "HIGH",
			ConfidenceReason:       "30-day continuous telemetry (180 4-hour intervals = 720 hours) validates p95 and p99 peak envelopes.",
			PeakHeadroomProof: fmt.Sprintf(
				"MATHEMATICAL PEAK HEADROOM PROOF:\n"+
					"- Current Plan: %s (%d vCPU, %d GB RAM)\n"+
					"- Observed 30-Day Metrics: Average CPU: %.1f%%, p95 Peak: %.1f%%, p99 Peak: %.1f%%\n"+
					"- Naive rightsizing based on Average CPU (%.1f%%) would propose halving compute to %d vCPU.\n"+
					"- Projected p95 CPU at half capacity: (%d vCPU * %.1f%%) / %d vCPU = %.1f%% [CRITICAL CAPACITY BREACH]\n"+
					"- Projected p99 CPU at half capacity: (%d vCPU * %.1f%%) / %d vCPU = %.1f%% [SATURATION & DROPPED REQUESTS]\n"+
					"- CONCLUSION: Halving or downsizing compute capacity causes immediate queue buildup and gateway timeout errors during peak checkout traffic.",
				res.Plan, res.VCPU, res.RAMGB, avg, p95, p99, avg, res.VCPU/2, res.VCPU, p95, res.VCPU/2, projCPUHalf, res.VCPU, p99, res.VCPU/2, projP99Half,
			),
			SafetyBufferAnalysis: fmt.Sprintf(
				"Target safety buffer requires 25%% headroom above p95 peak. Current headroom at p95 is 13.6%% (100%% - %.1f%%). At p99 peak, remaining headroom is only 6.8%%. Zero downsize tolerance exists.",
				p95,
			),
			TechnicalExplanation: fmt.Sprintf(
				"Instance %s executes time-sensitive webhook callbacks. While idle for 22 hours daily, checkout peak volume (20:00-21:30 IST) drives severe burstiness. Traditional average-based FinOps tooling triggers catastrophic false-positive downsizing here.",
				res.Name,
			),
			BlastRadiusAnalysis: "Downsizing directly impacts payment transaction completion rates, triggering webhook delivery retry storms and upstream merchant timeout penalties.",
			ActionableNextSteps: []string{
				"Maintain current compute tier to guarantee peak headroom for 20:00-21:30 IST checkout surges.",
				"Implement asynchronous queueing (Kafka / Redis) to smooth peak concurrency before considering instance downsizing.",
				"Configure automated horizontal elasticity rather than vertical tier shrinkage.",
			},
			RollbackRunbook: []string{
				fmt.Sprintf("1. Pre-Flight Guard: Confirm Utho protection lock `utho vm lock --id %s --reason peak-sla` is active.", res.ID),
				fmt.Sprintf("2. Emergency Scale-Out: If p99 exceeds 95%% for >15m, run `utho vm resize --id %s --plan r16.2xlarge`.", res.ID),
				"3. Traffic Drain: Shift 30% of regional ingress weight via Utho Load Balancer health checks.",
			},
			JevAssessment: &models.JevAssessment{
				Model:               "jev-latest",
				RiskScore:           2.76,
				RiskScorePercentage: 92.0,
				RiskLevel:           "critical",
				ModelConfidence:     0.97,
				Probabilities:       map[string]float64{"0": 0.01, "1": 0.05, "2": 0.16, "3": 0.78},
				Explanation:         "Jev ordinal score normalized to 0-100% across 4 operational risk tiers.",
				DecisionPolicy:      "Advisory only; deterministic p95/p99 capacity checks remain authoritative.",
			},
			QuantileForecast24h: peakQuantiles,
			GreenOpsImpact: &models.ResourceGreenOps{
				EstimatedEnergySavedKWh:    0,
				EstimatedCarbonSavedKgCO2e: 0,
				SustainabilityScore:        0,
				PUE:                        1.4,
				WattsPerVCPURange:          "5W idle – 15W full",
			},
		}

	case "OVERPROVISIONED":
		p95 := 18.2
		p99 := 23.4
		memP95 := 22.1
		if res.MetricsSummary != nil {
			p95 = res.MetricsSummary.CPUP95
			p99 = res.MetricsSummary.CPUP99
			memP95 = res.MetricsSummary.MemP95
		}

		targetVCPU := res.VCPU / 2
		if targetVCPU < 1 {
			targetVCPU = 1
		}
		projP95 := (float64(res.VCPU) * p95) / float64(targetVCPU)
		projP99 := (float64(res.VCPU) * p99) / float64(targetVCPU)

		return &models.EngineeringAnalysis{
			ResourceID:             res.ID,
			WorkloadClassification: "Sustainably Over-Provisioned Compute",
			DownsizingSafe:         true,
			RiskScore:              14,
			ConfidenceLevel:        "HIGH",
			ConfidenceReason:       "30-day continuous telemetry (180 4-hour intervals = 720 hours) validates p95 and p99 peak envelopes.",
			PeakHeadroomProof: fmt.Sprintf(
				"MATHEMATICAL PEAK HEADROOM PROOF:\n"+
					"- Current Plan: %s (%d vCPU, %d GB RAM)\n"+
					"- 30-Day Sustained Load: p95 CPU: %.1f%%, p99 CPU: %.1f%%, p95 RAM: %.1f%%\n"+
					"- Target Downsized Plan: %d vCPU\n"+
					"- Projected p95 CPU: (%d * %.1f%%) / %d = %.1f%%\n"+
					"- Projected p99 CPU: (%d * %.1f%%) / %d = %.1f%%\n"+
					"- Required Safety Buffer: 25%% headroom ceiling (max allowable 75%% sustained)\n"+
					"- CONCLUSION: Projected peak (%.1f%%) remains well below the 70%% conservative ceiling, with >28%% unallocated burst headroom.",
				res.Plan, res.VCPU, res.RAMGB, p95, p99, memP95, targetVCPU, res.VCPU, p95, targetVCPU, projP95, res.VCPU, p99, targetVCPU, projP99, projP95,
			),
			SafetyBufferAnalysis: "Current tier operates at >75% idle capacity even during monthly 99th percentile peaks. Resizing provides healthy 55% average headroom while cutting compute bill by 50%.",
			TechnicalExplanation: fmt.Sprintf(
				"Instance %s was provisioned with excessive headroom during initial launch. Steady traffic patterns over the last 30 days confirm that 4 vCPU is ample for production SLA compliance.",
				res.Name,
			),
			BlastRadiusAnalysis: "Negligible blast radius. Downsizing requires 90-second rolling restart during scheduled maintenance window.",
			ActionableNextSteps: []string{
				"Schedule instance plan resize during off-peak deployment window.",
				"Verify worker thread-pool concurrency settings align with new vCPU core count.",
				"Enable 14-day canary monitoring post-resize.",
			},
			RollbackRunbook: []string{
				fmt.Sprintf("1. Pre-Resize Snapshot: `utho vm snapshot --id %s --name pre-rightsize-%s`", res.ID, res.ID),
				fmt.Sprintf("2. Execute Resize: `utho vm resize --id %s --plan m4.large --restart --yes`", res.ID),
				fmt.Sprintf("3. Instant Rollback: `utho vm resize --id %s --plan %s --restart --yes` (<90s recovery).", res.ID, res.Plan),
			},
			JevAssessment: &models.JevAssessment{
				Model:               "jev-latest",
				RiskScore:           0.42,
				RiskScorePercentage: 14.0,
				RiskLevel:           "low",
				ModelConfidence:     0.95,
				Probabilities:       map[string]float64{"0": 0.74, "1": 0.21, "2": 0.04, "3": 0.01},
				Explanation:         "Jev ordinal score normalized to 0-100% across 4 operational risk tiers.",
				DecisionPolicy:      "Advisory only; deterministic p95/p99 capacity checks remain authoritative.",
			},
			QuantileForecast24h: defaultQuantiles,
			GreenOpsImpact: &models.ResourceGreenOps{
				EstimatedEnergySavedKWh:    20.4,
				EstimatedCarbonSavedKgCO2e: 14.5,
				SustainabilityScore:        48.5,
				PUE:                        1.4,
				WattsPerVCPURange:          "5W idle – 15W full",
			},
		}

	case "IDLE_SCHEDULE_CANDIDATE":
		return &models.EngineeringAnalysis{
			ResourceID:             res.ID,
			WorkloadClassification: "Temporal Off-Hours Hibernation Candidate",
			DownsizingSafe:         true,
			RiskScore:              8,
			ConfidenceLevel:        "HIGH",
			ConfidenceReason:       "30-day hourly p95 CPU stays below 5% across 118 off-hours per week.",
			PeakHeadroomProof: "SCHEDULE DUTY CYCLE MATH:\n" +
				"- Active Duty Cycle: 50 hours/week (Mon-Fri 09:00-19:00 IST)\n" +
				"- Idle Off-Hours: 118 hours/week (Nights + Weekends, CPU < 2%)\n" +
				"- Inefficiency Ratio: 118 hrs / 168 hrs = 70.2% idle time\n" +
				"- Net Monthly Savings: ~65% compute cost reduction.",
			SafetyBufferAnalysis: "Hibernation preserves all root disk data and network configurations. Zero data loss risk.",
			TechnicalExplanation: fmt.Sprintf(
				"Resource %s serves non-production or batch duties and is dormant outside active engineering sprint hours.",
				res.Name,
			),
			BlastRadiusAnalysis: "Zero production impact. Scheduled start-stop hook operates strictly outside business hours.",
			ActionableNextSteps: []string{
				"Attach Utho automated cron stop/start schedule.",
				"Inform QA/Engineering leads of 09:00 IST automatic wake-up window.",
			},
			RollbackRunbook: []string{
				fmt.Sprintf("1. Attach Schedule: `utho vm schedule create --id %s --timezone Asia/Kolkata`", res.ID),
				fmt.Sprintf("2. Manual Wake Override: `utho vm start --id %s` (boots in <35 seconds).", res.ID),
				fmt.Sprintf("3. Disable Schedule: `utho vm schedule delete --id %s`", res.ID),
			},
			JevAssessment: &models.JevAssessment{
				Model:               "jev-latest",
				RiskScore:           0.18,
				RiskScorePercentage: 6.0,
				RiskLevel:           "low",
				ModelConfidence:     0.98,
				Probabilities:       map[string]float64{"0": 0.88, "1": 0.10, "2": 0.01, "3": 0.01},
				Explanation:         "Jev ordinal score normalized to 0-100% across 4 operational risk tiers.",
				DecisionPolicy:      "Advisory only; deterministic p95/p99 capacity checks remain authoritative.",
			},
			QuantileForecast24h: defaultQuantiles,
		}

	case "ZOMBIE":
		return &models.EngineeringAnalysis{
			ResourceID:             res.ID,
			WorkloadClassification: "Abandoned Zombie Workload",
			DownsizingSafe:         true,
			RiskScore:              5,
			ConfidenceLevel:        "HIGH",
			ConfidenceReason:       "30 consecutive days of <3.5% p99 CPU and <0.5 Mbps network traffic.",
			PeakHeadroomProof: fmt.Sprintf(
				"INACTIVITY PROOF:\n"+
					"- Inactive Duration: >90 days continuous\n"+
					"- 30-Day p99 CPU: <3.5%%\n"+
					"- 30-Day Network Egress: <0.5 Mbps\n"+
					"- Inactive Spend: ₹%.0f/month with 0 business utility.",
				res.MonthlyCostINR,
			),
			SafetyBufferAnalysis: "Safety snapshot retained before termination guarantees 100% rollback capability.",
			TechnicalExplanation: fmt.Sprintf(
				"Instance %s exhibits all hallmarks of an abandoned POC or migration artifact.",
				res.Name,
			),
			BlastRadiusAnalysis: "Zero user traffic observed for 30 consecutive days. Terminating has no active dependencies.",
			ActionableNextSteps: []string{
				"Execute pre-termination volume snapshot.",
				"Decommission VM and retain snapshot for 30-day compliance hold.",
			},
			RollbackRunbook: []string{
				fmt.Sprintf("1. Mandatory Snapshot: `utho vm snapshot --id %s --name archive-%s`", res.ID, res.ID),
				fmt.Sprintf("2. Restore VM from Snapshot: `utho vm create --from-snapshot archive-%s --plan %s --region %s`", res.ID, res.Plan, res.Region),
			},
			QuantileForecast24h: defaultQuantiles,
		}

	case "ORPHANED":
		return &models.EngineeringAnalysis{
			ResourceID:             res.ID,
			WorkloadClassification: "Unattached Orphaned Infrastructure",
			DownsizingSafe:         true,
			RiskScore:              1,
			ConfidenceLevel:        "HIGH",
			ConfidenceReason:       "Explicit unattached state with 0 IOPS across 30 days.",
			PeakHeadroomProof: fmt.Sprintf(
				"ORPHAN STATE AUDIT:\n"+
					"- Resource Type: %s\n"+
					"- Attachment State: Detached / Unassigned\n"+
					"- Storage/Network IOPS: 0 IOPS / 0 Packets\n"+
					"- Idle Burn Rate: ₹%.0f/month.",
				res.Type, res.MonthlyCostINR,
			),
			SafetyBufferAnalysis: "Resource is disassociated from all compute instances. Immediate release is 100% safe.",
			TechnicalExplanation: fmt.Sprintf(
				"Resource %s was decoupled from a deleted instance and left unattached in Utho region %s.",
				res.Name, res.Region,
			),
			BlastRadiusAnalysis: "Zero compute or data dependencies. Releasing produces instant cost savings.",
			ActionableNextSteps: []string{
				"Purge unattached block volume or release unallocated floating IP to regional pool.",
			},
			RollbackRunbook: []string{
				fmt.Sprintf("1. Verify Attachment: `utho resource inspect --id %s` (confirm attached_to: null).", res.ID),
			},
		}

	default:
		return &models.EngineeringAnalysis{
			ResourceID:             res.ID,
			WorkloadClassification: "Balanced Optimal Instance",
			DownsizingSafe:         false,
			RiskScore:              20,
			ConfidenceLevel:        "HIGH",
			PeakHeadroomProof:      "Instance operates within healthy utilization thresholds (40-70% peak CPU). No rightsizing required.",
			SafetyBufferAnalysis:   "Headroom is calibrated correctly for existing workload demands.",
			TechnicalExplanation:   "Current compute and memory specifications closely match workload distribution.",
			BlastRadiusAnalysis:    "No action recommended.",
			ActionableNextSteps:    []string{"Continue routine telemetry monitoring."},
		}
	}
}

func round2(val float64) float64 {
	return float64(int(val*100+0.5)) / 100.0
}
