package web

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flowline-io/flowbot-registry/pkg/store"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestApp(s store.StoreQuerier) *fiber.App {
	app := fiber.New()
	RegisterWebRoutes(app, s)
	return app
}

func TestBrowsePage(t *testing.T) {
	tests := []struct {
		name       string
		mock       *mockStore
		wantStatus int
		wantBody   []string
	}{
		{
			name: "happy path with plugins",
			mock: &mockStore{
				plugins: []store.PluginRecord{
					{ID: 1, NamespaceID: 1, Name: "test-plugin", DisplayName: "Test Plugin", Description: "A test plugin"},
				},
				pluginTotal: 1,
				namespaces: map[int]*store.NamespaceRecord{
					1: {ID: 1, Name: "testns", Type: "user"},
				},
			},
			wantStatus: http.StatusOK,
			wantBody:   []string{"Test Plugin", "testns", "A test plugin", `href="/testns/test-plugin"`},
		},
		{
			name: "empty store",
			mock: &mockStore{
				plugins:     []store.PluginRecord{},
				pluginTotal: 0,
				namespaces:  map[int]*store.NamespaceRecord{},
			},
			wantStatus: http.StatusOK,
			wantBody:   []string{"No plugins have been published yet"},
		},
		{
			name: "store returns error",
			mock: &mockStore{
				listPluginsErr: errors.New("database connection failed"),
			},
			wantStatus: http.StatusInternalServerError,
			wantBody:   []string{"Something went wrong"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := setupTestApp(tt.mock)
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.wantStatus, resp.StatusCode)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			for _, s := range tt.wantBody {
				assert.Contains(t, string(body), s)
			}
		})
	}
}

// mockStore implements store.StoreQuerier for testing.
type mockStore struct {
	plugins     []store.PluginRecord
	pluginTotal int
	namespaces  map[int]*store.NamespaceRecord
	versions    []store.PluginVersionRecord

	listPluginsErr  error
	getNSByNameErr  error
	getNSByIDErr    error
	getPluginErr    error
	listVersionsErr error
	getVersionErr   error
}

func (m *mockStore) PluginList(_ context.Context, _ string, _, _ int) ([]store.PluginRecord, int, error) {
	if m.listPluginsErr != nil {
		return nil, 0, m.listPluginsErr
	}
	return m.plugins, m.pluginTotal, nil
}

func (m *mockStore) NamespaceGetByID(_ context.Context, id int) (*store.NamespaceRecord, error) {
	if m.getNSByIDErr != nil {
		return nil, m.getNSByIDErr
	}
	if ns, ok := m.namespaces[id]; ok {
		return ns, nil
	}
	return nil, store.ErrNotFound
}

func (m *mockStore) NamespaceGetByName(_ context.Context, name string) (*store.NamespaceRecord, error) {
	if m.getNSByNameErr != nil {
		return nil, m.getNSByNameErr
	}
	for _, ns := range m.namespaces {
		if ns.Name == name {
			return ns, nil
		}
	}
	return nil, store.ErrNotFound
}

func (m *mockStore) PluginGetByNamespaceAndName(_ context.Context, namespaceID int, name string) (*store.PluginRecord, error) {
	if m.getPluginErr != nil {
		return nil, m.getPluginErr
	}
	for _, p := range m.plugins {
		if p.NamespaceID == namespaceID && p.Name == name {
			return &p, nil
		}
	}
	return nil, store.ErrNotFound
}

func (m *mockStore) PluginListByNamespace(_ context.Context, _ int, _ string, _, _ int) ([]store.PluginRecord, int, error) {
	if m.listPluginsErr != nil {
		return nil, 0, m.listPluginsErr
	}
	return m.plugins, m.pluginTotal, nil
}

func (m *mockStore) PluginVersionListByPlugin(_ context.Context, _ int) ([]store.PluginVersionRecord, error) {
	if m.listVersionsErr != nil {
		return nil, m.listVersionsErr
	}
	return m.versions, nil
}

func (m *mockStore) PluginVersionGetByPluginAndVersion(_ context.Context, _ int, _ string) (*store.PluginVersionRecord, error) {
	if m.getVersionErr != nil {
		return nil, m.getVersionErr
	}
	if len(m.versions) > 0 {
		return &m.versions[0], nil
	}
	return nil, store.ErrNotFound
}

func TestDetailPage(t *testing.T) {
	tests := []struct {
		name       string
		namespace  string
		pluginName string
		mock       *mockStore
		wantStatus int
		wantBody   []string
	}{
		{
			name:       "happy path renders plugin detail",
			namespace:  "testns",
			pluginName: "test-plugin",
			mock: &mockStore{
				namespaces: map[int]*store.NamespaceRecord{
					1: {ID: 1, Name: "testns", Type: "user"},
				},
				plugins: []store.PluginRecord{
					{ID: 1, NamespaceID: 1, Name: "test-plugin", DisplayName: "Test Plugin", Description: "A test plugin"},
				},
				versions: []store.PluginVersionRecord{
					{ID: 1, PluginID: 1, Version: "2.0.0", ReadmeHTML: "<p>v2 readme</p>", ManifestJSON: map[string]any{"name": "test-plugin"}},
					{ID: 2, PluginID: 1, Version: "1.0.0", ReadmeHTML: "<p>v1 readme</p>", ManifestJSON: map[string]any{"name": "test-plugin"}},
				},
			},
			wantStatus: http.StatusOK,
			wantBody:   []string{"Test Plugin", "testns", "A test plugin", "2.0.0", "v2 readme", "1.0.0"},
		},
		{
			name:       "namespace not found",
			namespace:  "nonexistent",
			pluginName: "some-plugin",
			mock: &mockStore{
				namespaces: map[int]*store.NamespaceRecord{},
			},
			wantStatus: http.StatusNotFound,
			wantBody:   []string{"Not Found", "nonexistent", "Back to Home"},
		},
		{
			name:       "plugin not found",
			namespace:  "testns",
			pluginName: "nonexistent",
			mock: &mockStore{
				namespaces: map[int]*store.NamespaceRecord{
					1: {ID: 1, Name: "testns", Type: "user"},
				},
				plugins: []store.PluginRecord{},
			},
			wantStatus: http.StatusNotFound,
			wantBody:   []string{"Not Found", "nonexistent", "Back to Home"},
		},
		{
			name:       "store error",
			namespace:  "testns",
			pluginName: "test-plugin",
			mock: &mockStore{
				namespaces: map[int]*store.NamespaceRecord{
					1: {ID: 1, Name: "testns", Type: "user"},
				},
				plugins: []store.PluginRecord{
					{ID: 1, NamespaceID: 1, Name: "test-plugin"},
				},
				listVersionsErr: errors.New("database connection failed"),
			},
			wantStatus: http.StatusInternalServerError,
			wantBody:   []string{"Something went wrong"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := setupTestApp(tt.mock)
			req := httptest.NewRequest(http.MethodGet, "/"+tt.namespace+"/"+tt.pluginName, nil)
			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.wantStatus, resp.StatusCode)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			for _, s := range tt.wantBody {
				assert.Contains(t, string(body), s)
			}
		})
	}
}
