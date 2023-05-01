package snclient

import (
	"fmt"
	"regexp"
	"strconv"
)

var (
	bUnits = map[string]float64{"KB": 1e3, "MB": 1e6, "GB": 1e9, "TB": 1e12, "PB": 1e15}
	tUnits = map[string]float64{"m": 60, "h": 3600, "d": 86400}
)

func CompareMetrics(metrics map[string]string, threshold Threshold) bool {
	for key, val := range metrics {
		if key != threshold.name {
			continue
		}

		switch threshold.unit {
		case "KB", "MB", "GB", "TB", "PB":
			value, _ := strconv.ParseFloat(threshold.value, 64)
			threshold.value = strconv.FormatFloat(value*bUnits[threshold.unit], 'f', 0, 64)
		case "m", "h", "d":
			value, _ := strconv.ParseFloat(threshold.value, 64)
			threshold.value = strconv.FormatFloat(value*tUnits[threshold.unit], 'f', 0, 64)
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
		case "!=", "not":
			return val != threshold.value
		case "like":
			res, _ := regexp.MatchString(fmt.Sprintf(".*%s.*", regexp.QuoteMeta(threshold.value)), val)

			return res
		case "not like":
			res, _ := regexp.MatchString(fmt.Sprintf(".*%s.*", regexp.QuoteMeta(threshold.value)), val)

			return !res
		}
	}

	return false
}
