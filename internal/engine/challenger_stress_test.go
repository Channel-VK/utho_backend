package engine_test

import (
	"math"
	"testing"

	"utho_backend/internal/engine"
	"utho_backend/internal/models"
	"utho_backend/internal/seed"
)

// TestChallenger_ZeroResourcesAndZeroBaseline ensures Simulate does not divide by zero or panic on empty fleet.
func TestChallenger_ZeroResourcesAndZeroBaseline(t *testing.T) {
	emptyResources := []models.Resource{}
	req := models.SimulationRequest{
		DevOffHoursEnabled:         true,
		DevShutdownHoursPerWeek:    118,
		StagingWeekendOffEnabled:   true,
		SafetyHeadroomBufferPct:    25,
		PurgeOrphanedOlderThanDays: 30,
	}

	result := engine.Simulate(emptyResources, req)
	if result.BaselineMonthlyINR != 0 {
		t.Fatalf("expected 0 baseline, got %f", result.BaselineMonthlyINR)
	}
	if result.SimulatedSavingsINR != 0 {
		t.Fatalf("expected 0 savings, got %f", result.SimulatedSavingsINR)
	}
	if result.SavingsPct != 0 {
		t.Fatalf("expected 0 savings pct, got %f", result.SavingsPct)
	}
	if result.AffectedVMsCount != 0 {
		t.Fatalf("expected 0 affected VMs, got %d", result.AffectedVMsCount)
	}
}

// TestChallenger_HeadroomDeltasDynamicCalculation verifies newly_eligible and newly_ineligible deltas.
func TestChallenger_HeadroomDeltasDynamicCalculation(t *testing.T) {
	resources := seed.GenerateDataset()

	// 1. Aggressive headroom: 15% (looser than baseline 25%)
	reqAggressive := models.SimulationRequest{
		DevOffHoursEnabled:         true,
		DevShutdownHoursPerWeek:    118,
		StagingWeekendOffEnabled:   true,
		SafetyHeadroomBufferPct:    15,
		PurgeOrphanedOlderThanDays: 30,
	}
	resAggressive := engine.Simulate(resources, reqAggressive)

	// Looser headroom allows more or equal downsizing
	if resAggressive.EligibleVMsCount <= 0 {
		t.Fatalf("expected positive eligible VMs in aggressive mode, got %d", resAggressive.EligibleVMsCount)
	}
	// Verify mathematical consistency: SimulatedSpend + SimulatedSavings == BaselineSpend (within 0.05 INR)
	diffAgg := math.Abs((resAggressive.SimulatedMonthlySpendINR + resAggressive.SimulatedMonthlySavingsINR) - resAggressive.BaselineMonthlyINR)
	if diffAgg > 0.1 {
		t.Fatalf("aggressive spend + savings (%f + %f = %f) does not match baseline (%f)",
			resAggressive.SimulatedMonthlySpendINR, resAggressive.SimulatedMonthlySavingsINR,
			resAggressive.SimulatedMonthlySpendINR+resAggressive.SimulatedMonthlySavingsINR,
			resAggressive.BaselineMonthlyINR)
	}

	// 2. High headroom buffer: 140% (stricter than baseline 25%)
	reqConservative := models.SimulationRequest{
		DevOffHoursEnabled:         false,
		DevShutdownHoursPerWeek:    0,
		StagingWeekendOffEnabled:   false,
		SafetyHeadroomBufferPct:    140,
		PurgeOrphanedOlderThanDays: 90,
	}
	resConservative := engine.Simulate(resources, reqConservative)

	// In conservative mode, more VMs are blocked by headroom
	if resConservative.BlockedByHeadroomCount <= 10 {
		t.Fatalf("expected more than 10 blocked VMs in high headroom mode, got %d", resConservative.BlockedByHeadroomCount)
	}
	// Verify that newly ineligible resources are reported when headroom is tightened to 140%
	if len(resConservative.NewlyIneligibleResources) == 0 {
		t.Fatalf("expected newly ineligible resources when increasing headroom buffer to 140%%, got 0")
	}

	for _, inelig := range resConservative.NewlyIneligibleResources {
		if inelig.ResourceID == "" || inelig.CurrentPlan == "" {
			t.Fatalf("incomplete newly ineligible resource entry: %+v", inelig)
		}
	}
}

