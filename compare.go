package snclient

import (
	"strconv"
	"strings"
)

func CompareMetrics(metrics map[string]string, threshold *Threshold) bool {
	if threshold == nil {
		return false
	}

	for key, val := range metrics {
		if key != threshold.name {
			continue
		}
		switch threshold.operator {
		case "<":
			m, _ := strconv.ParseFloat(val, 64)
			t, _ := strconv.ParseFloat(threshold.value, 64)

			return m < t
		case ">":
			m, _ := strconv.ParseFloat(val, 64)
			t, _ := strconv.ParseFloat(threshold.value, 64)

			return m > t
		case "<=":
			m, _ := strconv.ParseFloat(val, 64)
			t, _ := strconv.ParseFloat(threshold.value, 64)

			return m <= t
		case ">=":
			m, _ := strconv.ParseFloat(val, 64)
			t, _ := strconv.ParseFloat(threshold.value, 64)

			return m >= t
		case "=", "is":
			return val == threshold.value
		case "!=", "not", "is not":
			return val != threshold.value
		case "like":
			return strings.Contains(val, threshold.value)
		case "not like":
			return !strings.Contains(val, threshold.value)
		}
	}

	return false
}
