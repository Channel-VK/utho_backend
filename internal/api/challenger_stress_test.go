package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"utho_backend/internal/api"
	"utho_backend/internal/models"
)

// TestChallenger_RecommendationAPIAdversarialInputs tests boundary cases on PATCH and bulk recommendation handlers.
func TestChallenger_RecommendationAPIAdversarialInputs(t *testing.T) {
	store := api.NewStore()
	mux := http.NewServeMux()
	store.RegisterRoutes(mux)

	// 1. Non-existent recommendation ID on PATCH
	recReqNotFound := httptest.NewRequest(http.MethodPatch, "/api/v1/recommendations/rec-nonexistent-9999", bytes.NewBufferString(`{"status":"accepted"}`))
	recReqNotFound.Header.Set("Content-Type", "application/json")
	recNotFound := httptest.NewRecorder()
	mux.ServeHTTP(recNotFound, recReqNotFound)
	if recNotFound.Code != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found for missing rec ID, got %d", recNotFound.Code)
	}

	// 2. Invalid status on PATCH
	recReqInvalidStatus := httptest.NewRequest(http.MethodPatch, "/api/v1/recommendations/rec-001", bytes.NewBufferString(`{"status":"approved_by_boss"}`))
	recReqInvalidStatus.Header.Set("Content-Type", "application/json")
	recInvalidStatus := httptest.NewRecorder()
	mux.ServeHTTP(recInvalidStatus, recReqInvalidStatus)
	if recInvalidStatus.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid status, got %d", recInvalidStatus.Code)
	}

	// 3. Invalid status on bulk update
	bulkReqInvalidStatus := httptest.NewRequest(http.MethodPost, "/api/v1/recommendations/bulk", bytes.NewBufferString(`{"status":"unknown_status"}`))
	bulkReqInvalidStatus.Header.Set("Content-Type", "application/json")
	bulkRecInvalid := httptest.NewRecorder()
	mux.ServeHTTP(bulkRecInvalid, bulkReqInvalidStatus)
	if bulkRecInvalid.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid bulk status, got %d", bulkRecInvalid.Code)
	}

	// 4. Malformed JSON on bulk update
	bulkReqMalformed := httptest.NewRequest(http.MethodPost, "/api/v1/recommendations/bulk", bytes.NewBufferString(`{not-json}`))
	bulkReqMalformed.Header.Set("Content-Type", "application/json")
	bulkRecMalformed := httptest.NewRecorder()
	mux.ServeHTTP(bulkRecMalformed, bulkReqMalformed)
	if bulkRecMalformed.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for malformed bulk JSON, got %d", bulkRecMalformed.Code)
	}

	// 5. Bulk Pending operation succeeds and resets summary to 0 savings
	bulkReqPending := httptest.NewRequest(http.MethodPost, "/api/v1/recommendations/bulk", bytes.NewBufferString(`{"status":"pending","filter_risk":"ALL"}`))
	bulkReqPending.Header.Set("Content-Type", "application/json")
	bulkRecPending := httptest.NewRecorder()
	mux.ServeHTTP(bulkRecPending, bulkReqPending)
	if bulkRecPending.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for bulk pending, got %d", bulkRecPending.Code)
	}

	var respPending models.RecommendationsResponse
	if err := json.NewDecoder(bulkRecPending.Body).Decode(&respPending); err != nil {
		t.Fatalf("failed to decode bulk pending response: %v", err)
	}
	if respPending.Summary.AcceptedCount != 0 {
		t.Fatalf("expected 0 accepted count after bulk pending, got %d", respPending.Summary.AcceptedCount)
	}
	if respPending.Summary.AcceptedSavingsINR != 0 {
		t.Fatalf("expected 0 accepted savings after bulk pending, got %f", respPending.Summary.AcceptedSavingsINR)
	}
	if respPending.Summary.PendingCount != len(respPending.Items) {
		t.Fatalf("expected all %d items to be pending, got %d", len(respPending.Items), respPending.Summary.PendingCount)
	}
}

// TestChallenger_SimulateAPIAdversarialInputs tests boundary cases on Simulate endpoint.
func TestChallenger_SimulateAPIAdversarialInputs(t *testing.T) {
	store := api.NewStore()
	mux := http.NewServeMux()
	store.RegisterRoutes(mux)

	// 1. Malformed JSON
	simReqMalformed := httptest.NewRequest(http.MethodPost, "/api/v1/simulate", bytes.NewBufferString(`invalid json`))
	simReqMalformed.Header.Set("Content-Type", "application/json")
	simRecMalformed := httptest.NewRecorder()
	mux.ServeHTTP(simRecMalformed, simReqMalformed)
	if simRecMalformed.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for malformed simulate JSON, got %d", simRecMalformed.Code)
	}

	// 2. Empty JSON body defaults cleanly without panic
	simReqEmpty := httptest.NewRequest(http.MethodPost, "/api/v1/simulate", bytes.NewBufferString(`{}`))
	simReqEmpty.Header.Set("Content-Type", "application/json")
	simRecEmpty := httptest.NewRecorder()
	mux.ServeHTTP(simRecEmpty, simReqEmpty)
	if simRecEmpty.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for empty simulate JSON, got %d", simRecEmpty.Code)
	}

	var res models.SimulationResult
	if err := json.NewDecoder(simRecEmpty.Body).Decode(&res); err != nil {
		t.Fatalf("failed to decode simulate response: %v", err)
	}
	if res.BaselineMonthlyINR <= 0 {
		t.Fatalf("expected baseline spend > 0, got %f", res.BaselineMonthlyINR)
	}
}
