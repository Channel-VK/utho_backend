package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"utho_backend/internal/ai"
	"utho_backend/internal/catalog"
	"utho_backend/internal/engine"
	"utho_backend/internal/models"
	"utho_backend/internal/seed"
)

// Store provides thread-safe in-memory access to FinOps dataset and state.
type Store struct {
	mu              sync.RWMutex
	resources       []models.Resource
	resourceMap     map[string]models.Resource
	recommendations []models.Recommendation
	recommendMap    map[string]*models.Recommendation
	aiClient        *ai.Client
}

// NewStore initializes and seeds the in-memory store deterministically.
func NewStore() *Store {
	resources := seed.GenerateDataset()
	resourceMap := make(map[string]models.Resource, len(resources))
	for _, res := range resources {
		resourceMap[res.ID] = res
	}

	recs := engine.GenerateRecommendations(resources, 25.0)
	recommendMap := make(map[string]*models.Recommendation, len(recs))
	for i := range recs {
		recommendMap[recs[i].ID] = &recs[i]
	}

	return &Store{
		resources:       resources,
		resourceMap:     resourceMap,
		recommendations: recs,
		recommendMap:    recommendMap,
		aiClient:        ai.NewClient(),
	}
}

// RegisterRoutes registers all REST API endpoints on the provided ServeMux.
func (s *Store) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/status", s.HandleGetStatus)
	mux.HandleFunc("GET /api/v1/dashboard", s.HandleGetDashboard)
	mux.HandleFunc("GET /api/v1/resources", s.HandleGetResources)
	mux.HandleFunc("GET /api/v1/resources/{id}", s.HandleGetResourceByID)
	mux.HandleFunc("GET /api/v1/recommendations", s.HandleGetRecommendations)
	mux.HandleFunc("PATCH /api/v1/recommendations/{id}", s.HandlePatchRecommendation)
	mux.HandleFunc("POST /api/v1/recommendations/bulk", s.HandleBulkRecommendations)
	mux.HandleFunc("POST /api/v1/simulate", s.HandleSimulate)
	mux.HandleFunc("GET /api/v1/alerts", s.HandleGetAlerts)
	mux.HandleFunc("GET /api/v1/assumptions", s.HandleGetAssumptions)
	mux.HandleFunc("GET /api/v1/export/remediation", s.HandleExportRemediation)
	mux.HandleFunc("GET /api/v1/export", s.HandleExportRemediation)
	mux.HandleFunc("POST /api/v1/export/remediation", s.HandleExportRemediation)
	mux.HandleFunc("POST /api/v1/export", s.HandleExportRemediation)

	// Health and Telemetry Endpoints
	mux.HandleFunc("GET /api/v1/health", s.HandleHealth)
	mux.HandleFunc("GET /health", s.HandleHealth)
	mux.HandleFunc("GET /api/v1/ai/llm-health", s.HandleAILLMHealth)
	mux.HandleFunc("POST /api/v1/ai/llm-health", s.HandleAILLMHealth)
	mux.HandleFunc("GET /api/v1/health/llm", s.HandleAILLMHealth)
	mux.HandleFunc("GET /health/llm", s.HandleAILLMHealth)
	mux.HandleFunc("GET /ai/llm-health", s.HandleAILLMHealth)

	// Unified Python AI (:8001) Proxy & Orchestration Routes
	mux.HandleFunc("POST /api/v1/ai/parse-scenario", s.HandleAIParseScenario)
	mux.HandleFunc("GET /api/v1/ai/forecast", s.HandleAIForecast)
	mux.HandleFunc("POST /api/v1/ai/forecast", s.HandleAIForecast)
	mux.HandleFunc("POST /api/v1/ai/briefing", s.HandleAIBriefing)
	mux.HandleFunc("GET /api/v1/ai/alerts-analysis", s.HandleGetAlerts)
	mux.HandleFunc("POST /api/v1/ai/chat", s.HandleAIChat)
	mux.HandleFunc("GET /api/v1/ai/chat/{id}", s.HandleAIGetChatHistory)
	mux.HandleFunc("POST /api/v1/chat", s.HandleAIChat)
	mux.HandleFunc("GET /api/v1/chat/{id}", s.HandleAIGetChatHistory)
}

