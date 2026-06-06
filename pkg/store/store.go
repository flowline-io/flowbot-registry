package store

import (
	"context"
	"fmt"
	"time"

	"github.com/flowline-io/flowbot-registry/internal/ent"
	"github.com/flowline-io/flowbot-registry/internal/ent/namespace"
	"github.com/flowline-io/flowbot-registry/internal/ent/plugin"
	"github.com/flowline-io/flowbot-registry/internal/ent/pluginversion"
	"github.com/flowline-io/flowbot-registry/internal/ent/refreshtoken"
	"github.com/flowline-io/flowbot-registry/internal/ent/user"
)

// NamespaceRecord represents a namespace entity.
type NamespaceRecord struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	UserID *int   `json:"user_id,omitempty"`
}

// PluginRecord represents a plugin entity.
type PluginRecord struct {
	ID          int    `json:"id"`
	NamespaceID int    `json:"namespace_id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
	LogoURL     string `json:"logo_url"`
}

// PluginVersionRecord represents a plugin version entity.
type PluginVersionRecord struct {
	ID           int            `json:"id"`
	PluginID     int            `json:"plugin_id"`
	Version      string         `json:"version"`
	OciImageRef  string         `json:"oci_image_ref"`
	OciDigest    string         `json:"oci_digest"`
	ReadmeHTML   string         `json:"readme_html"`
	ManifestJSON map[string]any `json:"manifest_json"`
}

// UserRecord represents a user entity.
type UserRecord struct {
	ID           int       `json:"id"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// RefreshTokenRecord represents a refresh token entity.
type RefreshTokenRecord struct {
	ID        int       `json:"id"`
	UserID    int       `json:"user_id"`
	TokenHash string    `json:"-"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// StoreQuerier defines read operations for plugin store queries.
// Adapter implements this interface, enabling mock injection in tests.
type StoreQuerier interface {
	NamespaceGetByName(ctx context.Context, name string) (*NamespaceRecord, error)
	NamespaceGetByID(ctx context.Context, id int) (*NamespaceRecord, error)
	PluginGetByNamespaceAndName(ctx context.Context, namespaceID int, name string) (*PluginRecord, error)
	PluginList(ctx context.Context, query string, limit, offset int) ([]PluginRecord, int, error)
	PluginListByNamespace(ctx context.Context, namespaceID int, query string, limit, offset int) ([]PluginRecord, int, error)
	PluginVersionListByPlugin(ctx context.Context, pluginID int) ([]PluginVersionRecord, error)
	PluginVersionGetByPluginAndVersion(ctx context.Context, pluginID int, version string) (*PluginVersionRecord, error)
}

var _ StoreQuerier = (*Adapter)(nil)

// Adapter provides data access operations backed by the ent client.
type Adapter struct {
	client *ent.Client
}

// NewAdapter creates a new store adapter.
func NewAdapter(client *ent.Client) *Adapter {
	return &Adapter{client: client}
}

// TxAdapter provides transactional data access operations.
type TxAdapter struct {
	tx *ent.Tx
}

// BeginTx starts a new transaction.
func (a *Adapter) BeginTx(ctx context.Context) (*TxAdapter, error) {
	tx, err := a.client.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin transaction: %w", err)
	}
	return &TxAdapter{tx: tx}, nil
}

// Commit commits the transaction.
func (ta *TxAdapter) Commit() error {
	return ta.tx.Commit()
}

// Rollback rolls back the transaction.
func (ta *TxAdapter) Rollback() {
	_ = ta.tx.Rollback()
}

// NamespaceGetByName retrieves a namespace by its name.
func (a *Adapter) NamespaceGetByName(ctx context.Context, name string) (*NamespaceRecord, error) {
	n, err := a.client.Namespace.Query().Where(namespace.NameEQ(name)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("%w: namespace %s", ErrNotFound, name)
		}
		return nil, fmt.Errorf("query namespace: %w", err)
	}
	return &NamespaceRecord{
		ID:     n.ID,
		Name:   n.Name,
		Type:   n.Type,
		UserID: n.UserID,
	}, nil
}

// NamespaceGetByID retrieves a namespace by its ID.
func (a *Adapter) NamespaceGetByID(ctx context.Context, id int) (*NamespaceRecord, error) {
	n, err := a.client.Namespace.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("%w: namespace id %d", ErrNotFound, id)
		}
		return nil, fmt.Errorf("get namespace by id: %w", err)
	}
	return &NamespaceRecord{
		ID:     n.ID,
		Name:   n.Name,
		Type:   n.Type,
		UserID: n.UserID,
	}, nil
}

// NamespaceCreate creates a new namespace.
// userID is the owning user. Pass 0 for unowned namespaces.
func (a *Adapter) NamespaceCreate(ctx context.Context, name string, nsType string, userID int) (*NamespaceRecord, error) {
	create := a.client.Namespace.Create().
		SetName(name).
		SetType(nsType)
	if userID != 0 {
		create.SetUserID(userID)
	}
	n, err := create.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create namespace: %w", err)
	}
	return &NamespaceRecord{
		ID:     n.ID,
		Name:   n.Name,
		Type:   n.Type,
		UserID: n.UserID,
	}, nil
}

// PluginGetByNamespaceAndName retrieves a plugin by namespace and name.
func (a *Adapter) PluginGetByNamespaceAndName(ctx context.Context, namespaceID int, name string) (*PluginRecord, error) {
	p, err := a.client.Plugin.Query().
		Where(plugin.NamespaceIDEQ(namespaceID), plugin.NameEQ(name)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("%w: plugin %s", ErrNotFound, name)
		}
		return nil, fmt.Errorf("query plugin: %w", err)
	}
	return &PluginRecord{
		ID:          p.ID,
		NamespaceID: p.NamespaceID,
		Name:        p.Name,
		DisplayName: p.DisplayName,
		Description: p.Description,
		LogoURL:     p.LogoURL,
	}, nil
}

// PluginCreate creates a new plugin.
func (a *Adapter) PluginCreate(ctx context.Context, namespaceID int, name string, displayName string, description string, logoURL string) (*PluginRecord, error) {
	p, err := a.client.Plugin.Create().
		SetNamespaceID(namespaceID).
		SetName(name).
		SetDisplayName(displayName).
		SetDescription(description).
		SetLogoURL(logoURL).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create plugin: %w", err)
	}
	return &PluginRecord{
		ID:          p.ID,
		NamespaceID: p.NamespaceID,
		Name:        p.Name,
		DisplayName: p.DisplayName,
		Description: p.Description,
		LogoURL:     p.LogoURL,
	}, nil
}

// PluginUpdate updates an existing plugin's display metadata.
func (a *Adapter) PluginUpdate(ctx context.Context, id int, displayName string, description string, logoURL string) error {
	_, err := a.client.Plugin.UpdateOneID(id).
		SetDisplayName(displayName).
		SetDescription(description).
		SetLogoURL(logoURL).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("update plugin: %w", err)
	}
	return nil
}

// PluginVersionGetByPluginAndVersion retrieves a plugin version by plugin and version string.
func (a *Adapter) PluginVersionGetByPluginAndVersion(ctx context.Context, pluginID int, version string) (*PluginVersionRecord, error) {
	pv, err := a.client.PluginVersion.Query().
		Where(pluginversion.PluginIDEQ(pluginID), pluginversion.VersionEQ(version)).
		Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("%w: plugin version %s", ErrNotFound, version)
		}
		return nil, fmt.Errorf("query plugin version: %w", err)
	}
	return pluginVersionToRecord(pv), nil
}

// PluginVersionCreate creates a new plugin version.
func (a *Adapter) PluginVersionCreate(ctx context.Context, pluginID int, version string, ociRef string, ociDigest string, readmeHTML string, manifestJSON map[string]any) (*PluginVersionRecord, error) {
	pv, err := a.client.PluginVersion.Create().
		SetPluginID(pluginID).
		SetVersion(version).
		SetOciImageRef(ociRef).
		SetOciDigest(ociDigest).
		SetReadmeHTML(readmeHTML).
		SetManifestJSON(manifestJSON).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create plugin version: %w", err)
	}
	return pluginVersionToRecord(pv), nil
}

// PluginVersionUpdate updates an existing plugin version's OCI metadata.
func (a *Adapter) PluginVersionUpdate(ctx context.Context, id int, ociRef string, ociDigest string, readmeHTML string, manifestJSON map[string]any) error {
	_, err := a.client.PluginVersion.UpdateOneID(id).
		SetOciImageRef(ociRef).
		SetOciDigest(ociDigest).
		SetReadmeHTML(readmeHTML).
		SetManifestJSON(manifestJSON).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("update plugin version: %w", err)
	}
	return nil
}

// PluginVersionListByPlugin returns all versions for a plugin, newest first.
func (a *Adapter) PluginVersionListByPlugin(ctx context.Context, pluginID int) ([]PluginVersionRecord, error) {
	pvs, err := a.client.PluginVersion.Query().
		Where(pluginversion.PluginIDEQ(pluginID)).
		Order(ent.Desc(pluginversion.FieldID)).
		All(ctx)
	if err != nil {
		return nil, fmt.Errorf("list plugin versions: %w", err)
	}

	var records []PluginVersionRecord
	for _, pv := range pvs {
		records = append(records, *pluginVersionToRecord(pv))
	}

	return records, nil
}

// PluginList returns a paginated list of plugins matching the query.
func (a *Adapter) PluginList(ctx context.Context, query string, limit int, offset int) ([]PluginRecord, int, error) {
	q := a.client.Plugin.Query()
	if query != "" {
		q = q.Where(plugin.NameContainsFold(query))
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count plugins: %w", err)
	}

	ps, err := q.Limit(limit).Offset(offset).All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list plugins: %w", err)
	}

	var records []PluginRecord
	for _, p := range ps {
		records = append(records, PluginRecord{
			ID:          p.ID,
			NamespaceID: p.NamespaceID,
			Name:        p.Name,
			DisplayName: p.DisplayName,
			Description: p.Description,
			LogoURL:     p.LogoURL,
		})
	}

	return records, total, nil
}

// PluginListByNamespace returns plugins in a namespace with optional search and pagination.
func (a *Adapter) PluginListByNamespace(ctx context.Context, namespaceID int, query string, limit, offset int) ([]PluginRecord, int, error) {
	q := a.client.Plugin.Query().Where(plugin.NamespaceIDEQ(namespaceID))
	if query != "" {
		q = q.Where(plugin.NameContainsFold(query))
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("count plugins by namespace: %w", err)
	}

	ps, err := q.Limit(limit).Offset(offset).All(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("list plugins by namespace: %w", err)
	}

	var records []PluginRecord
	for _, p := range ps {
		records = append(records, PluginRecord{
			ID:          p.ID,
			NamespaceID: p.NamespaceID,
			Name:        p.Name,
			DisplayName: p.DisplayName,
			Description: p.Description,
			LogoURL:     p.LogoURL,
		})
	}

	return records, total, nil
}

func pluginVersionToRecord(pv *ent.PluginVersion) *PluginVersionRecord {
	manifestData := pv.ManifestJSON
	if manifestData == nil {
		manifestData = map[string]any{}
	}
	return &PluginVersionRecord{
		ID:           pv.ID,
		PluginID:     pv.PluginID,
		Version:      pv.Version,
		OciImageRef:  pv.OciImageRef,
		OciDigest:    pv.OciDigest,
		ReadmeHTML:   pv.ReadmeHTML,
		ManifestJSON: manifestData,
	}
}

// NamespaceGetByName retrieves a namespace by its name within a transaction.
func (ta *TxAdapter) NamespaceGetByName(ctx context.Context, name string) (*NamespaceRecord, error) {
	n, err := ta.tx.Namespace.Query().Where(namespace.NameEQ(name)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("%w: namespace %s", ErrNotFound, name)
		}
		return nil, fmt.Errorf("query namespace: %w", err)
	}
	return &NamespaceRecord{ID: n.ID, Name: n.Name, Type: n.Type, UserID: n.UserID}, nil
}

// NamespaceCreate creates a new namespace within a transaction.
// userID is the owning user. Pass 0 for unowned namespaces.
func (ta *TxAdapter) NamespaceCreate(ctx context.Context, name string, nsType string, userID int) (*NamespaceRecord, error) {
	create := ta.tx.Namespace.Create().SetName(name).SetType(nsType)
	if userID != 0 {
		create.SetUserID(userID)
	}
	n, err := create.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create namespace: %w", err)
	}
	return &NamespaceRecord{ID: n.ID, Name: n.Name, Type: n.Type, UserID: n.UserID}, nil
}

// PluginGetByNamespaceAndName retrieves a plugin by namespace and name within a transaction.
func (ta *TxAdapter) PluginGetByNamespaceAndName(ctx context.Context, namespaceID int, name string) (*PluginRecord, error) {
	p, err := ta.tx.Plugin.Query().
		Where(plugin.NamespaceIDEQ(namespaceID), plugin.NameEQ(name)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("%w: plugin %s", ErrNotFound, name)
		}
		return nil, fmt.Errorf("query plugin: %w", err)
	}
	return &PluginRecord{
		ID: p.ID, NamespaceID: p.NamespaceID, Name: p.Name,
		DisplayName: p.DisplayName, Description: p.Description, LogoURL: p.LogoURL,
	}, nil
}

// PluginCreate creates a new plugin within a transaction.
func (ta *TxAdapter) PluginCreate(ctx context.Context, namespaceID int, name string, displayName string, description string, logoURL string) (*PluginRecord, error) {
	p, err := ta.tx.Plugin.Create().
		SetNamespaceID(namespaceID).SetName(name).
		SetDisplayName(displayName).SetDescription(description).SetLogoURL(logoURL).Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create plugin: %w", err)
	}
	return &PluginRecord{
		ID: p.ID, NamespaceID: p.NamespaceID, Name: p.Name,
		DisplayName: p.DisplayName, Description: p.Description, LogoURL: p.LogoURL,
	}, nil
}

// PluginUpdate updates an existing plugin's display metadata within a transaction.
func (ta *TxAdapter) PluginUpdate(ctx context.Context, id int, displayName string, description string, logoURL string) error {
	_, err := ta.tx.Plugin.UpdateOneID(id).
		SetDisplayName(displayName).SetDescription(description).SetLogoURL(logoURL).Save(ctx)
	if err != nil {
		return fmt.Errorf("update plugin: %w", err)
	}
	return nil
}

// PluginVersionGetByPluginAndVersion retrieves a plugin version within a transaction.
func (ta *TxAdapter) PluginVersionGetByPluginAndVersion(ctx context.Context, pluginID int, version string) (*PluginVersionRecord, error) {
	pv, err := ta.tx.PluginVersion.Query().
		Where(pluginversion.PluginIDEQ(pluginID), pluginversion.VersionEQ(version)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("%w: plugin version %s", ErrNotFound, version)
		}
		return nil, fmt.Errorf("query plugin version: %w", err)
	}
	return pluginVersionToRecord(pv), nil
}

// PluginVersionCreate creates a new plugin version within a transaction.
func (ta *TxAdapter) PluginVersionCreate(ctx context.Context, pluginID int, version string, ociRef string, ociDigest string, readmeHTML string, manifestJSON map[string]any) (*PluginVersionRecord, error) {
	pv, err := ta.tx.PluginVersion.Create().
		SetPluginID(pluginID).SetVersion(version).
		SetOciImageRef(ociRef).SetOciDigest(ociDigest).
		SetReadmeHTML(readmeHTML).SetManifestJSON(manifestJSON).Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create plugin version: %w", err)
	}
	return pluginVersionToRecord(pv), nil
}

// PluginVersionUpdate updates an existing plugin version within a transaction.
func (ta *TxAdapter) PluginVersionUpdate(ctx context.Context, id int, ociRef string, ociDigest string, readmeHTML string, manifestJSON map[string]any) error {
	_, err := ta.tx.PluginVersion.UpdateOneID(id).
		SetOciImageRef(ociRef).SetOciDigest(ociDigest).
		SetReadmeHTML(readmeHTML).SetManifestJSON(manifestJSON).Save(ctx)
	if err != nil {
		return fmt.Errorf("update plugin version: %w", err)
	}
	return nil
}

// UserCreate creates a new user.
func (a *Adapter) UserCreate(ctx context.Context, email, passwordHash string) (*UserRecord, error) {
	u, err := a.client.User.Create().
		SetEmail(email).
		SetPasswordHash(passwordHash).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create user: %w", err)
	}
	return &UserRecord{
		ID:           u.ID,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}, nil
}

// UserGetByEmail retrieves a user by email.
func (a *Adapter) UserGetByEmail(ctx context.Context, email string) (*UserRecord, error) {
	u, err := a.client.User.Query().Where(user.EmailEQ(email)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("%w: user %s", ErrNotFound, email)
		}
		return nil, fmt.Errorf("query user: %w", err)
	}
	return &UserRecord{
		ID:           u.ID,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}, nil
}

// UserGetByID retrieves a user by ID.
func (a *Adapter) UserGetByID(ctx context.Context, id int) (*UserRecord, error) {
	u, err := a.client.User.Get(ctx, id)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("%w: user id %d", ErrNotFound, id)
		}
		return nil, fmt.Errorf("get user by id: %w", err)
	}
	return &UserRecord{
		ID:           u.ID,
		Email:        u.Email,
		PasswordHash: u.PasswordHash,
		CreatedAt:    u.CreatedAt,
		UpdatedAt:    u.UpdatedAt,
	}, nil
}

// RefreshTokenCreate creates a new refresh token record.
func (a *Adapter) RefreshTokenCreate(ctx context.Context, userID int, tokenHash string, expiresAt time.Time) (*RefreshTokenRecord, error) {
	rt, err := a.client.RefreshToken.Create().
		SetUserID(userID).
		SetTokenHash(tokenHash).
		SetExpiresAt(expiresAt).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create refresh token: %w", err)
	}
	return &RefreshTokenRecord{
		ID:        rt.ID,
		UserID:    rt.UserID,
		TokenHash: rt.TokenHash,
		ExpiresAt: rt.ExpiresAt,
		CreatedAt: rt.CreatedAt,
	}, nil
}

// RefreshTokenGetByHash retrieves a refresh token by its hash.
func (a *Adapter) RefreshTokenGetByHash(ctx context.Context, tokenHash string) (*RefreshTokenRecord, error) {
	rt, err := a.client.RefreshToken.Query().Where(refreshtoken.TokenHashEQ(tokenHash)).Only(ctx)
	if err != nil {
		if ent.IsNotFound(err) {
			return nil, fmt.Errorf("%w: refresh token", ErrNotFound)
		}
		return nil, fmt.Errorf("query refresh token: %w", err)
	}
	return &RefreshTokenRecord{
		ID:        rt.ID,
		UserID:    rt.UserID,
		TokenHash: rt.TokenHash,
		ExpiresAt: rt.ExpiresAt,
		CreatedAt: rt.CreatedAt,
	}, nil
}

// RefreshTokenDeleteByID deletes a refresh token by ID.
func (a *Adapter) RefreshTokenDeleteByID(ctx context.Context, id int) error {
	err := a.client.RefreshToken.DeleteOneID(id).Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete refresh token: %w", err)
	}
	return nil
}

// RefreshTokenDeleteExpired removes all expired refresh tokens.
func (a *Adapter) RefreshTokenDeleteExpired(ctx context.Context) (int64, error) {
	n, err := a.client.RefreshToken.Delete().
		Where(refreshtoken.ExpiresAtLT(time.Now())).
		Exec(ctx)
	if err != nil {
		return 0, fmt.Errorf("delete expired refresh tokens: %w", err)
	}
	return int64(n), nil
}
