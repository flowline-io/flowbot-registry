package sign

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateTestKey(t *testing.T, dir string) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	keyBytes := x509.MarshalPKCS1PrivateKey(key)
	keyPath := filepath.Join(dir, "cosign.key")
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyBytes,
	}), 0o600))

	return keyPath
}

func TestNewSignerValidKey(t *testing.T) {
	dir := t.TempDir()
	keyPath := generateTestKey(t, dir)

	signer, err := NewSigner(keyPath, "")
	require.NoError(t, err)
	assert.NotNil(t, signer)
}

func TestNewSignerMissingFile(t *testing.T) {
	_, err := NewSigner("/nonexistent/key.pem", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read key file")
}

func TestNewSignerInvalidPEM(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "bad.key")
	require.NoError(t, os.WriteFile(keyPath, []byte("not a pem file"), 0o600))

	_, err := NewSigner(keyPath, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "PEM")
}

func TestSign(t *testing.T) {
	dir := t.TempDir()
	keyPath := generateTestKey(t, dir)

	signer, err := NewSigner(keyPath, "")
	require.NoError(t, err)

	result, err := signer.Sign("ghcr.io/test/plugin@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855")
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Payload)
	assert.NotEmpty(t, result.Signature)
}
