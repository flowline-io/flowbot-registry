package oci

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"time"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

const (
	signatureMediaType = "application/vnd.dev.cosign.simplesigning.v1+json"
	signatureTagSuffix = ".sig"
)

// PushSignature pushes a Cosign signature as an OCI artifact tagged with the sig suffix.
func (c *Client) PushSignature(ctx context.Context, refStr string, payload []byte, signature []byte) error {
	_ = payload // accepted for API completeness
	ref, err := name.ParseReference(refStr)
	if err != nil {
		return fmt.Errorf("parse reference %s: %w", refStr, err)
	}

	remoteOpts := []remote.Option{remote.WithContext(ctx), remote.WithAuthFromKeychain(authn.DefaultKeychain)}
	if c.plainHTTP {
		remoteOpts = append(remoteOpts, remote.WithTransport(c.Transport()))
	}

	if _, err := remote.Head(ref, remoteOpts...); err != nil {
		return fmt.Errorf("source image not found: %w", err)
	}

	sigTag := ref.Context().Tag(ref.Identifier() + signatureTagSuffix)

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	if err := tw.WriteHeader(&tar.Header{
		Name: "cosign.sig",
		Size: int64(len(signature)),
		Mode: 0o644,
	}); err != nil {
		return fmt.Errorf("write sig tar header: %w", err)
	}
	if _, err := tw.Write(signature); err != nil {
		return fmt.Errorf("write sig content: %w", err)
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("close tar: %w", err)
	}
	if err := gw.Close(); err != nil {
		return fmt.Errorf("close gzip: %w", err)
	}

	layer := static.NewLayer(buf.Bytes(), types.MediaType(signatureMediaType))

	img, err := mutate.Append(empty.Image, mutate.Addendum{Layer: layer, MediaType: types.MediaType(signatureMediaType)})
	if err != nil {
		return fmt.Errorf("create signature image: %w", err)
	}

	img = mutate.MediaType(img, types.OCIManifestSchema1)
	img = mutate.ConfigMediaType(img, types.OCIConfigJSON)

	annotations := map[string]string{
		"org.opencontainers.image.created": time.Now().UTC().Format(time.RFC3339),
		"org.opencontainers.image.version": ref.Identifier() + signatureTagSuffix,
		"org.opencontainers.image.title":   ref.Context().RepositoryStr(),
	}
	img = mutate.Annotations(img, annotations).(v1.Image)

	if err := remote.Write(sigTag, img, remoteOpts...); err != nil {
		return fmt.Errorf("push signature: %w", err)
	}

	return nil
}
