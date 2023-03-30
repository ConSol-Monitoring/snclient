package snclient

import (
	"strconv"
)

var bUnits = map[string]float64{"KB": 1e3, "MB": 1e6, "GB": 1e9, "TB": 1e12, "PB": 1e15}

type MetricData struct {
	name  string
	value string
}

func CompareMetrics(metrics []MetricData, treshold Treshold) bool {
	for _, data := range metrics {
		if data.name != treshold.name {
			continue
		}

		if treshold.unit != "" && treshold.unit != "%" {
			value, _ := strconv.ParseFloat(treshold.value, 64)
			treshold.value = strconv.FormatFloat(value*bUnits[treshold.unit], 'f', 0, 64)
		}

		switch treshold.operator {
		case "<":
			m, _ := strconv.ParseFloat(data.value, 64)
			t, _ := strconv.ParseFloat(treshold.value, 64)

			return m < t
		case ">":
			m, _ := strconv.ParseFloat(data.value, 64)
			t, _ := strconv.ParseFloat(treshold.value, 64)

			return m > t
		case "<=":
			m, _ := strconv.ParseFloat(data.value, 64)
			t, _ := strconv.ParseFloat(treshold.value, 64)

			return m <= t
		case ">=":
			m, _ := strconv.ParseFloat(data.value, 64)
			t, _ := strconv.ParseFloat(treshold.value, 64)

			return m >= t
		case "=", "is":
			return data.value == treshold.value
		case "!=", "not":
			return data.value != treshold.value
		}
	}

	return false
}
