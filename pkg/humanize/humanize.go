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
	"B": Byte,

	"KB":  KByte,
	"KiB": KiByte,
	"Kb":  KiByte,

	"MB":  MByte,
	"MiB": MiByte,
	"Mb":  MiByte,

	"GB":  GByte,
	"GiB": GiByte,
	"Gb":  GiByte,

	"TB":  TByte,
	"TiB": TiByte,
	"Tb":  TiByte,

	"PB":  PByte,
	"PiB": PiByte,
	"Pb":  PiByte,

	"EB":  EByte,
	"EiB": EiByte,
	"Eb":  EiByte,

	// Without suffix
	"":   Byte,
	"KI": KiByte,
	"K":  KByte,
	"MI": MiByte,
	"M":  MByte,
	"GI": GiByte,
	"G":  GByte,
	"TI": TiByte,
	"T":  TByte,
	"PI": PiByte,
	"P":  PByte,
	"EI": EiByte,
	"E":  EByte,
}

// ParseBytes("83 M") -> 82854982
func ParseBytes(raw string) (uint64, error) {
	lastDigit := 0
	hasComma := false
	for _, r := range raw {
		if !unicode.IsDigit(r) && r != '.' && r != ',' {
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

	extra := strings.TrimSpace(raw[lastDigit:])
	if m, ok := getByteSize(extra); ok {
		fNum *= float64(m)
		if fNum >= math.MaxUint64 {
			return 0, fmt.Errorf("too large: %v", raw)
		}

		return uint64(fNum), nil
	}

	return 0, fmt.Errorf("unhandled size name: %v", extra)
}

// Num(82854982) -> 83 M
func Num(num int64) string {
	return NumF(num, 0)
}

func NumF(num int64, precision int) string {
	prefix := ""
	if num < 0 {
		num *= -1
		prefix = "-"
	}

	// useless but makes gosec G115 happy
	if num >= 0 {
		return prefix + humanizeBytes(uint64(num), 1000, []string{"", "k", "M", "G", "T", "P", "E"}, precision)
	}

	return ""
}

// Bytes(82854982) -> 83 MB
func Bytes(num uint64) string {
	return BytesF(num, 0)
}

// Bytes(82854982, 3) -> 83.000 MiB
func BytesF(num uint64, precision int) string {
	return humanizeBytes(num, 1000, []string{"B", "kB", "MB", "GB", "TB", "PB", "EB"}, precision)
}

// IBytes(82854982) -> 79 MiB
func IBytes(num uint64) string {
	return IBytesF(num, 0)
}

// IBytes(82854982, 3) -> 79.000 MiB
func IBytesF(num uint64, precision int) string {
	return humanizeBytes(num, 1024, []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}, precision)
}

func logn(n, b float64) float64 {
	return math.Log(n) / math.Log(b)
}

// returns bytes in target unit
func BytesUnit(num uint64, targetUnit string) float64 {
	return BytesUnitF(num, targetUnit, 0)
}

// returns bytes in target unit with given precision
func BytesUnitF(num uint64, targetUnit string, precision int) float64 {
	factor, ok := getByteSize(targetUnit)
	if !ok {
		return 0
	}

	return roundToPrecision(float64(num)/float64(factor), precision)
}

func humanizeBytes(num uint64, base float64, sizes []string, precision int) string {
	if num < 10 {
		if len(sizes) == 0 {
			return fmt.Sprintf("%d", num)
		}

		return fmt.Sprintf("%d %s", num, sizes[0])
	}
	exp := math.Floor(logn(float64(num), base))
	if len(sizes) <= int(exp) {
		return fmt.Sprintf("%d", num)
	}
	suffix := sizes[int(exp)]
	val := float64(num) / math.Pow(base, exp)
	switch {
	case precision > 0:
		return fmt.Sprintf(fmt.Sprintf("%%.%df %%s", precision), roundToPrecision(val, precision), suffix)
	case math.Floor(val) == val:
		return fmt.Sprintf("%.0f %s", val, suffix)
	case val < 3:
		val *= base
		suffix = sizes[int(exp-1)]

		return fmt.Sprintf("%.0f %s", val, suffix)
	default:
		return fmt.Sprintf("%.0f %s", val, suffix)
	}
}

func roundToPrecision(val float64, precision int) float64 {
	factor := math.Pow10(precision)

	return math.Round(val*factor) / factor
}

// find entry in the byte size table
func getByteSize(name string) (uint64, bool) {
	if m, ok := bytesSizeTable[name]; ok {
		return m, ok
	}

	// try with uppercase case name if nothing matched yet
	if m, ok := bytesSizeTable[strings.ToUpper(name)]; ok {
		return m, ok
	}

	// try case insensitive match
	for key, val := range bytesSizeTable {
		if strings.EqualFold(key, name) {
			return val, true
		}
	}

	return 1, false
}