// HandleGetStatus reports connection telemetry for both Go Engine (:8080) and Python AI (:8001).
func (s *Store) HandleGetStatus(w http.ResponseWriter, r *http.Request) {
	aiConnected, features, llmConfigured, googleModel := s.aiClient.GetStatusInfo()
	status := models.ServiceStatusResponse{
		GoEngineConnected: true,
		GoEnginePort:      8080,
		AIEngineConnected: aiConnected,
		AIEnginePort:      8001,
		AIEngineService:   "finsort-ai-engine",
		AIFeatures:        features,
		LLMConfigured:     llmConfigured,
		LLMModel:          googleModel,
	}
	writeJSON(w, http.StatusOK, status)
}

// HandleHealth reports overall platform health including Go and Python AI engines.
func (s *Store) HandleHealth(w http.ResponseWriter, r *http.Request) {
	aiConnected, features, llmConfigured, googleModel := s.aiClient.GetStatusInfo()
	resp := map[string]any{
		"status":              "ok",
		"service":             "finsort-backend",
		"go_engine_connected": true,
		"ai_engine_connected": aiConnected,
		"llm_configured":      llmConfigured,
		"google_model":        googleModel,
		"features":            features,
	}
	writeJSON(w, http.StatusOK, resp)
}

// HandleAILLMHealth reports live Gemini LLM connectivity, model latency, and active API key status.
func (s *Store) HandleAILLMHealth(w http.ResponseWriter, r *http.Request) {
	resp := s.aiClient.CheckLLMHealth()
	writeJSON(w, http.StatusOK, resp)
}