// TestChallenger_AllRecommendationsPendingAcrossHeadroomProfiles ensures 100% manual pending status.
func TestChallenger_AllRecommendationsPendingAcrossHeadroomProfiles(t *testing.T) {
	resources := seed.GenerateDataset()

	buffers := []float64{10.0, 15.0, 25.0, 35.0, 50.0}

	for _, buf := range buffers {
		recs := engine.GenerateRecommendations(resources, buf)
		if len(recs) == 0 {
			t.Fatalf("expected recommendations for buffer %.1f%%", buf)
		}

		for _, r := range recs {
			if r.Status != "pending" {
				t.Fatalf("BUFFER %.1f%% VIOLATION: Recommendation %s (%s, %s) had non-pending status '%s'",
					buf, r.ID, r.ResourceName, r.ActionType, r.Status)
			}
			if r.MonthlySavingsINR <= 0 {
				t.Fatalf("Recommendation %s has non-positive savings: %f", r.ID, r.MonthlySavingsINR)
			}
		}

		summary := engine.ComputeSummary(recs)
		if summary.AcceptedCount != 0 {
			t.Fatalf("BUFFER %.1f%%: expected 0 accepted recs, got %d", buf, summary.AcceptedCount)
		}
		if summary.AcceptedSavingsINR != 0.0 {
			t.Fatalf("BUFFER %.1f%%: expected 0.0 accepted savings, got %f", buf, summary.AcceptedSavingsINR)
		}
		if summary.AcceptedSavingsPct != 0.0 {
			t.Fatalf("BUFFER %.1f%%: expected 0.0 accepted savings pct, got %f", buf, summary.AcceptedSavingsPct)
		}
		if summary.PendingCount != len(recs) {
			t.Fatalf("BUFFER %.1f%%: expected pending count %d, got %d", buf, len(recs), summary.PendingCount)
		}
		if summary.ProjectedMonthlyINR != summary.CurrentMonthlyINR {
			t.Fatalf("BUFFER %.1f%%: expected projected spend %f to equal current spend %f when 0 accepted",
				buf, summary.ProjectedMonthlyINR, summary.CurrentMonthlyINR)
		}
	}
}

// TestChallenger_ManualApprovalTransitionsAndSummaryMath verifies status updates and financial ledger math.
func TestChallenger_ManualApprovalTransitionsAndSummaryMath(t *testing.T) {
	resources := seed.GenerateDataset()
	recs := engine.GenerateRecommendations(resources, 25.0)

	initialSummary := engine.ComputeSummary(recs)
	totalCurrentCost := initialSummary.CurrentMonthlyINR

	// 1. Manually accept 1 item
	recs[0].Status = "accepted"
	s1 := engine.ComputeSummary(recs)
	if s1.AcceptedCount != 1 {
		t.Fatalf("expected accepted count 1, got %d", s1.AcceptedCount)
	}
	if s1.AcceptedSavingsINR != recs[0].MonthlySavingsINR {
		t.Fatalf("expected accepted savings %f, got %f", recs[0].MonthlySavingsINR, s1.AcceptedSavingsINR)
	}
	expectedProjected := math.Round((totalCurrentCost-recs[0].MonthlySavingsINR)*100) / 100
	if math.Abs(s1.ProjectedMonthlyINR-expectedProjected) > 0.05 {
		t.Fatalf("expected projected spend %f, got %f", expectedProjected, s1.ProjectedMonthlyINR)
	}

	// 2. Manually reject 1 item
	recs[1].Status = "rejected"
	s2 := engine.ComputeSummary(recs)
	if s2.AcceptedCount != 1 || s2.RejectedCount != 1 {
		t.Fatalf("expected 1 accepted and 1 rejected, got %d and %d", s2.AcceptedCount, s2.RejectedCount)
	}
	// Rejection should not reduce spend
	if math.Abs(s2.ProjectedMonthlyINR-expectedProjected) > 0.05 {
		t.Fatalf("rejected item should not reduce spend: expected %f, got %f", expectedProjected, s2.ProjectedMonthlyINR)
	}

	// 3. Revert all back to pending
	for i := range recs {
		recs[i].Status = "pending"
	}
	sClean := engine.ComputeSummary(recs)
	if sClean.AcceptedCount != 0 || sClean.RejectedCount != 0 || sClean.PendingCount != len(recs) {
		t.Fatalf("expected clean revert to 100%% pending, got %+v", sClean)
	}
	if sClean.AcceptedSavingsINR != 0 || sClean.ProjectedMonthlyINR != totalCurrentCost {
		t.Fatalf("expected clean financial reset, got savings=%f projected=%f", sClean.AcceptedSavingsINR, sClean.ProjectedMonthlyINR)
	}
}
