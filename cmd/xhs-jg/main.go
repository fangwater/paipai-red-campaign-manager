package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"paipai-red-campaign-manager/internal/xhs"

	"golang.org/x/term"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		printUsage(os.Stderr)
		return errors.New("a command is required")
	}
	switch args[0] {
	case "token":
		return runToken(ctx, args[1:])
	case "refresh":
		return runRefresh(ctx, args[1:])
	case "help", "-h", "--help":
		printUsage(os.Stdout)
		return nil
	default:
		printUsage(os.Stderr)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runToken(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("token", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	authCodeFlag := flags.String("auth-code", "", "temporary authorization code; prompted when omitted")
	sessionPath := flags.String("session", defaultSessionPath(), "path to the persistent OAuth session")
	timeout := flags.Duration("timeout", 30*time.Second, "token request timeout")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *timeout <= 0 {
		return errors.New("--timeout must be positive")
	}

	authCode, err := readAuthCode(*authCodeFlag)
	if err != nil {
		return err
	}
	client, err := newClientFromEnvironment()
	if err != nil {
		return err
	}
	requestContext, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	token, err := client.ExchangeToken(requestContext, authCode)
	if err != nil {
		return err
	}
	if err := xhs.SaveSession(*sessionPath, token); err != nil {
		return err
	}
	printTokenSummary("authorization succeeded", token, *sessionPath)
	return nil
}

func runRefresh(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("refresh", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	sessionPath := flags.String("session", defaultSessionPath(), "path to the persistent OAuth session")
	timeout := flags.Duration("timeout", 30*time.Second, "token refresh timeout")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *timeout <= 0 {
		return errors.New("--timeout must be positive")
	}

	client, err := newClientFromEnvironment()
	if err != nil {
		return err
	}
	requestContext, cancel := context.WithTimeout(ctx, *timeout)
	defer cancel()
	token, err := client.RefreshSession(requestContext, *sessionPath)
	if err != nil {
		return err
	}
	printTokenSummary("token refresh succeeded", token, *sessionPath)
	return nil
}

func newClientFromEnvironment() (*xhs.Client, error) {
	appIDRaw := strings.TrimSpace(os.Getenv("XHS_JG_APP_ID"))
	secret := strings.TrimSpace(os.Getenv("XHS_JG_SECRET"))
	if appIDRaw == "" || secret == "" {
		return nil, errors.New("XHS_JG_APP_ID and XHS_JG_SECRET are required")
	}
	appID, err := strconv.ParseInt(appIDRaw, 10, 64)
	if err != nil || appID <= 0 {
		return nil, errors.New("XHS_JG_APP_ID must be a positive integer")
	}
	return xhs.NewClient(appID, secret)
}

func printTokenSummary(status string, token xhs.Token, sessionPath string) {
	fmt.Fprintln(os.Stdout, status)
	fmt.Fprintf(os.Stdout, "user_id: %s\n", token.UserID)
	fmt.Fprintf(os.Stdout, "app_id: %d\n", token.AppID)
	fmt.Fprintf(os.Stdout, "platform_type: %d\n", token.PlatformType)
	fmt.Fprintf(os.Stdout, "scope: %s\n", token.Scope)
	fmt.Fprintf(os.Stdout, "access_token_expires_at: %s\n", token.AccessTokenExpiresAt.Format(time.RFC3339))
	fmt.Fprintf(os.Stdout, "refresh_token_expires_at: %s\n", token.RefreshTokenExpiresAt.Format(time.RFC3339))
	fmt.Fprintf(os.Stdout, "session: %s\n", sessionPath)
	for _, advertiser := range token.ApprovalAdvertisers {
		fmt.Fprintf(os.Stdout, "advertiser: %d\t%s\n", advertiser.ID, advertiser.Name)
	}
}

func readAuthCode(flagValue string) (string, error) {
	if value := strings.TrimSpace(flagValue); value != "" {
		return value, nil
	}
	if value := strings.TrimSpace(os.Getenv("XHS_JG_AUTH_CODE")); value != "" {
		return value, nil
	}
	fmt.Fprint(os.Stderr, "Auth code: ")
	if term.IsTerminal(int(syscall.Stdin)) {
		value, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", fmt.Errorf("read auth code: %w", err)
		}
		authCode := strings.TrimSpace(string(value))
		if authCode == "" {
			return "", errors.New("auth code is required")
		}
		return authCode, nil
	}
	value, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read auth code: %w", err)
	}
	authCode := strings.TrimSpace(value)
	if authCode == "" {
		return "", errors.New("auth code is required")
	}
	return authCode, nil
}

func defaultSessionPath() string {
	if value := strings.TrimSpace(os.Getenv("XHS_JG_SESSION_FILE")); value != "" {
		return value
	}
	return filepath.Join(".xhs-jg", "session.json")
}

func printUsage(output io.Writer) {
	fmt.Fprintln(output, "Usage:")
	fmt.Fprintln(output, "  xhs-jg token [--auth-code CODE] [--session PATH]")
	fmt.Fprintln(output, "  xhs-jg refresh [--session PATH]")
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Commands:")
	fmt.Fprintln(output, "  token    exchange a temporary auth_code and securely save the OAuth tokens")
	fmt.Fprintln(output, "  refresh  rotate access_token and refresh_token using the saved session")
}
