package engine_test

import (
	"testing"

	"utho_backend/internal/catalog"
	"utho_backend/internal/engine"
	"utho_backend/internal/models"
	"utho_backend/internal/seed"
)

func TestPercentiles(t *testing.T) {
	vals := []float64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100}

	avg := engine.Average(vals)
	if avg != 55.0 {
		t.Fatalf("expected avg 55.0, got %f", avg)
	}

	p50 := engine.Percentile(vals, 50)
	if p50 < 54.0 || p50 > 56.0 {
		t.Fatalf("expected p50 approx 55.0, got %f", p50)
	}

	p95 := engine.Percentile(vals, 95)
	if p95 < 94.0 || p95 > 96.0 {
		t.Fatalf("expected p95 approx 95.5, got %f", p95)
	}

	p99 := engine.Percentile(vals, 99)
	if p99 < 98.0 || p99 > 100.0 {
		t.Fatalf("expected p99 approx 99.1, got %f", p99)
	}

	// Linear slope test for increasing trend
	slope := engine.LinearSlope([]float64{10, 20, 30, 40, 50})
	if slope != 10.0 {
		t.Fatalf("expected slope 10.0, got %f", slope)
	}
}

func TestSeedDatasetGeneration(t *testing.T) {
	resources := seed.GenerateDataset()

	if len(resources) != 85 {
		t.Fatalf("expected exactly 85 resources, got %d", len(resources))
	}

	var protectedCount, overprovCount, batchCount, devStagingCount, zombieCount, orphanCount int
	var foundWebhookVM, foundSearchIndexer, foundBatchRecon bool

	for _, r := range resources {
		switch r.OptimizationStatus {
		case "PROTECTED_PEAK_LOAD":
			protectedCount++
		case "OVERPROVISIONED":
			overprovCount++
		case "IDLE_SCHEDULE_CANDIDATE":
			if r.Env == "prod" {
				batchCount++
			} else {
				devStagingCount++
			}
		case "ZOMBIE":
			zombieCount++
		case "ORPHANED":
			orphanCount++
		}

		if r.Name == "prod-payment-webhook-01" {
			foundWebhookVM = true
			if r.OptimizationStatus != "PROTECTED_PEAK_LOAD" {
				t.Fatalf("prod-payment-webhook-01 must be PROTECTED_PEAK_LOAD, got %s", r.OptimizationStatus)
			}
			if r.MetricsSummary.CPUP99 < 90.0 {
				t.Fatalf("prod-payment-webhook-01 p99 CPU must be >90%%, got %f", r.MetricsSummary.CPUP99)
			}
		}
		if r.Name == "prod-search-indexer-04" {
			foundSearchIndexer = true
		}
		if r.Name == "prod-nightly-recon-batch" {
			foundBatchRecon = true
		}
	}

	if !foundWebhookVM {
		t.Fatal("prod-payment-webhook-01 not found in dataset")
	}
	if !foundSearchIndexer {
		t.Fatal("prod-search-indexer-04 not found in dataset")
	}
	if !foundBatchRecon {
		t.Fatal("prod-nightly-recon-batch not found in dataset")
	}

	if protectedCount != 10 {
		t.Fatalf("expected 10 protected peak VMs, got %d", protectedCount)
	}
	if overprovCount != 20 {
		t.Fatalf("expected 20 overprovisioned VMs, got %d", overprovCount)
	}
	if batchCount != 8 {
		t.Fatalf("expected 8 batch VMs, got %d", batchCount)
	}
	if devStagingCount != 22 {
		t.Fatalf("expected 22 dev/staging VMs, got %d", devStagingCount)
	}
	if zombieCount != 5 {
		t.Fatalf("expected 5 zombie VMs, got %d", zombieCount)
	}
	if orphanCount != 20 {
		t.Fatalf("expected 20 orphaned resources, got %d", orphanCount)
	}
}

func TestPeakSafetyRule(t *testing.T) {
	resources := seed.GenerateDataset()

	// Ensure no recommendations are generated for protected peak load
	for _, r := range resources {
		if r.OptimizationStatus == "PROTECTED_PEAK_LOAD" {
			rec := engine.EvaluateResource(r, 25.0, 1)
			if rec != nil {
				t.Fatalf("CRITICAL SAFETY VIOLATION: Recommendation was generated for protected peak resource: %s (%s)", r.Name, rec.ActionType)
			}
		}
	}
}

func TestSimulationAndRemediation(t *testing.T) {
	resources := seed.GenerateDataset()
	req := models.SimulationRequest{
		DevOffHoursEnabled:         true,
		DevShutdownHoursPerWeek:    118,
		StagingWeekendOffEnabled:   true,
		SafetyHeadroomBufferPct:    25,
		PurgeOrphanedOlderThanDays: 14,
	}

	result := engine.Simulate(resources, req)
	if result.SimulatedSavingsINR <= 0 {
		t.Fatalf("expected positive savings, got %f", result.SimulatedSavingsINR)
	}
	if result.AffectedVMsCount <= 0 {
		t.Fatalf("expected affected resources, got %d", result.AffectedVMsCount)
	}

	recs := engine.GenerateRecommendations(resources, 25.0)
	export := engine.GenerateRemediation(recs)
	if export.UthoCLIScript == "" {
		t.Fatal("expected non-empty utho CLI script")
	}
	if export.TerraformHCL == "" {
		t.Fatal("expected non-empty terraform HCL")
	}
	if export.CSVReport == "" {
		t.Fatal("expected non-empty CSV report")
	}
}

func TestPricingTiers(t *testing.T) {
	p, ok := catalog.GetPlan("m8.xlarge")
	if !ok || p.MonthlyRateINR != 9600.0 || p.VCPU != 8 || p.RAMGB != 32 {
		t.Fatalf("incorrect plan data for m8.xlarge: %+v", p)
	}

	pSmall, ok := catalog.GetPlan("c2.small")
	if !ok || pSmall.MonthlyRateINR != 950.0 || pSmall.VCPU != 1 || pSmall.RAMGB != 2 {
		t.Fatalf("incorrect plan data for c2.small: %+v", pSmall)
	}
}

func TestRecommendationsInitialStatus(t *testing.T) {
	resources := seed.GenerateDataset()
	recs := engine.GenerateRecommendations(resources, 25.0)

	if len(recs) == 0 {
		t.Fatal("expected recommendations to be generated")
	}

	for _, r := range recs {
		if r.Status != "pending" {
			t.Fatalf("expected all recommendations to initialize as 'pending', got %s for rec %s (%s)", r.Status, r.ID, r.ResourceName)
		}
	}

	summary := engine.ComputeSummary(recs)
	if summary.AcceptedCount != 0 {
		t.Fatalf("expected 0 initial accepted count, got %d", summary.AcceptedCount)
	}
	if summary.AcceptedSavingsINR != 0 {
		t.Fatalf("expected 0 initial accepted savings INR, got %f", summary.AcceptedSavingsINR)
	}
	if summary.RejectedCount != 0 {
		t.Fatalf("expected 0 initial rejected count, got %d", summary.RejectedCount)
	}
	if summary.PendingCount != len(recs) {
		t.Fatalf("expected pending count to equal total recommendations (%d), got %d", len(recs), summary.PendingCount)
	}
	if summary.AcceptedSavingsPct != 0.0 {
		t.Fatalf("expected 0.0 accepted savings pct, got %f", summary.AcceptedSavingsPct)
	}
}
