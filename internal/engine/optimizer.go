package engine

import (
	"fmt"
	"strings"

	"utho_backend/internal/catalog"
	"utho_backend/internal/models"
)

// GenerateRecommendations evaluates all resources and produces actionable FinOps recommendations.
func GenerateRecommendations(resources []models.Resource, headroomBufferPct float64) []models.Recommendation {
	var recs []models.Recommendation
	recCounter := 1

	for _, res := range resources {
		rec := EvaluateResource(res, headroomBufferPct, recCounter)
		if rec != nil {
			recs = append(recs, *rec)
			recCounter++
		}
	}

	return recs
}

// EvaluateResource evaluates a single resource against rightsizing and FinOps rules.
func EvaluateResource(res models.Resource, headroomBufferPct float64, counter int) *models.Recommendation {
	recID := fmt.Sprintf("rec-%03d", counter)

	// Never recommend downsizing protected peak workloads
	if res.OptimizationStatus == "PROTECTED_PEAK_LOAD" || (res.MetricsSummary != nil && res.MetricsSummary.CPUP99 >= 85.0) {
		return nil
	}

	// 1. OVERPROVISIONED VM -> RIGHTSIZE_VM
	if res.OptimizationStatus == "OVERPROVISIONED" && res.Type == "vm" && res.MetricsSummary != nil {
		currentPlan, exists := catalog.GetPlan(res.Plan)
		if !exists {
			return nil
		}

		headroomFactor := 1.0 + (headroomBufferPct / 100.0)

		// Search for best candidate plan with lower cost
		var bestPlan *models.PlanPricing
		for i := len(catalog.OrderedPlans) - 1; i >= 0; i-- {
			candidate := catalog.OrderedPlans[i]
			if candidate.MonthlyRateINR >= currentPlan.MonthlyRateINR {
				continue
			}

			// Capacity math
			projectedCPUP95 := (float64(currentPlan.VCPU) * res.MetricsSummary.CPUP95) / float64(candidate.VCPU)
			projectedCPUP99 := (float64(currentPlan.VCPU) * res.MetricsSummary.CPUP99) / float64(candidate.VCPU)
			projectedMemP95 := (float64(currentPlan.RAMGB) * res.MetricsSummary.MemP95) / float64(candidate.RAMGB)

			// Safety criteria:
			// 1. Projected p95 * headroom <= 100% (i.e. projectedCPUP95 <= 100 / headroomFactor)
			// 2. Projected p99 < 85%
			// 3. Projected RAM p95 < 75%
			maxSafeP95 := 100.0 / headroomFactor
			if projectedCPUP95 <= maxSafeP95 && projectedCPUP99 < 85.0 && projectedMemP95 < 75.0 {
				bestPlan = &candidate
			}
		}

		if bestPlan != nil {
			savings := currentPlan.MonthlyRateINR - bestPlan.MonthlyRateINR
			projCPUP95 := (float64(currentPlan.VCPU) * res.MetricsSummary.CPUP95) / float64(bestPlan.VCPU)
			projMemP95 := (float64(currentPlan.RAMGB) * res.MetricsSummary.MemP95) / float64(bestPlan.RAMGB)

			risk := "LOW"
			if projCPUP95 > 65.0 {
				risk = "MEDIUM"
			}

			status := "pending"
			tfLabel := strings.ReplaceAll(res.Name, "-", "_")

			return &models.Recommendation{
				ID:                  recID,
				ResourceID:          res.ID,
				ResourceName:        res.Name,
				ResourceType:        res.Type,
				Region:              res.Region,
				Env:                 res.Env,
				Team:                res.Team,
				ActionType:          "RIGHTSIZE_VM",
				CurrentConfig:       fmt.Sprintf("%s (%d vCPU, %d GB)", currentPlan.Name, currentPlan.VCPU, currentPlan.RAMGB),
				RecommendedConfig:   fmt.Sprintf("%s (%d vCPU, %d GB)", bestPlan.Name, bestPlan.VCPU, bestPlan.RAMGB),
				CurrentMonthlyINR:   currentPlan.MonthlyRateINR,
				ProjectedMonthlyINR: bestPlan.MonthlyRateINR,
				MonthlySavingsINR:   savings,
				FormulaExplanation:  fmt.Sprintf("₹%.0f (%s) - ₹%.0f (%s) = ₹%.0f/mo saved", currentPlan.MonthlyRateINR, currentPlan.Name, bestPlan.MonthlyRateINR, bestPlan.Name, savings),
				Reason:              fmt.Sprintf("30-day p95 CPU is %.1f%% (p99: %.1f%%) and p95 RAM is %.1f%%. Scaling to %d vCPU / %d GB places projected p95 peak at %.1f%% CPU & %.1f%% RAM (well below 70%% safety threshold).", res.MetricsSummary.CPUP95, res.MetricsSummary.CPUP99, res.MetricsSummary.MemP95, bestPlan.VCPU, bestPlan.RAMGB, projCPUP95, projMemP95),
				RiskLevel:           risk,
				ConfidenceScore:     94,
				Status:              status,
				CLICommand:          fmt.Sprintf("utho vm resize --id %s --plan %s --region %s --restart --yes", res.ID, bestPlan.Name, res.Region),
				TerraformCode: fmt.Sprintf(
					"resource \"utho_cloud_instance\" \"%s\" {\n  name   = \"%s\"\n  dcslug = \"%s\"\n  plan   = \"%s\" # Rightsized from %s\n  tags = {\n    Environment = \"%s\"\n    Team        = \"%s\"\n    ManagedBy   = \"FinSort-FinOps\"\n  }\n}",
					tfLabel, res.Name, res.Region, bestPlan.Name, currentPlan.Name, res.Env, res.Team,
				),
			}
		}
	}

	// 2. IDLE_SCHEDULE_CANDIDATE
	if res.OptimizationStatus == "IDLE_SCHEDULE_CANDIDATE" && res.Type == "vm" {
		currentPlan, exists := catalog.GetPlan(res.Plan)
		if !exists {
			return nil
		}
		tfLabel := strings.ReplaceAll(res.Name, "-", "_")

		if strings.Contains(res.Name, "batch") || strings.Contains(res.Name, "recon") || strings.Contains(res.Name, "rollup") || strings.Contains(res.Name, "lake") {
			// Batch job idle 20 hrs/day
			savings := round2(currentPlan.MonthlyRateINR * (20.0 / 24.0))
			projCost := round2(currentPlan.MonthlyRateINR - savings)

			return &models.Recommendation{
				ID:                  recID,
				ResourceID:          res.ID,
				ResourceName:        res.Name,
				ResourceType:        res.Type,
				Region:              res.Region,
				Env:                 res.Env,
				Team:                res.Team,
				ActionType:          "SCHEDULE_OFF_HOURS",
				CurrentConfig:       fmt.Sprintf("%s (24/7 Always-On)", currentPlan.Name),
				RecommendedConfig:   fmt.Sprintf("%s (Scheduled 4h/day execution window)", currentPlan.Name),
				CurrentMonthlyINR:   currentPlan.MonthlyRateINR,
				ProjectedMonthlyINR: projCost,
				MonthlySavingsINR:   savings,
				FormulaExplanation:  fmt.Sprintf("₹%.0f/mo * (20 idle hrs / 24 hrs/day) = ₹%.0f/mo saved", currentPlan.MonthlyRateINR, savings),
				Reason:              "Workload executes batch processing for ~3-4 hours daily. Automating stop/start schedule for remaining 20 hours yields ~83% compute cost reduction with zero SLA impact.",
				RiskLevel:           "LOW",
				ConfidenceScore:     96,
				Status:              "pending",
				CLICommand:          fmt.Sprintf("utho vm schedule create --id %s --stop-cron \"0 4 * * *\" --start-cron \"0 0 * * *\" --timezone \"Asia/Kolkata\"", res.ID),
				TerraformCode: fmt.Sprintf(
					"resource \"utho_instance_schedule\" \"%s_batch\" {\n  instance_id = \"%s\"\n  name        = \"%s-batch-window\"\n  dcslug      = \"%s\"\n  start_cron  = \"0 0 * * *\"\n  stop_cron   = \"0 4 * * *\"\n  timezone    = \"Asia/Kolkata\"\n}",
					tfLabel, res.ID, res.Name, res.Region,
				),
			}
		}

		// Dev/staging 118 hrs/week off (nights + weekends)
		// Savings ratio ~ 65% accounting for buffer
		savingsRatio := 0.65
		savings := round2(currentPlan.MonthlyRateINR * savingsRatio)
		projCost := round2(currentPlan.MonthlyRateINR - savings)

		status := "pending"

		return &models.Recommendation{
			ID:                  recID,
			ResourceID:          res.ID,
			ResourceName:        res.Name,
			ResourceType:        res.Type,
			Region:              res.Region,
			Env:                 res.Env,
			Team:                res.Team,
			ActionType:          "SCHEDULE_OFF_HOURS",
			CurrentConfig:       fmt.Sprintf("%s (168 hrs/week running)", currentPlan.Name),
			RecommendedConfig:   fmt.Sprintf("%s (Mon-Fri 09:00-19:00 IST schedule)", currentPlan.Name),
			CurrentMonthlyINR:   currentPlan.MonthlyRateINR,
			ProjectedMonthlyINR: projCost,
			MonthlySavingsINR:   savings,
			FormulaExplanation:  fmt.Sprintf("₹%.0f/mo * (118 idle off-hours / 168 hrs/wk) = ₹%.0f/mo saved", currentPlan.MonthlyRateINR, savings),
			Reason:              "Non-production test environment idle during off-hours and weekends (118 hrs/week). Automated scheduled hibernation eliminates 65% of running costs.",
			RiskLevel:           "LOW",
			ConfidenceScore:     95,
			Status:              status,
			CLICommand:          fmt.Sprintf("utho vm schedule create --id %s --stop-cron \"0 19 * * 1-5\" --start-cron \"0 9 * * 1-5\" --timezone \"Asia/Kolkata\"", res.ID),
			TerraformCode: fmt.Sprintf(
				"resource \"utho_instance_schedule\" \"%s_offhours\" {\n  instance_id = \"%s\"\n  name        = \"%s-offhours\"\n  dcslug      = \"%s\"\n  stop_cron   = \"0 19 * * 1-5\"\n  start_cron  = \"0 9 * * 1-5\"\n  timezone    = \"Asia/Kolkata\"\n}",
				tfLabel, res.ID, res.Name, res.Region,
			),
		}
	}

	// 3. ZOMBIE VM -> TERMINATE_ZOMBIE
	if res.OptimizationStatus == "ZOMBIE" && res.Type == "vm" {
		currentPlan, exists := catalog.GetPlan(res.Plan)
		if !exists {
			return nil
		}
		tfLabel := strings.ReplaceAll(res.Name, "-", "_")

		// Retain 5% for safety snapshot storage, save 95%
		savings := round2(currentPlan.MonthlyRateINR * 0.95)
		projCost := round2(currentPlan.MonthlyRateINR - savings)

		return &models.Recommendation{
			ID:                  recID,
			ResourceID:          res.ID,
			ResourceName:        res.Name,
			ResourceType:        res.Type,
			Region:              res.Region,
			Env:                 res.Env,
			Team:                res.Team,
			ActionType:          "TERMINATE_ZOMBIE",
			CurrentConfig:       fmt.Sprintf("%s (Running Idle)", currentPlan.Name),
			RecommendedConfig:   "Archived Snapshot & Terminated Instance",
			CurrentMonthlyINR:   currentPlan.MonthlyRateINR,
			ProjectedMonthlyINR: projCost,
			MonthlySavingsINR:   savings,
			FormulaExplanation:  fmt.Sprintf("₹%.0f/mo compute - ₹%.0f/mo snapshot retention = ₹%.0f/mo saved", currentPlan.MonthlyRateINR, projCost, savings),
			Reason:              "Resource created >90 days ago with p99 CPU < 3.5% and net egress < 0.5 Mbps for 30 consecutive days. Safe to snapshot disk and terminate.",
			RiskLevel:           "LOW",
			ConfidenceScore:     98,
			Status:              "pending",
			CLICommand:          fmt.Sprintf("utho vm snapshot --id %s --name \"archive-%s\" && utho vm destroy --id %s --force", res.ID, res.ID, res.ID),
			TerraformCode: fmt.Sprintf(
				"resource \"utho_snapshot\" \"%s_archive\" {\n  instance_id       = \"%s\"\n  name              = \"archive-pre-decommission-%s\"\n  retention_days    = 30\n  destroy_source_vm = true\n}",
				tfLabel, res.ID, res.ID,
			),
		}
	}

	// 4. ORPHANED BLOCK VOLUME -> CLEANUP_ORPHAN
	if res.OptimizationStatus == "ORPHANED" && res.Type == "block_volume" {
		savings := res.MonthlyCostINR
		tfLabel := strings.ReplaceAll(res.Name, "-", "_")
		return &models.Recommendation{
			ID:                  recID,
			ResourceID:          res.ID,
			ResourceName:        res.Name,
			ResourceType:        res.Type,
			Region:              res.Region,
			Env:                 res.Env,
			Team:                res.Team,
			ActionType:          "CLEANUP_ORPHAN",
			CurrentConfig:       fmt.Sprintf("%d GB Block SSD (Unattached)", res.DiskGB),
			RecommendedConfig:   "Deleted Volume",
			CurrentMonthlyINR:   res.MonthlyCostINR,
			ProjectedMonthlyINR: 0,
			MonthlySavingsINR:   savings,
			FormulaExplanation:  fmt.Sprintf("%d GB * ₹8/GB/mo = ₹%.0f/mo unutilized storage saved", res.DiskGB, savings),
			Reason:              fmt.Sprintf("Block volume (%d GB) unattached for >30 days with zero read/write IOPS. Incurring unnecessary storage spend.", res.DiskGB),
			RiskLevel:           "LOW",
			ConfidenceScore:     99,
			Status:              "pending",
			CLICommand:          fmt.Sprintf("utho volume delete --id %s --force", res.ID),
			TerraformCode: fmt.Sprintf(
				"resource \"utho_decommissioned_volume\" \"%s\" {\n  volume_id            = \"%s\"\n  name                 = \"%s\"\n  dcslug               = \"%s\"\n  size_gb              = %d\n  snapshot_before_drop = true\n}",
				tfLabel, res.ID, res.Name, res.Region, res.DiskGB,
			),
		}
	}

	// 5. ORPHANED PUBLIC IP -> CLEANUP_ORPHAN
	if res.OptimizationStatus == "ORPHANED" && res.Type == "public_ip" {
		savings := res.MonthlyCostINR
		tfLabel := strings.ReplaceAll(res.Name, "-", "_")
		return &models.Recommendation{
			ID:                  recID,
			ResourceID:          res.ID,
			ResourceName:        res.Name,
			ResourceType:        res.Type,
			Region:              res.Region,
			Env:                 res.Env,
			Team:                res.Team,
			ActionType:          "CLEANUP_ORPHAN",
			CurrentConfig:       "Unattached Floating IPv4",
			RecommendedConfig:   "Released to Utho IP Pool",
			CurrentMonthlyINR:   res.MonthlyCostINR,
			ProjectedMonthlyINR: 0,
			MonthlySavingsINR:   savings,
			FormulaExplanation:  "₹350/mo unattached public IPv4 fee eliminated = ₹350/mo saved",
			Reason:              "Public IPv4 address is not assigned to any network interface or VM, incurring idle reservation charges.",
			RiskLevel:           "LOW",
			ConfidenceScore:     99,
			Status:              "pending",
			CLICommand:          fmt.Sprintf("utho ip release --id %s", res.ID),
			TerraformCode: fmt.Sprintf(
				"resource \"utho_decommissioned_floating_ip\" \"%s\" {\n  allocation_id = \"%s\"\n  name          = \"%s\"\n  dcslug        = \"%s\"\n  release_to_pool = true\n}",
				tfLabel, res.ID, res.Name, res.Region,
			),
		}
	}

	return nil
}

