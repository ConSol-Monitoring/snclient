package humanize

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

// IEC Sizes.
const (
	Byte = 1 << (iota * 10)
	KiByte
	MiByte
	GiByte
	TiByte
	PiByte
	EiByte
)

// SI Sizes.
const (
	IByte = 1
	KByte = IByte * 1000
	MByte = KByte * 1000
	GByte = MByte * 1000
	TByte = GByte * 1000
	PByte = TByte * 1000
	EByte = PByte * 1000
)

var bytesSizeTable = map[string]uint64{
	"b":   Byte,
	"kib": KiByte,
	"kb":  KByte,
	"mib": MiByte,
	"mb":  MByte,
	"gib": GiByte,
	"gb":  GByte,
	"tib": TiByte,
	"tb":  TByte,
	"pib": PiByte,
	"pb":  PByte,
	"eib": EiByte,
	"eb":  EByte,
	// Without suffix
	"":   Byte,
	"ki": KiByte,
	"k":  KByte,
	"mi": MiByte,
	"m":  MByte,
	"gi": GiByte,
	"g":  GByte,
	"ti": TiByte,
	"t":  TByte,
	"pi": PiByte,
	"p":  PByte,
	"ei": EiByte,
	"e":  EByte,
}

func ParseBytes(raw string) (uint64, error) {
	lastDigit := 0
	hasComma := false
	for _, r := range raw {
		if !(unicode.IsDigit(r) || r == '.' || r == ',') {
			break
		}
		if r == ',' {
			hasComma = true
		}
		lastDigit++
	}

	strNum := raw[:lastDigit]
	if hasComma {
		strNum = strings.ReplaceAll(strNum, ",", "")
	}

	fNum, err := strconv.ParseFloat(strNum, 64)
	if err != nil {
		return 0, fmt.Errorf("parsefloat %s: %s", raw, err.Error())
	}

	extra := strings.ToLower(strings.TrimSpace(raw[lastDigit:]))
	if m, ok := bytesSizeTable[extra]; ok {
		fNum *= float64(m)
		if fNum >= math.MaxUint64 {
			return 0, fmt.Errorf("too large: %v", raw)
		}

		return uint64(fNum), nil
	}

	return 0, fmt.Errorf("unhandled size name: %v", extra)
}

// Bytes(82854982) -> 83 MB
func Bytes(num uint64) string {
	return humanizeBytes(num, 1000, []string{"B", "kB", "MB", "GB", "TB", "PB", "EB"}, 0)
}

// IBytes(82854982) -> 79 MiB
func IBytes(num uint64) string {
	return humanizeBytes(num, 1024, []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}, 0)
}

// IBytes(82854982, 3) -> 79.000 MiB
func IBytesF(num uint64, precision int) string {
	return humanizeBytes(num, 1024, []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}, precision)
}

func logn(n, b float64) float64 {
	return math.Log(n) / math.Log(b)
}

func humanizeBytes(num uint64, base float64, sizes []string, precision int) string {
	if num < 10 {
		return fmt.Sprintf("%d B", num)
	}
	exp := math.Floor(logn(float64(num), base))
	suffix := sizes[int(exp)]
	val := float64(num) / math.Pow(base, exp)
	switch {
	case precision > 0:
		return fmt.Sprintf(fmt.Sprintf("%%.%df %%s", precision), roundToPrecision(val, precision), suffix)
	case math.Floor(val) == val:
		return fmt.Sprintf("%.0f %s", val, suffix)
	case val < 3:
		val *= base
		suffix := sizes[int(exp-1)]

		return fmt.Sprintf("%.0f %s", val, suffix)
	default:
		return fmt.Sprintf("%.0f %s", val, suffix)
	}
}

func roundToPrecision(val float64, precision int) float64 {
	factor := math.Pow10(precision)

	return math.Round(val*factor) / factor
}
