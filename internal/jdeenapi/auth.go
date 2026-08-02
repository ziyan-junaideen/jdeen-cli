package jdeenapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ziyan-junaideen/jdeen-cli/internal/secrets"
)

func (client *Client) Login(ctx context.Context, email, password, deviceName string) (LoginResult, error) {
	payload := map[string]string{"email": email, "password": password}
	if strings.TrimSpace(deviceName) != "" {
		payload["device_name"] = strings.TrimSpace(deviceName)
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return LoginResult{}, err
	}
	response, responseBody, err := client.perform(ctx, http.MethodPost, client.endpoint("auth/login"), "application/json", body, "")
	if err != nil {
		return LoginResult{}, err
	}
	switch response.StatusCode {
	case http.StatusCreated:
		var tokens TokenResponse
		if err := json.Unmarshal(responseBody, &tokens); err != nil {
			return LoginResult{}, fmt.Errorf("decode login response: %w", err)
		}
		return LoginResult{Tokens: &tokens}, nil
	case http.StatusAccepted:
		var challenge TwoFactorChallenge
		if err := json.Unmarshal(responseBody, &challenge); err != nil {
			return LoginResult{}, fmt.Errorf("decode two-factor challenge: %w", err)
		}
		return LoginResult{Challenge: &challenge}, nil
	default:
		return LoginResult{}, decodeAPIError(response.StatusCode, responseBody)
	}
}

func (client *Client) CompleteTwoFactor(ctx context.Context, challengeToken, code string) (TokenResponse, error) {
	var tokens TokenResponse
	err := client.publicJSON(ctx, http.MethodPost, "auth/two-factor", map[string]string{
		"challenge_token": challengeToken,
		"code":            code,
	}, &tokens, http.StatusCreated)
	return tokens, err
}

func (client *Client) SaveTokens(tokens TokenResponse) error {
	if client.store == nil {
		return errors.New("credential store is unavailable")
	}
	now := client.now()
	return client.store.Set(client.profileName, secrets.Session{
		AccessToken: tokens.AccessToken, RefreshToken: tokens.RefreshToken, SessionID: tokens.SessionID,
		AccessExpiresAt:  now.Add(time.Duration(tokens.ExpiresIn) * time.Second),
		RefreshExpiresAt: now.Add(time.Duration(tokens.RefreshTokenExpiresIn) * time.Second),
	})
}

func (client *Client) StoredSession() (secrets.Session, error) {
	if client.store == nil {
		return secrets.Session{}, secrets.ErrNotFound
	}
	return client.store.Get(client.profileName)
}

func (client *Client) DeleteStoredSession() error {
	if client.store == nil {
		return nil
	}
	return client.store.Delete(client.profileName)
}

func (client *Client) ListSessions(ctx context.Context) ([]APISession, error) {
	var response struct {
		Sessions []APISession `json:"sessions"`
	}
	err := client.authJSON(ctx, http.MethodGet, "auth/sessions", nil, &response, http.StatusOK)
	return response.Sessions, err
}

func (client *Client) RevokeSession(ctx context.Context, sessionID string) error {
	return client.authJSON(ctx, http.MethodDelete, "auth/sessions/"+sessionID, nil, nil, http.StatusNoContent)
}

