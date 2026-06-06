package oci

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// ErrNotFound indicates a resource was not found on the registry.
var ErrNotFound = errors.New("not found")

// ArtifactFile represents a file to include in the OCI artifact.
type ArtifactFile struct {
	Name    string
	Content []byte
}

// PushArtifactOption configures a push operation.
type PushArtifactOption func(*pushArtifactConfig)

type pushArtifactConfig struct {
	auth authn.Authenticator
}

// WithAuth sets the authenticator for the push operation.
func WithAuth(a authn.Authenticator) PushArtifactOption {
	return func(cfg *pushArtifactConfig) {
		cfg.auth = a
	}
}

// PushArtifact creates tar.gz OCI layers from files and pushes an OCI artifact.
func (c *Client) PushArtifact(ctx context.Context, refStr string, files []ArtifactFile, opts ...PushArtifactOption) (v1.Hash, error) {
	cfg := &pushArtifactConfig{}
	for _, o := range opts {
		o(cfg)
	}

	ref, err := name.ParseReference(refStr)
	if err != nil {
		return v1.Hash{}, fmt.Errorf("parse reference %s: %w", refStr, err)
	}

	var layers []v1.Layer
	for _, f := range files {
		layer, err := fileToLayer(f)
		if err != nil {
			return v1.Hash{}, fmt.Errorf("create layer for %s: %w", f.Name, err)
		}
		layers = append(layers, layer)
	}

	if len(layers) == 0 {
		return v1.Hash{}, errors.New("no files to push")
	}

	img, err := mutate.Append(empty.Image, mutate.Addendum{Layer: layers[0], MediaType: types.OCILayer})
	if err != nil {
		return v1.Hash{}, fmt.Errorf("append first layer: %w", err)
	}

	for _, layer := range layers[1:] {
		img, err = mutate.Append(img, mutate.Addendum{Layer: layer, MediaType: types.OCILayer})
		if err != nil {
			return v1.Hash{}, fmt.Errorf("append layer: %w", err)
		}
	}

	img = mutate.MediaType(img, types.OCIManifestSchema1)
	img = mutate.ConfigMediaType(img, types.OCIConfigJSON)

	annotations := map[string]string{
		"org.opencontainers.image.created": time.Now().UTC().Format(time.RFC3339),
		"org.opencontainers.image.version": ref.Identifier(),
		"org.opencontainers.image.title":   ref.Context().RepositoryStr(),
	}
	if v, ok := mutate.Annotations(img, annotations).(v1.Image); ok {
		img = v
	} else {
		return v1.Hash{}, fmt.Errorf("annotations: expected v1.Image")
	}

	remoteOpts := []remote.Option{remote.WithContext(ctx)}
	if c.plainHTTP {
		remoteOpts = append(remoteOpts, remote.WithTransport(c.Transport()))
	}
	if cfg.auth != nil {
		remoteOpts = append(remoteOpts, remote.WithAuth(cfg.auth))
	} else {
		remoteOpts = append(remoteOpts, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	}

	if err := remote.Write(ref, img, remoteOpts...); err != nil {
		return v1.Hash{}, fmt.Errorf("write image: %w", err)
	}

	digest, err := img.Digest()
	if err != nil {
		return v1.Hash{}, fmt.Errorf("get digest: %w", err)
	}

	return digest, nil
}

// HeadManifest checks if a manifest exists at the given reference.
func (c *Client) HeadManifest(ctx context.Context, refStr string) (v1.Hash, error) {
	ref, err := name.ParseReference(refStr)
	if err != nil {
		return v1.Hash{}, fmt.Errorf("parse reference %s: %w", refStr, err)
	}

	remoteOpts := []remote.Option{remote.WithContext(ctx), remote.WithAuthFromKeychain(authn.DefaultKeychain)}
	if c.plainHTTP {
		remoteOpts = append(remoteOpts, remote.WithTransport(c.Transport()))
	}

	desc, err := remote.Head(ref, remoteOpts...)
	if err != nil {
		return v1.Hash{}, fmt.Errorf("%w: %w", ErrNotFound, err)
	}

	return desc.Digest, nil
}

func fileToLayer(f ArtifactFile) (v1.Layer, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	if err := tw.WriteHeader(&tar.Header{
		Name: f.Name,
		Size: int64(len(f.Content)),
		Mode: 0o644,
	}); err != nil {
		return nil, fmt.Errorf("write tar header: %w", err)
	}

	if _, err := tw.Write(f.Content); err != nil {
		return nil, fmt.Errorf("write tar content: %w", err)
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("close tar: %w", err)
	}
	if err := gw.Close(); err != nil {
		return nil, fmt.Errorf("close gzip: %w", err)
	}

	return static.NewLayer(buf.Bytes(), types.OCILayer), nil
}
