package convert

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// Float64 converts anything into a float64
// errors will fall back to 0
func Float64(raw any) float64 {
	val, _ := Float64E(raw)

	return val
}

// Float64E converts anything into a float64
// errors will be returned
func Float64E(raw any) (float64, error) {
	switch val := raw.(type) {
	case float64:
		return val, nil
	case float32:
		return float64(val), nil
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

// Int64 converts anything into a int64
// errors will fall back to 0
func Int64(raw any) int64 {
	val, _ := Int64E(raw)

	return val
}

// Int64E converts anything into a int64
// errors will be returned
func Int64E(raw any) (int64, error) {
	switch val := raw.(type) {
	case int64:
		return val, nil
	case int8:
		return int64(val), nil
	case int16:
		return int64(val), nil
	case int32:
		return int64(val), nil
	case int:
		return int64(val), nil
	case uint8:
		return int64(val), nil
	case uint16:
		return int64(val), nil
	case uint32:
		return int64(val), nil
	case float64:
		return int64(val), nil
	default:
		num, err := Float64E(raw)
		if err != nil {
			return 0, fmt.Errorf("cannot parse int64 value from %v (%T)", raw, raw)
		}

		return int64(num), nil
	}
}

// UInt64 converts anything into a uint64
// errors will fall back to 0
func UInt64(raw any) uint64 {
	val, _ := UInt64E(raw)

	return val
}

// UInt64E converts anything into a uint64
// errors will be returned
func UInt64E(raw any) (uint64, error) {
	switch val := raw.(type) {
	case uint64:
		return val, nil
	default:
		num, err := Float64E(raw)
		if err != nil {
			return 0, fmt.Errorf("cannot parse uint64 value from %v (%T)", raw, raw)
		}

		if num < 0 {
			return 0, fmt.Errorf("number to small for uint64")
		}

		return uint64(num), nil
	}
}

// Int converts anything into a int
// errors will fall back to 0
func Int(raw any) int {
	val, _ := IntE(raw)

	return val
}

// IntE converts anything into a int
// errors will be returned
func IntE(raw any) (int, error) {
	switch val := raw.(type) {
	case int:
		return val, nil
	default:
		num, err := Int64E(val)
		if err != nil {
			return 0, err
		}

		if num > math.MaxInt {
			return 0, fmt.Errorf("number to large for int")
		}

		return int(num), nil
	}
}

// Int32 converts anything into a int32
// errors will fall back to 0
func Int32(raw any) int32 {
	val, _ := Int32E(raw)

	return val
}

// Int32E converts anything into a int32
// errors will be returned
func Int32E(raw any) (int32, error) {
	switch val := raw.(type) {
	case int32:
		return val, nil
	default:
		num, err := Int64E(val)
		if err != nil {
			return 0, err
		}

		if num > math.MaxInt32 {
			return 0, fmt.Errorf("number to large for int32")
		}

		return int32(num), nil
	}
}

// UInt32 converts anything into a uint32
// errors will fall back to 0
func UInt32(raw any) uint32 {
	val, _ := UInt32E(raw)

	return val
}

// UInt32E converts anything into a uint32
// errors will be returned
func UInt32E(raw any) (uint32, error) {
	switch val := raw.(type) {
	case uint32:
		return val, nil
	default:
		num, err := Int64E(val)
		if err != nil {
			return 0, err
		}

		if num > math.MaxUint32 {
			return 0, fmt.Errorf("number to large for uint32")
		}

		if num < 0 {
			return 0, fmt.Errorf("number to small for uint32")
		}

		return uint32(num), nil
	}
}

// UInt16 converts anything into a uint16
// errors will fall back to 0
func UInt16(raw any) uint16 {
	val, _ := UInt16E(raw)

	return val
}

// UInt16E converts anything into a uint16
// errors will be returned
func UInt16E(raw any) (uint16, error) {
	switch val := raw.(type) {
	case uint16:
		return val, nil
	default:
		num, err := Int64E(val)
		if err != nil {
			return 0, err
		}

		if num > math.MaxUint16 {
			return 0, fmt.Errorf("number to large for uint16")
		}

		if num < 0 {
			return 0, fmt.Errorf("number to small for uint16")
		}

		return uint16(num), nil
	}
}

// Bool converts anything into a bool
// errors will fall back to false
func Bool(raw any) bool {
	b, _ := BoolE(raw)

	return b
}

// BoolE converts anything into a bool
// errors will be returned
func BoolE(raw any) (bool, error) {
	switch val := raw.(type) {
	case bool:
		return val, nil
	default:
		switch strings.ToLower(fmt.Sprintf("%v", raw)) {
		case "1", "enable", "enabled", "true", "yes", "on":
			return true, nil
		case "0", "disable", "disabled", "false", "no", "off":
			return false, nil
		}
	}

	return false, fmt.Errorf("cannot parse boolean value from %v (%T)", raw, raw)
}

// VersionF64 converts any version into a float64
// errors will fall back to 0
func VersionF64(raw any) float64 {
	val, _ := VersionF64E(raw)

	return val
}

// VersionF64E converts any version into a float64
// errors will be returned
func VersionF64E(raw any) (float64, error) {
	str := fmt.Sprintf("%v", raw)
	if str == "" {
		return 0, fmt.Errorf("cannot parse version float64 value from %v (%T)", raw, raw)
	}
	matches := regexp.MustCompile(`[\d.\-]+`).FindStringSubmatch(str)
	if len(matches) == 0 {
		return 0, fmt.Errorf("cannot parse version float64 value from %v (%T)", raw, raw)
	}
	matches[0] = strings.ReplaceAll(matches[0], "-", ".")
	dots := strings.Split(matches[0], ".")
	str = dots[0]
	for idx := range dots {
		switch idx {
		case 0:
			continue
		case 1:
			str += "." + dots[idx]
		default:
			str += "" + dots[idx]
		}
	}
	num, err := Float64E(str)
	if err != nil {
		return 0, fmt.Errorf("cannot parse version float64 value from %v (%T)", raw, raw)
	}

	return num, nil
}

// Num2String converts any number into a string
// errors will fall back to empty string
func Num2String(raw any) string {
	s, _ := Num2StringE(raw)

	return s
}

// Num2StringE converts any number into a string
// errors will be returned
func Num2StringE(raw any) (string, error) {
	switch num := raw.(type) {
	case float64:
		if strconv.FormatFloat(num, 'f', -1, 64) != fmt.Sprintf("%d", int64(num)) {
			return strconv.FormatFloat(num, 'f', -1, 64), nil
		}

		return fmt.Sprintf("%d", int64(num)), nil
	case int64:
		return fmt.Sprintf("%d", num), nil
	default:
		fNum, err := Float64E(raw)
		if err != nil {
			return "", fmt.Errorf("cannot convert %v (%T) into string", raw, raw)
		}

		return Num2StringE(fNum)
	}
}

// StateString returns the string corresponding to a monitoring plugin exit code
func StateString(state int64) string {
	switch state {
	case 0:
		return "OK"
	case 1:
		return "WARNING"
	case 2:
		return "CRITICAL"
	}

	return "UNKNOWN"
}
