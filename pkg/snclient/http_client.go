package snclient

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"

	"pkg/utils"
)

var DefaultHTTPClientConfig = ConfigData{
	"insecure":            "false",
	"tls min version":     "tls1.2",
	"request timeout":     "60",
	"username":            "",
	"password":            "",
	"client certificates": "",
}

type HTTPClientOptions struct {
	tlsConfig  *tls.Config
	reqTimeout int64
	user       string
	password   string
}

func (snc *Agent) httpClient(options *HTTPClientOptions) *http.Client {
	timeout := time.Duration(options.reqTimeout) * time.Second
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: options.tlsConfig,
			Dial: (&net.Dialer{
				Timeout: timeout,
			}).Dial,
			ResponseHeaderTimeout: timeout,
			TLSHandshakeTimeout:   timeout,
			IdleConnTimeout:       timeout,
		},
	}

	return client
}

func (snc *Agent) httpDo(ctx context.Context, options *HTTPClientOptions, method, url string, header map[string]string) (*http.Response, error) {
	client := snc.httpClient(options)
	req, err := http.NewRequestWithContext(ctx, method, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("new request: %s", err.Error())
	}

	// authenticated?
	if options.user != "" {
		req.SetBasicAuth(options.user, options.password)
	}

	// set optional headers
	for key, val := range header {
		req.Header.Add(key, val)
	}

	log.Tracef("http %s %s", method, url)
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http fetch failed %s: %s", url, err.Error())
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http fetch failed %s: %s", url, resp.Status)
	}

	return resp, nil
}

// create http client options from config section
func (snc *Agent) buildClientHTTPOptions(section *ConfigSection) (options *HTTPClientOptions, err error) {
	options = &HTTPClientOptions{
		tlsConfig: &tls.Config{
			InsecureSkipVerify: false,
			MinVersion:         tls.VersionTLS12,
		},
	}

	// skip certificate verification
	insecure, ok, err := section.GetBool("insecure")
	switch {
	case err != nil:
		return nil, fmt.Errorf("insecure: %s", err.Error())
	case ok:
		options.tlsConfig.InsecureSkipVerify = insecure
	}

	// tls minimum version
	if tlsMin, ok2 := section.GetString("tls min version"); ok2 {
		min, err2 := utils.ParseTLSMinVersion(tlsMin)
		if err2 != nil {
			return nil, fmt.Errorf("tls min version: %s", err2.Error())
		}
		options.tlsConfig.MinVersion = min
	}

	// client certificate authentication
	clientCert, ok2 := section.GetString("client certificate")
	if ok2 {
		clientKey, _ := section.GetString("certificate key")
		clientTLSCert, err2 := tls.LoadX509KeyPair(clientCert, clientKey)
		if err2 != nil {
			return nil, fmt.Errorf("reading client certificate failed: %s", err2.Error())
		}
		options.tlsConfig.Certificates = []tls.Certificate{clientTLSCert}
	}

	// basic auth
	if user, ok2 := section.GetString("user"); ok2 {
		options.user = user
	}
	if password, ok2 := section.GetString("password"); ok2 {
		options.password = password
	}

	// request timeout
	timeout, ok, err := section.GetInt("request timeout")
	switch {
	case err != nil:
		return nil, fmt.Errorf("request timeout: %s", err.Error())
	case ok:
		options.reqTimeout = timeout
	}

	return options, nil
}
