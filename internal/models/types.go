package models

import "time"

// MetricPoint represents a single metric observation in time.
type MetricPoint struct {
	Timestamp string  `json:"timestamp"`
	CPUPct    float64 `json:"cpu_pct"`
	MemPct    float64 `json:"mem_pct"`
	DiskIOPS  float64 `json:"disk_iops"`
	NetMbps   float64 `json:"net_mbps"`
}

// MetricsSummary contains aggregated percentile and average statistics.
type MetricsSummary struct {
	CPUAvg      float64 `json:"cpu_avg"`
	CPUP50      float64 `json:"cpu_p50"`
	CPUP95      float64 `json:"cpu_p95"`
	CPUP99      float64 `json:"cpu_p99"`
	MemAvg      float64 `json:"mem_avg"`
	MemP50      float64 `json:"mem_p50"`
	MemP95      float64 `json:"mem_p95"`
	MemP99      float64 `json:"mem_p99"`
	DiskIOPSAvg float64 `json:"disk_iops_avg"`
	DiskIOPSP95 float64 `json:"disk_iops_p95"`
	NetMbpsAvg  float64 `json:"net_mbps_avg"`
	NetMbpsP95  float64 `json:"net_mbps_p95"`
}

// Resource represents any cloud resource (VM, Block Volume, Public IP).
type Resource struct {
	ID                 string          `json:"id"`
	Name               string          `json:"name"`
	Type               string          `json:"type"` // "vm", "block_volume", "public_ip"
	Plan               string          `json:"plan"` // e.g. "m8.xlarge", "block_ssd", "public_ipv4"
	Region             string          `json:"region"`
	Env                string          `json:"env"`  // "prod", "staging", "dev"
	Team               string          `json:"team"` // "payments-core", etc.
	VCPU               int             `json:"vcpu"`
	RAMGB              int             `json:"ram_gb"`
	DiskGB             int             `json:"disk_gb"`
	CreatedAt          string          `json:"created_at"`
	MonthlyCostINR     float64         `json:"monthly_cost_inr"`
	OptimizationStatus string          `json:"optimization_status"` // "OPTIMAL", "OVERPROVISIONED", "PROTECTED_PEAK_LOAD", "IDLE_SCHEDULE_CANDIDATE", "ZOMBIE", "ORPHANED"
	StatusReason       string          `json:"status_reason"`
	MetricsSummary     *MetricsSummary `json:"metrics_summary,omitempty"`
	Timeseries30d      []MetricPoint   `json:"timeseries_30d,omitempty"`
}

// Recommendation defines a rightsizing or cost optimization action.
type Recommendation struct {
	ID                  string  `json:"id"`
	ResourceID          string  `json:"resource_id"`
	ResourceName        string  `json:"resource_name"`
	ResourceType        string  `json:"resource_type"`
	Region              string  `json:"region"`
	Env                 string  `json:"env"`
	Team                string  `json:"team"`
	ActionType          string  `json:"action_type"` // "RIGHTSIZE_VM", "SCHEDULE_OFF_HOURS", "TERMINATE_ZOMBIE", "CLEANUP_ORPHAN"
	CurrentConfig       string  `json:"current_config"`
	RecommendedConfig   string  `json:"recommended_config"`
	CurrentMonthlyINR   float64 `json:"current_monthly_inr"`
	ProjectedMonthlyINR float64 `json:"projected_monthly_inr"`
	MonthlySavingsINR   float64 `json:"monthly_savings_inr"`
	FormulaExplanation  string  `json:"formula_explanation"`
	Reason              string  `json:"reason"`
	RiskLevel           string  `json:"risk_level"` // "LOW", "MEDIUM", "HIGH"
	ConfidenceScore     int     `json:"confidence_score"`
	Status              string  `json:"status"` // "pending", "accepted", "rejected"
}

// SummaryBreakdown represents aggregation by category.
type SummaryBreakdown struct {
	Key                 string  `json:"key"`
	CurrentMonthlyINR   float64 `json:"current_monthly_inr"`
	ProjectedMonthlyINR float64 `json:"projected_monthly_inr"`
	SavingsINR          float64 `json:"savings_inr"`
	Count               int     `json:"count"`
}