func (s *Store) computeDashboardSnapshot(checkAI bool) models.DashboardData {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var currentSpend float64
	protectedPeakCount := 0
	totalResources := len(s.resources)

	envSpend := make(map[string]*models.SpendBreakdown)
	regionSpend := make(map[string]*models.SpendBreakdown)

	for _, res := range s.resources {
		currentSpend += res.MonthlyCostINR
		if res.OptimizationStatus == "PROTECTED_PEAK_LOAD" {
			protectedPeakCount++
		}

		// Group by env
		if _, ok := envSpend[res.Env]; !ok {
			envSpend[res.Env] = &models.SpendBreakdown{Env: res.Env}
		}
		envSpend[res.Env].CurrentINR += res.MonthlyCostINR

		// Group by region
		if _, ok := regionSpend[res.Region]; !ok {
			regionSpend[res.Region] = &models.SpendBreakdown{Region: res.Region}
		}
		regionSpend[res.Region].CurrentINR += res.MonthlyCostINR
	}

	// Calculate accepted savings & total potential savings from recommendations
	var acceptedSavings, totalPotentialSavings float64
	for _, rec := range s.recommendations {
		totalPotentialSavings += rec.MonthlySavingsINR
		if rec.Status == "accepted" {
			acceptedSavings += rec.MonthlySavingsINR
			if b, ok := envSpend[rec.Env]; ok {
				b.SavingsINR += rec.MonthlySavingsINR
			}
			if b, ok := regionSpend[rec.Region]; ok {
				b.SavingsINR += rec.MonthlySavingsINR
			}
		}
	}

	projectedSpend := currentSpend - acceptedSavings
	acceptedPct := 0.0
	if currentSpend > 0 {
		acceptedPct = round2((acceptedSavings / currentSpend) * 100.0)
	}

	var spendByEnv []models.SpendBreakdown
	for _, b := range envSpend {
		b.CurrentINR = round2(b.CurrentINR)
		b.SavingsINR = round2(b.SavingsINR)
		b.ProjectedINR = round2(b.CurrentINR - b.SavingsINR)
		spendByEnv = append(spendByEnv, *b)
	}

	var spendByRegion []models.SpendBreakdown
	for _, b := range regionSpend {
		b.CurrentINR = round2(b.CurrentINR)
		b.SavingsINR = round2(b.SavingsINR)
		b.ProjectedINR = round2(b.CurrentINR - b.SavingsINR)
		spendByRegion = append(spendByRegion, *b)
	}

	// Sustainability metrics
	kwhSaved := round2((acceptedSavings / 950.0) * 32.85) // Normalized per small VM
	co2Saved := round2(kwhSaved * catalog.GridCarbonFactorKgKWh)
	trees := round2(co2Saved / 21.77)

	// Historical 6 months (₹4,85,000 -> ₹6,79,000)
	historical := []models.HistoricalSpend{
		{Month: "May 2026", ComputeINR: 345000, StorageINR: 82000, NetworkINR: 28000, WasteINR: 30000, TotalINR: 485000},
		{Month: "Jun 2026", ComputeINR: 368000, StorageINR: 88000, NetworkINR: 31000, WasteINR: 31000, TotalINR: 518000},
		{Month: "Jul 2026", ComputeINR: 394000, StorageINR: 94000, NetworkINR: 32000, WasteINR: 34000, TotalINR: 554000},
		{Month: "Aug 2026", ComputeINR: 423000, StorageINR: 101000, NetworkINR: 35000, WasteINR: 36000, TotalINR: 595000},
		{Month: "Sep 2026", ComputeINR: 452000, StorageINR: 108000, NetworkINR: 37000, WasteINR: 39000, TotalINR: 636000},
		{Month: "Oct 2026", ComputeINR: 483000, StorageINR: 115000, NetworkINR: 40000, WasteINR: 41000, TotalINR: 679000},
	}

	// Forecast 60 days
	var forecast []models.ForecastPoint
	baseDate := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)
	for d := 0; d <= 60; d += 10 {
		dateStr := baseDate.AddDate(0, 0, d).Format("2006-01-02")
		doNothing := currentSpend * (1.0 + (float64(d) * 0.0015))
		opt := projectedSpend * (1.0 + (float64(d) * 0.0003))
		forecast = append(forecast, models.ForecastPoint{
			Date:         dateStr,
			DoNothingINR: round2(doNothing),
			OptimizedINR: round2(opt),
		})
	}

	alerts := engine.GetActiveAlerts()

	aiConnected := false
	var aiFeatures []string
	aiStatus := "Offline (Deterministic Fallback)"
	if checkAI {
		aiConnected, aiFeatures = s.aiClient.CheckHealth()
		if aiConnected {
			aiStatus = "Connected (:8001)"
		}
	}

	return models.DashboardData{
		CurrentMonthlySpendINR:    round2(currentSpend),
		ProjectedMonthlySpendINR:  round2(projectedSpend),
		AcceptedMonthlySavingsINR: round2(acceptedSavings),
		AcceptedSavingsPct:        acceptedPct,
		TotalPotentialSavingsINR:  round2(totalPotentialSavings),
		BillGrowth6mPct:           40.0,
		ProtectedPeakVMsCount:     protectedPeakCount,
		ActiveAlertsCount:         len(alerts),
		TotalResourcesCount:       totalResources,
		AIEngineConnected:         aiConnected,
		AIEngineStatus:            aiStatus,
		AIFeatures:                aiFeatures,
		Sustainability: models.Sustainability{
			MonthlyKWhSaved:   kwhSaved,
			MonthlyCO2KgSaved: co2Saved,
			TreesEquivalent:   trees,
		},
		HistoricalSpend6m: historical,
		Forecast60d:       forecast,
		SpendByEnv:        spendByEnv,
		SpendByRegion:     spendByRegion,
	}
}

// HandleGetDashboard calculates and returns high-level FinOps dashboard metrics.
func (s *Store) HandleGetDashboard(w http.ResponseWriter, r *http.Request) {
	dashboard := s.computeDashboardSnapshot(true)
	writeJSON(w, http.StatusOK, dashboard)
}

