package web

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/a-h/templ"
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRender(t *testing.T) {
	tests := []struct {
		name      string
		component templ.Component
		wantBody  string
	}{
		{
			name: "renders simple component",
			component: templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
				_, err := io.WriteString(w, "<p>hello</p>")
				return err
			}),
			wantBody: "<p>hello</p>",
		},
		{
			name: "renders empty component",
			component: templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
				return nil
			}),
			wantBody: "",
		},
		{
			name: "renders component with unicode",
			component: templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
				_, err := io.WriteString(w, "<span>cafe</span>")
				return err
			}),
			wantBody: "<span>cafe</span>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			app.Get("/", func(c fiber.Ctx) error {
				return Render(c, tt.component)
			})

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			resp, err := app.Test(req)
			require.NoError(t, err)
			defer resp.Body.Close()

			assert.Equal(t, http.StatusOK, resp.StatusCode)
			assert.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			assert.Equal(t, tt.wantBody, strings.TrimSpace(string(body)))
		})
	}
}

func TestRenderContentType(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error {
		return Render(c, templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
			return nil
		}))
	})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	resp, err := app.Test(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	assert.Equal(t, fiber.MIMETextHTMLCharsetUTF8, resp.Header.Get(fiber.HeaderContentType))
}
