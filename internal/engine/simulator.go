package engine

import (
	"fmt"
	"strings"
	"time"

	"utho_backend/internal/catalog"
	"utho_backend/internal/models"
)

// Simulate calculates financial and sustainability outcomes based on simulation parameters.
func Simulate(resources []models.Resource, req models.SimulationRequest) models.SimulationResult {
	baselineSpend := 0.0
	for _, res := range resources {
		baselineSpend += res.MonthlyCostINR
	}

	simulatedSavings := 0.0
	affectedCount := 0
	eligibleVMsCount := 0
	blockedByHeadroomCount := 0

	var devSavings, stagingWeekendSavings, rightsizingSavings, orphanSavings float64

	var newlyEligible []models.EligibleChange
	var newlyIneligible []models.EligibleChange

	// Default baseline buffer is 25%
	baselineBuffer := 25.0
	currentBuffer := req.SafetyHeadroomBufferPct
	if currentBuffer <= 0 {
		currentBuffer = 25.0
	}

	for _, res := range resources {
		switch res.OptimizationStatus {
		case "PROTECTED_PEAK_LOAD":
			blockedByHeadroomCount++

		case "OVERPROVISIONED":
			if res.Type != "vm" || res.MetricsSummary == nil {
				continue
			}

			// Evaluate with baseline 25%
			baseRec := EvaluateResource(res, baselineBuffer, 1)
			// Evaluate with current requested buffer
			simRec := EvaluateResource(res, currentBuffer, 1)

			if baseRec == nil && simRec != nil {
				// Newly eligible with looser headroom
				newlyEligible = append(newlyEligible, models.EligibleChange{
					ResourceID:   res.ID,
					ResourceName: res.Name,
					CurrentPlan:  res.Plan,
					ProposedPlan: simRec.RecommendedConfig,
					P95Peak:      res.MetricsSummary.CPUP95,
					P99Peak:      res.MetricsSummary.CPUP99,
					Reason:       fmt.Sprintf("Headroom buffer adjusted to %.0f%% allowed safe downsizing to %s", currentBuffer, simRec.RecommendedConfig),
				})
			} else if baseRec != nil && simRec == nil {
				// Newly ineligible with stricter headroom
				newlyIneligible = append(newlyIneligible, models.EligibleChange{
					ResourceID:   res.ID,
					ResourceName: res.Name,
					CurrentPlan:  res.Plan,
					ProposedPlan: "None (Downsizing blocked)",
					P95Peak:      res.MetricsSummary.CPUP95,
					P99Peak:      res.MetricsSummary.CPUP99,
					Reason:       fmt.Sprintf("Headroom buffer increased to %.0f%% requires more safety capacity than smaller tiers provide", currentBuffer),
				})
			}

			if simRec != nil {
				simulatedSavings += simRec.MonthlySavingsINR
				rightsizingSavings += simRec.MonthlySavingsINR
				affectedCount++
				eligibleVMsCount++
			} else {
				blockedByHeadroomCount++
			}

		case "IDLE_SCHEDULE_CANDIDATE":
			if res.Type != "vm" {
				continue
			}

			if strings.Contains(res.Name, "batch") || strings.Contains(res.Name, "recon") || strings.Contains(res.Name, "rollup") || strings.Contains(res.Name, "lake") {
				// Batch job
				savings := res.MonthlyCostINR * (20.0 / 24.0)
				simulatedSavings += savings
				devSavings += savings
				affectedCount++
				eligibleVMsCount++
			} else {
				// Dev / Staging schedule
				isDev := res.Env == "dev"
				isStaging := res.Env == "staging"

				if isDev && req.DevOffHoursEnabled {
					hoursOff := float64(req.DevShutdownHoursPerWeek)
					if hoursOff <= 0 {
						hoursOff = 118.0
					}
					ratio := (hoursOff / 168.0) * 0.95
					savings := res.MonthlyCostINR * ratio
					simulatedSavings += savings
					devSavings += savings
					affectedCount++
					eligibleVMsCount++
				} else if isStaging && req.StagingWeekendOffEnabled {
					hoursOff := float64(req.StagingShutdownHoursPerWeek)
					if hoursOff <= 0 {
						hoursOff = 64.0 // Staging weekend off fallback (Friday 19:00 to Monday 07:00 approx 60h)
					}
					ratio := (hoursOff / 168.0) * 0.95
					savings := res.MonthlyCostINR * ratio
					simulatedSavings += savings
					stagingWeekendSavings += savings
					affectedCount++
					eligibleVMsCount++
				}
			}

		case "ZOMBIE":
			savings := res.MonthlyCostINR * 0.95
			simulatedSavings += savings
			rightsizingSavings += savings
			affectedCount++
			eligibleVMsCount++

		case "ORPHANED":
			if req.PurgeOrphanedOlderThanDays <= 30 {
				simulatedSavings += res.MonthlyCostINR
				orphanSavings += res.MonthlyCostINR
				affectedCount++
			} else if req.PurgeOrphanedOlderThanDays <= 60 {
				partial := res.MonthlyCostINR * 0.65
				simulatedSavings += partial
				orphanSavings += partial
				affectedCount++
			}
		}
	}

	simulatedSpend := baselineSpend - simulatedSavings
	if simulatedSpend < 0 {
		simulatedSpend = 0
	}

	savingsPct := 0.0
	if baselineSpend > 0 {
		savingsPct = round2((simulatedSavings / baselineSpend) * 100.0)
	}

	// Sustainability math:
	// Approx 45W per VM saved. 730 hours in a month = 32.85 kWh/mo per VM.
	kwhSaved := float64(affectedCount) * 32.85
	co2Saved := kwhSaved * catalog.GridCarbonFactorKgKWh
	trees := co2Saved / 21.77 // ~21.77 kg CO2 absorbed per mature tree per year

	riskSummary := fmt.Sprintf("Balanced Production Grade: Recommended +%.0f%% peak headroom buffer applied (%d bursty VMs protected).", currentBuffer, blockedByHeadroomCount)
	if currentBuffer >= 40 {
		riskSummary = fmt.Sprintf("Ultra-Conservative Safety Buffer (+%.0f%%): Maximum SLA cushion; %d VMs blocked from downsizing.", currentBuffer, blockedByHeadroomCount)
	} else if currentBuffer <= 20 {
		riskSummary = fmt.Sprintf("Aggressive Rightsizing Profile (+%.0f%%): Higher compute yield, tighter burst headroom (%d protected).", currentBuffer, blockedByHeadroomCount)
	}

	return models.SimulationResult{
		BaselineMonthlyINR:         round2(baselineSpend),
		SimulatedMonthlyINR:        round2(simulatedSpend),
		SimulatedSavingsINR:        round2(simulatedSavings),
		SavingsPct:                 savingsPct,
		AffectedVMsCount:           affectedCount,
		SimulatedMonthlySpendINR:   round2(simulatedSpend),
		SimulatedMonthlySavingsINR: round2(simulatedSavings),
		SimulatedSavingsPct:        savingsPct,
		EligibleVMsCount:           eligibleVMsCount,
		BlockedByHeadroomCount:     blockedByHeadroomCount,
		CarbonSavedKg:              round2(co2Saved),
		RiskSummary:                riskSummary,
		Breakdown: models.SimulationBreakdown{
			DevSchedulingSavingsINR:  round2(devSavings),
			StagingWeekendSavingsINR: round2(stagingWeekendSavings),
			RightsizingSavingsINR:    round2(rightsizingSavings),
			OrphanPurgeSavingsINR:    round2(orphanSavings),
		},
		NewlyEligibleResources:   newlyEligible,
		NewlyIneligibleResources: newlyIneligible,
		Sustainability: models.Sustainability{
			MonthlyKWhSaved:   round2(kwhSaved),
			MonthlyCO2KgSaved: round2(co2Saved),
			TreesEquivalent:   round2(trees),
		},
	}
}

