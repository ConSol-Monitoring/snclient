package snclient

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"strings"
	"syscall"

	"github.com/goccy/go-json"
	"github.com/subuk/csrtool/pkg/csrtool"
)

func init() {
	RegisterModule(&AvailableListeners,
		"WEBAdminServer",
		"/settings/WEBAdmin/server",
		NewHandlerAdmin,
		ConfigInit{
			ConfigData{
				"port":            "${/settings/WEB/server/port}",
				"use ssl":         "${/settings/WEB/server/use ssl}",
				"allow arguments": "true",
			},
			"/settings/default",
			DefaultListenHTTPConfig,
		},
	)
}

const DefaultPrivateKeySize = 4096

type HandlerAdmin struct {
	noCopy          noCopy
	handler         http.Handler
	password        string
	requirePassword bool
	snc             *Agent
	listener        *Listener
	allowedHosts    *AllowedHostConfig
}

type csrRequestJSON struct {
	HostName           string `json:"HostName"`
	NewKey             bool   `json:"NewKey"`
	Country            string `json:"Country"`
	State              string `json:"State"`
	Locality           string `json:"Locality"`
	Organization       string `json:"Organization"`
	OrganizationalUnit string `json:"OrganizationalUnit"`
	KeyLength          int    `json:"KeyLength"`
	ChallengePassword  string `json:"ChallengePassword"`
}

type replaceCertData struct {
	CertData string `json:"CertData"`
	KeyData  string `json:"KeyData"`
	Reload   bool   `json:"Reload"`
}

// ensure we fully implement the RequestHandlerHTTP type
var _ RequestHandlerHTTP = &HandlerAdmin{}

func NewHandlerAdmin() Module {
	l := &HandlerAdmin{}
	l.handler = &HandlerWebAdmin{Handler: l}

	return l
}

func (l *HandlerAdmin) Type() string {
	return "admin"
}

func (l *HandlerAdmin) BindString() string {
	return l.listener.BindString()
}

func (l *HandlerAdmin) Listener() *Listener {
	return l.listener
}

func (l *HandlerAdmin) Start() error {
	return l.listener.Start()
}

func (l *HandlerAdmin) Stop() {
	if l.listener != nil {
		l.listener.Stop()
	}
}

func (l *HandlerAdmin) Init(snc *Agent, conf *ConfigSection, _ *Config, runSet *AgentRunSet) error {
	l.snc = snc

	err := setListenerAuthInit(&l.password, &l.requirePassword, conf)
	if err != nil {
		return err
	}

	listener, err := SharedWebListener(snc, conf, l, runSet)
	if err != nil {
		return err
	}
	l.listener = listener

	allowedHosts, err := NewAllowedHostConfig(conf)
	if err != nil {
		return err
	}
	l.allowedHosts = allowedHosts

	return nil
}

func (l *HandlerAdmin) GetAllowedHosts() *AllowedHostConfig {
	return l.allowedHosts
}

func (l *HandlerAdmin) CheckPassword(req *http.Request, _ URLMapping) bool {
	return verifyRequestPassword(l.snc, req, l.password, l.requirePassword)
}

func (l *HandlerAdmin) GetMappings(*Agent) []URLMapping {
	return []URLMapping{
		{URL: "/api/v1/admin/*", Handler: l.handler},
	}
}

type HandlerWebAdmin struct {
	noCopy  noCopy
	Handler *HandlerAdmin
}

func (l *HandlerWebAdmin) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	path := strings.TrimSuffix(req.URL.Path, "/")
	switch path {
	case "/api/v1/admin/reload":
		l.serveReload(res, req)
	case "/api/v1/admin/certs/replace":
		l.serveCertsReplace(res, req)
	case "/api/v1/admin/csr":
		l.serveCertsCSR(res, req)
	case "/api/v1/admin/updates/install":
		l.serveUpdate(res, req)
	default:
		res.WriteHeader(http.StatusNotFound)
		LogError2(res.Write([]byte("404 - nothing here\n")))
	}
}

