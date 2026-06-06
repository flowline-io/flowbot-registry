package oci

import (
	"context"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRegistry(t *testing.T) (string, func()) {
	t.Helper()
	srv := httptest.NewServer(registry.New())
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	return u.Host, srv.Close
}

func TestHeadManifestFound(t *testing.T) {
	host, registryClose := setupTestRegistry(t)
	defer registryClose()

	c := NewClient(host)
	ref := host + "/test/plugin:v1"

	img, err := random.Image(256, 1)
	require.NoError(t, err)
	r, err := name.ParseReference(ref)
	require.NoError(t, err)
	require.NoError(t, remote.Write(r, img))

	digest, err := c.HeadManifest(context.Background(), ref)
	require.NoError(t, err)
	assert.NotEmpty(t, digest.String())
}

func TestHeadManifestNotFound(t *testing.T) {
	host, registryClose := setupTestRegistry(t)
	defer registryClose()

	c := NewClient(host)
	ref := host + "/test/nonexistent:latest"

	_, err := c.HeadManifest(context.Background(), ref)
	require.Error(t, err)
}

func TestPushArtifactAndPullBack(t *testing.T) {
	host, registryClose := setupTestRegistry(t)
	defer registryClose()

	c := NewClient(host)
	ref := host + "/test/push-artifact:v1"

	files := []ArtifactFile{
		{Name: "plugin.yaml", Content: []byte("name: test\nversion: v1\nruntime: grpc\ngrpc:\n  binary: ./plugin-server\n")},
		{Name: "plugin-server", Content: []byte("binary-content")},
	}

	_, err := c.PushArtifact(context.Background(), ref, files)
	require.NoError(t, err)

	img, err := c.FetchManifest(context.Background(), ref)
	require.NoError(t, err)

	layers, err := ExtractLayers(img, []string{"plugin.yaml"})
	require.NoError(t, err)
	require.Len(t, layers, 1)
	assert.Equal(t, "plugin.yaml", layers[0].Name)
	assert.Equal(t, "name: test\nversion: v1\nruntime: grpc\ngrpc:\n  binary: ./plugin-server\n", string(layers[0].Content))
}
