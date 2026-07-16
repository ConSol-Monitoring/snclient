package snclient

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	updateSignatureTestPublicKey = "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEyrMXXwiKA1GJ4J04JuivIeznP4hvVJbSNQVRRs9HvJsOP1HJFnqwcjvFtaZtArURBFc1KVzN4+HYRVACtAdCFw=="
	updateSignatureTestVector    = "MEQCIHFVLuEjKUx0fvKClMYd/LpBrfJZn/drWgBIKvZJKd02AiA6qoJTbJlNtK2Ao3hHcboTGf/y2KSy+K//5ie7ClP7iw=="
)

func TestVerifyUpdateSignature(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	updatePath := t.TempDir() + "/snclient.rpm"
	updateData := []byte("signed update package")
	require.NoError(t, os.WriteFile(updatePath, updateData, 0o600))

	digest := sha256.Sum256(updateData)
	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, digest[:])
	require.NoError(t, err)

	require.NoError(t, verifyUpdateSignatureWithKey(updatePath, signature, &privateKey.PublicKey))

	require.NoError(t, os.WriteFile(updatePath, []byte("tampered update package"), 0o600))
	err = verifyUpdateSignatureWithKey(updatePath, signature, &privateKey.PublicKey)
	assert.ErrorContains(t, err, "signature verification failed")
}

func TestUpdatePublicKey(t *testing.T) {
	publicKey, err := updatePublicKey()
	require.NoError(t, err)
	assert.Equal(t, elliptic.P256(), publicKey.Curve)
}

func TestVerifyUpdateSignatureCompatibilityVector(t *testing.T) {
	publicKeyDER, err := base64.StdEncoding.DecodeString(updateSignatureTestPublicKey)
	require.NoError(t, err)
	parsedKey, err := x509.ParsePKIXPublicKey(publicKeyDER)
	require.NoError(t, err)
	publicKey, ok := parsedKey.(*ecdsa.PublicKey)
	require.True(t, ok)
	signature, err := base64.StdEncoding.DecodeString(updateSignatureTestVector)
	require.NoError(t, err)

	updatePath := t.TempDir() + "/update.pkg"
	require.NoError(t, os.WriteFile(updatePath, []byte("update package"), 0o600))
	require.NoError(t, verifyUpdateSignatureWithKey(updatePath, signature, publicKey))
}
