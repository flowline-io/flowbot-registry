package oci

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPushSignature(t *testing.T) {
	host, close := setupTestRegistry(t)
	defer close()

	c := NewClient(host)

	ref := host + "/test/signed-plugin:v1"
	files := []ArtifactFile{
		{Name: "plugin.yaml", Content: []byte("name: test-sig\nversion: v1\nruntime: grpc\ngrpc:\n  binary: ./plugin-server\n")},
		{Name: "plugin-server", Content: []byte("fake-binary")},
	}
	_, err := c.PushArtifact(context.Background(), ref, files)
	require.NoError(t, err)

	err = c.PushSignature(context.Background(), ref, []byte("sig-payload"), []byte("sig-data"))
	require.NoError(t, err)
}

func TestPushSignatureNoImage(t *testing.T) {
	host, close := setupTestRegistry(t)
	defer close()

	c := NewClient(host)
	ref := host + "/test/no-image:v99"

	err := c.PushSignature(context.Background(), ref, []byte("payload"), []byte("sig"))
	require.Error(t, err)
}