// HandleGetResources returns all resources, supporting filter query parameters.
func (s *Store) HandleGetResources(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := r.URL.Query()
	qRegion := query.Get("region")
	qEnv := query.Get("env")
	qTeam := query.Get("team")
	qType := query.Get("type")
	qStatus := query.Get("status")
	if qStatus == "" {
		qStatus = query.Get("optimization_status")
	}
	qSearch := strings.ToLower(query.Get("search"))

	var filtered []models.Resource
	for _, res := range s.resources {
		if qRegion != "" && !strings.EqualFold(res.Region, qRegion) {
			continue
		}
		if qEnv != "" && !strings.EqualFold(res.Env, qEnv) {
			continue
		}
		if qTeam != "" && !strings.EqualFold(res.Team, qTeam) {
			continue
		}
		if qType != "" && !strings.EqualFold(res.Type, qType) {
			continue
		}
		if qStatus != "" && !strings.EqualFold(res.OptimizationStatus, qStatus) {
			continue
		}
		if qSearch != "" {
			match := strings.Contains(strings.ToLower(res.Name), qSearch) ||
				strings.Contains(strings.ToLower(res.ID), qSearch) ||
				strings.Contains(strings.ToLower(res.Plan), qSearch)
			if !match {
				continue
			}
		}

		// For listing, omit the 180 timeseries points for fast transport
		lightRes := res
		lightRes.Timeseries30d = nil
		filtered = append(filtered, lightRes)
	}

	writeJSON(w, http.StatusOK, filtered)
}

// HandleGetResourceByID returns full resource details, 30d timeseries, recommendation, and AI/engineering analysis.
func (s *Store) HandleGetResourceByID(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, `{"error":"resource id required"}`, http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	res, exists := s.resourceMap[id]
	if !exists {
		s.mu.RUnlock()
		http.Error(w, `{"error":"resource not found"}`, http.StatusNotFound)
		return
	}

	// Find associated recommendation
	var matchedRec *models.Recommendation
	for i := range s.recommendations {
		if s.recommendations[i].ResourceID == id {
			matchedRec = &s.recommendations[i]
			break
		}
	}
	s.mu.RUnlock()

	// Perform AI/Engineering analysis (calls Python service or deterministic fallback)
	analysis := s.aiClient.ExplainResource(res)

	resp := models.ResourceDetailResponse{
		Resource:            res,
		Timeseries30d:       res.Timeseries30d,
		Recommendation:      matchedRec,
		EngineeringAnalysis: analysis,
	}

	writeJSON(w, http.StatusOK, resp)
}

// HandleGetRecommendations returns the summary and array of recommendations.
func (s *Store) HandleGetRecommendations(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	summary := engine.ComputeSummary(s.recommendations)
	resp := models.RecommendationsResponse{
		Summary: summary,
		Items:   s.recommendations,
	}

	writeJSON(w, http.StatusOK, resp)
}

// HandlePatchRecommendation updates status of a single recommendation.
func (s *Store) HandlePatchRecommendation(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, `{"error":"recommendation id required"}`, http.StatusBadRequest)
		return
	}

	var req models.UpdateRecommendationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json body"}`, http.StatusBadRequest)
		return
	}

	status := strings.ToLower(req.Status)
	if status != "accepted" && status != "rejected" && status != "pending" {
		http.Error(w, `{"error":"status must be 'accepted', 'rejected', or 'pending'"}`, http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	rec, exists := s.recommendMap[id]
	if !exists {
		s.mu.Unlock()
		http.Error(w, `{"error":"recommendation not found"}`, http.StatusNotFound)
		return
	}
	rec.Status = status

	// Sync slice
	for i := range s.recommendations {
		if s.recommendations[i].ID == id {
			s.recommendations[i].Status = status
			break
		}
	}
	summary := engine.ComputeSummary(s.recommendations)
	resp := models.RecommendationsResponse{
		Summary: summary,
		Items:   s.recommendations,
	}
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, resp)
}

