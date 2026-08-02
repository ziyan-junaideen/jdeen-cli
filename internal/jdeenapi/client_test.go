package jdeenapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ziyan-junaideen/jdeen-cli/internal/jsonapi"
	"github.com/ziyan-junaideen/jdeen-cli/internal/secrets"
)

type memoryStore struct {
	mu      sync.Mutex
	session secrets.Session
	exists  bool
}

func (store *memoryStore) Get(string) (secrets.Session, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if !store.exists {
		return secrets.Session{}, secrets.ErrNotFound
	}
	return store.session, nil
}

func (store *memoryStore) Set(_ string, session secrets.Session) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.session, store.exists = session, true
	return nil
}

func (store *memoryStore) Delete(string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.session, store.exists = secrets.Session{}, false
	return nil
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (function roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return function(request)
}

func response(status int, body string) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}
}

func TestLoginAndTwoFactor(t *testing.T) {
	handler := func(request *http.Request) (*http.Response, error) {
		if request.Header.Get("User-Agent") != "JDeen CLI/test" {
			t.Errorf("unexpected user agent %q", request.Header.Get("User-Agent"))
		}
		switch request.URL.Path {
		case "/v1/auth/login":
			return response(http.StatusAccepted, `{"two_factor_required":true,"challenge_token":"jdeen_challenge_test","expires_in":300}`), nil
		case "/v1/auth/two-factor":
			return response(http.StatusCreated, string(tokenJSON("access", "refresh"))), nil
		default:
			return response(http.StatusNotFound, ""), nil
		}
	}
	store := &memoryStore{}
	client := testClient(t, store, handler)
	result, err := client.Login(context.Background(), "admin@example.com", "password", "Test CLI")
	if err != nil || result.Challenge == nil {
		t.Fatalf("login = %#v, %v", result, err)
	}
	tokens, err := client.CompleteTwoFactor(context.Background(), result.Challenge.ChallengeToken, "123456")
	if err != nil {
		t.Fatal(err)
	}
	if err := client.SaveTokens(tokens); err != nil {
		t.Fatal(err)
	}
	stored, _ := store.Get("test")
	if stored.AccessToken != "access" || stored.RefreshToken != "refresh" || stored.SessionID != "session-id" {
		t.Fatalf("unexpected stored tokens: %#v", stored)
	}
}

