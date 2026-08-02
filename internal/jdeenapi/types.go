package jdeenapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ziyan-junaideen/jdeen-cli/internal/jsonapi"
)

type TokenResponse struct {
	AccessToken           string `json:"access_token"`
	RefreshToken          string `json:"refresh_token"`
	TokenType             string `json:"token_type"`
	ExpiresIn             int    `json:"expires_in"`
	RefreshTokenExpiresIn int    `json:"refresh_token_expires_in"`
	SessionID             string `json:"session_id"`
}

type TwoFactorChallenge struct {
	TwoFactorRequired bool   `json:"two_factor_required"`
	ChallengeToken    string `json:"challenge_token"`
	ExpiresIn         int    `json:"expires_in"`
}

type LoginResult struct {
	Tokens    *TokenResponse
	Challenge *TwoFactorChallenge
}

type APISession struct {
	ID         string     `json:"id"`
	DeviceName string     `json:"device_name"`
	UserAgent  string     `json:"user_agent"`
	IPAddress  string     `json:"ip_address"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at"`
	ExpiresAt  time.Time  `json:"expires_at"`
}

type QueryOptions struct {
	Filters  map[string][]string
	Includes []string
	Fields   map[string][]string
	Sort     string
	PageSize int
	After    string
	Before   string
	PageURL  string
}

type APIError struct {
	StatusCode int
	Code       string
	Message    string
	Errors     []jsonapi.Error
	Body       string
}

func (apiError *APIError) Error() string {
	prefix := fmt.Sprintf("API request failed: %d %s", apiError.StatusCode, http.StatusText(apiError.StatusCode))
	parts := make([]string, 0, len(apiError.Errors))
	for _, item := range apiError.Errors {
		message := firstNonEmpty(item.Detail, item.Title)
		if item.Code != "" && message != "" {
			message = item.Code + ": " + message
		} else if item.Code != "" {
			message = item.Code
		}
		if len(item.Source) > 0 {
			var source struct {
				Pointer string `json:"pointer"`
				Header  string `json:"header"`
			}
			if json.Unmarshal(item.Source, &source) == nil {
				location := firstNonEmpty(source.Pointer, source.Header)
				if location != "" {
					message += " (" + location + ")"
				}
			}
		}
		if message != "" {
			parts = append(parts, message)
		}
	}
	if len(parts) > 0 {
		return prefix + ": " + strings.Join(parts, "; ")
	}
	if apiError.Message != "" {
		if apiError.Code != "" {
			return prefix + ": " + apiError.Code + ": " + apiError.Message
		}
		return prefix + ": " + apiError.Message
	}
	if apiError.Code != "" {
		return prefix + ": " + apiError.Code
	}
	if strings.TrimSpace(apiError.Body) != "" {
		return prefix + ": " + strings.TrimSpace(apiError.Body)
	}
	return prefix
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
