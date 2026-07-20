package snclient

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"io"
	"os"
)

const (
	updateSignatureMaxSize = 1024

	// Fork validation key. Upstream releases must replace it with a maintainer-controlled key.
	updatePublicKeyBase64 = "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAELNoFBtr9CSXY9910vzXBiSfctTh9iq3/VOOBf+4EQGbj17xG4FLd0pCgOxLBi40sagqmGJrJAQs07iPIHk0qrg=="
)

func verifyUpdateSignature(updatePath, signaturePath string) error {
	publicKey, err := updatePublicKey()
	if err != nil {
		return err
	}

	signatureFile, err := os.Open(signaturePath)
	if err != nil {
		return fmt.Errorf("reading update signature: %s", err.Error())
	}
	defer signatureFile.Close()
	signatureData, err := io.ReadAll(io.LimitReader(signatureFile, updateSignatureMaxSize+1))
	if err != nil {
		return fmt.Errorf("reading update signature: %s", err.Error())
	}
	if len(signatureData) > updateSignatureMaxSize {
		return fmt.Errorf("update signature exceeds maximum size of %d bytes", updateSignatureMaxSize)
	}

	return verifyUpdateSignatureWithKey(updatePath, signatureData, publicKey)
}

func verifyUpdateSignatureWithKey(updatePath string, signature []byte, publicKey *ecdsa.PublicKey) error {
	file, err := os.Open(updatePath)
	if err != nil {
		return fmt.Errorf("opening update for signature verification: %s", err.Error())
	}
	defer file.Close()

	digest := sha256.New()
	if _, err = io.Copy(digest, file); err != nil {
		return fmt.Errorf("hashing update for signature verification: %s", err.Error())
	}

	if !ecdsa.VerifyASN1(publicKey, digest.Sum(nil), signature) {
		return fmt.Errorf("update signature verification failed")
	}

	return nil
}

func updatePublicKey() (*ecdsa.PublicKey, error) {
	publicKeyDER, err := base64.StdEncoding.DecodeString(updatePublicKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("decoding update public key: %s", err.Error())
	}
	parsedKey, err := x509.ParsePKIXPublicKey(publicKeyDER)
	if err != nil {
		return nil, fmt.Errorf("parsing update public key: %s", err.Error())
	}
	publicKey, ok := parsedKey.(*ecdsa.PublicKey)
	if !ok || publicKey.Curve != elliptic.P256() {
		return nil, fmt.Errorf("invalid update public key")
	}

	return publicKey, nil
}
