package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/ziyan-junaideen/jdeen-cli/internal/jdeenapi"
	"github.com/ziyan-junaideen/jdeen-cli/internal/output"
	"github.com/ziyan-junaideen/jdeen-cli/internal/secrets"
	"golang.org/x/term"
)

func newAuthCommand(options *globalOptions) *cobra.Command {
	auth := &cobra.Command{Use: "auth", Short: "Manage API authentication"}
	var email, deviceName string
	var nonInteractive bool
	login := &cobra.Command{Use: "login", Short: "Log in with a confirmed administrator account", RunE: func(command *cobra.Command, _ []string) error {
		client, runtime, err := newAPIClient(options)
		if err != nil {
			return err
		}
		resolvedEmail := firstValue(email, os.Getenv("JDEEN_EMAIL"))
		if resolvedEmail == "" && !nonInteractive {
			resolvedEmail, err = promptLine(command, "Email: ")
			if err != nil {
				return err
			}
		}
		password := os.Getenv("JDEEN_PASSWORD")
		if password == "" && !nonInteractive {
			password, err = promptSecret(command, "Password: ")
			if err != nil {
				return err
			}
		}
		if resolvedEmail == "" || password == "" {
			return errors.New("email and password are required; set JDEEN_EMAIL and JDEEN_PASSWORD for non-interactive login")
		}
		if deviceName == "" {
			deviceName = os.Getenv("JDEEN_DEVICE_NAME")
		}
		if deviceName == "" {
			hostname, _ := os.Hostname()
			deviceName = "JDeen CLI on " + hostname
		}
		result, err := client.Login(context.Background(), resolvedEmail, password, deviceName)
		if err != nil {
			return err
		}
		var tokens jdeenapi.TokenResponse
		if result.Tokens != nil {
			tokens = *result.Tokens
		} else {
			code := os.Getenv("JDEEN_TOTP_CODE")
			for attempt := 0; attempt < 5; attempt++ {
				if code == "" && !nonInteractive {
					code, err = promptSecret(command, "Authentication code: ")
					if err != nil {
						return err
					}
				}
				if code == "" {
					return errors.New("two-factor code is required; set JDEEN_TOTP_CODE for non-interactive login")
				}
				tokens, err = client.CompleteTwoFactor(context.Background(), result.Challenge.ChallengeToken, code)
				if err == nil {
					break
				}
				var apiError *jdeenapi.APIError
				if !errors.As(err, &apiError) || apiError.Code != "invalid_two_factor_code" || nonInteractive {
					return err
				}
				fmt.Fprintln(command.ErrOrStderr(), "Invalid authentication code; try again.")
				code = ""
			}
			if err != nil {
				return err
			}
		}
		if err := client.SaveTokens(tokens); err != nil {
			return err
		}
		if options.jsonOutput {
			return output.JSON(command.OutOrStdout(), map[string]any{"profile": runtime.ProfileName, "session_id": tokens.SessionID, "expires_in": tokens.ExpiresIn})
		}
		fmt.Fprintf(command.OutOrStdout(), "Logged in to profile %q (session %s)\n", runtime.ProfileName, tokens.SessionID)
		return nil
	}}
	login.Flags().StringVar(&email, "email", "", "administrator email address")
	login.Flags().StringVar(&deviceName, "device-name", "", "human-readable session device name")
	login.Flags().BoolVar(&nonInteractive, "non-interactive", false, "do not prompt for missing credentials")

	status := &cobra.Command{Use: "status", Short: "Show local API session status", RunE: func(command *cobra.Command, _ []string) error {
		client, runtime, err := newAPIClient(options)
		if err != nil {
			return err
		}
		if token := strings.TrimSpace(os.Getenv("JDEEN_ACCESS_TOKEN")); token != "" {
			value := map[string]any{"profile": runtime.ProfileName, "api_url": runtime.APIURL, "source": "JDEEN_ACCESS_TOKEN", "authenticated": true}
			if options.jsonOutput {
				return output.JSON(command.OutOrStdout(), value)
			}
			fmt.Fprintf(command.OutOrStdout(), "Profile: %s\nAPI URL: %s\nAuthentication: JDEEN_ACCESS_TOKEN\n", runtime.ProfileName, runtime.APIURL)
			return nil
		}
		session, err := client.StoredSession()
		if errors.Is(err, secrets.ErrNotFound) {
			if options.jsonOutput {
				return output.JSON(command.OutOrStdout(), map[string]any{"profile": runtime.ProfileName, "api_url": runtime.APIURL, "authenticated": false})
			}
			fmt.Fprintf(command.OutOrStdout(), "No session configured for profile %q\n", runtime.ProfileName)
			return nil
		}
		if err != nil {
			return err
		}
		value := map[string]any{"profile": runtime.ProfileName, "api_url": runtime.APIURL, "authenticated": true, "session_id": session.SessionID, "access_expires_at": session.AccessExpiresAt, "refresh_expires_at": session.RefreshExpiresAt}
		if options.jsonOutput {
			return output.JSON(command.OutOrStdout(), value)
		}
		fmt.Fprintf(command.OutOrStdout(), "Profile: %s\nAPI URL: %s\nSession: %s\nAccess expires: %s\nSession expires: %s\n", runtime.ProfileName, runtime.APIURL, session.SessionID, session.AccessExpiresAt.Local().Format(time.RFC3339), session.RefreshExpiresAt.Local().Format(time.RFC3339))
		return nil
	}}

	var localOnly bool
	logout := &cobra.Command{Use: "logout", Short: "Revoke and remove the current API session", RunE: func(command *cobra.Command, _ []string) error {
		if os.Getenv("JDEEN_ACCESS_TOKEN") != "" {
			return errors.New("JDEEN_ACCESS_TOKEN is set; unset it to log out or revoke its session explicitly")
		}
		client, runtime, err := newAPIClient(options)
		if err != nil {
			return err
		}
		session, err := client.StoredSession()
		if errors.Is(err, secrets.ErrNotFound) {
			fmt.Fprintf(command.OutOrStdout(), "No session configured for profile %q\n", runtime.ProfileName)
			return nil
		}
		if err != nil {
			return err
		}
		if !localOnly {
			if err := client.RevokeSession(context.Background(), session.SessionID); err != nil {
				return fmt.Errorf("revoke remote session (credentials kept; use --local-only to discard them): %w", err)
			}
		}
		if err := client.DeleteStoredSession(); err != nil {
			return err
		}
		fmt.Fprintf(command.OutOrStdout(), "Logged out of profile %q\n", runtime.ProfileName)
		return nil
	}}
	logout.Flags().BoolVar(&localOnly, "local-only", false, "discard local credentials without revoking the server session")

	sessions := &cobra.Command{Use: "sessions", Short: "Manage active API sessions"}
	sessionsList := &cobra.Command{Use: "list", Short: "List active API sessions", RunE: func(command *cobra.Command, _ []string) error {
		client, _, err := newAPIClient(options)
		if err != nil {
			return err
		}
		items, err := client.ListSessions(context.Background())
		if err != nil {
			return err
		}
		if options.jsonOutput {
			return output.JSON(command.OutOrStdout(), map[string]any{"sessions": items})
		}
		return output.Sessions(command.OutOrStdout(), items)
	}}
	sessionsRevoke := &cobra.Command{Use: "revoke <session-id>", Short: "Revoke an API session", Args: cobra.ExactArgs(1), RunE: func(command *cobra.Command, args []string) error {
		if err := validateID(args[0]); err != nil {
			return err
		}
		client, _, err := newAPIClient(options)
		if err != nil {
			return err
		}
		current, _ := client.StoredSession()
		if err := client.RevokeSession(context.Background(), args[0]); err != nil {
			return err
		}
		if current.SessionID == args[0] {
			_ = client.DeleteStoredSession()
		}
		if options.jsonOutput {
			return output.JSON(command.OutOrStdout(), map[string]any{"revoked": true, "session_id": args[0]})
		}
		fmt.Fprintf(command.OutOrStdout(), "Revoked session %s\n", args[0])
		return nil
	}}
	sessions.AddCommand(sessionsList, sessionsRevoke)
	auth.AddCommand(login, status, logout, sessions)
	return auth
}

func promptLine(command *cobra.Command, label string) (string, error) {
	fmt.Fprint(command.ErrOrStderr(), label)
	line, err := bufio.NewReader(command.InOrStdin()).ReadString('\n')
	return strings.TrimSpace(line), errUnlessEOF(err)
}

func promptSecret(command *cobra.Command, label string) (string, error) {
	fmt.Fprint(command.ErrOrStderr(), label)
	if file, ok := command.InOrStdin().(*os.File); ok && term.IsTerminal(int(file.Fd())) {
		value, err := term.ReadPassword(int(file.Fd()))
		fmt.Fprintln(command.ErrOrStderr())
		return strings.TrimSpace(string(value)), err
	}
	line, err := bufio.NewReader(command.InOrStdin()).ReadString('\n')
	return strings.TrimSpace(line), errUnlessEOF(err)
}

func errUnlessEOF(err error) error {
	if errors.Is(err, io.EOF) {
		return nil
	}
	return err
}

func firstValue(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
