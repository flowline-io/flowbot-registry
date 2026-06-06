package web

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/flowline-io/flowbot-registry/pkg/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSearchFragment(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		mock       *mockStore
		wantStatus int
		wantBody   []string
	}{
		{
			name:  "search returns matching plugins",
			query: "test",
			mock: &mockStore{
				plugins: []store.PluginRecord{
					{ID: 1, NamespaceID: 1, Name: "test-plugin", DisplayName: "Test Plugin"},
				},
				pluginTotal: 1,
				namespaces: map[int]*store.NamespaceRecord{
					1: {ID: 1, Name: "testns", Type: "user"},
				},
			},
			wantStatus: http.StatusOK,
			wantBody:   []string{"Test Plugin", "testns"},
		},
		{
			name:  "search returns no results",
			query: "nonexistent",
			mock: &mockStore{
				plugins:     []store.PluginRecord{},
				pluginTotal: 0,
				namespaces:  map[int]*store.NamespaceRecord{},
			},
			wantStatus: http.StatusOK,
			wantBody:   []string{"No plugins matching"},
		},
		{
			name: "store returns error",
			mock: &mockStore{
				listPluginsErr: store.ErrNotFound,
			},
			wantStatus: http.StatusInternalServerError,
			wantBody:   []string{"Something went wrong"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := setupTestApp(tt.mock)
			req := httptest.NewRequest(http.MethodGet, "/web/search?q="+tt.query, nil)
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

func TestPluginGridFragment(t *testing.T) {
	tests := []struct {
		name       string
		offset     string
		mock       *mockStore
		wantStatus int
		wantBody   []string
	}{
		{
			name:   "load more returns cards and new button",
			offset: "0",
			mock: &mockStore{
				plugins: []store.PluginRecord{
					{ID: 1, NamespaceID: 1, Name: "p1", DisplayName: "Plugin 1"},
				},
				pluginTotal: 25,
				namespaces: map[int]*store.NamespaceRecord{
					1: {ID: 1, Name: "ns1", Type: "user"},
				},
			},
			wantStatus: http.StatusOK,
			wantBody:   []string{"Plugin 1", "Load More"},
		},
		{
			name:   "last page returns no button",
			offset: "20",
			mock: &mockStore{
				plugins: []store.PluginRecord{
					{ID: 2, NamespaceID: 1, Name: "p2", DisplayName: "Plugin 2"},
				},
				pluginTotal: 25,
				namespaces: map[int]*store.NamespaceRecord{
					1: {ID: 1, Name: "ns1", Type: "user"},
				},
			},
			wantStatus: http.StatusOK,
			wantBody:   []string{"Plugin 2"},
		},
		{
			name: "store error returns error fragment",
			mock: &mockStore{
				listPluginsErr: store.ErrNotFound,
			},
			wantStatus: http.StatusInternalServerError,
			wantBody:   []string{"Something went wrong"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := setupTestApp(tt.mock)
			req := httptest.NewRequest(http.MethodGet, "/web/plugins?offset="+tt.offset+"&limit=20", nil)
			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.wantStatus, resp.StatusCode)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			for _, s := range tt.wantBody {
				assert.Contains(t, string(body), s)
			}
			// Load More button should NOT appear on last page
			if tt.name == "last page returns no button" {
				assert.NotContains(t, string(body), "Load More")
			}
		})
	}
}

func TestVersionReadmeFragment(t *testing.T) {
	tests := []struct {
		name        string
		namespace   string
		pluginName  string
		version     string
		mock        *mockStore
		wantStatus  int
		wantBody    []string
		wantNotBody []string
	}{
		{
			name:       "happy path renders readme",
			namespace:  "testns",
			pluginName: "test-plugin",
			version:    "1.0.0",
			mock: &mockStore{
				namespaces: map[int]*store.NamespaceRecord{
					1: {ID: 1, Name: "testns", Type: "user"},
				},
				plugins: []store.PluginRecord{
					{ID: 1, NamespaceID: 1, Name: "test-plugin"},
				},
				versions: []store.PluginVersionRecord{
					{ID: 1, PluginID: 1, Version: "1.0.0", ReadmeHTML: "<h1>Hello</h1>", ManifestJSON: map[string]any{"name": "test-plugin"}},
				},
			},
			wantStatus: http.StatusOK,
			wantBody:   []string{"<h1>Hello</h1>", "1.0.0"},
		},
		{
			name:       "version not found returns inline error",
			namespace:  "testns",
			pluginName: "test-plugin",
			version:    "9.9.9",
			mock: &mockStore{
				namespaces: map[int]*store.NamespaceRecord{
					1: {ID: 1, Name: "testns", Type: "user"},
				},
				plugins: []store.PluginRecord{
					{ID: 1, NamespaceID: 1, Name: "test-plugin"},
				},
				versions: []store.PluginVersionRecord{},
			},
			wantStatus: http.StatusOK,
			wantBody:   []string{"Version not found"},
		},
		{
			name:       "namespace not found returns inline error",
			namespace:  "nonexistent",
			pluginName: "test-plugin",
			version:    "1.0.0",
			mock: &mockStore{
				namespaces: map[int]*store.NamespaceRecord{},
			},
			wantStatus: http.StatusOK,
			wantBody:   []string{"Version not found"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := setupTestApp(tt.mock)
			url := "/web/versions/" + tt.namespace + "/" + tt.pluginName + "/" + tt.version
			req := httptest.NewRequest(http.MethodGet, url, nil)
			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, tt.wantStatus, resp.StatusCode)
			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			for _, s := range tt.wantBody {
				assert.Contains(t, string(body), s)
			}
			for _, s := range tt.wantNotBody {
				assert.NotContains(t, string(body), s)
			}
		})
	}
}
