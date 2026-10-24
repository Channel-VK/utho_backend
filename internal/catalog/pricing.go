package catalog

import (
	"utho_backend/internal/models"
)

// Storage and IP constants
const (
	BlockSSDPricePerGBMonth = 8.0   // ₹8 / GB / month
	PublicIPv4PriceMonth    = 350.0 // ₹350 / month when unattached
	GridCarbonFactorKgKWh   = 0.71  // CEA India Baseline: 0.71 kg CO2/kWh
	AverageVMWatts          = 45.0  // Approx avg power draw per VM instance in Watts
)

// Plans maps plan name to PlanPricing
var Plans = map[string]models.PlanPricing{
	"c2.small": {
		Name:           "c2.small",
		VCPU:           1,
		RAMGB:          2,
		HourlyRateINR:  1.30,
		MonthlyRateINR: 950.0,
	},
	"c4.medium": {
		Name:           "c4.medium",
		VCPU:           2,
		RAMGB:          4,
		HourlyRateINR:  2.60,
		MonthlyRateINR: 1900.0,
	},
	"m4.large": {
		Name:           "m4.large",
		VCPU:           4,
		RAMGB:          16,
		HourlyRateINR:  6.58,
		MonthlyRateINR: 4800.0,
	},
	"m8.xlarge": {
		Name:           "m8.xlarge",
		VCPU:           8,
		RAMGB:          32,
		HourlyRateINR:  13.15,
		MonthlyRateINR: 9600.0,
	},
	"r8.xlarge": {
		Name:           "r8.xlarge",
		VCPU:           8,
		RAMGB:          64,
		HourlyRateINR:  18.49,
		MonthlyRateINR: 13500.0,
	},
	"c16.2xlarge": {
		Name:           "c16.2xlarge",
		VCPU:           16,
		RAMGB:          32,
		HourlyRateINR:  24.93,
		MonthlyRateINR: 18200.0,
	},
	"r16.2xlarge": {
		Name:           "r16.2xlarge",
		VCPU:           16,
		RAMGB:          64,
		HourlyRateINR:  30.68,
		MonthlyRateINR: 22400.0,
	},
}

// OrderedPlans lists plans in ascending order of monthly cost for sizing evaluation.
var OrderedPlans = []models.PlanPricing{
	Plans["c2.small"],
	Plans["c4.medium"],
	Plans["m4.large"],
	Plans["m8.xlarge"],
	Plans["r8.xlarge"],
	Plans["c16.2xlarge"],
	Plans["r16.2xlarge"],
}

// GetPlan returns pricing details for a given plan name.
func GetPlan(name string) (models.PlanPricing, bool) {
	plan, exists := Plans[name]
	return plan, exists
}

// CalculateComputeCost returns the monthly cost for a VM plan.
func CalculateComputeCost(planName string) float64 {
	if p, ok := Plans[planName]; ok {
		return p.MonthlyRateINR
	}
	return 0.0
}

// CalculateVolumeCost returns the monthly cost for a block volume.
func CalculateVolumeCost(sizeGB int) float64 {
	return float64(sizeGB) * BlockSSDPricePerGBMonth
}

// CalculateIPCost returns the monthly cost for unattached public IP.
func CalculateIPCost(unattached bool) float64 {
	if unattached {
		return PublicIPv4PriceMonth
	}
	return 0.0
}
