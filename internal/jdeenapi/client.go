package jdeenapi

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/adrg/xdg"
	"github.com/ziyan-junaideen/jdeen-cli/internal/jsonapi"
	"github.com/ziyan-junaideen/jdeen-cli/internal/secrets"
)

type Config struct {
	APIURL             string
	ProfileName        string
	CACert             string
	InsecureSkipVerify bool
	AccessToken        string
	Version            string
	Store              secrets.Store
	HTTPClient         *http.Client
	LockDir            string
}

type Client struct {
	baseURL     *url.URL
	httpClient  *http.Client
	profileName string
	accessToken string
	store       secrets.Store
	userAgent   string
	lockDir     string
	refreshSkew time.Duration
	now         func() time.Time
}

func New(config Config) (*Client, error) {
	baseURL, err := url.Parse(config.APIURL)
	if err != nil {
		return nil, fmt.Errorf("parse API URL: %w", err)
	}
	httpClient := config.HTTPClient
	if httpClient == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		tlsConfig, err := tlsClientConfig(config)
		if err != nil {
			return nil, err
		}
		transport.TLSClientConfig = tlsConfig
		httpClient = &http.Client{Timeout: 30 * time.Second, Transport: transport}
	}
	lockDir := config.LockDir
	if lockDir == "" {
		lockDir, err = xdg.CacheFile(filepath.Join("jdeen", "locks"))
		if err != nil {
			return nil, fmt.Errorf("resolve lock directory: %w", err)
		}
	}
	version := firstNonEmpty(config.Version, "dev")
	return &Client{
		baseURL: baseURL, httpClient: httpClient, profileName: config.ProfileName,
		accessToken: strings.TrimSpace(config.AccessToken), store: config.Store,
		userAgent: "JDeen CLI/" + version, lockDir: lockDir,
		refreshSkew: 30 * time.Second, now: time.Now,
	}, nil
}

func tlsClientConfig(config Config) (*tls.Config, error) {
	tlsConfig := &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: config.InsecureSkipVerify} // #nosec G402 -- explicit development option
	if strings.TrimSpace(config.CACert) == "" {
		return tlsConfig, nil
	}
	certificate, err := os.ReadFile(config.CACert)
	if err != nil {
		return nil, fmt.Errorf("read CA certificate: %w", err)
	}
	pool, err := x509.SystemCertPool()
	if err != nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(certificate) {
		return nil, errors.New("CA certificate did not contain a valid PEM certificate")
	}
	tlsConfig.RootCAs = pool
	return tlsConfig, nil
}

func (client *Client) endpoint(path string) string {
	copy := *client.baseURL
	copy.Path = strings.TrimRight(client.baseURL.Path, "/") + "/" + strings.TrimLeft(path, "/")
	copy.RawQuery = ""
	copy.Fragment = ""
	return copy.String()
}

func (client *Client) resourceURL(path string, query QueryOptions) (string, error) {
	if query.PageURL != "" {
		candidate, err := url.Parse(query.PageURL)
		if err != nil {
			return "", fmt.Errorf("parse page URL: %w", err)
		}
		if !sameOrigin(client.baseURL, candidate) || !strings.HasPrefix(candidate.Path, strings.TrimSuffix(client.baseURL.Path, "/")+"/") {
			return "", errors.New("page URL must belong to the configured JDeen API")
		}
		return candidate.String(), nil
	}
	raw := client.endpoint(path)
	parsed, _ := url.Parse(raw)
	values := parsed.Query()
	for key, entries := range query.Filters {
		values.Set("filter["+key+"]", strings.Join(entries, ","))
	}
	if len(query.Includes) > 0 {
		values.Set("include", strings.Join(query.Includes, ","))
	}
	for resourceType, fields := range query.Fields {
		values.Set("fields["+resourceType+"]", strings.Join(fields, ","))
	}
	if query.Sort != "" {
		values.Set("sort", query.Sort)
	}
	if query.PageSize > 0 {
		values.Set("page[size]", fmt.Sprintf("%d", query.PageSize))
	}
	if query.After != "" {
		values.Set("page[after]", query.After)
	}
	if query.Before != "" {
		values.Set("page[before]", query.Before)
	}
	parsed.RawQuery = values.Encode()
	return parsed.String(), nil
}

func sameOrigin(a, b *url.URL) bool {
	return strings.EqualFold(a.Scheme, b.Scheme) && strings.EqualFold(a.Host, b.Host)
}

func (client *Client) publicJSON(ctx context.Context, method, path string, payload any, result any, expected ...int) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	response, responseBody, err := client.perform(ctx, method, client.endpoint(path), "application/json", body, "")
	if err != nil {
		return err
	}
	if !containsStatus(expected, response.StatusCode) {
		return decodeAPIError(response.StatusCode, responseBody)
	}
	if result != nil && len(responseBody) > 0 {
		if err := json.Unmarshal(responseBody, result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

func (client *Client) perform(ctx context.Context, method, rawURL, contentType string, body []byte, token string) (*http.Response, []byte, error) {
	request, err := http.NewRequestWithContext(ctx, method, rawURL, bytes.NewReader(body))
	if err != nil {
		return nil, nil, err
	}
	request.Header.Set("Accept", contentType)
	request.Header.Set("User-Agent", client.userAgent)
	if len(body) > 0 {
		request.Header.Set("Content-Type", contentType)
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := client.httpClient.Do(request)
	if err != nil {
		return nil, nil, err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(response.Body)
	return response, responseBody, err
}

func decodeAPIError(status int, body []byte) error {
	apiError := &APIError{StatusCode: status, Body: string(body)}
	var response struct {
		Error            string          `json:"error"`
		ErrorDescription string          `json:"error_description"`
		Errors           []jsonapi.Error `json:"errors"`
	}
	if json.Unmarshal(body, &response) == nil {
		apiError.Code = response.Error
		apiError.Message = response.ErrorDescription
		apiError.Errors = response.Errors
		if apiError.Code == "" && len(response.Errors) > 0 {
			apiError.Code = response.Errors[0].Code
		}
	}
	return apiError
}

func containsStatus(statuses []int, status int) bool {
	for _, expected := range statuses {
		if status == expected {
			return true
		}
	}
	return false
}
