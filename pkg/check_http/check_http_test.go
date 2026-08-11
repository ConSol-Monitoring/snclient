package check_http

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestHTTPProxyPlain(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "direct-target-response")
	}))
	defer target.Close()

	targetURL, err := url.Parse(target.URL)
	require.NoError(t, err)

	var proxied atomic.Bool
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		proxied.Store(true)
		fmt.Fprint(w, "proxied-response")
	}))
	defer proxy.Close()

	var output strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	code := Check(ctx, &output, []string{
		"check_http", "-H", targetURL.Host, "-p", "80",
		"--proxy", proxy.URL, "-s", "proxied-response",
	})

	// it should connect to the proxy first
	// the mock proxy just returns a text and does not actually proxy the request
	assert.Equalf(t, OK, code, "expected exit code OK (0), got %d, output: %s", code, output.String())
	assert.Truef(t, proxied.Load(), "request did not go through the proxy, output: %s", output.String())
	assert.Containsf(t, output.String(), "proxied-response", "expected output to contain 'proxied-response'")
	assert.NotContainsf(t, output.String(), "direct-target-response", "response should come from the proxy, not the target")
}

func TestHTTPProxySSL(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	// the target is a TLS address on a TLS server
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "proxied-target-response")
	}))
	defer target.Close()

	targetURL, err := url.Parse(target.URL)
	require.NoError(t, err)

	var proxied atomic.Int64
	// this is a more realistic proxy server, which actually proxies the requests
	// it is an http server and not HTTPS
	proxy := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		// only accept the HTTP CONNECT method. This should be set by the http client
		if req.Method != http.MethodConnect {
			http.Error(writer, "expected CONNECT request", http.StatusBadRequest)

			return
		}
		proxied.Add(1)

		// connect to the target
		dest, err := net.Dial("tcp", req.Host)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadGateway)

			return
		}
		defer dest.Close()

		// hijacker stops the default handling and lifecycle management over the connection
		hijacker, ok := writer.(http.Hijacker)
		if !ok {
			http.Error(writer, "hijacking not supported", http.StatusInternalServerError)

			return
		}

		clientConn, _, err := hijacker.Hijack()
		if err != nil {
			return
		}
		defer clientConn.Close()

		// this is the header for the proxy connection establishment
		fmt.Fprint(clientConn, "HTTP/1.1 200 Connection Established\r\n\r\n")

		// continuously copy the proxy client request to the target
		go func() {
			_, _ = io.Copy(dest, clientConn)
		}()

		// continuously copy the target responses to client
		_, _ = io.Copy(clientConn, dest)
	}))
	defer proxy.Close()

	var output strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// since the proxy actually does proxying, this should get the result of the target
	// the proxy is using http, there are no certificates to verify about proxy.
	// the target is using https, but we are not in the --certificate mode where we perform certificate checks
	code := Check(ctx, &output, []string{
		"check_http", "-H", targetURL.Host, "-S", "--sni",
		"--proxy", proxy.URL, "-s", "proxied-target-response",
	})

	assert.Equalf(t, OK, code, "expected exit code OK (0), got %d, output: %s", code, output.String())
	assert.GreaterOrEqualf(t, proxied.Load(), int64(1), "expected a CONNECT request to go through the proxy, output: %s", output.String())
	assert.Containsf(t, output.String(), "proxied-target-response", "expected output to contain 'proxied-target-response'")
}

func TestMakeProxyTLSConfig(t *testing.T) {
	opts := &commandOpts{}
	opts.tlsMinVersion = tls.VersionTLS12
	opts.tlsMaxVersion = tls.VersionTLS13

	proxyURL, err := url.Parse("https://proxy.example.com:8080")
	require.NoError(t, err)

	// the default config to the proxy, when its using TLS is to
	// NOT skip TLS verification, and PERFORM the TLS verification
	conf := makeProxyTLSConfig(opts, proxyURL)
	assert.Equal(t, "proxy.example.com", conf.ServerName)
	assert.False(t, conf.InsecureSkipVerify)
	assert.Equal(t, uint16(tls.VersionTLS12), conf.MinVersion)
	assert.Equal(t, uint16(tls.VersionTLS13), conf.MaxVersion)
}

func TestHTTPProxySSLSelfSignedProxy(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping network test in short mode")
	}

	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "target-response")
	}))
	defer target.Close()

	targetURL, err := url.Parse(target.URL)
	require.NoError(t, err)

	// a TLS proxy with a self-signed certificate (httptest.NewTLSServer default).
	// The CONNECT-tunnel handler is never reached in this test: the client
	// fails to verify the proxy certificate during the TLS handshake.
	proxy := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodConnect {
			http.Error(writer, "expected CONNECT request", http.StatusBadRequest)

			return
		}

		dest, err := net.Dial("tcp", req.Host)
		if err != nil {
			http.Error(writer, err.Error(), http.StatusBadGateway)

			return
		}
		defer dest.Close()

		hijacker, ok := writer.(http.Hijacker)
		if !ok {
			http.Error(writer, "hijacking not supported", http.StatusInternalServerError)

			return
		}

		clientConn, _, err := hijacker.Hijack()
		if err != nil {
			return
		}
		defer clientConn.Close()

		fmt.Fprint(clientConn, "HTTP/1.1 200 Connection Established\r\n\r\n")

		go func() {
			_, _ = io.Copy(dest, clientConn)
		}()
		_, _ = io.Copy(clientConn, dest)
	}))
	defer proxy.Close()

	var output strings.Builder
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// the proxy is using HTTPS, so the proxy's HTTPS connection should be verified
	// the target is using HTTP, but before reaching the target, the proxy TLS check fails
	code := Check(ctx, &output, []string{
		"check_http", "-H", targetURL.Host, "--proxy", proxy.URL, "-u", "/",
	})

	assert.Equalf(t, CRITICAL, code, "expected exit code CRITICAL (2), got %d, output: %s", code, output.String())
	assert.Containsf(t, output.String(), "failed to verify certificate", "expected a proxy certificate verification error, output: %s", output.String())
	assert.Containsf(t, output.String(), "unknown authority", "expected an untrusted certificate error, output: %s", output.String())
}
