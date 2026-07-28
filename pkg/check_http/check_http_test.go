package check_http

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	testHost = "omd.consol.de"
	testURI  = "/impressum/"
)

func TestHTTP(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	var output strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	code := Check(ctx, &output, []string{"check_http", "-H", testHost, "-u", "/"})
	assert.Equalf(t, OK, code, "expected exit code OK (0), got %d", code)
}

func TestHTTPSAutoNegotiated(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	var output strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	code := Check(ctx, &output, []string{"check_http", "-H", testHost, "-u", testURI, "-S"})

	assert.Equalf(t, OK, code, "expected exit code OK (0), got %d", code)
	assert.Containsf(t, output.String(), "HTTP OK", "expected output to contain 'HTTP OK'")
}

func TestHTTPSMaxVersion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	var output strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	code := Check(ctx, &output, []string{"check_http", "-H", testHost, "-u", testURI, "-S", "--tls-min", "1.3"})

	assert.Equalf(t, OK, code, "expected exit code OK (0), got %d", code)
	assert.Containsf(t, output.String(), "HTTP OK", "expected output to contain 'HTTP OK'")
}

func TestHTTPCertificateCheckWarn3Days(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	var output strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	code := Check(ctx, &output, []string{"check_http", "-H", testHost, "-S", "-C", "3"})

	assert.Equalf(t, OK, code, "expected exit code OK (0), got %d", code)
	assert.Containsf(t, output.String(), "HTTP OK", "expected output to contain 'HTTP OK'")
	assert.Containsf(t, output.String(), "days_chain_elem1=", "expected perfdata to contain 'days_chain_elem1='")
}

func TestHTTPCertificateCheckWarn100000Days(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	var output strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	code := Check(ctx, &output, []string{"check_http", "-H", testHost, "-S", "-C", "100000"})

	assert.Equalf(t, WARNING, code, "expected exit code WARNING (1), got %d", code)
	assert.Containsf(t, output.String(), "HTTP WARNING", "expected output to contain 'HTTP WARNING'")
}

func TestHTTPRegex(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	var output strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	code := Check(ctx, &output, []string{"check_http", "-H", testHost, "-u", testURI, "-S", "-r", `HRB \d+`})

	assert.Equalf(t, OK, code, "expected exit code OK (0), got %d", code)
	assert.Containsf(t, output.String(), "HRB 97371", "expected output to contain 'HRB 97371'")
}

func TestHTTPRegexLong(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	var output strings.Builder

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	code := Check(ctx, &output, []string{"check_http", "-H", testHost, "-u", testURI, "-S", "--regex", `HRB \d+`})

	assert.Equalf(t, OK, code, "expected exit code OK (0), got %d", code)
	assert.Containsf(t, output.String(), "HRB 97371", "expected output to contain 'HRB 97371'")
}

func TestHTTPRegexNoMatch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	var output strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	code := Check(ctx, &output, []string{"check_http", "-H", testHost, "-u", testURI, "-S", "--regex", `XYZZY-NONEXISTENT`})

	assert.Equalf(t, CRITICAL, code, "expected exit code CRITICAL (2), got %d", code)
	assert.Containsf(t, output.String(), "HTTP CRITICAL", "expected output to contain 'HTTP CRITICAL'")
}

func TestHTTPRegexiShort(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	var output strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	code := Check(ctx, &output, []string{"check_http", "-H", testHost, "-u", testURI, "-S", "-R", "consol"})

	assert.Equalf(t, OK, code, "expected exit code OK (0), got %d", code)
	assert.Containsf(t, strings.ToLower(output.String()), "consol", "expected output to contain 'consol' (case-insensitive)")
}

func TestHTTPRegexiLong(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	var output strings.Builder

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	code := Check(ctx, &output, []string{"check_http", "-H", testHost, "-u", testURI, "-S", "--regexi", "consol"})

	assert.Equalf(t, OK, code, "expected exit code OK (0), got %d", code)
	assert.Containsf(t, strings.ToLower(output.String()), "consol", "expected output to contain 'consol' (case-insensitive)")
}

func TestHTTPBase64String(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	var output strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// echo "Q29uU29s" | base64 --decode -> ConSol
	code := Check(ctx, &output, []string{"check_http", "-H", testHost, "-u", testURI, "-S", "--base64-string", "Q29uU29s"})

	assert.Equalf(t, OK, code, "expected exit code OK (0), got %d", code)

	exceptStr := `Response body matched: [base64: 'Q29uU29s' , string: 'ConSol']`
	assert.Containsf(t, output.String(), exceptStr, "expected output to contain: '%s'", exceptStr)
}

func TestHTTPStringContent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	var output strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	code := Check(ctx, &output, []string{"check_http", "-H", testHost, "-u", testURI, "-S", "-s", "Commercial register"})

	assert.Equalf(t, OK, code, "expected exit code OK (0), got %d", code)

	exceptStr := `Response body matched: [string: 'Commercial register']`
	assert.Containsf(t, output.String(), exceptStr, "expected output to contain '%s'", exceptStr)
}

func TestHTTPCertificateChainPerfdata(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	var output strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	code := Check(ctx, &output, []string{"check_http", "-H", testHost, "-S", "-C", "30"})

	assert.Equalf(t, OK, code, "expected exit code OK (0), got %d", code)
	assert.Containsf(t, output.String(), "days_chain_elem1=", "expected perfdata to contain 'days_chain_elem1='")
	assert.Containsf(t, output.String(), "days_chain_elem2=", "expected perfdata to contain 'days_chain_elem2=' for chain cert")
}

func TestHTTPExpectStatusCode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	var output strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	code := Check(ctx, &output, []string{"check_http", "-H", testHost, "-u", testURI, "-S", "-e", "200"})

	assert.Equalf(t, OK, code, "expected exit code OK (0), got %d", code)

	expectedStr := `matched option '200'`
	assert.Containsf(t, output.String(), expectedStr, "expected output to contain '%s'", expectedStr)
}