// HandleBulkRecommendations applies bulk status update to all or low-risk recommendations.
func (s *Store) HandleBulkRecommendations(w http.ResponseWriter, r *http.Request) {
	var req models.BulkUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json body"}`, http.StatusBadRequest)
		return
	}

	status := strings.ToLower(req.Status)
	if status != "accepted" && status != "rejected" && status != "pending" {
		http.Error(w, `{"error":"status must be 'accepted', 'rejected', or 'pending'"}`, http.StatusBadRequest)
		return
	}

	filterRisk := strings.ToUpper(req.FilterRisk)
	if filterRisk == "" {
		filterRisk = "ALL"
	}

	s.mu.Lock()
	for i := range s.recommendations {
		if filterRisk == "LOW_ONLY" && s.recommendations[i].RiskLevel != "LOW" {
			continue
		}
		s.recommendations[i].Status = status
		if r, ok := s.recommendMap[s.recommendations[i].ID]; ok {
			r.Status = status
		}
	}
	summary := engine.ComputeSummary(s.recommendations)
	resp := models.RecommendationsResponse{
		Summary: summary,
		Items:   s.recommendations,
	}
	s.mu.Unlock()

	writeJSON(w, http.StatusOK, resp)
}

// HandleSimulate evaluates what-if scenarios based on user parameters.
func (s *Store) HandleSimulate(w http.ResponseWriter, r *http.Request) {
	var req models.SimulationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid simulation request json"}`, http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	result := engine.Simulate(s.resources, req)
	s.mu.RUnlock()

	writeJSON(w, http.StatusOK, result)
}

// HandleGetAlerts returns active anomaly alerts enriched with Python AI root-cause & TTF correlation.
func (s *Store) HandleGetAlerts(w http.ResponseWriter, r *http.Request) {
	baseAlerts := engine.GetActiveAlerts()
	s.mu.RLock()
	resCopy := s.resources
	s.mu.RUnlock()

	enriched := s.aiClient.CorrelateAlerts(baseAlerts, resCopy)
	writeJSON(w, http.StatusOK, enriched)
}

// HandleAIParseScenario parses natural-language "What-If" queries via Python :8001 and runs live simulation.
func (s *Store) HandleAIParseScenario(w http.ResponseWriter, r *http.Request) {
	var req models.NLScenarioRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid natural language query json"}`, http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	resCopy := s.resources
	s.mu.RUnlock()

	resp := s.aiClient.ParseNLScenario(req.Query, resCopy)
	writeJSON(w, http.StatusOK, resp)
}

// HandleAIForecast returns 30/60/90-day component cost forecasts & GreenOps PUE impact via Python :8001.
func (s *Store) HandleAIForecast(w http.ResponseWriter, r *http.Request) {
	days := 30
	headroom := 1.25

	if r.Method == http.MethodPost && r.Body != nil {
		var req models.AIForecastRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			if req.Days > 0 {
				days = req.Days
			}
			if req.Headroom >= 1.0 {
				headroom = req.Headroom
			}
		}
	} else {
		if qDays := r.URL.Query().Get("days"); qDays != "" {
			if v, err := strconv.Atoi(qDays); err == nil && v > 0 {
				days = v
			}
		}
		if qHead := r.URL.Query().Get("headroom"); qHead != "" {
			if v, err := strconv.ParseFloat(qHead, 64); err == nil && v >= 1.0 {
				headroom = v
			}
		}
	}

	s.mu.RLock()
	var lightResources []models.Resource
	for _, res := range s.resources {
		lr := res
		lr.Timeseries30d = nil
		lightResources = append(lightResources, lr)
	}
	recsCopy := make([]models.Recommendation, len(s.recommendations))
	copy(recsCopy, s.recommendations)
	s.mu.RUnlock()

	resp := s.aiClient.GetAIForecast(days, headroom, lightResources, recsCopy)
	writeJSON(w, http.StatusOK, resp)
}

// HandleAIBriefing returns role-based executive/finance/engineering briefings via Python :8001.
func (s *Store) HandleAIBriefing(w http.ResponseWriter, r *http.Request) {
	var req models.AIBriefingRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	if req.Role == "" {
		req.Role = r.URL.Query().Get("role")
	}
	if req.Role == "" {
		req.Role = "executive"
	}

	dash := s.computeDashboardSnapshot(false)
	resp := s.aiClient.GetRoleBriefing(req.Role, dash)
	writeJSON(w, http.StatusOK, resp)
}

