package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

// writeTestPrivateKey generates a temporary RSA private key and returns the file path.
func writeTestPrivateKey(t *testing.T) string {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err, "failed to generate RSA key")

	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err, "failed to marshal private key")

	block := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: der,
	}

	keyPath := filepath.Join(t.TempDir(), "test-private.pem")
	err = os.WriteFile(keyPath, pem.EncodeToMemory(block), 0o600)
	require.NoError(t, err, "failed to write test private key")

	return keyPath
}

// TestAllModules verifies the full fx dependency graph resolves.
func TestAllModules(t *testing.T) {
	t.Parallel()

	keyPath := writeTestPrivateKey(t)

	app := fxtest.New(t,
		fx.Options(
			AllModules(),
			fx.Decorate(func(v *viper.Viper) *viper.Viper {
				v.Set("auth.jwt_private_key_path", keyPath)
				return v
			}),
		),
	)
	app.RequireStart()
	app.RequireStop()
}

// TestAllModulesOption verifies AllModules returns a valid fx.Option.
func TestAllModulesOption(t *testing.T) {
	t.Parallel()

	assert.NotNil(t, AllModules())
	_ = fx.Options(AllModules())
}
