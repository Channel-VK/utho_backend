package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"utho_backend/internal/api"
	"utho_backend/internal/models"
)

func setupTestServer() http.Handler {
	store := api.NewStore()
	mux := http.NewServeMux()
	store.RegisterRoutes(mux)
	return mux
}

func TestGetDashboard(t *testing.T) {
	handler := setupTestServer()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/dashboard", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var data models.DashboardData
	if err := json.NewDecoder(rec.Body).Decode(&data); err != nil {
		t.Fatalf("failed to decode dashboard json: %v", err)
	}

	if data.TotalResourcesCount != 85 {
		t.Fatalf("expected 85 total resources, got %d", data.TotalResourcesCount)
	}
	if data.ProtectedPeakVMsCount != 10 {
		t.Fatalf("expected 10 protected peak VMs, got %d", data.ProtectedPeakVMsCount)
	}
	if data.ActiveAlertsCount != 3 {
		t.Fatalf("expected 3 active alerts, got %d", data.ActiveAlertsCount)
	}
	if data.BillGrowth6mPct != 40.0 {
		t.Fatalf("expected 40%% bill growth, got %f", data.BillGrowth6mPct)
	}
}

func TestGetResources(t *testing.T) {
	handler := setupTestServer()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/resources?env=prod", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var list []models.Resource
	if err := json.NewDecoder(rec.Body).Decode(&list); err != nil {
		t.Fatalf("failed to decode resources json: %v", err)
	}

	if len(list) == 0 {
		t.Fatal("expected non-empty list of prod resources")
	}

	for _, res := range list {
		if res.Env != "prod" {
			t.Fatalf("expected all returned resources to have env 'prod', got %s", res.Env)
		}
	}
}

func TestGetResourceDetailAndAnalysis(t *testing.T) {
	handler := setupTestServer()

	// prod-payment-webhook-01 is res-vm-021 in seed order or res-vm-001 depending on id
	// Let's query by id from resources
	reqList := httptest.NewRequest(http.MethodGet, "/api/v1/resources?search=prod-payment-webhook-01", nil)
	recList := httptest.NewRecorder()
	handler.ServeHTTP(recList, reqList)

	var list []models.Resource
	_ = json.NewDecoder(recList.Body).Decode(&list)
	if len(list) == 0 {
		t.Fatal("prod-payment-webhook-01 not found")
	}

	targetID := list[0].ID

	req := httptest.NewRequest(http.MethodGet, "/api/v1/resources/"+targetID, nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	var detail models.ResourceDetailResponse
	if err := json.NewDecoder(rec.Body).Decode(&detail); err != nil {
		t.Fatalf("failed to decode resource detail json: %v", err)
	}

	if len(detail.Timeseries30d) != 180 {
		t.Fatalf("expected 180 timeseries points, got %d", len(detail.Timeseries30d))
	}
	if detail.EngineeringAnalysis == nil {
		t.Fatal("expected engineering analysis to be populated")
	}
	if detail.EngineeringAnalysis.DownsizingSafe {
		t.Fatal("expected DownsizingSafe to be false for bursty webhook instance")
	}
}

func TestPatchAndBulkRecommendations(t *testing.T) {
	handler := setupTestServer()

	// 1. Get initial recommendations - verify 0 accepted on load
	req := httptest.NewRequest(http.MethodGet, "/api/v1/recommendations", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var initial models.RecommendationsResponse
	_ = json.NewDecoder(rec.Body).Decode(&initial)
	if len(initial.Items) == 0 {
		t.Fatal("expected recommendations items")
	}
	if initial.Summary.AcceptedCount != 0 {
		t.Fatalf("expected 0 initial accepted recommendations, got %d", initial.Summary.AcceptedCount)
	}
	if initial.Summary.AcceptedSavingsINR != 0 {
		t.Fatalf("expected 0 initial accepted savings INR, got %f", initial.Summary.AcceptedSavingsINR)
	}
	if initial.Summary.PendingCount != len(initial.Items) {
		t.Fatalf("expected all %d recommendations to be pending, got %d", len(initial.Items), initial.Summary.PendingCount)
	}

	firstID := initial.Items[0].ID

	// 2. Patch single item to accepted
	patchBody := []byte(`{"status":"accepted"}`)
	patchReq := httptest.NewRequest(http.MethodPatch, "/api/v1/recommendations/"+firstID, bytes.NewBuffer(patchBody))
	patchReq.Header.Set("Content-Type", "application/json")
	patchRec := httptest.NewRecorder()
	handler.ServeHTTP(patchRec, patchReq)

	if patchRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on PATCH, got %d", patchRec.Code)
	}

	var patchedResp models.RecommendationsResponse
	_ = json.NewDecoder(patchRec.Body).Decode(&patchedResp)

	if patchedResp.Summary.AcceptedCount != 1 {
		t.Fatalf("expected 1 accepted recommendation after patch, got %d", patchedResp.Summary.AcceptedCount)
	}

	found := false
	for _, it := range patchedResp.Items {
		if it.ID == firstID {
			found = true
			if it.Status != "accepted" {
				t.Fatalf("expected status 'accepted', got %s", it.Status)
			}
		}
	}
	if !found {
		t.Fatal("patched item not found")
	}

	// 3. Bulk accept low risk
	bulkBody := []byte(`{"status":"accepted","filter_risk":"LOW_ONLY"}`)
	bulkReq := httptest.NewRequest(http.MethodPost, "/api/v1/recommendations/bulk", bytes.NewBuffer(bulkBody))
	bulkReq.Header.Set("Content-Type", "application/json")
	bulkRec := httptest.NewRecorder()
	handler.ServeHTTP(bulkRec, bulkReq)

	if bulkRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on bulk, got %d", bulkRec.Code)
	}

	var bulkResp models.RecommendationsResponse
	_ = json.NewDecoder(bulkRec.Body).Decode(&bulkResp)
	if bulkResp.Summary.AcceptedCount == 0 {
		t.Fatal("expected accepted items after bulk accept")
	}

	// 4. Bulk revert to pending
	revertBody := []byte(`{"status":"pending","filter_risk":"ALL"}`)
	revertReq := httptest.NewRequest(http.MethodPost, "/api/v1/recommendations/bulk", bytes.NewBuffer(revertBody))
	revertReq.Header.Set("Content-Type", "application/json")
	revertRec := httptest.NewRecorder()
	handler.ServeHTTP(revertRec, revertReq)

	if revertRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on bulk revert to pending, got %d", revertRec.Code)
	}

	var revertResp models.RecommendationsResponse
	_ = json.NewDecoder(revertRec.Body).Decode(&revertResp)
	if revertResp.Summary.AcceptedCount != 0 {
		t.Fatalf("expected 0 accepted items after bulk revert to pending, got %d", revertResp.Summary.AcceptedCount)
	}
	if revertResp.Summary.PendingCount != len(revertResp.Items) {
		t.Fatalf("expected all %d items to be pending, got %d", len(revertResp.Items), revertResp.Summary.PendingCount)
	}
}

