// Package service contains business logic for authentication and plugin operations.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/flowline-io/flowbot-registry/pkg/jwt"
	"github.com/flowline-io/flowbot-registry/pkg/manifest"
	"github.com/flowline-io/flowbot-registry/pkg/oci"
	"github.com/flowline-io/flowbot-registry/pkg/store"
)

// ErrForbidden is returned when the caller lacks permission.
var ErrForbidden = errors.New("forbidden")

// ErrNotFound is returned when a requested resource is not found.
var ErrNotFound = errors.New("not found")

// ErrInvalidInput is returned when the request contains invalid data.
var ErrInvalidInput = errors.New("invalid input")

// ScopeEntry represents a parsed Docker registry scope.
type ScopeEntry struct {
	Type    string
	Name    string
	Actions []string
}

// ParseScopes parses Docker registry v2 token scope strings.
// Example input: "repository:namespace/name:pull,push repository:ns2/plugin:pull"
func ParseScopes(raw string) ([]ScopeEntry, error) {
	if raw == "" {
		return nil, nil
	}

	var scopes []ScopeEntry
	for part := range strings.SplitSeq(raw, " ") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		c1 := strings.Index(part, ":")
		if c1 < 0 {
			return nil, fmt.Errorf("invalid scope format: %s", part)
		}

		c2 := strings.Index(part[c1+1:], ":")
		if c2 < 0 {
			return nil, fmt.Errorf("invalid scope format: %s", part)
		}

		scope := ScopeEntry{}
		scope.Type = part[:c1]
		scope.Name = part[c1+1 : c1+1+c2]
		scope.Actions = strings.Split(part[c1+1+c2+1:], ",")

		scopes = append(scopes, scope)
	}

	return scopes, nil
}

// AuthService handles Docker Registry v2 token authentication.
type AuthService struct {
	jwtSvc *jwt.TokenService
	store  *store.Adapter
}

// NewAuthService creates a new AuthService.
func NewAuthService(jwtSvc *jwt.TokenService, a *store.Adapter) *AuthService {
	return &AuthService{jwtSvc: jwtSvc, store: a}
}

// IssueJWT issues a JWT token for the given service and scopes after validating the user's namespace access.
func (s *AuthService) IssueJWT(ctx context.Context, service string, clientID string, rawScope string) (*jwt.TokenResponse, error) {
	scopes, err := ParseScopes(rawScope)
	if err != nil {
		return nil, fmt.Errorf("invalid scope: %w", err)
	}

	var accesses []jwt.AccessEntry
	for _, sc := range scopes {
		nsName := sc.Name
		if idx := strings.Index(nsName, "/"); idx >= 0 {
			nsName = nsName[:idx]
		}

		_, err := s.store.NamespaceGetByName(ctx, nsName)
		if err != nil {
			slog.Warn("auth: namespace not found",
				"namespace", nsName, "error", err, "client_id", clientID,
			)
			return nil, fmt.Errorf("%w: namespace %s: %w", ErrForbidden, nsName, err)
		}

		accesses = append(accesses, jwt.AccessEntry{
			Type:    sc.Type,
			Name:    sc.Name,
			Actions: sc.Actions,
		})
	}

	return s.jwtSvc.GenerateToken(service, accesses, clientID)
}

// PluginService handles plugin publishing operations.
type PluginService struct {
	store       *store.Adapter
	ociClient   *oci.Client
	registryURL string
}

// NewPluginService creates a new PluginService.
func NewPluginService(a *store.Adapter, ociClient *oci.Client, registryURL string) *PluginService {
	return &PluginService{store: a, ociClient: ociClient, registryURL: registryURL}
}

// PublishRequest contains the data required to publish a plugin version.
type PublishRequest struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	OciDigest string `json:"oci_digest"`
}

// PublishResult contains the result of a plugin publish operation.
type PublishResult struct {
	PluginID        int  `json:"plugin_id"`
	PluginVersionID int  `json:"plugin_version_id"`
	Created         bool `json:"created"`
}

// Publish fetches the OCI manifest, extracts plugin.yaml and README.md, validates,
// and upserts the database records.
func (s *PluginService) Publish(ctx context.Context, req PublishRequest) (*PublishResult, error) {
	logger := slog.With("namespace", req.Namespace, "name", req.Name, "version", req.Version)

	logger.Info("publish: started", "digest", req.OciDigest)

	rawManifest, rawReadme, err := s.fetchOCILayers(ctx, req)
	if err != nil {
		logger.Error("publish: fetch OCI layers failed", "error", err)
		return nil, err
	}

	logger.Debug("publish: layers extracted", "manifest_len", len(rawManifest), "readme_len", len(rawReadme))

	m, err := manifest.ParseManifest(rawManifest)
	if err != nil {
		logger.Error("publish: parse manifest failed", "error", err)
		return nil, fmt.Errorf("parse plugin.yaml: %w", err)
	}

	if m.Version != req.Version {
		logger.Warn("publish: version mismatch",
			"manifest_version", m.Version, "request_version", req.Version,
		)
		return nil, fmt.Errorf("%w: version mismatch: manifest=%s request=%s", ErrInvalidInput, m.Version, req.Version)
	}

	manifestName := m.Name
	if idx := strings.LastIndex(manifestName, "/"); idx >= 0 {
		manifestName = manifestName[idx+1:]
	}
	if manifestName != req.Name {
		logger.Warn("publish: name mismatch",
			"manifest_name", m.Name, "request_name", req.Name,
		)
		return nil, fmt.Errorf("%w: manifest name %q does not match request name %q", ErrInvalidInput, m.Name, req.Name)
	}

	manifestData := map[string]any{
		"name":        m.Name,
		"version":     m.Version,
		"description": m.Description,
		"author":      m.Author,
	}

	result, err := s.upsertRecords(ctx, req, manifestData, string(rawReadme))
	if err != nil {
		logger.Error("publish: upsert failed", "error", err)
		return nil, err
	}

	logger.Info("publish: success",
		"plugin_id", result.PluginID,
		"plugin_version_id", result.PluginVersionID,
		"created", result.Created,
	)

	return result, nil
}

