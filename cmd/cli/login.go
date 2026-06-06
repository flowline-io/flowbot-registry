package main

import (
	"bufio"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/flowline-io/flowbot-registry/pkg/json"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var loginArgs struct {
	storeURL string
}

func loginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in to the Flowbot plugin registry",
		Long: `Authenticate with the Flowbot plugin registry using email and password.

Stores access and refresh tokens in ~/.flowbot/config.json for subsequent use.`,
		RunE: runLogin,
	}

	cmd.Flags().StringVar(&loginArgs.storeURL, "store-url", envOrFlag("FLOWBOT_STORE_URL", "http://localhost:8128"), "Store API URL")

	return cmd
}

func runLogin(_ *cobra.Command, _ []string) error {
	slog.Debug("login: starting", "store_url", loginArgs.storeURL)
	_, _ = fmt.Fprintf(os.Stderr, "Logging in to %s\n", loginArgs.storeURL)
	_, _ = fmt.Fprint(os.Stderr, "Email: ")

	reader := bufio.NewReader(os.Stdin)
	email, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("read email: %w", err)
	}
	email = strings.TrimSpace(email)

	_, _ = fmt.Fprint(os.Stderr, "Password: ")
	passBytes, err := readPassword()
	if err != nil {
		return fmt.Errorf("read password: %w", err)
	}
	password := strings.TrimSpace(passBytes)
	_, _ = fmt.Fprintln(os.Stderr)

	return doLogin(email, password)
}

func doLogin(email, password string) error {
	apiURL := strings.TrimRight(loginArgs.storeURL, "/") + "/api/v1/auth/login"

	body := map[string]string{
		"email":    email,
		"password": password,
	}

	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, apiURL, strings.NewReader(string(bodyBytes)))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("login request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return fmt.Errorf("read response: %w", readErr)
	}

	if resp.StatusCode != http.StatusOK {
		slog.Error("login: API error", "status", resp.StatusCode, "body", string(respBytes))
		return fmt.Errorf("login failed (%d): %s", resp.StatusCode, string(respBytes))
	}

	var result struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresAt    string `json:"expires_at"`
	}
	if err := json.Unmarshal(respBytes, &result); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	expiresAt, err := parseTime(result.ExpiresAt)
	if err != nil {
		slog.Warn("login: cannot parse expires_at, using zero time", "value", result.ExpiresAt, "error", err)
	}

	cfg := &CLIConfig{
		StoreURL:     loginArgs.storeURL,
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		ExpiresAt:    expiresAt,
	}

	if err := saveConfig(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	_, _ = fmt.Println("Logged in successfully!")
	return nil
}

// readPassword reads a password from stdin without echoing.
func readPassword() (string, error) {
	bytePass, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", err
	}
	return string(bytePass), nil
}

// parseTime attempts to parse a timestamp string using common layouts.
func parseTime(s string) (time.Time, error) {
	layouts := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05-07:00",
	}
	for _, layout := range layouts {
		t, err := time.Parse(layout, s)
		if err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse time: %s", s)
}
