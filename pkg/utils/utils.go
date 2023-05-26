package utils

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/kdar/factorlog"
)

// ExpandDuration expand duration string into seconds
func ExpandDuration(val string) (res float64, err error) {
	var num float64

	factors := []struct {
		suffix string
		factor float64
	}{
		{"ms", 0.001},
		{"s", 1},
		{"m", 60},
		{"h", 3600},
		{"d", 86400},
	}

	for _, f := range factors {
		if strings.HasSuffix(val, f.suffix) {
			num, err = strconv.ParseFloat(strings.TrimSuffix(val, f.suffix), 64)
			res = num * f.factor
			if err != nil {
				return 0, fmt.Errorf("expandDuration: %s", err.Error())
			}

			return res, nil
		}
	}
	if IsDigitsOnly(val) {
		res, err = strconv.ParseFloat(val, 64)

		if err != nil {
			return 0, fmt.Errorf("expandDuration: %s", err.Error())
		}

		return res, nil
	}

	return 0, fmt.Errorf("expandDuration: cannot parse duration, unknown format in %s", val)
}

// IsDigitsOnly returns true if string only contains numbers
func IsDigitsOnly(s string) bool {
	for _, c := range s {
		if !unicode.IsDigit(c) {
			return false
		}
	}

	return true
}

// IsFloatVal returns true if given val is a real float64 with fractions
// or false if value can be represented as int64
func IsFloatVal(val interface{}) bool {
	switch num := val.(type) {
	case float64:
		return strconv.FormatFloat(num, 'f', -1, 64) != fmt.Sprintf("%d", int64(num))
	case int64:
		return true
	default:
		return false
	}
}

// ToPrecision converts float64 to given precision, ex.: 5.12345 -> 5.1
func ToPrecision(val float64, precision int) float64 {
	format := fmt.Sprintf("%%.%df", precision)
	valStr := fmt.Sprintf(format, val)
	short, _ := strconv.ParseFloat(valStr, 64)

	return short
}

func DurationString(dur time.Duration) string {
	seconds := int64(dur.Seconds())

	years := seconds / (86400 * 365)
	seconds -= years * (86400 * 365)

	weeks := seconds / (86400 * 7)
	seconds -= weeks * (86400 * 7)

	days := seconds / 86400
	seconds -= days * 86400

	hours := seconds / 3600
	seconds -= hours * 3600

	minutes := seconds / 60

	switch {
	case years > 0:
		return fmt.Sprintf("%dy %dw", years, weeks)
	case weeks > 3:
		return fmt.Sprintf("%dw %dd", weeks, days)
	case days > 0:
		return fmt.Sprintf("%dd %02d:%02dh", days+weeks*7, hours, minutes)
	default:
		return fmt.Sprintf("%02d:%02dh", hours, minutes)
	}
}

// GetExecutablePath returns path to executable information
// execDir: folder of the executable
// execFile: file name (basename) of executable
// execPath: full path to executable (dir/file)
func GetExecutablePath() (execDir, execFile, execPath string, err error) {
	executable, err := os.Executable()
	if err != nil {
		return "", "", "", fmt.Errorf("executable error: %s", err.Error())
	}

	executable, err = filepath.Abs(executable)
	if err != nil {
		return "", "", "", fmt.Errorf("abs error: %s", err.Error())
	}

	return filepath.Dir(executable), filepath.Base(execPath), executable, nil
}

func ReadPid(pidfile string) (int, error) {
	dat, err := os.ReadFile(pidfile)
	if err != nil {
		return 0, fmt.Errorf("read %s failed: %s", pidfile, err.Error())
	}

	pid, err := strconv.Atoi(strings.TrimSpace(string(dat)))
	if err != nil {
		return 0, fmt.Errorf("read %s failed: %s", pidfile, err.Error())
	}

	return pid, nil
}