// RecommendationSummary aggregates metrics over all recommendations.
type RecommendationSummary struct {
	CurrentMonthlyINR   float64            `json:"current_monthly_inr"`
	ProjectedMonthlyINR float64            `json:"projected_monthly_inr"`
	AcceptedSavingsINR  float64            `json:"accepted_savings_inr"`
	AcceptedSavingsPct  float64            `json:"accepted_savings_pct"`
	AcceptedCount       int                `json:"accepted_count"`
	RejectedCount       int                `json:"rejected_count"`
	PendingCount        int                `json:"pending_count"`
	ByEnv               []SummaryBreakdown `json:"by_env"`
	ByAction            []SummaryBreakdown `json:"by_action"`
}

// RecommendationsResponse encapsulates the recommendations endpoint return structure.
type RecommendationsResponse struct {
	Summary RecommendationSummary `json:"summary"`
	Items   []Recommendation      `json:"items"`
}

// Sustainability contains environmental impact metrics.
type Sustainability struct {
	MonthlyKWhSaved   float64 `json:"monthly_kwh_saved"`
	MonthlyCO2KgSaved float64 `json:"monthly_co2_kg_saved"`
	TreesEquivalent   float64 `json:"trees_equivalent"`
}

// HistoricalSpend holds monthly billing breakdown.
type HistoricalSpend struct {
	Month      string  `json:"month"`
	ComputeINR float64 `json:"compute_inr"`
	StorageINR float64 `json:"storage_inr"`
	NetworkINR float64 `json:"network_inr"`
	WasteINR   float64 `json:"waste_inr"`
	TotalINR   float64 `json:"total_inr"`
}

// ForecastPoint holds forecast billing projections.
type ForecastPoint struct {
	Date         string  `json:"date"`
	DoNothingINR float64 `json:"do_nothing_inr"`
	OptimizedINR float64 `json:"optimized_inr"`
}

// SpendBreakdown holds spend aggregated by environment or region.
type SpendBreakdown struct {
	Env          string  `json:"env,omitempty"`
	Region       string  `json:"region,omitempty"`
	CurrentINR   float64 `json:"current_inr"`
	ProjectedINR float64 `json:"projected_inr"`
	SavingsINR   float64 `json:"savings_inr"`
}

// DashboardData is the complete response for GET /api/v1/dashboard.
type DashboardData struct {
	CurrentMonthlySpendINR    float64           `json:"current_monthly_spend_inr"`
	ProjectedMonthlySpendINR  float64           `json:"projected_monthly_spend_inr"`
	AcceptedMonthlySavingsINR float64           `json:"accepted_monthly_savings_inr"`
	AcceptedSavingsPct        float64           `json:"accepted_savings_pct"`
	TotalPotentialSavingsINR  float64           `json:"total_potential_savings_inr"`
	BillGrowth6mPct           float64           `json:"bill_growth_6m_pct"`
	ProtectedPeakVMsCount     int               `json:"protected_peak_vms_count"`
	ActiveAlertsCount         int               `json:"active_alerts_count"`
	TotalResourcesCount       int               `json:"total_resources_count"`
	AIEngineConnected         bool              `json:"ai_engine_connected"`
	AIEngineStatus            string            `json:"ai_engine_status"`
	AIFeatures                []string          `json:"ai_features,omitempty"`
	Sustainability            Sustainability    `json:"sustainability"`
	HistoricalSpend6m         []HistoricalSpend `json:"historical_spend_6m"`
	Forecast60d               []ForecastPoint   `json:"forecast_60d"`
	SpendByEnv                []SpendBreakdown  `json:"spend_by_env"`
	SpendByRegion             []SpendBreakdown  `json:"spend_by_region"`
}

// ServiceStatusResponse is returned by GET /api/v1/status.
type ServiceStatusResponse struct {
	GoEngineConnected bool     `json:"go_engine_connected"`
	GoEnginePort      int      `json:"go_engine_port"`
	AIEngineConnected bool     `json:"ai_engine_connected"`
	AIEnginePort      int      `json:"ai_engine_port"`
	AIEngineService   string   `json:"ai_engine_service"`
	AIFeatures        []string `json:"ai_features"`
}

