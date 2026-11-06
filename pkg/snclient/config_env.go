package snclient

import (
	"fmt"
	"os"
	"strings"
)

const (
	envMacroStart  = "$ENV{"
	envMacroEscape = "$$ENV{"
)

// ReplaceEnvMacros replaces $ENV{NAME} with the value of the environment variable NAME.
// The substituted value is not expanded again. $$ENV{NAME} is replaced with a literal $ENV{NAME}.
func ReplaceEnvMacros(value string) (string, error) {
	var result strings.Builder
	result.Grow(len(value))

	var firstErr error
	for pos := 0; pos < len(value); {
		switch {
		case strings.HasPrefix(value[pos:], envMacroEscape):
			result.WriteString(envMacroStart)
			pos += len(envMacroEscape)
		case strings.HasPrefix(value[pos:], envMacroStart):
			end := strings.IndexByte(value[pos+len(envMacroStart):], '}')
			if end == -1 {
				// no closing brace, keep the rest as is
				result.WriteString(value[pos:])

				pos = len(value)

				continue
			}
			end += pos + len(envMacroStart)

			nameStart := pos + len(envMacroStart)
			name := strings.TrimSpace(value[nameStart:end])

			if name == "" {
				if firstErr == nil {
					firstErr = fmt.Errorf("empty environment variable name")
				}
				result.WriteString(value[pos : end+1])

				pos = end + 1

				continue
			}

			envValue, ok := os.LookupEnv(name)
			if !ok {
				if firstErr == nil {
					firstErr = fmt.Errorf("environment variable %q is not set", name)
				}
				result.WriteString(value[pos : end+1])

				pos = end + 1

				continue
			}
			result.WriteString(envValue)

			pos = end + 1
		default:
			result.WriteByte(value[pos])

			pos++
		}
	}

	return result.String(), firstErr
}
