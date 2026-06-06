package main

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/flowline-io/flowbot-registry/pkg/json"
	"github.com/spf13/cobra"
)

var searchArgs struct {
	storeURL string
	limit    int
}

// errNotImplemented is returned when a feature is not yet implemented.
var errNotImplemented = errors.New("not implemented")

// httpClient is a shared HTTP client with timeout for CLI operations.
var httpClient = &http.Client{Timeout: 30 * time.Second}

func searchCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "search [query]",
		Short: "Search for plugins in the registry",
		Long: `Search the plugin registry for available plugins.

Example:
  flowbot plugin search github
  flowbot plugin search`,
		Args: cobra.MaximumNArgs(1),
		RunE: runSearch,
	}

	cmd.Flags().StringVar(&searchArgs.storeURL, "store-url", "http://localhost:8080", "Store API URL")
	cmd.Flags().IntVar(&searchArgs.limit, "limit", 20, "Maximum number of results")

	return cmd
}

func runSearch(_ *cobra.Command, args []string) error {
	query := ""
	if len(args) > 0 {
		query = args[0]
	}

	slog.Debug("search: querying", "query", query, "limit", searchArgs.limit)

	params := url.Values{}
	if query != "" {
		params.Set("q", query)
	}
	params.Set("limit", fmt.Sprintf("%d", searchArgs.limit))

	apiURL := fmt.Sprintf("%s/api/v1/plugins?%s", strings.TrimRight(searchArgs.storeURL, "/"), params.Encode())

	resp, err := httpClient.Get(apiURL)
	if err != nil {
		slog.Error("search: request failed", "error", err, "url", apiURL)
		return fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		slog.Error("search: API error", "status", resp.StatusCode, "body", string(body))
		return fmt.Errorf("store API returned %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	var result struct {
		Plugins []pluginResult `json:"plugins"`
		Total   int            `json:"total"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	slog.Info("search: results", "query", query, "count", len(result.Plugins), "total", result.Total)

	if len(result.Plugins) == 0 {
		_, _ = fmt.Println("No plugins found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "NAME\tDESCRIPTION")
	for _, p := range result.Plugins {
		desc := p.Description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\n", p.Name, desc)
	}
	_ = w.Flush()

	_, _ = fmt.Printf("\nShowing %d of %d plugins\n", len(result.Plugins), result.Total)

	return nil
}

type pluginResult struct {
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Description string `json:"description"`
}

// doJSONGet performs a GET request and unmarshals the JSON response into v.
func doJSONGet[T any](apiURL string) (*T, error) {
	resp, err := httpClient.Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		slog.Error("doJSONGet: API error", "url", apiURL, "status", resp.StatusCode, "body", string(body))
		return nil, fmt.Errorf("store API returned %d: %s", resp.StatusCode, string(body))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result T
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	return &result, nil
}