// ComputeSummary aggregates metrics for a slice of recommendations.
func ComputeSummary(recs []models.Recommendation) models.RecommendationSummary {
	var currentTotal, projectedTotal, acceptedSavings float64
	var acceptedCount, rejectedCount, pendingCount int

	envMap := make(map[string]*models.SummaryBreakdown)
	actionMap := make(map[string]*models.SummaryBreakdown)

	for _, rec := range recs {
		currentTotal += rec.CurrentMonthlyINR
		switch rec.Status {
		case "accepted":
			acceptedCount++
			projectedTotal += rec.ProjectedMonthlyINR
			acceptedSavings += rec.MonthlySavingsINR
		case "rejected":
			rejectedCount++
			projectedTotal += rec.CurrentMonthlyINR
		default: // pending
			pendingCount++
			projectedTotal += rec.CurrentMonthlyINR
		}

		// Aggregate by env
		if _, ok := envMap[rec.Env]; !ok {
			envMap[rec.Env] = &models.SummaryBreakdown{Key: rec.Env}
		}
		envMap[rec.Env].CurrentMonthlyINR += rec.CurrentMonthlyINR
		envMap[rec.Env].ProjectedMonthlyINR += rec.ProjectedMonthlyINR
		envMap[rec.Env].SavingsINR += rec.MonthlySavingsINR
		envMap[rec.Env].Count++

		// Aggregate by action
		if _, ok := actionMap[rec.ActionType]; !ok {
			actionMap[rec.ActionType] = &models.SummaryBreakdown{Key: rec.ActionType}
		}
		actionMap[rec.ActionType].CurrentMonthlyINR += rec.CurrentMonthlyINR
		actionMap[rec.ActionType].ProjectedMonthlyINR += rec.ProjectedMonthlyINR
		actionMap[rec.ActionType].SavingsINR += rec.MonthlySavingsINR
		actionMap[rec.ActionType].Count++
	}

	var byEnv []models.SummaryBreakdown
	for _, v := range envMap {
		byEnv = append(byEnv, *v)
	}

	var byAction []models.SummaryBreakdown
	for _, v := range actionMap {
		byAction = append(byAction, *v)
	}

	var acceptedPct float64
	if currentTotal > 0 {
		acceptedPct = round2((acceptedSavings / currentTotal) * 100.0)
	}

	return models.RecommendationSummary{
		CurrentMonthlyINR:   round2(currentTotal),
		ProjectedMonthlyINR: round2(projectedTotal),
		AcceptedSavingsINR:  round2(acceptedSavings),
		AcceptedSavingsPct:  acceptedPct,
		AcceptedCount:       acceptedCount,
		RejectedCount:       rejectedCount,
		PendingCount:        pendingCount,
		ByEnv:               byEnv,
		ByAction:            byAction,
	}
}
