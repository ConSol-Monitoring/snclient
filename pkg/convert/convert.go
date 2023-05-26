package convert

import (
	"fmt"
	"strconv"
	"strings"
)

// Float64 converts anything into a float64
// errors will fall back to 0
func Float64(raw interface{}) float64 {
	val, _ := Float64E(raw)

	return val
}

// Float64E converts anything into a float64
// errors will be returned
func Float64E(raw interface{}) (float64, error) {
	switch val := raw.(type) {
	case float64:
		return val, nil
	case int64:
		return float64(val), nil
	default:
		num, err := strconv.ParseFloat(fmt.Sprintf("%v", val), 64)
		if err != nil {
			return 0, fmt.Errorf("cannot parse float64 value from %v (%T)", raw, raw)
		}

		return num, nil
	}
}

// Bool converts anything into a bool
// errors will fall back to false
func Bool(raw interface{}) bool {
	b, _ := BoolE(raw)

	return b
}

// BoolE converts anything into a bool
// errors will be returned
func BoolE(raw interface{}) (bool, error) {
	switch val := raw.(type) {
	case bool:
		return val, nil
	default:
		switch strings.ToLower(fmt.Sprintf("%v", raw)) {
		case "1", "enabled", "true", "yes":
			return true, nil
		case "0", "disabled", "false", "no":
			return false, nil
		}
	}

	return false, fmt.Errorf("cannot parse boolean value from %v (%T)", raw, raw)
}

// Num2String converts any number into a string
// errors will fall back to empty string
func Num2String(raw interface{}) string {
	s, _ := Num2StringE(raw)

	return s
}

// Num2StringE converts any number into a string
// errors will be returned
func Num2StringE(raw interface{}) (string, error) {
	switch num := raw.(type) {
	case float64:
		if strconv.FormatFloat(num, 'f', -1, 64) != fmt.Sprintf("%d", int64(num)) {
			return strconv.FormatFloat(num, 'f', -1, 64), nil
		}

		return fmt.Sprintf("%d", int64(num)), nil
	case int64:
		return fmt.Sprintf("%d", num), nil
	default:
		fNum, err := strconv.ParseFloat(fmt.Sprintf("%v", raw), 64)
		if err != nil {
			return "", fmt.Errorf("cannot convert %v (%T) into string", raw, raw)
		}

		return Num2StringE(fNum)
	}
}
