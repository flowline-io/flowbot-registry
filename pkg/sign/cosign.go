// Package sign provides Cosign-based signing of OCI images.
package sign

import (
	"bytes"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/flowline-io/flowbot-registry/pkg/json"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/sigstore/sigstore/pkg/signature"
	sigPayload "github.com/sigstore/sigstore/pkg/signature/payload"
)

// SignResult holds the payload and signature from a signing operation.
type SignResult struct {
	Payload   []byte
	Signature []byte
}

// Signer signs OCI image references using Cosign with a static private key.
type Signer struct {
	verifier signature.SignerVerifier
}

// NewSigner creates a new Signer from a PEM-encoded private key.
// password is unused (reserved for encrypted key support in future).
func NewSigner(keyPath string, _ string) (*Signer, error) {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read key file: %w", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block from %s", keyPath)
	}

	var key crypto.Signer
	switch block.Type {
	case "RSA PRIVATE KEY":
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKCS1 RSA private key: %w", err)
		}
	case "PRIVATE KEY":
		k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKCS8 private key: %w", err)
		}
		var ok bool
		key, ok = k.(crypto.Signer)
		if !ok {
			return nil, fmt.Errorf("key does not implement crypto.Signer")
		}
	default:
		return nil, fmt.Errorf("unsupported PEM block type %q", block.Type)
	}

	sv, err := signature.LoadSignerVerifier(key, crypto.SHA256)
	if err != nil {
		return nil, fmt.Errorf("load signer-verifier: %w", err)
	}

	return &Signer{verifier: sv}, nil
}

// Sign signs an OCI image reference and returns the payload and signature.
func (s *Signer) Sign(imageRef string) (*SignResult, error) {
	digest, err := name.NewDigest(imageRef)
	if err != nil {
		return nil, fmt.Errorf("parse image digest: %w", err)
	}

	payload := sigPayload.Cosign{
		Image: digest,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	sig, err := s.verifier.SignMessage(bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("sign payload: %w", err)
	}

	return &SignResult{
		Payload:   payloadBytes,
		Signature: sig,
	}, nil
}