func LogThreadDump(log *factorlog.FactorLog) {
	buf := make([]byte, 1<<16)

	if n := runtime.Stack(buf, true); n < len(buf) {
		buf = buf[:n]
	}

	log.Errorf("ThreadDump:\n%s", buf)
}

// Tokenize returns list of string tokens
func Tokenize(str string) []string {
	return (TokenizeBy(str, " \t\n\r"))
}

// TokenizeBy returns list of string tokens separated by any char in separator
func TokenizeBy(str, separator string) []string {
	var tokens []string

	inQuotes := false
	inDbl := false
	token := make([]rune, 0)
	for _, char := range str {
		switch {
		case char == '"':
			if !inQuotes {
				inDbl = !inDbl
			}
			token = append(token, char)
		case char == '\'':
			if !inDbl {
				inQuotes = !inQuotes
			}
			token = append(token, char)
		case strings.ContainsRune(separator, char):
			switch {
			case inQuotes, inDbl:
				token = append(token, char)
			case len(token) > 0:
				tokens = append(tokens, string(token))
				token = make([]rune, 0)
			}
		default:
			token = append(token, char)
		}
	}
	tokens = append(tokens, string(token))

	return tokens
}

func ParseVersion(str string) (num float64) {
	str = strings.TrimPrefix(str, "v")
	token := strings.Split(str, ".")

	for i, t := range token {
		x, err := strconv.ParseFloat(t, 64)
		if err != nil {
			continue
		}
		num += x * math.Pow10(-i*3)
	}

	return num
}

func Sha256Sum(path string) (hash string, err error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %s", path, err.Error())
	}
	defer file.Close()

	h := sha256.New()
	if _, err := io.Copy(h, file); err != nil {
		return "", fmt.Errorf("read %s: %s", path, err.Error())
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// Copy file from src to destination
func CopyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open: %s", err.Error())
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("open: %s", err.Error())
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("write: %s", err.Error())
	}

	err = dstFile.Close()
	if err != nil {
		return fmt.Errorf("write: %s", err.Error())
	}

	return CopyFileMode(src, dst)
}

// Copy file modes from src to destination
func CopyFileMode(src, dst string) error {
	oldStat, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat %s: %s", src, err.Error())
	}

	err = os.Chmod(dst, oldStat.Mode())
	if err != nil {
		return fmt.Errorf("chmod %s: %s", dst, err.Error())
	}

	return nil
}

// Count lines in file
func LineCounter(reader *os.File) int {
	buf := make([]byte, 32*1024)
	count := 0
	lineSep := []byte{'\n'}

	for {
		c, err := reader.Read(buf)
		count += bytes.Count(buf[:c], lineSep)

		switch {
		case err == io.EOF:
			return count

		case err != nil:
			return count
		}
	}
}

func MimeType(fileName string) (mime string, err error) {
	zipFile, err := os.Open(fileName)
	if err != nil {
		return "", fmt.Errorf("open: %s", err.Error())
	}

	defer zipFile.Close()

	header := make([]byte, 500)
	_, err = io.ReadFull(zipFile, header)
	if err != nil {
		return "", fmt.Errorf("read: %s", err.Error())
	}
	mimeType := http.DetectContentType(header)

	if mimeType == "application/octet-stream" {
		// see https://en.wikipedia.org/wiki/List_of_file_signatures
		signatures := map[string]string{
			"0:EDABEEDB":           "application/rpm",
			"0:D0CF11E0A1B11AE1":   "application/msi",
			"257:7573746172003030": "application/x-tar",
			"257:7573746172202000": "application/x-tar",
		}
		for sig, mime := range signatures {
			sigData := strings.Split(sig, ":")
			offset, _ := strconv.Atoi(sigData[0])
			sig = sigData[1]
			sigBytes, err := hex.DecodeString(sig)
			if err == nil && bytes.HasPrefix(header[offset:], sigBytes) {
				return mime, nil
			}
		}
	}

	return mimeType, nil
}