// GenerateRemediation generates executable bash scripts, Terraform HCL, and CSV report for accepted recommendations.
func GenerateRemediation(recs []models.Recommendation) models.RemediationExport {
	var accepted []models.Recommendation
	for _, r := range recs {
		if r.Status == "accepted" {
			accepted = append(accepted, r)
		}
	}

	timestamp := time.Now().UTC().Format(time.RFC3339)

	// 1. Utho CLI Script
	var cliBuilder strings.Builder
	cliBuilder.WriteString("#!/usr/bin/env bash\n")
	cliBuilder.WriteString("# ==============================================================================\n")
	cliBuilder.WriteString("# FINSORT - AUTOMATED REMEDIATION RUNBOOK (UTHO CLOUD)\n")
	cliBuilder.WriteString(fmt.Sprintf("# Generated at: %s\n", timestamp))
	cliBuilder.WriteString(fmt.Sprintf("# Total Actions: %d Accepted FinOps Remediations\n", len(accepted)))
	cliBuilder.WriteString("# ==============================================================================\n\n")
	cliBuilder.WriteString("set -euo pipefail\n\n")
	cliBuilder.WriteString("echo \"[INFO] Authenticating with Utho Cloud API...\"\n")
	cliBuilder.WriteString("command -v utho >/dev/null 2>&1 || { echo >&2 \"[ERROR] 'utho' CLI tool not found in PATH. Aborting.\"; exit 1; }\n\n")

	// 2. Terraform HCL Snippet
	var hclBuilder strings.Builder
	hclBuilder.WriteString("# ==============================================================================\n")
	hclBuilder.WriteString("# FINSORT - TERRAFORM HCL RECONFIGURATIONS\n")
	hclBuilder.WriteString(fmt.Sprintf("# Generated at: %s\n", timestamp))
	hclBuilder.WriteString("# Provider: uthocloud/utho\n")
	hclBuilder.WriteString("# ==============================================================================\n\n")

	// 3. CSV Report
	var csvBuilder strings.Builder
	csvBuilder.WriteString("Recommendation ID,Resource ID,Resource Name,Type,Region,Environment,Team,Action,Current Config,Recommended Config,Current Monthly (INR),Projected Monthly (INR),Monthly Savings (INR),Confidence Score,Risk Level,Status\n")

	for _, r := range accepted {
		// Append CSV row
		csvBuilder.WriteString(fmt.Sprintf("\"%s\",\"%s\",\"%s\",\"%s\",\"%s\",\"%s\",\"%s\",\"%s\",\"%s\",\"%s\",%.2f,%.2f,%.2f,%d,\"%s\",\"%s\"\n",
			r.ID, r.ResourceID, r.ResourceName, r.ResourceType, r.Region, r.Env, r.Team, r.ActionType,
			r.CurrentConfig, r.RecommendedConfig, r.CurrentMonthlyINR, r.ProjectedMonthlyINR, r.MonthlySavingsINR,
			r.ConfidenceScore, r.RiskLevel, r.Status))

		// Append CLI & HCL
		switch r.ActionType {
		case "RIGHTSIZE_VM":
			targetPlan := strings.Fields(r.RecommendedConfig)[0]
			cliBuilder.WriteString(fmt.Sprintf("# Resize %s (%s) -> %s (Saves ₹%.0f/mo)\n", r.ResourceName, r.ResourceID, targetPlan, r.MonthlySavingsINR))
			cliBuilder.WriteString(fmt.Sprintf("echo \"[EXEC] Resizing %s to %s...\"\n", r.ResourceName, targetPlan))
			cliBuilder.WriteString(fmt.Sprintf("utho target --region %s\n", r.Region))
			cliBuilder.WriteString(fmt.Sprintf("utho vm resize --id %s --plan %s --restart --yes\n\n", r.ResourceID, targetPlan))

			hclBuilder.WriteString(fmt.Sprintf("resource \"utho_cloud_instance\" \"%s\" {\n", strings.ReplaceAll(r.ResourceName, "-", "_")))
			hclBuilder.WriteString(fmt.Sprintf("  name = \"%s\"\n", r.ResourceName))
			hclBuilder.WriteString(fmt.Sprintf("  plan = \"%s\" # Optimized from %s\n", targetPlan, strings.Fields(r.CurrentConfig)[0]))
			hclBuilder.WriteString(fmt.Sprintf("  dcslug = \"%s\"\n", r.Region))
			hclBuilder.WriteString("}\n\n")

		case "SCHEDULE_OFF_HOURS":
			cliBuilder.WriteString(fmt.Sprintf("# Configure Off-Hours Schedule for %s (Saves ₹%.0f/mo)\n", r.ResourceName, r.MonthlySavingsINR))
			cliBuilder.WriteString(fmt.Sprintf("echo \"[EXEC] Applying hibernation policy to %s...\"\n", r.ResourceName))
			cliBuilder.WriteString(fmt.Sprintf("utho vm schedule create --id %s --cron \"0 19 * * 1-5 stop\" --cron \"0 9 * * 1-5 start\"\n\n", r.ResourceID))

			hclBuilder.WriteString(fmt.Sprintf("resource \"utho_instance_schedule\" \"%s_offhours\" {\n", strings.ReplaceAll(r.ResourceName, "-", "_")))
			hclBuilder.WriteString(fmt.Sprintf("  instance_id = utho_cloud_instance.%s.id\n", strings.ReplaceAll(r.ResourceName, "-", "_")))
			hclBuilder.WriteString("  stop_cron   = \"0 19 * * 1-5\" # 19:00 IST weekdays\n")
			hclBuilder.WriteString("  start_cron  = \"0 9 * * 1-5\"  # 09:00 IST weekdays\n")
			hclBuilder.WriteString("}\n\n")

		case "TERMINATE_ZOMBIE":
			cliBuilder.WriteString(fmt.Sprintf("# Backup and Decommission Zombie VM: %s (Saves ₹%.0f/mo)\n", r.ResourceName, r.MonthlySavingsINR))
			cliBuilder.WriteString(fmt.Sprintf("echo \"[EXEC] Creating archival snapshot before deleting %s...\"\n", r.ResourceName))
			cliBuilder.WriteString(fmt.Sprintf("utho vm snapshot --id %s --name \"archive-pre-term-%s\"\n", r.ResourceID, r.ResourceID))
			cliBuilder.WriteString(fmt.Sprintf("utho vm destroy --id %s --force\n\n", r.ResourceID))

			hclBuilder.WriteString(fmt.Sprintf("# Resource %s was decommissioned to eliminate zombie compute spend.\n", r.ResourceName))
			hclBuilder.WriteString(fmt.Sprintf("# terraform state rm utho_cloud_instance.%s\n\n", strings.ReplaceAll(r.ResourceName, "-", "_")))

		case "CLEANUP_ORPHAN":
			if r.ResourceType == "block_volume" {
				cliBuilder.WriteString(fmt.Sprintf("# Purge Unattached Volume: %s (Saves ₹%.0f/mo)\n", r.ResourceName, r.MonthlySavingsINR))
				cliBuilder.WriteString(fmt.Sprintf("utho volume delete --id %s --force\n\n", r.ResourceID))
			} else {
				cliBuilder.WriteString(fmt.Sprintf("# Release Unattached Floating IP: %s (Saves ₹%.0f/mo)\n", r.ResourceName, r.MonthlySavingsINR))
				cliBuilder.WriteString(fmt.Sprintf("utho ip release --id %s\n\n", r.ResourceID))
			}
		}
	}

	cliBuilder.WriteString("echo \"[SUCCESS] All accepted FinOps remediations successfully executed!\"\n")

	return models.RemediationExport{
		GeneratedAt:   timestamp,
		AcceptedCount: len(accepted),
		UthoCLIScript: cliBuilder.String(),
		TerraformHCL:  hclBuilder.String(),
		CSVReport:     csvBuilder.String(),
	}
}