func TestSimulateEndpoint(t *testing.T) {
	handler := setupTestServer()

	simBody := []byte(`{
		"dev_off_hours_enabled": true,
		"dev_shutdown_hours_per_week": 118,
		"staging_weekend_off_enabled": true,
		"safety_headroom_buffer_pct": 25,
		"purge_orphaned_older_than_days": 14
	}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/simulate", bytes.NewBuffer(simBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on simulate, got %d", rec.Code)
	}

	var res models.SimulationResult
	if err := json.NewDecoder(rec.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode simulate response: %v", err)
	}

	if res.SimulatedSavingsINR <= 0 {
		t.Fatalf("expected simulated savings > 0, got %f", res.SimulatedSavingsINR)
	}
}

func TestAlertsAndAssumptionsAndExport(t *testing.T) {
	handler := setupTestServer()

	// Alerts
	reqAlerts := httptest.NewRequest(http.MethodGet, "/api/v1/alerts", nil)
	recAlerts := httptest.NewRecorder()
	handler.ServeHTTP(recAlerts, reqAlerts)
	if recAlerts.Code != http.StatusOK {
		t.Fatalf("alerts returned %d", recAlerts.Code)
	}

	var alerts []models.Alert
	_ = json.NewDecoder(recAlerts.Body).Decode(&alerts)
	if len(alerts) != 3 {
		t.Fatalf("expected 3 alerts, got %d", len(alerts))
	}

	// Assumptions
	reqAssumptions := httptest.NewRequest(http.MethodGet, "/api/v1/assumptions", nil)
	recAssumptions := httptest.NewRecorder()
	handler.ServeHTTP(recAssumptions, reqAssumptions)
	if recAssumptions.Code != http.StatusOK {
		t.Fatalf("assumptions returned %d", recAssumptions.Code)
	}

	// Export
	reqExport := httptest.NewRequest(http.MethodGet, "/api/v1/export/remediation", nil)
	recExport := httptest.NewRecorder()
	handler.ServeHTTP(recExport, reqExport)
	if recExport.Code != http.StatusOK {
		t.Fatalf("export returned %d", recExport.Code)
	}
}

func TestStatusAndAIGatewayRoutes(t *testing.T) {
	handler := setupTestServer()

	// 1. Status endpoint
	reqStatus := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	recStatus := httptest.NewRecorder()
	handler.ServeHTTP(recStatus, reqStatus)
	if recStatus.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/v1/status, got %d", recStatus.Code)
	}
	var st models.ServiceStatusResponse
	if err := json.NewDecoder(recStatus.Body).Decode(&st); err != nil {
		t.Fatalf("failed to decode status: %v", err)
	}
	if !st.GoEngineConnected || st.GoEnginePort != 8080 {
		t.Fatalf("unexpected go engine status: %+v", st)
	}

	// 2. AI Parse Scenario
	nlBody := []byte(`{"query":"What if we shut down all dev VMs in Bangalore from 8 PM to 8 AM and keep 30% peak headroom?"}`)
	reqNL := httptest.NewRequest(http.MethodPost, "/api/v1/ai/parse-scenario", bytes.NewBuffer(nlBody))
	reqNL.Header.Set("Content-Type", "application/json")
	recNL := httptest.NewRecorder()
	handler.ServeHTTP(recNL, reqNL)
	if recNL.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/v1/ai/parse-scenario, got %d", recNL.Code)
	}
	var nlResp models.NLScenarioResponse
	if err := json.NewDecoder(recNL.Body).Decode(&nlResp); err != nil {
		t.Fatalf("failed to decode nl scenario: %v", err)
	}
	if nlResp.ParsedParams.SafetyHeadroomBufferPct != 30 {
		t.Fatalf("expected headroom 30, got %f", nlResp.ParsedParams.SafetyHeadroomBufferPct)
	}
	if nlResp.SimulationResult.SimulatedMonthlySavingsINR <= 0 {
		t.Fatalf("expected positive simulated savings inr, got %f", nlResp.SimulationResult.SimulatedMonthlySavingsINR)
	}

	// 3. AI Forecast
	reqFc := httptest.NewRequest(http.MethodGet, "/api/v1/ai/forecast?days=60", nil)
	recFc := httptest.NewRecorder()
	handler.ServeHTTP(recFc, reqFc)
	if recFc.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/v1/ai/forecast, got %d", recFc.Code)
	}
	var fcResp models.AIForecastResponse
	if err := json.NewDecoder(recFc.Body).Decode(&fcResp); err != nil {
		t.Fatalf("failed to decode ai forecast: %v", err)
	}
	if fcResp.Days != 60 || fcResp.Currency != "INR" {
		t.Fatalf("unexpected forecast response: %+v", fcResp)
	}

	// 4. AI Briefing
	brBody := []byte(`{"role":"finance"}`)
	reqBr := httptest.NewRequest(http.MethodPost, "/api/v1/ai/briefing", bytes.NewBuffer(brBody))
	reqBr.Header.Set("Content-Type", "application/json")
	recBr := httptest.NewRecorder()
	handler.ServeHTTP(recBr, reqBr)
	if recBr.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/v1/ai/briefing, got %d", recBr.Code)
	}
	var brResp models.AIBriefingResponse
	if err := json.NewDecoder(recBr.Body).Decode(&brResp); err != nil {
		t.Fatalf("failed to decode ai briefing: %v", err)
	}
	if brResp.Role != "finance" || len(brResp.BulletPoints) == 0 {
		t.Fatalf("unexpected briefing response: %+v", brResp)
	}

	// 5. Health endpoint
	reqH := httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	recH := httptest.NewRecorder()
	handler.ServeHTTP(recH, reqH)
	if recH.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/v1/health, got %d", recH.Code)
	}

	// 6. LLM Health endpoint
	reqLLMH := httptest.NewRequest(http.MethodGet, "/api/v1/ai/llm-health", nil)
	recLLMH := httptest.NewRecorder()
	handler.ServeHTTP(recLLMH, reqLLMH)
	if recLLMH.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/v1/ai/llm-health, got %d", recLLMH.Code)
	}

	// 7. Chat Greeting endpoint
	chatBody := []byte(`{"message":"hi"}`)
	reqChat := httptest.NewRequest(http.MethodPost, "/api/v1/chat", bytes.NewBuffer(chatBody))
	reqChat.Header.Set("Content-Type", "application/json")
	recChat := httptest.NewRecorder()
	handler.ServeHTTP(recChat, reqChat)
	if recChat.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/v1/chat, got %d", recChat.Code)
	}
	var chatResp models.ChatResponse
	if err := json.NewDecoder(recChat.Body).Decode(&chatResp); err != nil {
		t.Fatalf("failed to decode chat response: %v", err)
	}
	if !strings.Contains(chatResp.Answer, "FinSort Copilot") || strings.Contains(chatResp.Answer, "tracking ₹") {
		t.Fatalf("expected natural greeting, got: %s", chatResp.Answer)
	}
}
