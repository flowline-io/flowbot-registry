package oci

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
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
	ref, err := name.ParseReference(refStr)
	if err != nil {
		return fmt.Errorf("parse reference %s: %w", refStr, err)
	}

	if _, err := remote.Head(ref, remote.WithContext(ctx), remote.WithAuthFromKeychain(authn.DefaultKeychain)); err != nil {
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

	if err := remote.Write(sigTag, img, remote.WithContext(ctx), remote.WithAuthFromKeychain(authn.DefaultKeychain)); err != nil {
		return fmt.Errorf("push signature: %w", err)
	}

	return nil
}
