package engine

import (
	"math"
	"sort"
)

// Average computes the arithmetic mean of a float slice.
func Average(vals []float64) float64 {
	if len(vals) == 0 {
		return 0.0
	}
	var sum float64
	for _, v := range vals {
		sum += v
	}
	return round2(sum / float64(len(vals)))
}

// Percentile computes the p-th percentile (0 <= p <= 100) using linear interpolation.
func Percentile(vals []float64, p float64) float64 {
	n := len(vals)
	if n == 0 {
		return 0.0
	}
	if n == 1 {
		return round2(vals[0])
	}

	// Copy and sort
	sorted := make([]float64, n)
	copy(sorted, vals)
	sort.Float64s(sorted)

	if p <= 0 {
		return round2(sorted[0])
	}
	if p >= 100 {
		return round2(sorted[n-1])
	}

	// Rank calculation
	rank := (p / 100.0) * float64(n-1)
	lowerIndex := int(math.Floor(rank))
	upperIndex := int(math.Ceil(rank))
	weight := rank - float64(lowerIndex)

	val := sorted[lowerIndex]*(1.0-weight) + sorted[upperIndex]*weight
	return round2(val)
}

// LinearSlope calculates the slope (rate of change per unit index) using ordinary least squares.
func LinearSlope(vals []float64) float64 {
	n := float64(len(vals))
	if n < 2 {
		return 0.0
	}

	var sumX, sumY, sumXY, sumXSquare float64
	for i, y := range vals {
		x := float64(i)
		sumX += x
		sumY += y
		sumXY += x * y
		sumXSquare += x * x
	}

	denominator := (n * sumXSquare) - (sumX * sumX)
	if math.Abs(denominator) < 1e-9 {
		return 0.0
	}

	slope := ((n * sumXY) - (sumX * sumY)) / denominator
	return round4(slope)
}

// StdDev calculates standard deviation of a slice.
func StdDev(vals []float64, mean float64) float64 {
	n := float64(len(vals))
	if n < 2 {
		return 0.0
	}
	var sumSquares float64
	for _, v := range vals {
		diff := v - mean
		sumSquares += diff * diff
	}
	return math.Sqrt(sumSquares / (n - 1))
}

// ZScore computes the standard score of value x.
func ZScore(x, mean, stdDev float64) float64 {
	if stdDev <= 1e-9 {
		return 0.0
	}
	return round2((x - mean) / stdDev)
}

func round2(val float64) float64 {
	return math.Round(val*100) / 100
}

func round4(val float64) float64 {
	return math.Round(val*10000) / 10000
}