func (l *HandlerWebAdmin) serveCertsCSR(res http.ResponseWriter, req *http.Request) {
	if !l.requirePostMethod(res, req) {
		return
	}
	// extract json payload
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	data := csrRequestJSON{}
	err := decoder.Decode(&data)
	if err != nil {
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusBadRequest)
		LogError(json.NewEncoder(res).Encode(map[string]any{
			"success": false,
			"error":   err.Error(),
		}))

		return
	}
	if data.HostName == "" {
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusBadRequest)
		LogError(json.NewEncoder(res).Encode(map[string]any{
			"success": false,
			"error":   "expected HostName not to be empty",
		}))

		return
	}

	var privateKey *rsa.PrivateKey
	if data.NewKey {
		if data.KeyLength == 0 {
			data.KeyLength = DefaultPrivateKeySize
		}
		privateKey, err = rsa.GenerateKey(rand.Reader, data.KeyLength)
	} else {
		defaultSection := l.Handler.snc.config.Section("/settings/default")
		keyFile, ok := defaultSection.GetString("certificate key")
		if !ok {
			l.sendError(res, fmt.Errorf("could not read certificate location from config"))
		}
		privateKey, err = l.readPrivateKey(keyFile)
	}
	if err != nil {
		l.sendError(res, err)

		return
	}

	csrPEM, err := l.createCSR(&data, privateKey)
	if err != nil {
		l.sendError(res, err)

		return
	}

	if data.NewKey {
		defSection := l.Handler.snc.config.Section("/settings/default")

		keyFile, _ := defSection.GetString("certificate key")
		keyFileTemp := keyFile + ".tmp"

		privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)
		if err = os.WriteFile(keyFileTemp, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privateKeyBytes}), 0o600); err != nil {
			l.sendError(res, fmt.Errorf("failed to write certificate key file %s: %s", keyFile, err.Error()))

			return
		}
	}

	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)
	_, err = res.Write(csrPEM)
	if err != nil {
		LogError(json.NewEncoder(res).Encode(map[string]any{
			"success": false,
			"error":   err.Error(),
		}))

		return
	}
}

func (l *HandlerWebAdmin) createCSR(data *csrRequestJSON, privateKey *rsa.PrivateKey) ([]byte, error) {
	subject := pkix.Name{
		Country:            []string{data.Country},
		Province:           []string{data.State},
		Locality:           []string{data.Locality},
		Organization:       []string{data.Organization},
		OrganizationalUnit: []string{data.OrganizationalUnit},
		CommonName:         data.HostName,
	}
	csrPEM, err := csrtool.GenerateCSR(privateKey, subject, []string{}, data.ChallengePassword)
	if err != nil {
		return nil, fmt.Errorf("generate csr: %s", err.Error())
	}

	return csrPEM, nil
}

func (l *HandlerWebAdmin) readPrivateKey(keyFile string) (*rsa.PrivateKey, error) {
	// read private key
	pemData, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, fmt.Errorf("could not read file: %s", err.Error())
	}

	block, _ := pem.Decode(pemData)
	switch block.Type {
	case "RSA PRIVATE KEY":
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("could not parse private key: %s", err.Error())
		}

		return key, nil
	case "PRIVATE KEY":
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("could not parse private key: %s", err.Error())
		}
		rsaKey, ok := key.(*rsa.PrivateKey)
		if ok {
			return rsaKey, nil
		}

		return nil, fmt.Errorf("private key in wrong format, expected rsa private key but got '%T'", key)
	default:
		return nil, fmt.Errorf("private key in wrong format, got '%s' but expected 'RSA PRIVATE KEY'", block.Type)
	}
}

func (l *HandlerWebAdmin) serveReload(res http.ResponseWriter, req *http.Request) {
	if !l.requirePostMethod(res, req) {
		return
	}

	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)
	LogError(json.NewEncoder(res).Encode(map[string]any{
		"success": true,
	}))
	l.Handler.snc.osSignalChannel <- syscall.SIGHUP
}

func (l *HandlerWebAdmin) serveCertsReplace(res http.ResponseWriter, req *http.Request) {
	if !l.requirePostMethod(res, req) {
		return
	}

	// extract json payload
	decoder := json.NewDecoder(req.Body)
	decoder.DisallowUnknownFields()
	data := replaceCertData{}
	err := decoder.Decode(&data)
	if err != nil {
		res.Header().Set("Content-Type", "application/json")
		res.WriteHeader(http.StatusBadRequest)
		LogError(json.NewEncoder(res).Encode(map[string]any{
			"success": false,
			"error":   err.Error(),
		}))

		return
	}

	certBytes, keyBytes, err := l.getBytesFromReplacementStructData(res, data)
	if err != nil {
		return
	}

	defSection := l.Handler.snc.config.Section("/settings/default")
	certFile, _ := defSection.GetString("certificate")
	keyFile, _ := defSection.GetString("certificate key")
	keyFileBak := keyFile + ".tmp"
	if data.KeyData == "" && data.CertData != "" {
		newPrivateKey, pubKey, certPublicKey, err := l.getRelevantRSAKeys(keyFileBak, certBytes)
		if err != nil {
			l.sendError(res, err)

			return
		}
		if pubKey.Equal(certPublicKey) {
			privateKeyBytes := x509.MarshalPKCS1PrivateKey(newPrivateKey)
			if err = os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: privateKeyBytes}), 0o600); err != nil {
				l.sendError(res, fmt.Errorf("failed to write certificate key file %s: %s", keyFile, err.Error()))

				return
			}
			os.Remove(keyFileBak)
		}
	}

	if data.CertData != "" {
		if err := os.WriteFile(certFile, certBytes, 0o600); err != nil {
			l.sendError(res, fmt.Errorf("failed to write certificate %s: %s", certFile, err.Error()))

			return
		}
	}

	if data.KeyData != "" {
		if err := os.WriteFile(keyFile, keyBytes, 0o600); err != nil {
			l.sendError(res, fmt.Errorf("failed to write certificate key file %s: %s", keyFile, err.Error()))

			return
		}
	}

	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)
	LogError(json.NewEncoder(res).Encode(map[string]any{
		"success": true,
	}))

	if data.Reload {
		l.Handler.snc.osSignalChannel <- syscall.SIGHUP
	}
}

