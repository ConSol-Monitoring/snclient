package snclient

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type configurationStruct struct {
	build            string
	identifier       string
	debug            int
	logfile          string
	logmode          string
	prometheusServer string
}

// setDefaultValues sets reasonable defaults
func (config *configurationStruct) setDefaultValues() {
	config.logmode = "automatic"
	config.debug = 0
	config.logmode = "automatic"
	hostname, _ := os.Hostname()
	config.identifier = hostname
	if config.identifier == "" {
		config.identifier = "unknown"
	}
}

// dump logs all config items
func (config *configurationStruct) dump() {
	logger.Debugf("build                         %s\n", config.build)
	logger.Debugf("identifier                    %s\n", config.identifier)
	logger.Debugf("debug                         %d\n", config.debug)
	logger.Debugf("logfile                       %s\n", config.logfile)
	logger.Debugf("logmode                       %s\n", config.logmode)
}

/**
* parses the key value pairs and stores them in the configuration struct
 */
func (config *configurationStruct) readSetting(values []string) {
	key := strings.ToLower(strings.Trim(values[0], " "))
	value := strings.Trim(values[1], " ")

	switch key {
	case "prometheus_server":
		config.prometheusServer = value
	case "config":
		config.readSettingsPath(value)
	case "debug":
		config.debug = getInt(value)
		if config.debug > LogLevelTrace2 {
			config.debug = LogLevelTrace2
		}
		createLogger(config)
	case "logfile":
		config.logfile = value
		createLogger(config)
	case "logmode":
		config.logmode = value
		createLogger(config)
	case "identifier":
		config.identifier = value
	default:
		logger.Warnf("unknown config option: %s", key)
	}
}

// read settings from file or folder
func (config *configurationStruct) readSettingsPath(path string) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		logger.Errorf("cannot read %s: %w", path, err)
		return
	}
	if fileInfo.IsDir() {
		filepath.Walk(path, func(path string, info fs.FileInfo, err error) error {
			if !info.IsDir() {
				if strings.HasSuffix(path, ".cfg") || strings.HasSuffix(path, ".conf") {
					config.readSettingsFile(path)
				}
			}
			return err
		})
		return
	}

	config.readSettingsFile(path)
}

// opens the config file and reads all key value pairs, separated through = and commented out with #
// also reads the config files specified in the config= value
func (config *configurationStruct) readSettingsFile(path string) {
	file, err := os.Open(path)

	if err != nil {
		logger.Errorf("cannot read file %s: %w", path, err)
		return
	}

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		// get line and remove whitespaces
		line := scanner.Text()
		line = strings.TrimSpace(line)
		// check if not commented out
		if len(line) > 0 && line[0] != '#' {
			// get both values
			values := strings.SplitN(line, "=", 2)
			config.readSetting(values)
		}
	}
}

func getInt(input string) int {
	if input == "" {
		return 0
	}
	result, err := strconv.Atoi(input)
	if err != nil {
		// check if it is an float value
		logger.Debugf("Error converting %s to int, try with float", input)
		result = int(getFloat(input))
	}
	return result
}

func getFloat(input string) float64 {
	if input == "" {
		return float64(0)
	}
	result, err := strconv.ParseFloat(input, 64)
	if err != nil {
		logger.Errorf("error Converting %s to float", input)
		result = 0
	}
	return result
}

func getBool(input string) bool {
	if input == "yes" || input == "on" || input == "1" {
		return true
	}
	return false
}

func fixGearmandServerAddress(address string) string {
	parts := strings.SplitN(address, ":", 2)
	// if no port is given, use default gearmand port
	if len(parts) == 1 {
		return address + ":4730"
	}
	// if no hostname is given, use all interfaces
	if len(parts) == 2 && parts[0] == "" {
		return "0.0.0.0:" + parts[1]
	}
	return address
}

func removeDuplicateStrings(elements []string) []string {
	encountered := map[string]bool{}
	uniq := []string{}

	for v := range elements {
		if !encountered[elements[v]] {
			encountered[elements[v]] = true
			uniq = append(uniq, elements[v])
		}
	}
	return uniq
}