// HandleGetAssumptions returns the complete pricing catalog, safety formulas, and constants.
func (s *Store) HandleGetAssumptions(w http.ResponseWriter, r *http.Request) {
	assumptions := models.AssumptionsData{
		PricingCatalog: catalog.Plans,
		SafetyFormulas: map[string]string{
			"required_capacity":       "Required Capacity = p95_peak * (1 + headroom_buffer_pct / 100)",
			"peak_safety_guard":       "If observed p99 >= 85.0% or bursty peak pattern, instance is classified as PROTECTED_PEAK_LOAD and downsizing is blocked.",
			"projected_peak_headroom": "Projected CPU p95 = (Current vCPU * Observed p95) / (Candidate vCPU) <= 80%",
			"projected_p99_ceiling":   "Projected CPU p99 = (Current vCPU * Observed p99) / (Candidate vCPU) < 85%",
			"ram_safety_ceiling":      "Projected RAM p95 = (Current RAM * Observed p95) / (Candidate RAM) < 75%",
			"zombie_qualification":    "Instance created >= 90 days ago with continuous 30-day p99 CPU < 3.5% and net egress < 0.5 Mbps.",
			"orphaned_qualification":  "Block volume unattached >= 30 days with 0 IOPS, or Public IPv4 unassociated with any active VM interface.",
		},
		OffHoursScheduleMath: map[string]string{
			"total_weekly_hours":    "168 hours total per week",
			"active_business_hours": "Mon-Fri 09:00-19:00 IST = 50 hours/week active duty cycle",
			"idle_off_hours":        "118 hours/week idle (nights 19:00-09:00 IST + entire weekends)",
			"idle_percentage":       "118 / 168 = 70.2% idle duty cycle",
			"conservative_savings":  "~65% compute savings per non-production VM accounting for 5% boot/shutdown buffer",
			"batch_schedule_math":   "Nightly batch runs 01:00-04:00 IST (3-4 hrs active). 20 hours idle / 24 hours = 83.3% savings.",
		},
		GridCarbonFactor: catalog.GridCarbonFactorKgKWh,
	}

	writeJSON(w, http.StatusOK, assumptions)
}

// HandleExportRemediation exports ready-to-run Utho CLI scripts, Terraform HCL, and CSV.
func (s *Store) HandleExportRemediation(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	export := engine.GenerateRemediation(s.recommendations)
	s.mu.RUnlock()

	writeJSON(w, http.StatusOK, export)
}

// HandleAIChat proxies the user chat query with full fleet, recommendation, forecast & alert context to Python AI (:8001 /chat).
func (s *Store) HandleAIChat(w http.ResponseWriter, r *http.Request) {
	var req models.ChatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, `{"error":"invalid json body"}`, http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Message) == "" {
		http.Error(w, `{"error":"message cannot be empty"}`, http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	lightResources := make([]models.Resource, len(s.resources))
	for i, res := range s.resources {
		rc := res
		rc.Timeseries30d = nil
		lightResources[i] = rc
	}
	recsCopy := make([]models.Recommendation, len(s.recommendations))
	copy(recsCopy, s.recommendations)
	s.mu.RUnlock()

	alertsCopy := engine.GetActiveAlerts()
	resp := s.aiClient.ChatWithContext(req, lightResources, recsCopy, alertsCopy)
	writeJSON(w, http.StatusOK, resp)
}

// HandleAIGetChatHistory fetches conversation turns for a given session ID.
func (s *Store) HandleAIGetChatHistory(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, `{"error":"conversation id required"}`, http.StatusBadRequest)
		return
	}

	history, err := s.aiClient.GetChatHistory(id)
	if err != nil {
		// Return empty history shell rather than failing
		history = models.ChatHistoryResponse{
			ConversationID: id,
			CreatedAt:      time.Now().UTC().Format(time.RFC3339),
			UpdatedAt:      time.Now().UTC().Format(time.RFC3339),
			Messages:       []models.ChatMessage{},
		}
	}
	writeJSON(w, http.StatusOK, history)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func round2(val float64) float64 {
	return float64(int(val*100+0.5)) / 100.0
}
