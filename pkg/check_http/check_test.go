package check_http_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/consol-monitoring/snclient/pkg/check_http"
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

	code := check_http.Check(ctx, &output, []string{"check_http", "-H", "example.com", "-u", "/"})

	t.Logf("output: %s", output.String())

	if code != check_http.OK {
		t.Errorf("expected exit code OK (0), got %d", code)
	}
}

func TestHTTPSAutoNegotiated(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	var output strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)

	defer cancel()

	code := check_http.Check(ctx, &output, []string{"check_http", "-H", testHost, "-u", testURI, "-S"})

	t.Logf("output: %s", output.String())

	if code != check_http.OK {
		t.Errorf("expected exit code OK (0), got %d", code)
	}

	if !strings.Contains(output.String(), "HTTP OK") {
		t.Errorf("expected output to contain 'HTTP OK'")
	}
}

func TestHTTPSMaxVersion(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	var output strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)

	defer cancel()

	code := check_http.Check(ctx, &output, []string{"check_http", "-H", testHost, "-u", testURI, "-S", "--tls-min", "1.3"})

	t.Logf("output: %s", output.String())

	if code != check_http.OK {
		t.Errorf("expected exit code OK (0), got %d", code)
	}

	if !strings.Contains(output.String(), "HTTP OK") {
		t.Errorf("expected output to contain 'HTTP OK'")
	}
}

func TestCertificateCheckWarn3Days(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	var output strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)

	defer cancel()

	code := check_http.Check(ctx, &output, []string{"check_http", "-H", testHost, "-S", "-C", "3"})

	t.Logf("output: %s", output.String())

	if code != check_http.OK {
		t.Errorf("expected exit code OK (0), got %d", code)
	}

	if !strings.Contains(output.String(), "HTTP OK") {
		t.Errorf("expected output to contain 'HTTP OK'")
	}

	if !strings.Contains(output.String(), "days_chain_elem1=") {
		t.Errorf("expected perfdata to contain 'days_chain_elem1='")
	}
}

func TestCertificateCheckWarn100000Days(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	var output strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)

	defer cancel()

	code := check_http.Check(ctx, &output, []string{"check_http", "-H", testHost, "-S", "-C", "100000"})

	t.Logf("output: %s", output.String())

	if code != check_http.WARNING {
		t.Errorf("expected exit code WARNING (1), got %d", code)
	}

	if !strings.Contains(output.String(), "HTTP WARNING") {
		t.Errorf("expected output to contain 'HTTP WARNING'")
	}
}

func TestRegex(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	var output strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)

	defer cancel()

	code := check_http.Check(ctx, &output, []string{"check_http", "-H", testHost, "-u", testURI, "-S", "-r", `HRB \d+`})

	t.Logf("output: %s", output.String())

	if code != check_http.OK {
		t.Errorf("expected exit code OK (0), got %d", code)
	}

	if !strings.Contains(output.String(), "HRB 97371") {
		t.Errorf("expected output to contain 'HRB 97371'")
	}
}

func TestRegexLong(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	var output strings.Builder

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)

	defer cancel()

	code := check_http.Check(ctx, &output, []string{"check_http", "-H", testHost, "-u", testURI, "-S", "--regex", `HRB \d+`})

	t.Logf("output: %s", output.String())

	if code != check_http.OK {
		t.Errorf("expected exit code OK (0), got %d", code)
	}

	if !strings.Contains(output.String(), "HRB 97371") {
		t.Errorf("expected output to contain 'HRB 97371'")
	}
}

func TestRegexNoMatch(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	var output strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)

	defer cancel()

	code := check_http.Check(ctx, &output, []string{"check_http", "-H", testHost, "-u", testURI, "-S", "--regex", `XYZZY-NONEXISTENT`})

	t.Logf("output: %s", output.String())

	if code != check_http.CRITICAL {
		t.Errorf("expected exit code CRITICAL (2), got %d", code)
	}

	if !strings.Contains(output.String(), "HTTP CRITICAL") {
		t.Errorf("expected output to contain 'HTTP CRITICAL'")
	}
}

func TestRegexiShort(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	var output strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)

	defer cancel()

	code := check_http.Check(ctx, &output, []string{"check_http", "-H", testHost, "-u", testURI, "-S", "-R", "consol"})

	t.Logf("output: %s", output.String())

	if code != check_http.OK {
		t.Errorf("expected exit code OK (0), got %d", code)
	}

	if !strings.Contains(strings.ToLower(output.String()), "consol") {
		t.Errorf("expected output to contain 'consol' (case-insensitive)")
	}
}

func TestRegexiLong(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	var output strings.Builder

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)

	defer cancel()

	code := check_http.Check(ctx, &output, []string{"check_http", "-H", testHost, "-u", testURI, "-S", "--regexi", "consol"})

	t.Logf("output: %s", output.String())

	if code != check_http.OK {
		t.Errorf("expected exit code OK (0), got %d", code)
	}

	if !strings.Contains(strings.ToLower(output.String()), "consol") {
		t.Errorf("expected output to contain 'consol' (case-insensitive)")
	}
}

func TestBase64String(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	var output strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)

	defer cancel()

	// echo "Q29uU29s" | base64 --decode -> ConSol
	code := check_http.Check(ctx, &output, []string{"check_http", "-H", testHost, "-u", testURI, "-S", "--base64-string", "Q29uU29s"})

	t.Logf("output: %s", output.String())

	if code != check_http.OK {
		t.Errorf("expected exit code OK (0), got %d", code)
	}

	exceptStr := `Response body matched: [base64: 'Q29uU29s' , string: 'ConSol']`
	if !strings.Contains(output.String(), exceptStr) {
		t.Errorf("expected output to contain: '%s'", exceptStr)
	}
}

func TestStringContent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	var output strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)

	defer cancel()

	code := check_http.Check(ctx, &output, []string{"check_http", "-H", testHost, "-u", testURI, "-S", "-s", "Commercial register"})

	t.Logf("output: %s", output.String())

	if code != check_http.OK {
		t.Errorf("expected exit code OK (0), got %d", code)
	}

	exceptStr := `Response body matched: [string: 'Commercial register']`
	if !strings.Contains(output.String(), exceptStr) {
		t.Errorf("expected output to contain '%s'", exceptStr)
	}
}

func TestCertificateChainPerfdata(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	var output strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)

	defer cancel()

	code := check_http.Check(ctx, &output, []string{"check_http", "-H", testHost, "-S", "-C", "30"})

	t.Logf("output: %s", output.String())

	if code != check_http.OK {
		t.Errorf("expected exit code OK (0), got %d", code)
	}

	if !strings.Contains(output.String(), "days_chain_elem1=") {
		t.Errorf("expected perfdata to contain 'days_chain_elem1='")
	}

	if !strings.Contains(output.String(), "days_chain_elem2=") {
		t.Errorf("expected perfdata to contain 'days_chain_elem2=' for chain cert")
	}
}

func TestExpectStatusCode(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	var output strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)

	defer cancel()

	code := check_http.Check(ctx, &output, []string{"check_http", "-H", testHost, "-u", testURI, "-S", "-e", "200"})

	t.Logf("output: %s", output.String())

	if code != check_http.OK {
		t.Errorf("expected exit code OK (0), got %d", code)
	}

	expectedStr := `matched option '200'`
	if !strings.Contains(output.String(), expectedStr) {
		t.Errorf("expected output to contain '%s'", expectedStr)
	}
}