// Alert represents an anomaly or capacity hazard, enriched with Python AI root-cause & TTF analysis.
type Alert struct {
	ID                   string    `json:"id"`
	Severity             string    `json:"severity"` // "CRITICAL", "HIGH", "MEDIUM"
	Title                string    `json:"title"`
	ResourceID           string    `json:"resource_id"`
	ResourceName         string    `json:"resource_name"`
	Region               string    `json:"region"`
	Env                  string    `json:"env"`
	DetectedAt           time.Time `json:"detected_at"`
	MetricTrigger        string    `json:"metric_trigger"`
	WhyRaised            string    `json:"why_raised"`
	FinancialImpactINR   float64   `json:"financial_impact_inr"`
	RecommendedAction    string    `json:"recommended_action"`
	AIProvider           string    `json:"ai_provider,omitempty"`
	RootCauseCorrelation string    `json:"root_cause_correlation,omitempty"`
	TimeToFailureHours   float64   `json:"time_to_failure_hours,omitempty"`
	TTFFormula           string    `json:"ttf_formula,omitempty"`
	SlopePctPerDay       float64   `json:"slope_pct_per_day,omitempty"`
	StatisticalZScore    float64   `json:"statistical_z_score,omitempty"`
	BlastRadius          string    `json:"blast_radius,omitempty"`
	MitigationCommand    string    `json:"mitigation_command,omitempty"`
}

// SimulationRequest is the payload for POST /api/v1/simulate.
type SimulationRequest struct {
	DevOffHoursEnabled         bool    `json:"dev_off_hours_enabled"`
	DevShutdownHoursPerWeek    int     `json:"dev_shutdown_hours_per_week"`
	StagingWeekendOffEnabled   bool    `json:"staging_weekend_off_enabled"`
	SafetyHeadroomBufferPct    float64 `json:"safety_headroom_buffer_pct"`
	PurgeOrphanedOlderThanDays int     `json:"purge_orphaned_older_than_days"`
}

// EligibleChange details changes in recommendation eligibility.
type EligibleChange struct {
	ResourceID   string  `json:"resource_id"`
	ResourceName string  `json:"resource_name"`
	CurrentPlan  string  `json:"current_plan"`
	ProposedPlan string  `json:"proposed_plan"`
	P95Peak      float64 `json:"p95_peak"`
	P99Peak      float64 `json:"p99_peak"`
	Reason       string  `json:"reason"`
}

// SimulationBreakdown holds categorized savings in INR.
type SimulationBreakdown struct {
	DevSchedulingSavingsINR  float64 `json:"dev_scheduling_savings_inr"`
	StagingWeekendSavingsINR float64 `json:"staging_weekend_savings_inr"`
	RightsizingSavingsINR    float64 `json:"rightsizing_savings_inr"`
	OrphanPurgeSavingsINR    float64 `json:"orphan_purge_savings_inr"`
}

// SimulationResult is returned from POST /api/v1/simulate (includes both Go & React field aliases).
type SimulationResult struct {
	BaselineMonthlyINR         float64             `json:"baseline_monthly_inr"`
	SimulatedMonthlyINR        float64             `json:"simulated_monthly_inr"`
	SimulatedSavingsINR        float64             `json:"simulated_savings_inr"`
	SavingsPct                 float64             `json:"savings_pct"`
	AffectedVMsCount           int                 `json:"affected_vms_count"`
	SimulatedMonthlySpendINR   float64             `json:"simulated_monthly_spend_inr"`
	SimulatedMonthlySavingsINR float64             `json:"simulated_monthly_savings_inr"`
	SimulatedSavingsPct        float64             `json:"simulated_savings_pct"`
	EligibleVMsCount           int                 `json:"eligible_vms_count"`
	BlockedByHeadroomCount     int                 `json:"blocked_by_headroom_count"`
	CarbonSavedKg              float64             `json:"carbon_saved_kg"`
	RiskSummary                string              `json:"risk_summary"`
	Breakdown                  SimulationBreakdown `json:"breakdown"`
	NewlyEligibleResources     []EligibleChange    `json:"newly_eligible_resources"`
	NewlyIneligibleResources   []EligibleChange    `json:"newly_ineligible_resources"`
	Sustainability             Sustainability      `json:"sustainability"`
}

// AssumptionsData encapsulates catalog, formulas, and parameters for GET /api/v1/assumptions.
type AssumptionsData struct {
	PricingCatalog       map[string]PlanPricing `json:"pricing_catalog"`
	SafetyFormulas       map[string]string      `json:"safety_formulas"`
	OffHoursScheduleMath map[string]string      `json:"off_hours_schedule_math"`
	GridCarbonFactor     float64                `json:"grid_carbon_factor_kg_co2_per_kwh"`
}