func TestResourceRequestHeadersAndQuery(t *testing.T) {
	handler := func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/v1/posts" {
			t.Errorf("path = %s", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer direct-token" || request.Header.Get("Accept") != jsonapi.MediaType {
			t.Errorf("unexpected headers: %#v", request.Header)
		}
		query := request.URL.Query()
		if query.Get("filter[state]") != "draft,published" || query.Get("fields[posts]") != "title,state" || query.Get("include") != "author,categories" || query.Get("page[size]") != "50" {
			t.Errorf("unexpected query: %s", request.URL.RawQuery)
		}
		return response(http.StatusOK, `{"data":[],"links":{"next":"https://api.example/next"},"jsonapi":{"version":"1.0"}}`), nil
	}
	client, err := New(Config{APIURL: "https://api.test/v1", ProfileName: "test", AccessToken: "direct-token", Version: "test", HTTPClient: httpClient(handler), LockDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	_, document, err := client.List(context.Background(), "posts", QueryOptions{
		Filters: map[string][]string{"state": {"draft", "published"}}, Fields: map[string][]string{"posts": {"title", "state"}},
		Includes: []string{"author", "categories"}, PageSize: 50,
	})
	if err != nil {
		t.Fatal(err)
	}
	if document.Links.Next == "" {
		t.Fatal("expected next link")
	}
}

func TestConcurrentRefreshIsCoalesced(t *testing.T) {
	var refreshes atomic.Int32
	handler := func(request *http.Request) (*http.Response, error) {
		switch request.URL.Path {
		case "/v1/auth/refresh":
			refreshes.Add(1)
			return response(http.StatusOK, string(tokenJSON("new-access", "new-refresh"))), nil
		case "/v1/posts":
			if request.Header.Get("Authorization") != "Bearer new-access" {
				t.Errorf("resource used %q", request.Header.Get("Authorization"))
			}
			return response(http.StatusOK, `{"data":[]}`), nil
		default:
			return response(http.StatusNotFound, ""), nil
		}
	}
	store := &memoryStore{exists: true, session: secrets.Session{
		AccessToken: "old-access", RefreshToken: "old-refresh", SessionID: "session-id",
		AccessExpiresAt: time.Now().Add(-time.Minute), RefreshExpiresAt: time.Now().Add(time.Hour),
	}}
	lockDir := t.TempDir()
	clients := []*Client{testClientWithLock(t, store, lockDir, handler), testClientWithLock(t, store, lockDir, handler)}
	var wait sync.WaitGroup
	errorsFound := make(chan error, 2)
	for _, client := range clients {
		wait.Add(1)
		go func(client *Client) {
			defer wait.Done()
			_, _, err := client.List(context.Background(), "posts", QueryOptions{})
			errorsFound <- err
		}(client)
	}
	wait.Wait()
	close(errorsFound)
	for err := range errorsFound {
		if err != nil {
			t.Fatal(err)
		}
	}
	if refreshes.Load() != 1 {
		t.Fatalf("refresh requests = %d, want 1", refreshes.Load())
	}
}

func TestExpiredResponseRefreshesAndReplaysOnce(t *testing.T) {
	var calls atomic.Int32
	handler := func(request *http.Request) (*http.Response, error) {
		if request.URL.Path == "/v1/auth/refresh" {
			return response(http.StatusOK, string(tokenJSON("new-access", "new-refresh"))), nil
		}
		if calls.Add(1) == 1 {
			return response(http.StatusUnauthorized, `{"errors":[{"status":401,"code":"access_token_expired","title":"Unauthorized"}]}`), nil
		}
		return response(http.StatusOK, `{"data":[]}`), nil
	}
	store := &memoryStore{exists: true, session: secrets.Session{AccessToken: "old", RefreshToken: "refresh", SessionID: "session-id", AccessExpiresAt: time.Now().Add(time.Hour), RefreshExpiresAt: time.Now().Add(time.Hour)}}
	client := testClient(t, store, handler)
	if _, _, err := client.List(context.Background(), "posts", QueryOptions{}); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("resource calls = %d, want 2", calls.Load())
	}
}

func TestAmbiguousRefreshClearsCredentials(t *testing.T) {
	store := &memoryStore{exists: true, session: secrets.Session{AccessToken: "old", RefreshToken: "refresh", AccessExpiresAt: time.Now().Add(-time.Minute), RefreshExpiresAt: time.Now().Add(time.Hour)}}
	handler := func(*http.Request) (*http.Response, error) { return nil, errors.New("connection lost") }
	client := testClient(t, store, handler)
	_, _, err := client.List(context.Background(), "posts", QueryOptions{})
	if err == nil {
		t.Fatal("expected refresh transport failure")
	}
	if _, err := store.Get("test"); !errors.Is(err, secrets.ErrNotFound) {
		t.Fatalf("credentials were not cleared: %v", err)
	}
}

