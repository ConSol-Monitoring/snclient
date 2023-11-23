package utils

import (
	"bytes"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/kdar/factorlog"
)

var TimeFactors = []struct {
	suffix string
	factor float64
}{
	{"ms", 0.001},
	{"s", 1},
	{"m", 60},
	{"h", 3600},
	{"d", 86400},
	{"w", 86400 * 7},
	{"y", 86400 * 365},
}

// ExpandDuration expand duration string into seconds
func ExpandDuration(val string) (res float64, err error) {
	var num float64

	factor := float64(1)
	if strings.HasPrefix(val, "-") {
		factor = -1
		val = val[1:]
	}

	for _, f := range TimeFactors {
		if strings.HasSuffix(val, f.suffix) {
			num, err = strconv.ParseFloat(strings.TrimSuffix(val, f.suffix), 64)
			res = num * f.factor
			if err != nil {
				return 0, fmt.Errorf("expandDuration: %s", err.Error())
			}

			return factor * res, nil
		}
	}
	if IsDigitsOnly(val) {
		res, err = strconv.ParseFloat(val, 64)

		if err != nil {
			return 0, fmt.Errorf("expandDuration: %s", err.Error())
		}

		return factor * res, nil
	}

	return 0, fmt.Errorf("expandDuration: cannot parse duration, unknown format in %s", val)
}

// returns time/duration in target unit with given precision
func TimeUnitF(num uint64, targetUnit string, precision int) float64 {
	for _, factor := range TimeFactors {
		if strings.EqualFold(factor.suffix, targetUnit) {
			return ToPrecision(float64(num)/factor.factor, precision)
		}
	}

	return 0
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

func CloneStringMap(src map[string]string) (clone map[string]string) {
	clone = make(map[string]string)
	for k, v := range src {
		clone[k] = v
	}

	return clone
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
// token will still have quotes around after tokenizing
func Tokenize(str string) []string {
	return (TokenizeBy(str, " \t\n\r", true, false))
}

// TokenizeBy returns list of string tokens separated by any char in separator
func TokenizeBy(str, separator string, keepQuotes, keepSeparator bool) []string {
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
			if keepQuotes || inQuotes {
				token = append(token, char)
			}
		case char == '\'':
			if !inDbl {
				inQuotes = !inQuotes
			}
			if keepQuotes || inDbl {
				token = append(token, char)
			}
		case strings.ContainsRune(separator, char):
			switch {
			case inQuotes, inDbl:
				token = append(token, char)
			case len(token) > 0:
				tokens = append(tokens, string(token))
				token = make([]rune, 0)
				if keepSeparator {
					tokens = append(tokens, string(char))
				}
			}
		default:
			token = append(token, char)
		}
	}

	// append empty token if no token found so far
	if len(token) > 0 || len(tokens) == 0 {
		tokens = append(tokens, string(token))
	}

	return tokens
}

func TrimQuotes(str string) (res string, err error) {
	switch {
	case strings.HasPrefix(str, "'"):
		if !strings.HasSuffix(str, "'") || len(str) == 1 {
			return "", fmt.Errorf("unbalanced quotes in '%s'", str)
		}
		str = strings.TrimPrefix(str, "'")
		str = strings.TrimSuffix(str, "'")
	case strings.HasPrefix(str, `"`):
		if !strings.HasSuffix(str, `"`) || len(str) == 1 {
			return "", fmt.Errorf("unbalanced quotes in '%s'", str)
		}
		str = strings.TrimPrefix(str, `"`)
		str = strings.TrimSuffix(str, `"`)
	case strings.HasSuffix(str, "'"):
		return "", fmt.Errorf("unbalanced quotes in '%s'", str)
	case strings.HasSuffix(str, `"`):
		return "", fmt.Errorf("unbalanced quotes in '%s'", str)
	}

	return str, nil
}

func TrimQuotesAll(str []string) (res []string, err error) {
	res = make([]string, len(str))
	for i, s := range str {
		t, err := TrimQuotes(s)
		if err != nil {
			return nil, err
		}
		res[i] = t
	}

	return res, err
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

// Sha256FileSum returns sha256 sum for given file
func Sha256FileSum(path string) (hash string, err error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open %s: %s", path, err.Error())
	}
	defer file.Close()

	sha := sha256.New()
	if _, err := io.Copy(sha, file); err != nil {
		return "", fmt.Errorf("read %s: %s", path, err.Error())
	}

	return fmt.Sprintf("%x", sha.Sum(nil)), nil
}

// Sha256Sum returns sha256 sum for given string
func Sha256Sum(text string) (hash string, err error) {
	sha := sha256.New()
	_, err = fmt.Fprint(sha, text)
	if err != nil {
		return "", fmt.Errorf("sha256: %s", err.Error())
	}

	return fmt.Sprintf("%x", sha.Sum(nil)), nil
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

func ParseTLSMinVersion(version string) (uint16, error) {
	switch strings.ToLower(version) {
	case "":
		return 0, nil
	case "tls10", "tls1.0":
		return tls.VersionTLS10, nil
	case "tls11", "tls1.1":
		return tls.VersionTLS11, nil
	case "tls12", "tls1.2":
		return tls.VersionTLS12, nil
	case "tls13", "tls1.3":
		return tls.VersionTLS13, nil
	default:
		err := fmt.Errorf("cannot parse %s into tls version, supported values are: tls1.0, tls1.1, tls1.2, tls1.3", version)

		return 0, err
	}
}

func GetSecureCiphers() (ciphers []uint16) {
	ciphers = []uint16{}
	for _, cipher := range tls.CipherSuites() {
		if cipher.Insecure {
			continue
		}
		ciphers = append(ciphers, cipher.ID)
	}

	return
}

// WordRank is used to sort []string lists by ranked prefixes
type WordRank struct {
	Word string
	Rank int
}
type ByRank []WordRank

func (a ByRank) Len() int      { return len(a) }
func (a ByRank) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByRank) Less(i, j int) bool {
	if a[i].Rank == a[j].Rank {
		return a[i].Word < a[j].Word
	}

	return a[i].Rank < a[j].Rank
}

func SortRanked(list []string, ranks map[string]int) []string {
	wordRanks := make([]WordRank, len(list))
	for num, word := range list {
		rank, ok := ranks[word]
		if ok {
			wordRanks[num] = WordRank{Word: word, Rank: rank}

			continue
		}

		for prefix, num := range ranks {
			if strings.HasPrefix(word, prefix) {
				if rank == 0 || rank > num {
					rank = num + 2
				}
			}
		}
		if rank != 0 {
			wordRanks[num] = WordRank{Word: word, Rank: rank}
		}

		wordRanks[num] = WordRank{Word: word, Rank: ranks["default"]}
	}

	sort.Sort(ByRank(wordRanks))

	sorted := make([]string, len(list))
	for i, el := range wordRanks {
		sorted[i] = el.Word
	}

	return sorted
}

// List2String converts a list of strings into a single quoted comma separated list: 'a', 'b', 'c'...
func List2String(list []string) (res string) {
	for i, e := range list {
		if i > 0 {
			res += ", "
		}
		res += "'" + e + "'"
	}

	return res
}

// IsFolder returns an err if the path does not exist or is not a folder
func IsFolder(path string) error {
	path = filepath.Join(path, ".")
	stat, err := os.Stat(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("%s does not exist: %s", path, err.Error())
	}
	if !stat.IsDir() {
		return fmt.Errorf("%s is not a directory", path)
	}

	return nil
}