func (s *PluginService) fetchOCILayers(ctx context.Context, req PublishRequest) ([]byte, []byte, error) {
	ociRef := fmt.Sprintf("%s/%s/%s", s.registryURL, req.Namespace, req.Name)

	slog.Debug("publish: fetching OCI manifest", "ref", ociRef, "digest", req.OciDigest)

	img, err := s.ociClient.FetchManifestByDigest(ctx, ociRef, req.OciDigest)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch oci manifest: %w", err)
	}

	layers, err := oci.ExtractLayers(img, []string{"plugin.yaml", "README.md"})
	if err != nil {
		return nil, nil, fmt.Errorf("extract layers: %w", err)
	}

	var rawManifest, rawReadme []byte
	for _, lf := range layers {
		switch lf.Name {
		case "plugin.yaml":
			rawManifest = lf.Content
		case "README.md":
			rawReadme = lf.Content
		}
	}

	if rawManifest == nil {
		return nil, nil, fmt.Errorf("%w: plugin.yaml not found in OCI image layers", ErrInvalidInput)
	}

	return rawManifest, rawReadme, nil
}

func (s *PluginService) upsertRecords(ctx context.Context, req PublishRequest, manifestData map[string]any, readmeHTML string) (*PublishResult, error) {
	slog.Debug("publish: beginning transaction")

	tx, err := s.store.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	ns, err := tx.NamespaceGetByName(ctx, req.Namespace)
	if errors.Is(err, store.ErrNotFound) {
		slog.Debug("publish: creating namespace", "namespace", req.Namespace)
		ns, err = tx.NamespaceCreate(ctx, req.Namespace, "user")
	}
	if err != nil {
		return nil, fmt.Errorf("namespace: %w", err)
	}

	p, err := tx.PluginGetByNamespaceAndName(ctx, ns.ID, req.Name)
	if errors.Is(err, store.ErrNotFound) {
		slog.Debug("publish: creating plugin", "namespace", req.Namespace, "name", req.Name)
		p, err = tx.PluginCreate(ctx, ns.ID, req.Name, "", "", "")
	}
	if err != nil {
		return nil, fmt.Errorf("plugin: %w", err)
	}

	imageRef := fmt.Sprintf("%s/%s/%s:%s", s.registryURL, req.Namespace, req.Name, req.Version)

	created := false
	pv, err := tx.PluginVersionGetByPluginAndVersion(ctx, p.ID, req.Version)
	if errors.Is(err, store.ErrNotFound) {
		slog.Debug("publish: creating plugin version", "version", req.Version, "digest", req.OciDigest)
		pv, err = tx.PluginVersionCreate(ctx, p.ID, req.Version, imageRef, req.OciDigest, readmeHTML, manifestData)
		created = true
	}
	if err != nil {
		return nil, fmt.Errorf("plugin version: %w", err)
	}

	if !created {
		slog.Debug("publish: updating plugin version", "id", pv.ID)
		err = tx.PluginVersionUpdate(ctx, pv.ID, imageRef, req.OciDigest, readmeHTML, manifestData)
		if err != nil {
			return nil, fmt.Errorf("plugin version update: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		slog.Error("publish: commit failed", "error", err)
		return nil, fmt.Errorf("commit transaction: %w", err)
	}

	slog.Debug("publish: transaction committed")

	return &PublishResult{
		PluginID:        p.ID,
		PluginVersionID: pv.ID,
		Created:         created,
	}, nil
}

// ListPlugins searches plugins by query string.
func (s *PluginService) ListPlugins(ctx context.Context, query string, limit int, offset int) ([]store.PluginRecord, int, error) {
	return s.store.PluginList(ctx, query, limit, offset)
}

// GetPluginVersion retrieves a specific plugin version by namespace, name, and version.
func (s *PluginService) GetPluginVersion(ctx context.Context, namespace string, name string, version string) (*store.PluginVersionRecord, error) {
	ns, err := s.store.NamespaceGetByName(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("namespace: %w", err)
	}

	p, err := s.store.PluginGetByNamespaceAndName(ctx, ns.ID, name)
	if err != nil {
		return nil, fmt.Errorf("plugin: %w", err)
	}

	pv, err := s.store.PluginVersionGetByPluginAndVersion(ctx, p.ID, version)
	if err != nil {
		return nil, fmt.Errorf("version: %w", err)
	}

	return pv, nil
}