func TestPageURLMustStayOnConfiguredAPI(t *testing.T) {
	client, err := New(Config{APIURL: "https://api.jdeen.com/v1", ProfileName: "test", AccessToken: "token", HTTPClient: httpClient(func(*http.Request) (*http.Response, error) { return response(200, "{}"), nil }), LockDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.resourceURL("posts", QueryOptions{PageURL: "https://evil.example/v1/posts"}); err == nil {
		t.Fatal("expected cross-origin page URL to fail")
	}
	if _, err := client.resourceURL("posts", QueryOptions{PageURL: "https://api.jdeen.com/v1/posts?page%5Bafter%5D=opaque"}); err != nil {
		t.Fatal(err)
	}
}

func TestInvalidEnvironmentTokenDoesNotDeleteStoredSession(t *testing.T) {
	store := &memoryStore{exists: true, session: secrets.Session{AccessToken: "stored", RefreshToken: "stored-refresh"}}
	handler := func(*http.Request) (*http.Response, error) {
		return response(http.StatusUnauthorized, `{"errors":[{"status":401,"code":"invalid_token","title":"Unauthorized"}]}`), nil
	}
	client, err := New(Config{APIURL: "https://api.test/v1", ProfileName: "test", AccessToken: "environment", Store: store, HTTPClient: httpClient(handler), LockDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := client.List(context.Background(), "posts", QueryOptions{}); err == nil {
		t.Fatal("expected invalid token error")
	}
	stored, err := store.Get("test")
	if err != nil || stored.AccessToken != "stored" {
		t.Fatalf("stored session was changed: %#v, %v", stored, err)
	}
}

func TestCreateDoesNotRetryAmbiguousFailure(t *testing.T) {
	var calls atomic.Int32
	handler := func(request *http.Request) (*http.Response, error) {
		calls.Add(1)
		if request.Method != http.MethodPost || request.URL.Path != "/v1/categories" {
			t.Errorf("unexpected request %s %s", request.Method, request.URL.Path)
		}
		body, _ := io.ReadAll(request.Body)
		if !strings.Contains(string(body), `"type":"categories"`) || !strings.Contains(string(body), `"name":"Engineering"`) {
			t.Errorf("unexpected body: %s", body)
		}
		return nil, errors.New("connection reset after write")
	}
	client, err := New(Config{APIURL: "https://api.test/v1", ProfileName: "test", AccessToken: "direct", Version: "test", HTTPClient: httpClient(handler), LockDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	_, _, err = client.Create(context.Background(), "categories", jsonapi.ResourceObject{Type: "categories", Attributes: map[string]any{"name": "Engineering"}}, QueryOptions{})
	if err == nil {
		t.Fatal("expected ambiguous write failure")
	}
	if calls.Load() != 1 {
		t.Fatalf("write attempts = %d, want 1", calls.Load())
	}
}

func TestUploadUsesMultipartAndReturnsChecksum(t *testing.T) {
	uploadPath := filepath.Join(t.TempDir(), "banner.png")
	contents := []byte("\x89PNG\r\n\x1a\nimage")
	if err := os.WriteFile(uploadPath, contents, 0o600); err != nil {
		t.Fatal(err)
	}
	handler := func(request *http.Request) (*http.Response, error) {
		if request.Method != http.MethodPost || request.URL.Path != "/v1/uploads" {
			t.Errorf("unexpected request %s %s", request.Method, request.URL.Path)
		}
		reader, err := request.MultipartReader()
		if err != nil {
			t.Fatal(err)
		}
		fields := map[string]string{}
		for {
			part, partErr := reader.NextPart()
			if errors.Is(partErr, io.EOF) {
				break
			}
			if partErr != nil {
				t.Fatal(partErr)
			}
			value, _ := io.ReadAll(part)
			if part.FormName() == "file" && part.Header.Get("Content-Type") != "image/png" {
				t.Errorf("upload content type = %q", part.Header.Get("Content-Type"))
			}
			fields[part.FormName()] = string(value)
		}
		if fields["description"] != "Architecture diagram" || fields["alt_text"] != "Connected services" || fields["file"] != string(contents) {
			t.Errorf("unexpected multipart fields: %#v", fields)
		}
		return response(http.StatusCreated, `{"data":{"type":"uploads","id":"0ee67de0-b9e9-43dd-9c50-3804533ddf80","attributes":{"description":"Architecture diagram"}}}`), nil
	}
	client, err := New(Config{APIURL: "https://api.test/v1", ProfileName: "test", AccessToken: "direct", Version: "test", HTTPClient: httpClient(handler), LockDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	resource, _, checksum, err := client.Upload(context.Background(), uploadPath, "Architecture diagram", stringPointerForTest("Connected services"))
	if err != nil {
		t.Fatal(err)
	}
	if resource.Type != "uploads" || len(checksum) != 64 {
		t.Fatalf("resource=%#v checksum=%q", resource, checksum)
	}
}

func testClient(t *testing.T, store secrets.Store, handler roundTripFunc) *Client {
	return testClientWithLock(t, store, t.TempDir(), handler)
}

func testClientWithLock(t *testing.T, store secrets.Store, lockDir string, handler roundTripFunc) *Client {
	t.Helper()
	client, err := New(Config{APIURL: "https://api.test/v1", ProfileName: "test", Store: store, Version: "test", HTTPClient: httpClient(handler), LockDir: lockDir})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func httpClient(handler roundTripFunc) *http.Client { return &http.Client{Transport: handler} }

func tokenJSON(access, refresh string) []byte {
	value, _ := json.Marshal(TokenResponse{AccessToken: access, RefreshToken: refresh, TokenType: "Bearer", ExpiresIn: 900, RefreshTokenExpiresIn: 3600, SessionID: "session-id"})
	return value
}

func stringPointerForTest(value string) *string { return &value }
