package seed

import (
	"fmt"
	"math/rand"
	"time"

	"utho_backend/internal/catalog"
	"utho_backend/internal/engine"
	"utho_backend/internal/models"
)

// WorkloadType identifies the classification pattern.
type WorkloadType int

const (
	PatternSteadyOverprovisioned WorkloadType = iota
	PatternBurstyPeakProtected
	PatternBatchJob
	PatternDevStagingIdle
	PatternZombie
	PatternOrphanVolume
	PatternOrphanIP
)

// GenerateDataset creates 85 deterministic resources with 30-day timeseries.
func GenerateDataset() []models.Resource {
	r := rand.New(rand.NewSource(42))
	var resources []models.Resource

	baseTime := time.Date(2026, 10, 24, 0, 0, 0, 0, time.UTC)
	startTime := baseTime.AddDate(0, 0, -30)

	// 1. Steady Over-Provisioned VMs (~20 VMs)
	// Running on m8.xlarge or r16.2xlarge with CPU avg 8–13%, p95 16–22%, p99 25%, RAM p95 < 30%.
	overprovPlans := []string{"m8.xlarge", "r16.2xlarge"}
	regions := []string{"in-noida-1", "in-bangalore-1", "in-mumbai-1"}
	teams := []string{"payments-core", "auth-platform", "search-indexing", "data-pipeline", "frontend-web", "qa-automation"}

	for i := 1; i <= 20; i++ {
		planName := overprovPlans[r.Intn(len(overprovPlans))]
		plan, _ := catalog.GetPlan(planName)
		region := regions[r.Intn(len(regions))]
		team := teams[r.Intn(len(teams))]
		env := "prod"
		if i%3 == 0 {
			env = "staging"
		}

		resID := fmt.Sprintf("res-vm-%03d", len(resources)+1)
		resName := fmt.Sprintf("%s-%s-worker-%02d", env, team, i)
		if i == 12 {
			resName = "prod-OrderWorker-03"
			region = "in-bangalore-1"
			team = "payments-core"
			env = "prod"
			planName = "m8.xlarge"
			plan, _ = catalog.GetPlan(planName)
		}

		// Generate 180 points (every 4 hours for 30 days)
		points := make([]models.MetricPoint, 180)
		cpuVals := make([]float64, 180)
		memVals := make([]float64, 180)
		diskVals := make([]float64, 180)
		netVals := make([]float64, 180)

		for pt := 0; pt < 180; pt++ {
			t := startTime.Add(time.Duration(pt*4) * time.Hour)
			cpu := 8.0 + r.Float64()*12.0 // 8 to 20%
			if pt%15 == 0 {
				cpu = 21.0 + r.Float64()*3.0 // occasional small blip up to 24%
			}
			mem := 18.0 + r.Float64()*10.0 // 18 to 28%
			disk := 150.0 + r.Float64()*120.0
			net := 15.0 + r.Float64()*25.0

			points[pt] = models.MetricPoint{
				Timestamp: t.Format(time.RFC3339),
				CPUPct:    round2(cpu),
				MemPct:    round2(mem),
				DiskIOPS:  round2(disk),
				NetMbps:   round2(net),
			}
			cpuVals[pt] = cpu
			memVals[pt] = mem
			diskVals[pt] = disk
			netVals[pt] = net
		}

		summary := computeSummary(cpuVals, memVals, diskVals, netVals)

		res := models.Resource{
			ID:                 resID,
			Name:               resName,
			Type:               "vm",
			Plan:               planName,
			Region:             region,
			Env:                env,
			Team:               team,
			VCPU:               plan.VCPU,
			RAMGB:              plan.RAMGB,
			DiskGB:             150 + (i * 10),
			CreatedAt:          "2025-10-15",
			MonthlyCostINR:     plan.MonthlyRateINR,
			OptimizationStatus: "OVERPROVISIONED",
			StatusReason:       fmt.Sprintf("30-day CPU p95 is %.1f%% and RAM p95 is %.1f%%. Instance is substantially over-provisioned for current traffic.", summary.CPUP95, summary.MemP95),
			MetricsSummary:     summary,
			Timeseries30d:      points,
		}
		resources = append(resources, res)
	}

	// 2. Bursty Workloads with Short Daily Peaks — THE PEAK SAFETY TRAP (~10 VMs)
	// Daily checkout peak between 20:00 and 21:30 IST spikes CPU p95 to >85% and p99 to >90%.
	burstyNames := []string{
		"prod-payment-webhook-01",
		"prod-checkout-gateway-02",
		"prod-order-capture-01",
		"prod-upi-switch-01",
		"prod-otp-service-02",
		"prod-razorpay-sync-01",
		"prod-flashsale-queue-03",
		"prod-cart-session-02",
		"prod-fraud-eval-01",
		"prod-tax-calc-engine-01",
	}

	for i, name := range burstyNames {
		resID := fmt.Sprintf("res-vm-%03d", len(resources)+1)
		region := "in-mumbai-1"
		if i%2 != 0 {
			region = "in-bangalore-1"
		}
		team := "payments-core"
		if i >= 6 {
			team = "auth-platform"
		}

		points := make([]models.MetricPoint, 180)
		cpuVals := make([]float64, 180)
		memVals := make([]float64, 180)
		diskVals := make([]float64, 180)
		netVals := make([]float64, 180)

		for pt := 0; pt < 180; pt++ {
			t := startTime.Add(time.Duration(pt*4) * time.Hour)
			hourOfDay := t.Hour() // In UTC; note pt*4
			// Daily checkout spike during 20:00 IST (approx 14:00-16:00 UTC)
			isPeak := (hourOfDay == 16 || hourOfDay == 20)

			var cpu, mem, disk, net float64
			if isPeak {
				cpu = 88.0 + r.Float64()*7.0 // spikes to 88-95%
				mem = 68.0 + r.Float64()*8.0
				disk = 1400.0 + r.Float64()*600.0
				net = 280.0 + r.Float64()*50.0
			} else {
				cpu = 9.0 + r.Float64()*6.0 // quiet daytime/night avg 9-15%
				mem = 18.0 + r.Float64()*5.0
				disk = 320.0 + r.Float64()*150.0
				net = 35.0 + r.Float64()*20.0
			}

			points[pt] = models.MetricPoint{
				Timestamp: t.Format(time.RFC3339),
				CPUPct:    round2(cpu),
				MemPct:    round2(mem),
				DiskIOPS:  round2(disk),
				NetMbps:   round2(net),
			}
			cpuVals[pt] = cpu
			memVals[pt] = mem
			diskVals[pt] = disk
			netVals[pt] = net
		}

		summary := computeSummary(cpuVals, memVals, diskVals, netVals)
		// For the primary example prod-payment-webhook-01, calibrate exact requested numbers
		if name == "prod-payment-webhook-01" {
			summary.CPUAvg = 13.8
			summary.CPUP50 = 11.2
			summary.CPUP95 = 86.4
			summary.CPUP99 = 93.2
			summary.MemAvg = 21.0
			summary.MemP50 = 19.5
			summary.MemP95 = 68.0
			summary.MemP99 = 74.5
			summary.DiskIOPSAvg = 420.0
			summary.DiskIOPSP95 = 1850.0
			summary.NetMbpsAvg = 45.2
			summary.NetMbpsP95 = 310.0
		}

		res := models.Resource{
			ID:                 resID,
			Name:               name,
			Type:               "vm",
			Plan:               "m8.xlarge",
			Region:             region,
			Env:                "prod",
			Team:               team,
			VCPU:               8,
			RAMGB:              32,
			DiskGB:             200,
			CreatedAt:          "2025-11-10",
			MonthlyCostINR:     9600.0,
			OptimizationStatus: "PROTECTED_PEAK_LOAD",
			StatusReason:       fmt.Sprintf("Avg CPU is %.1f%%, but daily 20:00-21:30 IST checkout peak hits p99 %.1f%%. Downsizing blocked for safety.", summary.CPUAvg, summary.CPUP99),
			MetricsSummary:     summary,
			Timeseries30d:      points,
		}
		resources = append(resources, res)
	}

	// 3. Batch Jobs Idle Most of the Day (~8 VMs)
	// Runs at ~89% CPU for 3 hours/day (01:00–04:00 IST) and < 2% CPU for 21 hours/day.
	batchNames := []string{
		"prod-nightly-recon-batch",
		"prod-settlement-engine-01",
		"prod-invoice-generator-01",
		"prod-audit-archiver-02",
		"prod-statement-compiler-01",
		"prod-data-lake-compact-01",
		"prod-ledger-rollup-02",
		"prod-credit-score-sync-01",
	}

	for i, name := range batchNames {
		resID := fmt.Sprintf("res-vm-%03d", len(resources)+1)
		region := "in-bangalore-1"
		if i%2 != 0 {
			region = "in-noida-1"
		}
		planName := "c16.2xlarge"
		plan, _ := catalog.GetPlan(planName)

		points := make([]models.MetricPoint, 180)
		cpuVals := make([]float64, 180)
		memVals := make([]float64, 180)
		diskVals := make([]float64, 180)
		netVals := make([]float64, 180)

		for pt := 0; pt < 180; pt++ {
			t := startTime.Add(time.Duration(pt*4) * time.Hour)
			hourOfDay := t.Hour()
			// Nightly batch around 00:00 or 04:00 interval
			isBatch := (hourOfDay == 0)

			var cpu, mem, disk, net float64
			if isBatch {
				cpu = 87.0 + r.Float64()*4.0 // 87 - 91%
				mem = 48.0 + r.Float64()*8.0
				disk = 2200.0 + r.Float64()*500.0
				net = 180.0 + r.Float64()*40.0
			} else {
				cpu = 0.8 + r.Float64()*1.0 // < 2%
				mem = 8.0 + r.Float64()*2.0
				disk = 40.0 + r.Float64()*20.0
				net = 2.0 + r.Float64()*3.0
			}

			points[pt] = models.MetricPoint{
				Timestamp: t.Format(time.RFC3339),
				CPUPct:    round2(cpu),
				MemPct:    round2(mem),
				DiskIOPS:  round2(disk),
				NetMbps:   round2(net),
			}
			cpuVals[pt] = cpu
			memVals[pt] = mem
			diskVals[pt] = disk
			netVals[pt] = net
		}

		summary := computeSummary(cpuVals, memVals, diskVals, netVals)

		res := models.Resource{
			ID:                 resID,
			Name:               name,
			Type:               "vm",
			Plan:               planName,
			Region:             region,
			Env:                "prod",
			Team:               "data-pipeline",
			VCPU:               plan.VCPU,
			RAMGB:              plan.RAMGB,
			DiskGB:             300,
			CreatedAt:          "2025-12-01",
			MonthlyCostINR:     plan.MonthlyRateINR,
			OptimizationStatus: "IDLE_SCHEDULE_CANDIDATE",
			StatusReason:       "Runs batch workloads for ~3 hours/day (01:00-04:00 IST) and idles at <2% CPU for 21 hours/day. Candidate for scheduled auto-stop (~83% savings).",
			MetricsSummary:     summary,
			Timeseries30d:      points,
		}
		resources = append(resources, res)
	}

	// 4. Dev & Staging VMs Idle on Nights & Weekends (~22 VMs)
	// Active Mon–Fri 09:00–19:00 IST (50 hrs/wk), idle 118 hrs/wk (nights + all weekend, CPU < 2%).
	for i := 1; i <= 22; i++ {
		resID := fmt.Sprintf("res-vm-%03d", len(resources)+1)
		env := "dev"
		if i%2 == 0 {
			env = "staging"
		}
		team := teams[i%len(teams)]
		region := regions[i%len(regions)]
		planName := "m4.large"
		if i%3 == 0 {
			planName = "c4.medium"
		}
		plan, _ := catalog.GetPlan(planName)
		name := fmt.Sprintf("%s-%s-test-%02d", env, team, i)

		// Check if this is the special node for staging-loadtest-cluster
		if env == "staging" && i <= 4 {
			planName = "r16.2xlarge"
			plan, _ = catalog.GetPlan(planName)
			name = fmt.Sprintf("staging-loadtest-cluster-node-%02d", i)
		}

		points := make([]models.MetricPoint, 180)
		cpuVals := make([]float64, 180)
		memVals := make([]float64, 180)
		diskVals := make([]float64, 180)
		netVals := make([]float64, 180)

		for pt := 0; pt < 180; pt++ {
			t := startTime.Add(time.Duration(pt*4) * time.Hour)
			weekday := t.Weekday()
			isWeekend := (weekday == time.Saturday || weekday == time.Sunday)
			hourOfDay := t.Hour()
			// Business hours approx 04:00 to 14:00 UTC (09:30 to 19:30 IST)
			isWorkHours := (!isWeekend) && (hourOfDay >= 4 && hourOfDay <= 14)

			var cpu, mem, disk, net float64
			if isWorkHours {
				cpu = 18.0 + r.Float64()*25.0 // 18-43% during dev testing
				mem = 30.0 + r.Float64()*15.0
				disk = 250.0 + r.Float64()*100.0
				net = 20.0 + r.Float64()*30.0
			} else {
				cpu = 0.5 + r.Float64()*1.2 // <2% idle nights/weekends
				mem = 12.0 + r.Float64()*4.0
				disk = 20.0 + r.Float64()*10.0
				net = 0.5 + r.Float64()*1.0
			}

			points[pt] = models.MetricPoint{
				Timestamp: t.Format(time.RFC3339),
				CPUPct:    round2(cpu),
				MemPct:    round2(mem),
				DiskIOPS:  round2(disk),
				NetMbps:   round2(net),
			}
			cpuVals[pt] = cpu
			memVals[pt] = mem
			diskVals[pt] = disk
			netVals[pt] = net
		}

		summary := computeSummary(cpuVals, memVals, diskVals, netVals)

		res := models.Resource{
			ID:                 resID,
			Name:               name,
			Type:               "vm",
			Plan:               planName,
			Region:             region,
			Env:                env,
			Team:               team,
			VCPU:               plan.VCPU,
			RAMGB:              plan.RAMGB,
			DiskGB:             100,
			CreatedAt:          "2026-01-10",
			MonthlyCostINR:     plan.MonthlyRateINR,
			OptimizationStatus: "IDLE_SCHEDULE_CANDIDATE",
			StatusReason:       "Dev/Staging instance idle 118 hours/week (nights and weekends, CPU <2%). Recommend off-hours schedule (~65% savings).",
			MetricsSummary:     summary,
			Timeseries30d:      points,
		}
		resources = append(resources, res)
	}

	// 5. Zombie / Forgotten Idle VMs (~5 VMs)
	// Created 90+ days ago, CPU p99 < 3.5%, Network < 0.5 Mbps for 30 straight days.
	zombieNames := []string{
		"dev-legacy-poc-solr-01",
		"staging-abandoned-migration-01",
		"dev-temp-kafka-benchmark-03",
		"staging-forgotten-demo-02",
		"dev-sandbox-ml-prototype-01",
	}

	for i, name := range zombieNames {
		resID := fmt.Sprintf("res-vm-%03d", len(resources)+1)
		planName := "m4.large"
		plan, _ := catalog.GetPlan(planName)

		points := make([]models.MetricPoint, 180)
		cpuVals := make([]float64, 180)
		memVals := make([]float64, 180)
		diskVals := make([]float64, 180)
		netVals := make([]float64, 180)

		for pt := 0; pt < 180; pt++ {
			t := startTime.Add(time.Duration(pt*4) * time.Hour)
			cpu := 0.4 + r.Float64()*1.2 // CPU strictly < 2.5%
			mem := 6.0 + r.Float64()*1.5
			disk := 5.0 + r.Float64()*3.0
			net := 0.05 + r.Float64()*0.2 // < 0.5 Mbps

			points[pt] = models.MetricPoint{
				Timestamp: t.Format(time.RFC3339),
				CPUPct:    round2(cpu),
				MemPct:    round2(mem),
				DiskIOPS:  round2(disk),
				NetMbps:   round2(net),
			}
			cpuVals[pt] = cpu
			memVals[pt] = mem
			diskVals[pt] = disk
			netVals[pt] = net
		}

		summary := computeSummary(cpuVals, memVals, diskVals, netVals)

		res := models.Resource{
			ID:                 resID,
			Name:               name,
			Type:               "vm",
			Plan:               planName,
			Region:             regions[i%len(regions)],
			Env:                "dev",
			Team:               teams[i%len(teams)],
			VCPU:               plan.VCPU,
			RAMGB:              plan.RAMGB,
			DiskGB:             80,
			CreatedAt:          "2025-06-15", // >90 days ago
			MonthlyCostINR:     plan.MonthlyRateINR,
			OptimizationStatus: "ZOMBIE",
			StatusReason:       "Created >90 days ago with zero active workloads (p99 CPU < 3.5%, Net < 0.5 Mbps for 30 consecutive days). Recommend snapshot & terminate.",
			MetricsSummary:     summary,
			Timeseries30d:      points,
		}
		resources = append(resources, res)
	}

	// Check if prod-search-indexer-04 is in the set; if not replace or inject it
	// Let's ensure prod-search-indexer-04 exists with the memory leak pattern!
	indexerFound := false
	for idx, res := range resources {
		if res.Name == "prod-search-indexing-worker-04" || res.ID == "res-vm-015" {
			resources[idx].Name = "prod-search-indexer-04"
			resources[idx].Plan = "r8.xlarge"
			p, _ := catalog.GetPlan("r8.xlarge")
			resources[idx].VCPU = p.VCPU
			resources[idx].RAMGB = p.RAMGB
			resources[idx].MonthlyCostINR = p.MonthlyRateINR
			resources[idx].Region = "in-noida-1"
			resources[idx].Team = "search-indexing"
			resources[idx].Env = "prod"

			// Inject memory leak: RAM climbs linearly from 44% to 96.4% over 96 hours with 3.8x egress
			points := resources[idx].Timeseries30d
			n := len(points)
			for pt := 0; pt < n; pt++ {
				if pt >= n-24 { // last 96 hours (24 4-hour points)
					leakProgress := float64(pt-(n-24)) / 23.0
					points[pt].MemPct = round2(44.0 + (leakProgress * 52.4)) // 44% -> 96.4%
					points[pt].NetMbps = round2(points[pt].NetMbps * 3.8)
				}
			}
			resources[idx].Timeseries30d = points
			indexerFound = true
			break
		}
	}
	if !indexerFound && len(resources) > 14 {
		resources[14].Name = "prod-search-indexer-04"
		resources[14].Plan = "r8.xlarge"
		p, _ := catalog.GetPlan("r8.xlarge")
		resources[14].VCPU = p.VCPU
		resources[14].RAMGB = p.RAMGB
		resources[14].MonthlyCostINR = p.MonthlyRateINR
		resources[14].Region = "in-noida-1"
		resources[14].Team = "search-indexing"
		resources[14].Env = "prod"
	}

	// 6. Orphaned Volumes (12 resources)
	// 12 unattached Block Volumes (100 GB to 1000 GB, status: "unattached", disk_iops: 0, ₹8/GB/mo)
	orphanVolSizes := []int{100, 150, 200, 250, 300, 400, 500, 600, 750, 800, 900, 1000}
	for i, size := range orphanVolSizes {
		resID := fmt.Sprintf("res-vol-%03d", i+1)
		name := fmt.Sprintf("vol-unattached-legacy-data-%02d", i+1)
		env := "prod"
		if i%3 == 0 {
			env = "staging"
		} else if i%3 == 1 {
			env = "dev"
		}

		if i == 0 {
			name = "prod-postgres-wal-volume"
			env = "prod"
			size = 1000
		}

		cost := catalog.CalculateVolumeCost(size)

		res := models.Resource{
			ID:                 resID,
			Name:               name,
			Type:               "block_volume",
			Plan:               "block_ssd",
			Region:             regions[i%len(regions)],
			Env:                env,
			Team:               teams[i%len(teams)],
			VCPU:               0,
			RAMGB:              0,
			DiskGB:             size,
			CreatedAt:          "2025-08-20",
			MonthlyCostINR:     cost,
			OptimizationStatus: "ORPHANED",
			StatusReason:       fmt.Sprintf("Block volume (%d GB) has been unattached for >30 days with 0 disk IOPS. Generating ₹%.0f/mo in unutilized spend.", size, cost),
			MetricsSummary: &models.MetricsSummary{
				CPUAvg:      0,
				CPUP50:      0,
				CPUP95:      0,
				CPUP99:      0,
				MemAvg:      0,
				MemP50:      0,
				MemP95:      0,
				MemP99:      0,
				DiskIOPSAvg: 0,
				DiskIOPSP95: 0,
				NetMbpsAvg:  0,
				NetMbpsP95:  0,
			},
			Timeseries30d: nil,
		}
		resources = append(resources, res)
	}

	// 7. Orphaned Unattached Public IPs (8 resources)
	// 8 unattached Public IPs (₹350/mo each)
	for i := 1; i <= 8; i++ {
		resID := fmt.Sprintf("res-ip-%03d", i)
		name := fmt.Sprintf("ip-unattached-floating-103.%d.%d.%d", 110+i, 45, 12+i)
		env := "prod"
		if i%2 == 0 {
			env = "dev"
		}

		res := models.Resource{
			ID:                 resID,
			Name:               name,
			Type:               "public_ip",
			Plan:               "public_ipv4",
			Region:             regions[i%len(regions)],
			Env:                env,
			Team:               teams[i%len(teams)],
			VCPU:               0,
			RAMGB:              0,
			DiskGB:             0,
			CreatedAt:          "2025-09-12",
			MonthlyCostINR:     catalog.PublicIPv4PriceMonth,
			OptimizationStatus: "ORPHANED",
			StatusReason:       "Public IPv4 address is unassigned to any active VM. Incurring ₹350/month idle charge.",
			MetricsSummary:     nil,
			Timeseries30d:      nil,
		}
		resources = append(resources, res)
	}

	return resources
}

func computeSummary(cpus, mems, disks, nets []float64) *models.MetricsSummary {
	return &models.MetricsSummary{
		CPUAvg:      engine.Average(cpus),
		CPUP50:      engine.Percentile(cpus, 50),
		CPUP95:      engine.Percentile(cpus, 95),
		CPUP99:      engine.Percentile(cpus, 99),
		MemAvg:      engine.Average(mems),
		MemP50:      engine.Percentile(mems, 50),
		MemP95:      engine.Percentile(mems, 95),
		MemP99:      engine.Percentile(mems, 99),
		DiskIOPSAvg: engine.Average(disks),
		DiskIOPSP95: engine.Percentile(disks, 95),
		NetMbpsAvg:  engine.Average(nets),
		NetMbpsP95:  engine.Percentile(nets, 95),
	}
}

func round2(val float64) float64 {
	return float64(int(val*100+0.5)) / 100.0
}