// PlanPricing represents pricing details for a single plan.
type PlanPricing struct {
	Name           string  `json:"name"`
	VCPU           int     `json:"vcpu"`
	RAMGB          int     `json:"ram_gb"`
	HourlyRateINR  float64 `json:"hourly_rate_inr"`
	MonthlyRateINR float64 `json:"monthly_rate_inr"`
}

// RemediationExport holds generated executable scripts and reports.
type RemediationExport struct {
	GeneratedAt   string `json:"generated_at"`
	AcceptedCount int    `json:"accepted_count"`
	UthoCLIScript string `json:"utho_cli_script"`
	TerraformHCL  string `json:"terraform_hcl"`
	CSVReport     string `json:"csv_report"`
}

// JevAssessment represents TypeSafe Jev ordinal risk scoring from Python AI (:8001).
type JevAssessment struct {
	Model               string             `json:"model"`
	RiskScore           float64            `json:"risk_score"`
	RiskScorePercentage float64            `json:"risk_score_percentage"`
	RiskLevel           string             `json:"risk_level"`
	ModelConfidence     float64            `json:"model_confidence"`
	Probabilities       map[string]float64 `json:"probabilities"`
	Explanation         string             `json:"explanation"`
	DecisionPolicy      string             `json:"decision_policy"`
}

// QuantileForecastPoint represents a 4-hour q10/q50/q90 forecast step.
type QuantileForecastPoint struct {
	Timestamp string  `json:"timestamp"`
	HourLabel string  `json:"hour_label"`
	CPUQ10    float64 `json:"cpu_q10"`
	CPUQ50    float64 `json:"cpu_q50"`
	CPUQ90    float64 `json:"cpu_q90"`
	MemQ10    float64 `json:"mem_q10"`
	MemQ50    float64 `json:"mem_q50"`
	MemQ90    float64 `json:"mem_q90"`
}

// ResourceGreenOps represents per-resource PUE/wattage sustainability metrics.
type ResourceGreenOps struct {
	EstimatedEnergySavedKWh    float64 `json:"estimated_energy_saved_kwh"`
	EstimatedCarbonSavedKgCO2e float64 `json:"estimated_carbon_saved_kg_co2e"`
	SustainabilityScore        float64 `json:"sustainability_score"`
	PUE                        float64 `json:"pue"`
	WattsPerVCPURange          string  `json:"watts_per_vcpu_range"`
}

// EngineeringAnalysis represents the deep root-cause explanation & AI audit for a resource.
type EngineeringAnalysis struct {
	ResourceID             string                  `json:"resource_id"`
	Provider               string                  `json:"provider"` // "Python-AI-Service (:8001)" or "Deterministic-Rule-Engine"
	PeakHeadroomProof      string                  `json:"peak_headroom_proof"`
	SafetyBufferAnalysis   string                  `json:"safety_buffer_analysis"`
	WorkloadClassification string                  `json:"workload_classification"`
	DownsizingSafe         bool                    `json:"downsizing_safe"`
	RiskScore              int                     `json:"risk_score"`
	ConfidenceLevel        string                  `json:"confidence_level,omitempty"`
	ConfidenceReason       string                  `json:"confidence_reason,omitempty"`
	TechnicalExplanation   string                  `json:"technical_explanation"`
	BlastRadiusAnalysis    string                  `json:"blast_radius_analysis"`
	ActionableNextSteps    []string                `json:"actionable_next_steps"`
	RollbackRunbook        []string                `json:"rollback_runbook,omitempty"`
	JevAssessment          *JevAssessment          `json:"jev_assessment,omitempty"`
	QuantileForecast24h    []QuantileForecastPoint `json:"quantile_forecast_24h,omitempty"`
	TerraformHCL           string                  `json:"terraform_hcl,omitempty"`
	GreenOpsImpact         *ResourceGreenOps       `json:"greenops_impact,omitempty"`
}

// ResourceDetailResponse is returned by GET /api/v1/resources/{id}.
type ResourceDetailResponse struct {
	Resource
	Timeseries30d       []MetricPoint        `json:"timeseries_30d"`
	Recommendation      *Recommendation      `json:"recommendation"`
	EngineeringAnalysis *EngineeringAnalysis `json:"engineering_analysis"`
}

// UpdateRecommendationRequest is the payload for PATCH /api/v1/recommendations/{id}.
type UpdateRecommendationRequest struct {
	Status string `json:"status"` // "accepted", "rejected", "pending"
}