func (client *Client) authJSON(ctx context.Context, method, path string, payload any, result any, expected ...int) error {
	var body []byte
	var err error
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return err
		}
	}
	token, err := client.validAccessToken(ctx)
	if err != nil {
		return err
	}
	response, responseBody, err := client.perform(ctx, method, client.endpoint(path), "application/json", body, token)
	if err != nil {
		return err
	}
	if response.StatusCode == http.StatusUnauthorized && errorCode(responseBody) == "access_token_expired" {
		token, err = client.refresh(ctx, token)
		if err != nil {
			return err
		}
		response, responseBody, err = client.perform(ctx, method, client.endpoint(path), "application/json", body, token)
		if err != nil {
			return err
		}
	}
	if !containsStatus(expected, response.StatusCode) {
		code := errorCode(responseBody)
		if code == "invalid_token" || code == "invalid_grant" || code == "account_not_allowed" {
			client.clearStoredSessionForAuthFailure()
		}
		return decodeAPIError(response.StatusCode, responseBody)
	}
	if result != nil && len(responseBody) > 0 {
		if err := json.Unmarshal(responseBody, result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

func (client *Client) clearStoredSessionForAuthFailure() {
	if client.accessToken == "" {
		_ = client.DeleteStoredSession()
	}
}

func (client *Client) validAccessToken(ctx context.Context) (string, error) {
	if client.accessToken != "" {
		return client.accessToken, nil
	}
	if client.store == nil {
		return "", errors.New("API session is not configured; run `jdeen auth login` or set JDEEN_ACCESS_TOKEN")
	}
	session, err := client.store.Get(client.profileName)
	if errors.Is(err, secrets.ErrNotFound) {
		return "", errors.New("API session is not configured; run `jdeen auth login` or set JDEEN_ACCESS_TOKEN")
	}
	if err != nil {
		return "", err
	}
	if session.AccessExpiresAt.After(client.now().Add(client.refreshSkew)) {
		return session.AccessToken, nil
	}
	return client.refresh(ctx, session.AccessToken)
}

func (client *Client) refresh(ctx context.Context, usedAccessToken string) (string, error) {
	if client.accessToken != "" {
		return "", errors.New("JDEEN_ACCESS_TOKEN expired; provide a new token or run `jdeen auth login`")
	}
	release, err := client.acquireRefreshLock(ctx)
	if err != nil {
		return "", err
	}
	defer release()

	session, err := client.store.Get(client.profileName)
	if err != nil {
		return "", err
	}
	if session.AccessToken != usedAccessToken && session.AccessExpiresAt.After(client.now().Add(client.refreshSkew)) {
		return session.AccessToken, nil
	}
	if session.RefreshToken == "" || !session.RefreshExpiresAt.After(client.now()) {
		_ = client.store.Delete(client.profileName)
		return "", errors.New("API session has expired; run `jdeen auth login`")
	}
	var tokens TokenResponse
	err = client.publicJSON(ctx, http.MethodPost, "auth/refresh", map[string]string{"refresh_token": session.RefreshToken}, &tokens, http.StatusOK)
	if err != nil {
		// A transport error leaves refresh-token consumption unknowable. Clearing
		// the pair prevents a later process from replaying the single-use token.
		_ = client.store.Delete(client.profileName)
		var apiError *APIError
		if errors.As(err, &apiError) {
			return "", err
		}
		return "", fmt.Errorf("refresh outcome is unknown; stored credentials were cleared and a new login is required: %w", err)
	}
	if err := client.SaveTokens(tokens); err != nil {
		_ = client.store.Delete(client.profileName)
		return "", fmt.Errorf("store refreshed credentials: %w", err)
	}
	return tokens.AccessToken, nil
}

func (client *Client) acquireRefreshLock(ctx context.Context) (func(), error) {
	if err := os.MkdirAll(client.lockDir, 0o700); err != nil {
		return nil, fmt.Errorf("create refresh lock directory: %w", err)
	}
	name := strings.NewReplacer("/", "_", "\\", "_", ":", "_").Replace(client.profileName)
	path := filepath.Join(client.lockDir, "refresh-"+name+".lock")
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
		if err == nil {
			_ = file.Close()
			return func() { _ = os.Remove(path) }, nil
		}
		if !errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("acquire refresh lock: %w", err)
		}
		if info, statErr := os.Stat(path); statErr == nil && client.now().Sub(info.ModTime()) > 2*time.Minute {
			_ = os.Remove(path)
			continue
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

func errorCode(body []byte) string {
	var response struct {
		Error  string `json:"error"`
		Errors []struct {
			Code string `json:"code"`
		} `json:"errors"`
	}
	if json.Unmarshal(body, &response) != nil {
		return ""
	}
	if response.Error != "" {
		return response.Error
	}
	if len(response.Errors) > 0 {
		return response.Errors[0].Code
	}
	return ""
}
