// Package oci provides helpers for interacting with OCI-compliant container registries.
package oci

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"maps"
	"strings"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// Client wraps an OCI registry client for manifest operations.
type Client struct {
	registryURL string
}

// NewClient creates a new OCI registry client for the given registry URL.
func NewClient(registryURL string) *Client {
	return &Client{registryURL: registryURL}
}

// FetchManifest pulls the image for the given OCI reference.
func (c *Client) FetchManifest(ctx context.Context, ref string) (v1.Image, error) {
	_ = c.registryURL // used for configuration, may be used for auth in future

	r, err := name.ParseReference(ref)
	if err != nil {
		return nil, fmt.Errorf("failed to parse reference %s: %w", ref, err)
	}

	img, err := remote.Image(r, remote.WithContext(ctx), remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch image: %w", err)
	}

	return img, nil
}

// FetchManifestByDigest pulls the image by digest reference.
func (c *Client) FetchManifestByDigest(ctx context.Context, imageRef string, digest string) (v1.Image, error) {
	fullRef := imageRef + "@" + digest
	return c.FetchManifest(ctx, fullRef)
}

// LayerFile represents a file extracted from an OCI image layer.
type LayerFile struct {
	Name    string
	Content []byte
}

// ExtractLayers extracts specified files from an OCI image's layers.
func ExtractLayers(img v1.Image, fileNames []string) ([]LayerFile, error) {
	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("failed to get layers: %w", err)
	}

	want := make(map[string]bool)
	for _, n := range fileNames {
		want[n] = true
	}
	found := make(map[string][]byte)

	for _, layer := range layers {
		rc, err := layer.Compressed()
		if err != nil {
			continue
		}

		contents, err := readTarGzLayer(rc, want)
		rc.Close()
		if err != nil {
			continue
		}

		maps.Copy(found, contents)
	}

	var result []LayerFile
	for _, fileName := range fileNames {
		content, ok := found[fileName]
		if !ok {
			return nil, fmt.Errorf("file %s not found in image layers", fileName)
		}
		result = append(result, LayerFile{Name: fileName, Content: content})
	}

	return result, nil
}

func readTarGzLayer(rc io.ReadCloser, want map[string]bool) (map[string][]byte, error) {
	gzr, err := gzip.NewReader(rc)
	if err != nil {
		return nil, err
	}
	defer gzr.Close()

	result := make(map[string][]byte)
	tr := tar.NewReader(gzr)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		baseName := hdr.Name
		if idx := strings.LastIndex(baseName, "/"); idx >= 0 {
			baseName = baseName[idx+1:]
		}

		if want[baseName] {
			data, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			result[baseName] = data
		}
	}

	return result, nil
}
