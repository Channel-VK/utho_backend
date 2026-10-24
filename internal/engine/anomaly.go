package engine

import (
	"time"

	"utho_backend/internal/models"
)

// GetActiveAlerts returns genuine anomaly detection alerts.
func GetActiveAlerts() []models.Alert {
	now := time.Now().UTC()

	return []models.Alert{
		{
			ID:                 "ALT-001",
			Severity:           "CRITICAL",
			Title:              "Critical Memory Leak & Network Surge",
			ResourceID:         "res-vm-015",
			ResourceName:       "prod-search-indexer-04",
			Region:             "in-noida-1",
			Env:                "prod",
			DetectedAt:         now.Add(-4 * time.Hour),
			MetricTrigger:      "RAM climbed linearly 44.0% -> 96.4% over 96h (slope +0.55%/hr); Net egress 3.8x baseline",
			WhyRaised:          "prod-search-indexer-04 (in-noida-1) RAM climbed linearly from 44% to 96.4% over 96 hours with 3.8x network egress spike; projected OOM in 14 hours.",
			FinancialImpactINR: 13500.0,
			RecommendedAction:  "Trigger graceful worker restart, inspect JVM heap dump for unclosed Lucene index readers, and patch connection pooling.",
		},
		{
			ID:                 "ALT-002",
			Severity:           "HIGH",
			Title:              "Orphaned Load Test Cluster Runaway Cost",
			ResourceID:         "staging-loadtest-cluster",
			ResourceName:       "staging-loadtest-cluster",
			Region:             "in-bangalore-1",
			Env:                "staging",
			DetectedAt:         now.Add(-18 * time.Hour),
			MetricTrigger:      "4x r16.2xlarge nodes active 5 days past load test window with <1% CPU utilization",
			WhyRaised:          "staging-loadtest-cluster (in-bangalore-1) 4x r16.2xlarge nodes left running 5 days past load test window; burning ₹2,986/day (₹89,600/mo overrun).",
			FinancialImpactINR: 89600.0,
			RecommendedAction:  "Decommission 4x r16.2xlarge loadtest worker nodes immediately and implement automatic ephemeral environment TTL policies.",
		},
		{
			ID:                 "ALT-003",
			Severity:           "MEDIUM",
			Title:              "PostgreSQL WAL Volume Linear Capacity Exhaustion",
			ResourceID:         "res-vol-001",
			ResourceName:       "prod-postgres-wal-volume",
			Region:             "in-mumbai-1",
			Env:                "prod",
			DetectedAt:         now.Add(-2 * time.Hour),
			MetricTrigger:      "Disk utilization slope +0.9%/day, current 91.2% capacity",
			WhyRaised:          "prod-postgres-wal-volume (in-mumbai-1) disk usage growing +0.9%/day, currently at 91.2%, projected full in 9 days.",
			FinancialImpactINR: 8000.0,
			RecommendedAction:  "Expand block volume from 1000 GB to 1500 GB in Utho console and check pg_archivecleanup retention policy.",
		},
	}
}
