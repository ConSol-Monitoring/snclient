package snclient

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type configFiles []string

// String returns the config files list as string.
func (c *configFiles) String() string {
	return fmt.Sprintf("%s", *c)
}

// Set appends a config file to the list of config files.
func (c *configFiles) Set(value string) error {
	// check if the file exists but skip errors for file globs
	_, err := os.Stat(value)
	if err != nil && !strings.ContainsAny(value, "?*") {
		return fmt.Errorf("failed to read config file: %s", err.Error())
	}

	*c = append(*c, value)

	return nil
}

type Config map[string]map[string]string

func NewConfig() Config {
	conf := make(Config, 0)

	return conf
}

// opens the config file and reads all key value pairs, separated through = and commented out with ";".
func (config *Config) readSettingsFile(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("cannot read file %s: %w", path, err)
	}

	currentBlock := ""
	lineNr := 0

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lineNr++

		line := strings.TrimSpace(scanner.Text())
		if line == "" || line[0] == ';' {
			continue
		}

		if line[0] == '[' {
			currentBlock = strings.TrimSuffix(strings.TrimPrefix(line, "["), "]")

			if _, ok := (*config)[currentBlock]; !ok {
				block := make(map[string]string, 0)
				(*config)[currentBlock] = block
			}

			continue
		}

		if currentBlock == "" {
			return fmt.Errorf("parse error in %s:%d: found key=value outside of block", path, lineNr)
		}

		// get both values
		val := strings.SplitN(line, "=", 2)
		val[0] = strings.TrimSpace(val[0])
		val[1] = strings.TrimSpace(val[1])

		(*config)[currentBlock][val[0]] = val[1]
	}

	return nil
}