// BulkUpdateRequest is the payload for POST /api/v1/recommendations/bulk.
type BulkUpdateRequest struct {
	Status     string `json:"status"`      // "accepted", "rejected"
	FilterRisk string `json:"filter_risk"` // "ALL", "LOW_ONLY"
}

// NLScenarioRequest is the payload for POST /api/v1/ai/parse-scenario.
type NLScenarioRequest struct {
	Query string `json:"query"`
}

// NLScenarioScope describes detected region/env/window from natural language.
type NLScenarioScope struct {
	Environment   string `json:"environment"`
	Region        string `json:"region"`
	ShutdownStart string `json:"shutdown_start"`
	ShutdownEnd   string `json:"shutdown_end"`
}

// NLScenarioResponse is returned by POST /api/v1/ai/parse-scenario.
type NLScenarioResponse struct {
	Query            string            `json:"query"`
	Provider         string            `json:"provider"`
	ParsedParams     SimulationRequest `json:"parsed_params"`
	DetectedScope    NLScenarioScope   `json:"detected_scope"`
	ParsedTags       []string          `json:"parsed_tags"`
	Explanation      string            `json:"explanation"`
	SimulationResult SimulationResult  `json:"simulation_result"`
}

// AIForecastRequest is the payload for POST /api/v1/ai/forecast.
type AIForecastRequest struct {
	Days     int     `json:"days"`
	Headroom float64 `json:"headroom"`
}

// ComponentCostBreakdown holds component-level INR spend.
type ComponentCostBreakdown struct {
	VMCompute float64 `json:"vm_compute"`
	Storage   float64 `json:"storage"`
	PublicIP  float64 `json:"public_ip"`
	Total     float64 `json:"total"`
}

// FleetQuantileSummary holds fleet-wide q10/q50/q90 utilization bands.
type FleetQuantileSummary struct {
	CPUQ10Pct float64 `json:"cpu_q10_pct"`
	CPUQ50Pct float64 `json:"cpu_q50_pct"`
	CPUQ90Pct float64 `json:"cpu_q90_pct"`
	MemQ10Pct float64 `json:"mem_q10_pct"`
	MemQ50Pct float64 `json:"mem_q50_pct"`
	MemQ90Pct float64 `json:"mem_q90_pct"`
}

// AIForecastSustainability holds PUE and wattage sustainability details.
type AIForecastSustainability struct {
	EstimatedEnergySavedKWh    float64  `json:"estimated_energy_saved_kwh"`
	EstimatedCarbonSavedKgCO2e float64  `json:"estimated_carbon_saved_kg_co2e"`
	SustainabilityScore        float64  `json:"sustainability_score"`
	PUE                        float64  `json:"pue"`
	GridCarbonFactorKgPerKWh   float64  `json:"grid_carbon_factor_kg_per_kwh"`
	Assumptions                []string `json:"assumptions"`
}

// AIForecastResponse is returned by GET/POST /api/v1/ai/forecast.
type AIForecastResponse struct {
	Days                 int                      `json:"days"`
	Currency             string                   `json:"currency"`
	Provider             string                   `json:"provider"`
	HeadroomFactor       float64                  `json:"headroom_factor"`
	ForecastBasis        string                   `json:"forecast_basis"`
	CurrentCostINR       ComponentCostBreakdown   `json:"current_cost_inr"`
	RecommendedCostINR   ComponentCostBreakdown   `json:"recommended_cost_inr"`
	EstimatedSavingsINR  float64                  `json:"estimated_savings_inr"`
	SavingsPct           float64                  `json:"savings_pct"`
	FleetQuantileSummary FleetQuantileSummary     `json:"fleet_quantile_summary"`
	Sustainability       AIForecastSustainability `json:"sustainability"`
}

// AIBriefingRequest is the payload for POST /api/v1/ai/briefing.
type AIBriefingRequest struct {
	Role string `json:"role"` // "executive", "finance", "engineering"
}

// BriefingMetric represents a key-value pill in a role briefing.
type BriefingMetric struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// AIBriefingResponse is returned by POST /api/v1/ai/briefing.
type AIBriefingResponse struct {
	Role         string           `json:"role"`
	Provider     string           `json:"provider"`
	Model        string           `json:"model"`
	Headline     string           `json:"headline"`
	Summary      string           `json:"summary"`
	BulletPoints []string         `json:"bullet_points"`
	KeyMetrics   []BriefingMetric `json:"key_metrics"`
}