func (l *HandlerWebAdmin) getBytesFromReplacementStructData(res http.ResponseWriter, data replaceCertData) (certBytes, keyBytes []byte, err error) {
	if data.CertData != "" {
		certBytes, err = base64.StdEncoding.DecodeString(data.CertData)
		if err != nil {
			err = fmt.Errorf("failed to base64 decode certdata: %s", err.Error())
			l.sendError(res, err)

			return certBytes, keyBytes, err
		}
	}

	if data.KeyData != "" {
		keyBytes, err = base64.StdEncoding.DecodeString(data.KeyData)
		if err != nil {
			err = fmt.Errorf("failed to base64 decode keydata: %s", err.Error())
			l.sendError(res, err)

			return certBytes, keyBytes, err
		}
	}

	return certBytes, keyBytes, nil
}

func (l *HandlerWebAdmin) getRelevantRSAKeys(tempKeyFile string, certBytes []byte) (newPrivateKey *rsa.PrivateKey, privateKeyPublicPart, certPublicKey *rsa.PublicKey, err error) {
	newPrivateKey, err = l.readPrivateKey(tempKeyFile)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to read private key: %s", err.Error())
	}
	newPubKey := newPrivateKey.Public()
	rsaNewPublicKey, ok := newPubKey.(*rsa.PublicKey)
	if !ok {
		return nil, nil, nil, fmt.Errorf("rsa public key in wrong format")
	}
	block, _ := pem.Decode(certBytes)
	newCert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse certificate: %s", err.Error())
	}
	newCertPublicKey, ok := newCert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, nil, nil, fmt.Errorf("rsa public key from csr in wrong format")
	}

	return newPrivateKey, rsaNewPublicKey, newCertPublicKey, nil
}

func (l *HandlerWebAdmin) serveUpdate(res http.ResponseWriter, req *http.Request) {
	if !l.requirePostMethod(res, req) {
		return
	}

	task := l.Handler.snc.Tasks.Get("Updates")
	mod, ok := task.(*UpdateHandler)
	if !ok {
		l.sendError(res, fmt.Errorf("could not load update handler"))

		return
	}

	version, err := mod.CheckUpdates(req.Context(), true, true, true, false, "", "", false)
	if err != nil {
		l.sendError(res, fmt.Errorf("failed to fetch updates: %s", err.Error()))

		return
	}

	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusOK)
	if version != "" {
		LogError(json.NewEncoder(res).Encode(map[string]any{
			"success": true,
			"message": "update found and installed",
			"version": version,
		}))
	} else {
		LogError(json.NewEncoder(res).Encode(map[string]any{
			"success": true,
			"message": "no new update available",
		}))
	}
}

// check if request used method POST
func (l *HandlerWebAdmin) requirePostMethod(res http.ResponseWriter, req *http.Request) bool {
	if req.Method == http.MethodPost {
		return true
	}

	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusBadRequest)
	LogError(json.NewEncoder(res).Encode(map[string]any{
		"success": false,
		"error":   "POST method required",
	}))

	return false
}

// return error as json result
func (l *HandlerWebAdmin) sendError(res http.ResponseWriter, err error) {
	log.Debugf("admin request failed: %s", err.Error())
	res.Header().Set("Content-Type", "application/json")
	res.WriteHeader(http.StatusInternalServerError)
	LogError(json.NewEncoder(res).Encode(map[string]any{
		"success": false,
		"error":   err.Error(),
	}))
}
